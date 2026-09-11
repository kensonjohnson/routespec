package routespec_test

import (
	json "encoding/json/v2"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/kensonjohnson/routespec"
)

type referenceValidationValue struct{}

func TestDocumentMetadataSerializes(t *testing.T) {
	api := routespec.New(http.NewServeMux(), routespec.Info{
		Title:          "Metadata API",
		Version:        "1.0.0",
		TermsOfService: "https://example.com/terms",
		Contact: &routespec.Contact{
			Name:  "API team",
			URL:   "https://example.com/support",
			Email: "api@example.com",
		},
		License: &routespec.License{
			Name:       "Apache 2.0",
			Identifier: "Apache-2.0",
		},
		Servers: []routespec.Server{
			{
				URL:         "https://{environment}.example.com/{basePath}",
				Description: "Production",
				Variables: map[string]routespec.ServerVariable{
					"environment": {Enum: []string{"api", "staging"}, Default: "api"},
					"basePath":    {Default: "v1"},
				},
			},
		},
		Tags: []routespec.Tag{
			{
				Name:        "widgets",
				Description: "Widget operations",
				ExternalDocs: &routespec.ExternalDocs{
					Description: "Widget guide",
					URL:         "https://example.com/docs/widgets",
				},
			},
		},
		ExternalDocs: &routespec.ExternalDocs{
			Description: "Integration guide",
			URL:         "https://example.com/docs",
		},
		SecuritySchemes: map[string]routespec.SecurityScheme{
			"oauth": {
				Type: "oauth2",
				Flows: &routespec.OAuthFlows{
					AuthorizationCode: &routespec.OAuthFlow{
						AuthorizationURL: "https://example.com/authorize",
						TokenURL:         "https://example.com/token",
						RefreshURL:       "https://example.com/refresh",
						Scopes:           map[string]string{"widgets:read": "Read widgets"},
					},
				},
			},
			"oidc": {
				Type:             "openIdConnect",
				OpenIDConnectURL: "https://example.com/.well-known/openid-configuration",
			},
		},
	})
	api.RegisterFunc(http.MethodGet, "/health", func(http.ResponseWriter, *http.Request) {}, routespec.Operation{
		ID:        "health",
		Responses: routespec.Responses{http.StatusNoContent: {}},
	})

	document := api.Document()
	if got, want := document.Info.TermsOfService, "https://example.com/terms"; got != want {
		t.Fatalf("terms of service = %q, want %q", got, want)
	}
	if got, want := document.Info.Contact, (&routespec.Contact{Name: "API team", URL: "https://example.com/support", Email: "api@example.com"}); !reflect.DeepEqual(got, want) {
		t.Fatalf("contact = %#v, want %#v", got, want)
	}
	if got, want := document.Info.License, (&routespec.License{Name: "Apache 2.0", Identifier: "Apache-2.0"}); !reflect.DeepEqual(got, want) {
		t.Fatalf("license = %#v, want %#v", got, want)
	}
	if got, want := document.ExternalDocs, (&routespec.ExternalDocs{Description: "Integration guide", URL: "https://example.com/docs"}); !reflect.DeepEqual(got, want) {
		t.Fatalf("external docs = %#v, want %#v", got, want)
	}
	if got, want := document.Servers, []routespec.Server{{
		URL:         "https://{environment}.example.com/{basePath}",
		Description: "Production",
		Variables: map[string]routespec.ServerVariable{
			"environment": {Enum: []string{"api", "staging"}, Default: "api"},
			"basePath":    {Default: "v1"},
		},
	}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("servers = %#v, want %#v", got, want)
	}
	if got, want := document.Tags, []routespec.Tag{{
		Name:        "widgets",
		Description: "Widget operations",
		ExternalDocs: &routespec.ExternalDocs{
			Description: "Widget guide",
			URL:         "https://example.com/docs/widgets",
		},
	}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("tags = %#v, want %#v", got, want)
	}
	if got := document.Components.SecuritySchemes["oauth"].Flows.AuthorizationCode.Scopes["widgets:read"]; got != "Read widgets" {
		t.Fatalf("OAuth scope description = %q, want %q", got, "Read widgets")
	}
	if got, want := document.Components.SecuritySchemes["oidc"].OpenIDConnectURL, "https://example.com/.well-known/openid-configuration"; got != want {
		t.Fatalf("OpenID Connect URL = %q, want %q", got, want)
	}

	encoded, err := api.JSON()
	if err != nil {
		t.Fatalf("JSON: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(encoded, &got); err != nil {
		t.Fatalf("unmarshal document: %v", err)
	}
	wantInfo := map[string]any{
		"title":          "Metadata API",
		"version":        "1.0.0",
		"termsOfService": "https://example.com/terms",
		"contact": map[string]any{
			"name":  "API team",
			"url":   "https://example.com/support",
			"email": "api@example.com",
		},
		"license": map[string]any{
			"name":       "Apache 2.0",
			"identifier": "Apache-2.0",
		},
	}
	if gotInfo := got["info"]; !reflect.DeepEqual(gotInfo, wantInfo) {
		t.Fatalf("serialized info = %#v, want %#v", gotInfo, wantInfo)
	}
	wantServers := []any{map[string]any{
		"url":         "https://{environment}.example.com/{basePath}",
		"description": "Production",
		"variables": map[string]any{
			"environment": map[string]any{"enum": []any{"api", "staging"}, "default": "api"},
			"basePath":    map[string]any{"default": "v1"},
		},
	}}
	if gotServers := got["servers"]; !reflect.DeepEqual(gotServers, wantServers) {
		t.Fatalf("serialized servers = %#v, want %#v", gotServers, wantServers)
	}
	wantTags := []any{map[string]any{
		"name":        "widgets",
		"description": "Widget operations",
		"externalDocs": map[string]any{
			"description": "Widget guide",
			"url":         "https://example.com/docs/widgets",
		},
	}}
	if gotTags := got["tags"]; !reflect.DeepEqual(gotTags, wantTags) {
		t.Fatalf("serialized tags = %#v, want %#v", gotTags, wantTags)
	}
	wantExternalDocs := map[string]any{"description": "Integration guide", "url": "https://example.com/docs"}
	if gotExternalDocs := got["externalDocs"]; !reflect.DeepEqual(gotExternalDocs, wantExternalDocs) {
		t.Fatalf("serialized external docs = %#v, want %#v", gotExternalDocs, wantExternalDocs)
	}
	wantSecuritySchemes := map[string]any{
		"oauth": map[string]any{
			"type": "oauth2",
			"flows": map[string]any{
				"authorizationCode": map[string]any{
					"authorizationUrl": "https://example.com/authorize",
					"tokenUrl":         "https://example.com/token",
					"refreshUrl":       "https://example.com/refresh",
					"scopes":           map[string]any{"widgets:read": "Read widgets"},
				},
			},
		},
		"oidc": map[string]any{
			"type":             "openIdConnect",
			"openIdConnectUrl": "https://example.com/.well-known/openid-configuration",
		},
	}
	gotComponents := got["components"].(map[string]any)
	if gotSecuritySchemes := gotComponents["securitySchemes"]; !reflect.DeepEqual(gotSecuritySchemes, wantSecuritySchemes) {
		t.Fatalf("serialized security schemes = %#v, want %#v", gotSecuritySchemes, wantSecuritySchemes)
	}
}

func TestRegisterRejectsBrokenDocumentReferencesBeforeRouteRegistration(t *testing.T) {
	tests := []struct {
		name string
		api  func(*http.ServeMux) *routespec.API
		op   routespec.Operation
	}{
		{
			name: "response link operation ID",
			api: func(mux *http.ServeMux) *routespec.API {
				return routespec.New(mux, routespec.Info{Title: "References", Version: "1.0.0"})
			},
			op: routespec.Operation{
				ID: "brokenLink",
				Responses: routespec.Responses{http.StatusOK: {
					Links: map[string]routespec.LinkSpec{
						"next": {OperationID: "missingOperation"},
					},
				}},
			},
		},
		{
			name: "discriminator mapping component reference",
			api: func(mux *http.ServeMux) *routespec.API {
				return routespec.New(
					mux,
					routespec.Info{Title: "References", Version: "1.0.0"},
					routespec.WithSchemaOverride[referenceValidationValue](routespec.Schema{
						Type:       "object",
						Properties: map[string]routespec.Schema{"kind": {Type: "string"}},
						Required:   []string{"kind"},
						OneOf: []routespec.Schema{
							{Type: "object"},
						},
						Discriminator: &routespec.Discriminator{
							PropertyName: "kind",
							Mapping: map[string]string{
								"missing": "#/components/schemas/Missing",
							},
						},
					}),
				)
			},
			op: routespec.Operation{
				ID:        "brokenDiscriminator",
				Responses: routespec.Responses{http.StatusOK: routespec.Respond(routespec.JSON[referenceValidationValue]())},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mux := http.NewServeMux()
			api := test.api(mux)
			path := "/invalid"

			panicked := false
			func() {
				defer func() { panicked = recover() != nil }()
				api.RegisterFunc(http.MethodGet, path, func(http.ResponseWriter, *http.Request) {}, test.op)
			}()
			if !panicked {
				t.Fatal("registration did not panic")
			}

			response := httptest.NewRecorder()
			mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
			if response.Code != http.StatusNotFound {
				t.Fatalf("invalid route status = %d, want %d", response.Code, http.StatusNotFound)
			}
		})
	}
}
