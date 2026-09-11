package routespec_test

import (
	"bytes"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/kensonjohnson/routespec"
)

type publicContractPath struct {
	ID int64 `json:"id" openapi:"description=Widget identifier"`
}

type publicContractQuery struct {
	Limit int `json:"limit" openapi:"minimum=1,maximum=100,default=20"`
}

type publicContractRequest struct {
	Name string `json:"name" openapi:"required,minLength=1,maxLength=100"`
}

type publicContractResponse struct {
	ID   int64  `json:"id" openapi:"required"`
	Name string `json:"name"`
}

type publicContractProblem struct {
	Message string `json:"message" openapi:"required"`
}

func TestPublicContractGolden(t *testing.T) {
	api := newPublicContractAPI()
	document := api.Document()
	if document.OpenAPI != "3.1.0" || document.Info.Title != "Contract API" || document.Info.Version != "1.0.0" {
		t.Fatalf("unexpected document header: %#v", document)
	}
	if scheme := document.Components.SecuritySchemes["bearerAuth"]; scheme.Type != "http" || scheme.Scheme != "bearer" {
		t.Fatalf("security scheme = %#v", scheme)
	}
	operation := document.Paths["/widgets/{id}"].Post
	if operation == nil || operation.RequestBody == nil || len(operation.Parameters) != 2 || len(operation.Responses) != 2 {
		t.Fatalf("incomplete generated operation: %#v", operation)
	}

	got, err := api.JSON()
	if err != nil {
		t.Fatalf("JSON: %v", err)
	}
	fixturePath := filepath.Join("testdata", "public-contract-openapi.json")
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.MkdirAll(filepath.Dir(fixturePath), 0o755); err != nil {
			t.Fatalf("create fixture directory: %v", err)
		}
		if err := os.WriteFile(fixturePath, got, 0o644); err != nil {
			t.Fatalf("write fixture: %v", err)
		}
	}
	want, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("generated OpenAPI differs from %s\nwant:\n%s\ngot:\n%s", fixturePath, want, got)
	}
}

func TestUndeclaredSecuritySchemePanicsDuringRegistration(t *testing.T) {
	api := routespec.New(http.NewServeMux(), routespec.Info{Title: "Example", Version: "1.0.0"})
	defer func() {
		if recover() == nil {
			t.Fatal("registration did not panic")
		}
	}()
	api.RegisterFunc(http.MethodGet, "/widgets", func(http.ResponseWriter, *http.Request) {}, routespec.Operation{
		ID:        "listWidgets",
		Security:  []routespec.SecurityRequirement{{"bearerAuth": {}}},
		Responses: routespec.Responses{http.StatusOK: {}},
	})
}

func newPublicContractAPI() *routespec.API {
	mux := http.NewServeMux()
	api := routespec.New(mux, routespec.Info{
		Title:          "Contract API",
		Version:        "1.0.0",
		Description:    "A stable public contract fixture.",
		TermsOfService: "https://example.com/terms",
		Contact:        &routespec.Contact{Name: "Contract team", Email: "api@example.com"},
		License:        &routespec.License{Name: "Apache 2.0", Identifier: "Apache-2.0"},
		Servers: []routespec.Server{{
			URL:       "https://{environment}.example.com/v1",
			Variables: map[string]routespec.ServerVariable{"environment": {Enum: []string{"api", "staging"}, Default: "api"}},
		}},
		Tags: []routespec.Tag{{
			Name:         "widgets",
			Description:  "Widget operations.",
			ExternalDocs: &routespec.ExternalDocs{URL: "https://example.com/docs/widgets"},
		}},
		ExternalDocs: &routespec.ExternalDocs{Description: "Integration guide.", URL: "https://example.com/docs"},
		SecuritySchemes: map[string]routespec.SecurityScheme{
			"apiKeyAuth": {Type: "apiKey", Name: "X-API-Key", In: "header"},
			"bearerAuth": {Type: "http", Scheme: "bearer", BearerFormat: "JWT"},
			"mtlsAuth":   {Type: "mutualTLS"},
			"oauthAuth": {
				Type: "oauth2",
				Flows: &routespec.OAuthFlows{AuthorizationCode: &routespec.OAuthFlow{
					AuthorizationURL: "https://example.com/authorize",
					TokenURL:         "https://example.com/token",
					Scopes:           map[string]string{"widgets:write": "Update widgets"},
				}},
			},
			"oidcAuth": {Type: "openIdConnect", OpenIDConnectURL: "https://example.com/.well-known/openid-configuration"},
		},
	})
	api.RegisterFunc(http.MethodPost, "/widgets/{id}", func(http.ResponseWriter, *http.Request) {}, routespec.Operation{
		ID:          "updateWidget",
		Summary:     "Update a widget",
		Description: "Updates one widget.",
		Tags:        []string{"widgets"},
		Security:    []routespec.SecurityRequirement{{"bearerAuth": {}}},
		Path:        routespec.Path[publicContractPath](),
		Query:       routespec.Query[publicContractQuery](),
		RequestBody: routespec.Request(routespec.JSON[publicContractRequest]()),
		Responses: routespec.Responses{
			http.StatusOK: {
				Content: []routespec.Content{routespec.JSON[publicContractResponse]()},
				Links:   map[string]routespec.LinkSpec{"self": {OperationID: "updateWidget"}},
			},
			http.StatusBadRequest: routespec.Respond(routespec.JSON[publicContractProblem]()),
		},
	})
	return api
}
