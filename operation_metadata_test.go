package routespec_test

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/kensonjohnson/routespec"
)

type operationMetadataPath struct {
	UploadID string `json:"uploadID"`
}

type operationMetadataQuery struct {
	Download bool `json:"download"`
}

type operationMetadataHeader struct {
	RequestID string `json:"X-Request-ID"`
}

type operationMetadataCookie struct {
	Session string `json:"session"`
}

type operationMetadataRequest struct {
	Filename string `json:"filename"`
}

type operationMetadataResponse struct {
	Location string `json:"location"`
}

func TestOperationMetadataDocument(t *testing.T) {
	api := routespec.New(http.NewServeMux(), routespec.Info{Title: "Example", Version: "1.0.0"})
	api.RegisterFunc(http.MethodPost, "/uploads/{uploadID}", func(http.ResponseWriter, *http.Request) {}, routespec.Operation{
		ID: "createUpload",
		RequestBody: routespec.RequestSpec{Content: []routespec.Content{
			routespec.Media[operationMetadataRequest]("application/json"),
			routespec.Media[string]("text/plain"),
		}},
		Path:   routespec.Path[operationMetadataPath](),
		Query:  routespec.Query[operationMetadataQuery](),
		Header: routespec.Header[operationMetadataHeader](),
		Cookie: routespec.Cookie[operationMetadataCookie](),
		Responses: map[int]routespec.ResponseSpec{
			http.StatusCreated: {
				Description: "Upload created.",
				Content: []routespec.Content{
					routespec.Media[operationMetadataResponse]("application/json"),
					routespec.Media[[]byte]("application/octet-stream"),
				},
				Headers: map[string]routespec.HeaderSpec{
					"X-Upload-Version": routespec.ResponseHeader[int](),
				},
				Links: map[string]routespec.LinkSpec{
					"upload": {OperationID: "createUpload"},
				},
			},
		},
	})

	operation := api.Document().Paths["/uploads/{uploadID}"].Post
	if operation == nil {
		t.Fatal("POST operation is missing")
	}

	if operation.RequestBody == nil {
		t.Fatal("request body is missing")
	}
	if got := operation.RequestBody.Content["application/json"].Schema.Ref; got != "#/components/schemas/operationMetadataRequest" {
		t.Fatalf("JSON request schema ref = %q", got)
	}
	if got := operation.RequestBody.Content["text/plain"].Schema.Type; got != "string" {
		t.Fatalf("text request schema type = %q, want string", got)
	}

	parameters := make(map[string]routespec.Parameter, len(operation.Parameters))
	for _, parameter := range operation.Parameters {
		parameters[parameter.In+":"+parameter.Name] = parameter
	}
	for _, want := range []string{"path:uploadID", "query:download", "header:X-Request-ID", "cookie:session"} {
		if _, exists := parameters[want]; !exists {
			t.Fatalf("parameters = %#v, missing %q", operation.Parameters, want)
		}
	}
	if !parameters["path:uploadID"].Required {
		t.Fatal("path parameter is not required")
	}

	response := operation.Responses["201"]
	if got, want := response.Description, "Upload created."; got != want {
		t.Fatalf("response description = %q, want %q", got, want)
	}
	if got := response.Content["application/json"].Schema.Ref; got != "#/components/schemas/operationMetadataResponse" {
		t.Fatalf("JSON response schema ref = %q", got)
	}
	if got, want := response.Content["application/octet-stream"].Schema, (routespec.Schema{Type: "string", Format: "byte"}); !reflect.DeepEqual(got, want) {
		t.Fatalf("binary response schema = %#v, want %#v", got, want)
	}
	if got, want := response.Headers["X-Upload-Version"].Schema, (routespec.Schema{Type: "integer", Format: "int64"}); !reflect.DeepEqual(got, want) {
		t.Fatalf("response header schema = %#v, want %#v", got, want)
	}
	if got, want := response.Links["upload"].OperationID, "createUpload"; got != want {
		t.Fatalf("response link operation ID = %q, want %q", got, want)
	}
}

func TestRegisterRejectsInvalidOperationMetadataBeforeRegistration(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		operation routespec.Operation
	}{
		{
			name: "empty request content",
			path: "/empty-request-content",
			operation: routespec.Operation{
				ID:          "emptyRequestContent",
				RequestBody: routespec.RequestSpec{Content: []routespec.Content{}},
				Responses:   map[int]routespec.ResponseSpec{http.StatusNoContent: {Description: "No content."}},
			},
		},
		{
			name: "duplicate media types",
			path: "/duplicate-media-types",
			operation: routespec.Operation{
				ID: "duplicateMediaTypes",
				RequestBody: routespec.RequestSpec{Content: []routespec.Content{
					routespec.Media[string]("text/plain"),
					routespec.Media[string]("text/plain"),
				}},
				Responses: map[int]routespec.ResponseSpec{http.StatusNoContent: {Description: "No content."}},
			},
		},
		{
			name: "invalid cookie model",
			path: "/invalid-cookie",
			operation: routespec.Operation{
				ID:        "invalidCookie",
				Cookie:    routespec.Cookie[string](),
				Responses: map[int]routespec.ResponseSpec{http.StatusNoContent: {Description: "No content."}},
			},
		},
		{
			name: "path model does not match route",
			path: "/uploads/{uploadID}",
			operation: routespec.Operation{
				ID: "invalidPath",
				Path: routespec.Path[struct {
					Other string `json:"other"`
				}](),
				Responses: map[int]routespec.ResponseSpec{http.StatusNoContent: {Description: "No content."}},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mux := http.NewServeMux()
			api := routespec.New(mux, routespec.Info{Title: "Example", Version: "1.0.0"})

			panicked := false
			func() {
				defer func() { panicked = recover() != nil }()
				api.RegisterFunc(http.MethodPost, test.path, func(http.ResponseWriter, *http.Request) {}, test.operation)
			}()
			if !panicked {
				t.Fatal("RegisterFunc did not panic")
			}

			response := httptest.NewRecorder()
			mux.ServeHTTP(response, httptest.NewRequest(http.MethodPost, test.path, nil))
			if response.Code != http.StatusNotFound {
				t.Fatalf("invalid route status = %d, want %d", response.Code, http.StatusNotFound)
			}
		})
	}
}
