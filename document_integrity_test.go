package routespec_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kensonjohnson/routespec"
)

type documentIntegrityValue struct{}

func TestDocumentLinksDoNotExposeRegisteredOperationData(t *testing.T) {
	api := routespec.New(http.NewServeMux(), routespec.Info{Title: "Document", Version: "1.0.0"})
	api.RegisterFunc(http.MethodGet, "/document", func(http.ResponseWriter, *http.Request) {}, routespec.Operation{
		ID: "documentIntegrity",
		Responses: routespec.Responses{http.StatusOK: {
			Links: map[string]routespec.LinkSpec{
				"self": {
					OperationID: "documentIntegrity",
					Parameters:  map[string]any{"filter": map[string]any{"limit": 10}},
					RequestBody: map[string]any{"mode": "full"},
				},
			},
		}},
	})

	link := api.Document().Paths["/document"].Get.Responses["200"].Links["self"]
	link.Parameters["filter"].(map[string]any)["limit"] = 100
	link.RequestBody.(map[string]any)["mode"] = "summary"

	got := api.Document().Paths["/document"].Get.Responses["200"].Links["self"]
	if got.Parameters["filter"].(map[string]any)["limit"] != 10 {
		t.Fatalf("link parameters = %#v, want registered value", got.Parameters)
	}
	if got.RequestBody.(map[string]any)["mode"] != "full" {
		t.Fatalf("link request body = %#v, want registered value", got.RequestBody)
	}
}

func TestRegisterRejectsMalformedLocalReferencesBeforeRouteRegistration(t *testing.T) {
	tests := []struct {
		name      string
		operation routespec.Operation
	}{
		{
			name: "schema reference",
			operation: routespec.Operation{
				ID: "badSchemaReference",
				Responses: routespec.Responses{http.StatusOK: routespec.ResponseSpec{
					Content: []routespec.Content{routespec.JSON[documentIntegrityValue]()},
				}},
			},
		},
		{
			name: "link operation reference",
			operation: routespec.Operation{
				ID: "badLinkReference",
				Responses: routespec.Responses{http.StatusOK: routespec.ResponseSpec{
					Links: map[string]routespec.LinkSpec{"bad": {OperationRef: "#bad"}},
				}},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mux := http.NewServeMux()
			options := []routespec.Option(nil)
			if test.name == "schema reference" {
				options = append(options, routespec.WithSchemaOverride[documentIntegrityValue](routespec.Schema{Ref: "#bad"}))
			}
			api := routespec.New(mux, routespec.Info{Title: "Document", Version: "1.0.0"}, options...)

			panicked := false
			func() {
				defer func() { panicked = recover() != nil }()
				api.RegisterFunc(http.MethodGet, "/invalid", func(http.ResponseWriter, *http.Request) {}, test.operation)
			}()
			if !panicked {
				t.Fatal("registration did not panic")
			}

			response := httptest.NewRecorder()
			mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/invalid", nil))
			if response.Code != http.StatusNotFound {
				t.Fatalf("invalid route status = %d, want %d", response.Code, http.StatusNotFound)
			}
		})
	}
}
