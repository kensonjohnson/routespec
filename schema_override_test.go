package routespec_test

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/kensonjohnson/routespec"
)

type flexibleValue struct{}

func TestWithSchemaOverride(t *testing.T) {
	override := routespec.Schema{
		OneOf: []routespec.Schema{
			{Type: "string"},
			{Type: "integer", Format: "int64"},
		},
	}
	api := routespec.New(
		http.NewServeMux(),
		routespec.Info{Title: "Overrides", Version: "1.0.0"},
		routespec.WithSchemaOverride[flexibleValue](override),
	)
	api.RegisterFunc(http.MethodGet, "/value", func(http.ResponseWriter, *http.Request) {}, routespec.Operation{
		ID:        "getValue",
		Responses: routespec.Responses{http.StatusOK: routespec.JSON[flexibleValue]()},
	})

	got := api.Document().Paths["/value"].Get.Responses["200"].Content["application/json"].Schema
	if !reflect.DeepEqual(got, override) {
		t.Fatalf("response schema = %#v, want %#v", got, override)
	}

	response := httptest.NewRecorder()
	api.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/openapi.json", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("spec handler status = %d", response.Code)
	}
}
