package routespec_test

import (
	json "encoding/json/v2"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/kensonjohnson/routespec"
)

type createUserRequest struct {
	Email string `json:"email"`
	Name  string `json:"name,omitempty"`
}

type getUserPath struct {
	ID int64 `json:"id"`
}

type listUsersQuery struct {
	Limit int `json:"limit"`
}

type userProfile struct {
	DisplayName string `json:"displayName"`
}

type userResponse struct {
	CreatedAt  time.Time       `json:"createdAt"`
	Tags       []string        `json:"tags"`
	Attributes map[string]bool `json:"attributes"`
	Profile    userProfile     `json:"profile"`
	Next       *userResponse   `json:"next,omitempty"`
	Ignored    string          `json:"-"`
}

type nullableUserResponse struct {
	Next *nullableUserResponse `json:"next"`
}

type statefulHandler struct {
	called bool
}

func (handler *statefulHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	handler.called = true
	w.WriteHeader(http.StatusOK)
}

func TestRegisterAndDocument(t *testing.T) {
	mux := http.NewServeMux()
	api := routespec.New(mux, routespec.Info{
		Title:       "Example API",
		Version:     "1.0.0",
		Description: "An API used in tests.",
	})

	getHandler := &statefulHandler{}
	api.Register(http.MethodGet, "/users/{id}", getHandler, routespec.Operation{
		ID:        "getUser",
		Summary:   "Get a user",
		Path:      routespec.Path[getUserPath](),
		Query:     routespec.Query[listUsersQuery](),
		Responses: routespec.Responses{http.StatusOK: routespec.JSON[userResponse]()},
	})
	api.RegisterFunc(http.MethodPost, "/users", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}, routespec.Operation{
		ID:          "createUser",
		RequestBody: routespec.JSON[createUserRequest](),
		Responses:   routespec.Responses{http.StatusCreated: routespec.JSON[userResponse]()},
	})

	response := httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/users/42?limit=10", nil))
	if response.Code != http.StatusOK || !getHandler.called {
		t.Fatalf("registered http.Handler was not served: status=%d called=%t", response.Code, getHandler.called)
	}

	response = httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/users", nil))
	if response.Code != http.StatusCreated {
		t.Fatalf("registered http.HandlerFunc status = %d, want %d", response.Code, http.StatusCreated)
	}

	document := api.Document()
	if document.OpenAPI != "3.1.0" {
		t.Fatalf("OpenAPI version = %q, want 3.1.0", document.OpenAPI)
	}
	getOperation := document.Paths["/users/{id}"].Get
	if getOperation == nil || getOperation.OperationID != "getUser" {
		t.Fatalf("GET operation = %#v", getOperation)
	}
	if got, want := getOperation.Parameters, []routespec.Parameter{
		{Name: "id", In: "path", Required: true, Schema: routespec.Schema{Type: "integer", Format: "int64"}},
		{Name: "limit", In: "query", Schema: routespec.Schema{Type: "integer", Format: "int64"}},
	}; !reflect.DeepEqual(got, want) {
		t.Fatalf("parameters = %#v, want %#v", got, want)
	}

	postOperation := document.Paths["/users"].Post
	if postOperation == nil || postOperation.RequestBody == nil {
		t.Fatal("POST request body is missing")
	}
	if got := postOperation.RequestBody.Content["application/json"].Schema.Ref; got != "#/components/schemas/createUserRequest" {
		t.Fatalf("request schema ref = %q", got)
	}
	if got := postOperation.Responses["201"].Content["application/json"].Schema.Ref; got != "#/components/schemas/userResponse" {
		t.Fatalf("response schema ref = %q", got)
	}

	userSchema := document.Components.Schemas["userResponse"]
	if got, want := userSchema.Properties["createdAt"], (routespec.Schema{Type: "string", Format: "date-time"}); !reflect.DeepEqual(got, want) {
		t.Fatalf("createdAt schema = %#v", got)
	}
	if got := userSchema.Properties["tags"].Items.Type; got != "string" {
		t.Fatalf("slice item schema = %q, want string", got)
	}
	if got := userSchema.Properties["attributes"].AdditionalProperties.Type; got != "boolean" {
		t.Fatalf("map value schema = %q, want boolean", got)
	}
	if got := userSchema.Properties["profile"].Ref; got != "#/components/schemas/userProfile" {
		t.Fatalf("nested schema ref = %q", got)
	}
	if next := userSchema.Properties["next"]; next.Ref != "#/components/schemas/userResponse" || next.Nullable {
		t.Fatalf("omitempty recursive schema = %#v", next)
	}
	if _, exists := userSchema.Properties["Ignored"]; exists {
		t.Fatal("json:- field appears in schema")
	}
}

func TestDocumentJSONRoundTripPreservesSchemaExtensions(t *testing.T) {
	api := routespec.New(http.NewServeMux(), routespec.Info{Title: "Example", Version: "1.0.0"})
	api.RegisterFunc(http.MethodGet, "/users", func(http.ResponseWriter, *http.Request) {}, routespec.Operation{
		ID:        "listUsers",
		Responses: routespec.Responses{http.StatusOK: routespec.JSON[nullableUserResponse]()},
	})

	body, err := api.JSON()
	if err != nil {
		t.Fatalf("JSON: %v", err)
	}
	var document routespec.Document
	if err := json.Unmarshal(body, &document); err != nil {
		t.Fatalf("unmarshal document: %v", err)
	}
	if next := document.Components.Schemas["nullableUserResponse"].Properties["next"]; next.Ref != "#/components/schemas/nullableUserResponse" || !next.Nullable {
		t.Fatalf("round-trip nullable schema = %#v", next)
	}
}

func TestHandlerServesDeterministicJSON(t *testing.T) {
	api := routespec.New(http.NewServeMux(), routespec.Info{Title: "Example", Version: "1.0.0"})
	api.RegisterFunc(http.MethodGet, "/health", func(http.ResponseWriter, *http.Request) {}, routespec.Operation{
		ID:        "health",
		Responses: routespec.Responses{http.StatusNoContent: {}},
	})

	first, err := api.JSON()
	if err != nil {
		t.Fatal(err)
	}
	second, err := api.JSON()
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatal("generated JSON is not deterministic")
	}

	response := httptest.NewRecorder()
	api.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/openapi.json", nil))
	if got := response.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q", got)
	}
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}

	var document routespec.Document
	if err := json.Unmarshal(response.Body.Bytes(), &document); err != nil {
		t.Fatalf("unmarshal served document: %v", err)
	}
	if document.Paths["/health"].Get == nil {
		t.Fatal("served document is missing GET /health")
	}
}

func TestRegisterPanicsForInvalidMetadata(t *testing.T) {
	api := routespec.New(http.NewServeMux(), routespec.Info{Title: "Example", Version: "1.0.0"})

	mustPanic(t, func() {
		api.RegisterFunc(http.MethodGet, "/users/{id}", func(http.ResponseWriter, *http.Request) {}, routespec.Operation{
			ID:        "missingPathModel",
			Responses: routespec.Responses{http.StatusOK: {}},
		})
	})

	mustPanic(t, func() {
		api.RegisterFunc(http.MethodGet, "/invalid", func(http.ResponseWriter, *http.Request) {}, routespec.Operation{
			ID:        "invalidSchema",
			Responses: routespec.Responses{http.StatusOK: routespec.JSON[map[int]string]()},
		})
	})

	api.RegisterFunc(http.MethodGet, "/users", func(http.ResponseWriter, *http.Request) {}, routespec.Operation{
		ID:        "listUsers",
		Responses: routespec.Responses{http.StatusOK: {}},
	})
	mustPanic(t, func() {
		api.RegisterFunc(http.MethodGet, "/users", func(http.ResponseWriter, *http.Request) {}, routespec.Operation{
			ID:        "listUsersAgain",
			Responses: routespec.Responses{http.StatusOK: {}},
		})
	})
}

func mustPanic(t *testing.T, fn func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatal("function did not panic")
		}
	}()
	fn()
}
