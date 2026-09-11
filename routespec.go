package routespec

import (
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"sync"
)

// API records operations registered on a Go 1.27+ http.ServeMux.
type API struct {
	mux  *http.ServeMux
	info Info

	mu         sync.RWMutex
	operations map[routeKey]Operation
	overrides  map[reflect.Type]Schema
}

type routeKey struct {
	method string
	path   string
}

// Option configures an API registry.
type Option func(*API)

// WithSchemaOverride uses schema whenever T appears in a generated operation.
// It provides an escape hatch for schemas that field tags cannot express.
func WithSchemaOverride[T any](schema Schema) Option {
	return func(api *API) {
		api.overrides[deref(typeOf[T]())] = cloneSchema(schema)
	}
}

// New creates an API registry that registers routes on mux.
func New(mux *http.ServeMux, info Info, options ...Option) *API {
	if mux == nil {
		panic("routespec: nil http.ServeMux")
	}
	api := newAPI(info, options...)
	api.mux = mux
	return api
}

func cloneInfo(info Info) Info {
	clone := info
	if len(info.SecuritySchemes) > 0 {
		clone.SecuritySchemes = make(map[string]SecurityScheme, len(info.SecuritySchemes))
		for name, scheme := range info.SecuritySchemes {
			clone.SecuritySchemes[name] = scheme
		}
	}
	return clone
}

func newAPI(info Info, options ...Option) *API {
	if info.Title == "" {
		panic("routespec: document title is required")
	}
	if info.Version == "" {
		panic("routespec: document version is required")
	}
	for name, scheme := range info.SecuritySchemes {
		if name == "" {
			panic("routespec: security scheme name is required")
		}
		if scheme.Type == "" {
			panic(fmt.Sprintf("routespec: security scheme %q type is required", name))
		}
	}

	api := &API{
		info:       cloneInfo(info),
		operations: make(map[routeKey]Operation),
		overrides:  make(map[reflect.Type]Schema),
	}
	for _, option := range options {
		if option == nil {
			panic("routespec: nil option")
		}
		option(api)
	}
	return api
}

// Register registers handler and records operation. It panics when route or
// operation metadata is invalid because registration happens at application
// startup, like http.ServeMux.Handle.
func (api *API) Register(method, path string, handler http.Handler, operation Operation) {
	if api.mux == nil {
		panic("routespec: API has no http.ServeMux")
	}
	if handler == nil {
		panicRoute(method, path, "nil handler")
	}

	api.register(method, path, operation, func(pattern string) {
		api.mux.Handle(pattern, handler)
	})
}

// RegisterFunc is Register for an http.HandlerFunc.
func (api *API) RegisterFunc(method, path string, handler http.HandlerFunc, operation Operation) {
	if handler == nil {
		panicRoute(method, path, "nil handler")
	}
	api.Register(method, path, handler, operation)
}

func (api *API) register(method, path string, operation Operation, register func(pattern string)) {
	operation = cloneOperation(operation)
	validateOperation(method, path, operation)

	key := routeKey{method: method, path: path}
	api.mu.Lock()
	defer api.mu.Unlock()

	if _, exists := api.operations[key]; exists {
		panicRoute(method, path, "operation is already registered")
	}

	candidate := make(map[routeKey]Operation, len(api.operations)+1)
	for registeredKey, registeredOperation := range api.operations {
		candidate[registeredKey] = registeredOperation
	}
	candidate[key] = operation
	validateDocument(method, path, api.info, candidate, api.overrides)

	pattern := method + " " + path
	registerWithContext(method, path, func() {
		register(pattern)
	})
	api.operations[key] = operation
}

func cloneOperation(operation Operation) Operation {
	clone := operation
	clone.Tags = append([]string(nil), operation.Tags...)
	clone.Security = cloneSecurity(operation.Security)
	clone.Responses = make(Responses, len(operation.Responses))
	for status, content := range operation.Responses {
		clone.Responses[status] = content
	}
	return clone
}

func validateDocument(method, path string, info Info, operations map[routeKey]Operation, overrides map[reflect.Type]Schema) {
	defer func() {
		if recovered := recover(); recovered != nil {
			panicRoute(method, path, fmt.Sprint(recovered))
		}
	}()
	validateOperationIDs(operations)
	document := buildDocument(info, operations, overrides)
	validateSchemaCompositions(document)
}

func validateOperationIDs(operations map[routeKey]Operation) {
	seen := make(map[string]routeKey, len(operations))
	for key, operation := range operations {
		if previous, exists := seen[operation.ID]; exists {
			panic(fmt.Sprintf("operation ID %q is used by both %s %s and %s %s", operation.ID, previous.method, previous.path, key.method, key.path))
		}
		seen[operation.ID] = key
	}
}

func registerWithContext(method, path string, register func()) {
	defer func() {
		if recovered := recover(); recovered != nil {
			panicRoute(method, path, fmt.Sprint(recovered))
		}
	}()
	register()
}

func validateOperation(method, path string, operation Operation) {
	if method == "" {
		panicRoute(method, path, "HTTP method is required")
	}
	if method != strings.ToUpper(method) {
		panicRoute(method, path, "HTTP method must be uppercase")
	}
	if path == "" || !strings.HasPrefix(path, "/") {
		panicRoute(method, path, "path must begin with /")
	}
	if !supportedMethod(method) {
		panicRoute(method, path, "unsupported OpenAPI method")
	}
	if operation.ID == "" {
		panicRoute(method, path, "operation ID is required")
	}
	if len(operation.Responses) == 0 {
		panicRoute(method, path, "at least one response is required")
	}
	for status, content := range operation.Responses {
		if status < http.StatusContinue || status > 599 {
			panicRoute(method, path, fmt.Sprintf("invalid response status %d", status))
		}
		validateContent(method, path, content)
	}
	validateContent(method, path, operation.RequestBody)
	validateParameterModel(method, path, operation.Path, pathParameter)
	validateParameterModel(method, path, operation.Query, queryParameter)

	pathParameters := serveMuxPathParameters(path)
	if len(pathParameters) > 0 && operation.Path.typeOf == nil {
		panicRoute(method, path, "path parameters require a Path model")
	}
	if operation.Path.typeOf != nil {
		fields := parameterFields(deref(operation.Path.typeOf))
		if len(fields) != len(pathParameters) {
			panicRoute(method, path, "Path model fields must match path parameters")
		}
		for _, name := range pathParameters {
			if _, exists := fields[name]; !exists {
				panicRoute(method, path, fmt.Sprintf("Path model does not contain parameter %q", name))
			}
		}
	}
}

func supportedMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodPut, http.MethodPost, http.MethodDelete,
		http.MethodPatch, http.MethodHead, http.MethodOptions, http.MethodTrace:
		return true
	default:
		return false
	}
}

func validateContent(method, path string, content Content) {
	if content.typeOf == nil {
		return
	}
	if content.mediaType == "" {
		panicRoute(method, path, "content type is required")
	}
}

func validateParameterModel(method, path string, model ParameterModel, expected parameterLocation) {
	if model.typeOf == nil {
		return
	}
	if model.location != expected {
		panicRoute(method, path, "invalid parameter model")
	}
	if deref(model.typeOf).Kind() != reflect.Struct {
		panicRoute(method, path, fmt.Sprintf("%s model must be a struct", expected))
	}
}

func panicRoute(method, path, message string) {
	panic(fmt.Sprintf("routespec: register %s %s: %s", method, path, message))
}

// Document returns the current OpenAPI document.
func (api *API) Document() Document {
	api.mu.RLock()
	operations := make(map[routeKey]Operation, len(api.operations))
	for key, operation := range api.operations {
		operations[key] = operation
	}
	overrides := make(map[reflect.Type]Schema, len(api.overrides))
	for t, schema := range api.overrides {
		overrides[t] = schema
	}
	api.mu.RUnlock()

	return buildDocument(api.info, operations, overrides)
}

// JSON returns a pretty-printed OpenAPI document followed by a newline.
func (api *API) JSON() ([]byte, error) {
	jsonDocument, err := json.Marshal(api.Document(), json.Deterministic(true), jsontext.WithIndent("\t"))
	if err != nil {
		return nil, fmt.Errorf("marshal OpenAPI document: %w", err)
	}
	return append(jsonDocument, '\n'), nil
}

// Handler serves the current OpenAPI document as application/json.
func (api *API) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		document, err := api.JSON()
		if err != nil {
			http.Error(w, "could not generate OpenAPI document", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(document)
	})
}
