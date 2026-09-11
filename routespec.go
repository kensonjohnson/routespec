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
	clone.Contact = cloneContact(info.Contact)
	clone.License = cloneLicense(info.License)
	clone.Servers = cloneServers(info.Servers)
	clone.Tags = cloneTags(info.Tags)
	clone.ExternalDocs = cloneExternalDocs(info.ExternalDocs)
	clone.SecuritySchemes = cloneSecuritySchemes(info.SecuritySchemes)
	return clone
}

func newAPI(info Info, options ...Option) *API {
	validateInfo(info)

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
	clone.RequestBody.Content = cloneContents(operation.RequestBody.Content)
	clone.Responses = make(Responses, len(operation.Responses))
	for status, response := range operation.Responses {
		clone.Responses[status] = cloneResponseSpec(response)
	}
	return clone
}

func cloneResponseSpec(response ResponseSpec) ResponseSpec {
	clone := response
	clone.Content = cloneContents(response.Content)
	if response.Headers != nil {
		clone.Headers = make(map[string]HeaderSpec, len(response.Headers))
		for name, header := range response.Headers {
			header.Schema = cloneSchema(header.Schema)
			clone.Headers[name] = header
		}
	}
	if response.Links != nil {
		clone.Links = make(map[string]LinkSpec, len(response.Links))
		for name, link := range response.Links {
			clone.Links[name] = cloneLinkSpec(link)
		}
	}
	return clone
}

func cloneContents(contents []Content) []Content {
	if contents == nil {
		return nil
	}
	clone := make([]Content, len(contents))
	copy(clone, contents)
	return clone
}

func cloneLinkSpec(link LinkSpec) LinkSpec {
	clone := link
	if link.Parameters != nil {
		clone.Parameters = make(map[string]any, len(link.Parameters))
		for name, value := range link.Parameters {
			clone.Parameters[name] = cloneSchemaValue(value)
		}
	}
	clone.RequestBody = cloneSchemaValue(link.RequestBody)
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
	validateDocumentReferences(document)
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
	for status, response := range operation.Responses {
		if status < http.StatusContinue || status > 599 {
			panicRoute(method, path, fmt.Sprintf("invalid response status %d", status))
		}
		validateContents(method, path, response.Content, "response content")
		for name := range response.Headers {
			if name == "" {
				panicRoute(method, path, "response header name is required")
			}
		}
		for name, link := range response.Links {
			if name == "" || (link.OperationID == "" && link.OperationRef == "") || (link.OperationID != "" && link.OperationRef != "") {
				panicRoute(method, path, "response links require one operation ID or operation reference")
			}
		}
	}
	validateContents(method, path, operation.RequestBody.Content, "request content")
	validateParameterModel(method, path, operation.Path, pathParameter)
	validateParameterModel(method, path, operation.Query, queryParameter)
	validateParameterModel(method, path, operation.Header, headerParameter)
	validateParameterModel(method, path, operation.Cookie, cookieParameter)

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

func validateContents(method, path string, contents []Content, label string) {
	if contents == nil {
		return
	}
	if len(contents) == 0 {
		panicRoute(method, path, label+" requires at least one representation")
	}
	mediaTypes := make(map[string]struct{}, len(contents))
	for _, content := range contents {
		if content.typeOf == nil || content.mediaType == "" {
			panicRoute(method, path, "content type is required")
		}
		if _, exists := mediaTypes[content.mediaType]; exists {
			panicRoute(method, path, fmt.Sprintf("duplicate content type %q", content.mediaType))
		}
		mediaTypes[content.mediaType] = struct{}{}
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
	if !validParameterStyle(expected, model.style) {
		panicRoute(method, path, fmt.Sprintf("unsupported %s parameter style %q", expected, model.style))
	}
	if model.allowReserved && expected != queryParameter {
		panicRoute(method, path, "allowReserved is only valid for query parameters")
	}
}

func validParameterStyle(location parameterLocation, style string) bool {
	if style == "" {
		return true
	}
	switch location {
	case pathParameter:
		return style == "matrix" || style == "label" || style == "simple"
	case queryParameter:
		return style == "form" || style == "spaceDelimited" || style == "pipeDelimited" || style == "deepObject"
	case headerParameter:
		return style == "simple"
	case cookieParameter:
		return style == "form"
	default:
		return false
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
