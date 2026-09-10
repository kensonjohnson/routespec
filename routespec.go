package routespec

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"sync"
)

// API records operations registered on a Go 1.22+ http.ServeMux.
type API struct {
	mux  *http.ServeMux
	info Info

	mu         sync.RWMutex
	operations map[routeKey]Operation
}

type routeKey struct {
	method string
	path   string
}

// New creates an API registry that registers routes on mux.
func New(mux *http.ServeMux, info Info) *API {
	if mux == nil {
		panic("routespec: nil http.ServeMux")
	}
	if info.Title == "" {
		panic("routespec: document title is required")
	}
	if info.Version == "" {
		panic("routespec: document version is required")
	}

	return &API{
		mux:        mux,
		info:       info,
		operations: make(map[routeKey]Operation),
	}
}

// Register registers handler and records operation. It panics when route or
// operation metadata is invalid because registration happens at application
// startup, like http.ServeMux.Handle.
func (api *API) Register(method, path string, handler http.Handler, operation Operation) {
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
	validateDocument(method, path, api.info, candidate)

	pattern := method + " " + path
	registerWithContext(method, path, func() {
		register(pattern)
	})
	api.operations[key] = operation
}

func validateDocument(method, path string, info Info, operations map[routeKey]Operation) {
	defer func() {
		if recovered := recover(); recovered != nil {
			panicRoute(method, path, fmt.Sprint(recovered))
		}
	}()
	_ = buildDocument(info, operations)
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
		fields := jsonFields(deref(operation.Path.typeOf))
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
	api.mu.RUnlock()

	return buildDocument(api.info, operations)
}

// JSON returns a pretty-printed OpenAPI document followed by a newline.
func (api *API) JSON() ([]byte, error) {
	jsonDocument, err := json.MarshalIndent(api.Document(), "", "\t")
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
