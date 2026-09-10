package routespec

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/julienschmidt/httprouter"
)

// HTTPRouter registers documented operations on an httprouter.Router.
type HTTPRouter struct {
	router *httprouter.Router
	api    *API
}

// NewHTTPRouter creates an httprouter adapter and its OpenAPI registry.
func NewHTTPRouter(router *httprouter.Router, info Info, options ...Option) *HTTPRouter {
	if router == nil {
		panic("routespec: nil httprouter.Router")
	}
	return &HTTPRouter{
		router: router,
		api:    newAPI(info, options...),
	}
}

// Register registers handler on httprouter and records operation. It panics for
// invalid routes or metadata, like httprouter.Router.Handler.
func (api *HTTPRouter) Register(method, path string, handler http.Handler, operation Operation) {
	if handler == nil {
		panicRoute(method, path, "nil handler")
	}
	documentPath := normalizeHTTPRouterPath(method, path)
	api.api.register(method, documentPath, operation, func(string) {
		api.router.Handler(method, path, handler)
	})
}

// RegisterFunc is Register for an httprouter.Handle.
func (api *HTTPRouter) RegisterFunc(method, path string, handler httprouter.Handle, operation Operation) {
	if handler == nil {
		panicRoute(method, path, "nil handler")
	}
	documentPath := normalizeHTTPRouterPath(method, path)
	api.api.register(method, documentPath, operation, func(string) {
		api.router.Handle(method, path, handler)
	})
}

// Document returns the current OpenAPI document.
func (api *HTTPRouter) Document() Document {
	return api.api.Document()
}

// JSON returns a pretty-printed OpenAPI document followed by a newline.
func (api *HTTPRouter) JSON() ([]byte, error) {
	return api.api.JSON()
}

// Handler serves the current OpenAPI document as application/json.
func (api *HTTPRouter) Handler() http.Handler {
	return api.api.Handler()
}

func normalizeHTTPRouterPath(method, path string) string {
	if path == "" || !strings.HasPrefix(path, "/") {
		panicRoute(method, path, "path must begin with /")
	}

	parts := strings.Split(path, "/")
	for index, part := range parts {
		switch {
		case strings.HasPrefix(part, ":"):
			name := strings.TrimPrefix(part, ":")
			if name == "" {
				panicRoute(method, path, "empty httprouter parameter")
			}
			parts[index] = "{" + name + "}"
		case strings.HasPrefix(part, "*"):
			panicRoute(method, path, fmt.Sprintf("httprouter catch-all parameter %q cannot be represented by an OpenAPI path", part))
		}
	}
	return strings.Join(parts, "/")
}
