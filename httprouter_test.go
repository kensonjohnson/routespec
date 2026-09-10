package routespec_test

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/julienschmidt/httprouter"
	"github.com/kensonjohnson/routespec"
)

type httpRouterUserPath struct {
	UserID string `json:"userID" openapi:"description=User identifier"`
}

type httpRouterUserResponse struct {
	Name string `json:"name" openapi:"required,description=User display name"`
}

type httpRouterFilePath struct {
	Path string `json:"path"`
}

type httpRouterMismatchedPath struct {
	AccountID string `json:"accountID"`
}

func TestHTTPRouterRegisterAndDocument(t *testing.T) {
	router := httprouter.New()
	api := routespec.NewHTTPRouter(router, routespec.Info{Title: "Example", Version: "1.0.0"})

	called := false
	api.Register(http.MethodGet, "/health", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	}), routespec.Operation{
		ID:        "health",
		Responses: routespec.Responses{http.StatusNoContent: {}},
	})

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/health", nil))
	if !called || response.Code != http.StatusNoContent {
		t.Fatalf("registered http.Handler: called=%t status=%d", called, response.Code)
	}

	var received httprouter.Params
	api.RegisterFunc(http.MethodGet, "/users/:userID", func(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
		received = params
		w.WriteHeader(http.StatusOK)
	}, routespec.Operation{
		ID:   "getUser",
		Tags: []string{"users"},
		Path: routespec.Path[httpRouterUserPath](),
		Responses: routespec.Responses{
			http.StatusOK: routespec.JSON[httpRouterUserResponse](),
		},
	})

	response = httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/users/42", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("registered httprouter.Handle status = %d, want %d", response.Code, http.StatusOK)
	}
	if got, want := received.ByName("userID"), "42"; got != want {
		t.Fatalf("httprouter parameter = %q, want %q", got, want)
	}

	document := api.Document()
	operation := document.Paths["/users/{userID}"].Get
	if operation == nil {
		t.Fatal("document is missing GET /users/{userID}")
	}
	if got, want := operation.Tags, []string{"users"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("tags = %#v, want %#v", got, want)
	}
	if got, want := operation.Parameters, []routespec.Parameter{{
		Name:        "userID",
		In:          "path",
		Description: "User identifier",
		Required:    true,
		Schema:      routespec.Schema{Type: "string", Description: "User identifier"},
	}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("parameters = %#v, want %#v", got, want)
	}

	userSchema, ok := document.Components.Schemas["httpRouterUserResponse"]
	if !ok {
		t.Fatal("document is missing the response model schema")
	}
	if got, want := userSchema.Properties["name"].Description, "User display name"; got != want {
		t.Fatalf("response property description = %q, want %q", got, want)
	}
	if got, want := userSchema.Required, []string{"name"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("response required fields = %#v, want %#v", got, want)
	}

	if _, err := api.JSON(); err != nil {
		t.Fatalf("JSON: %v", err)
	}
	response = httptest.NewRecorder()
	api.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/openapi.json", nil))
	if got, want := response.Header().Get("Content-Type"), "application/json"; got != want {
		t.Fatalf("document Content-Type = %q, want %q", got, want)
	}
	if response.Code != http.StatusOK {
		t.Fatalf("document handler status = %d, want %d", response.Code, http.StatusOK)
	}
}

func TestHTTPRouterRegisterRejectsCatchAllWithoutRegisteringRoute(t *testing.T) {
	router := httprouter.New()
	api := routespec.NewHTTPRouter(router, routespec.Info{Title: "Example", Version: "1.0.0"})

	called := false
	mustHTTPRouterPanic(t, func() {
		api.RegisterFunc(http.MethodGet, "/files/*path", func(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
			called = true
		}, routespec.Operation{
			ID:        "getFile",
			Path:      routespec.Path[httpRouterFilePath](),
			Responses: routespec.Responses{http.StatusOK: {}},
		})
	})

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/files/readme.txt", nil))
	if called || response.Code != http.StatusNotFound {
		t.Fatalf("catch-all route was registered: called=%t status=%d", called, response.Code)
	}
}

func TestHTTPRouterRegisterRejectsMismatchedPathModel(t *testing.T) {
	api := routespec.NewHTTPRouter(httprouter.New(), routespec.Info{Title: "Example", Version: "1.0.0"})

	mustHTTPRouterPanic(t, func() {
		api.RegisterFunc(http.MethodGet, "/users/:userID", func(http.ResponseWriter, *http.Request, httprouter.Params) {}, routespec.Operation{
			ID:        "getUser",
			Path:      routespec.Path[httpRouterMismatchedPath](),
			Responses: routespec.Responses{http.StatusOK: {}},
		})
	})
}

func mustHTTPRouterPanic(t *testing.T, fn func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatal("function did not panic")
		}
	}()
	fn()
}
