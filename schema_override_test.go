package routespec_test

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/kensonjohnson/routespec"
)

type flexibleValue struct{}

type constrainedOverrideValue string

type constrainedOverrideHolder struct {
	Value constrainedOverrideValue `json:"value"`
}

type mutableOverrideValue struct{}

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
		Responses: routespec.Responses{http.StatusOK: routespec.Respond(routespec.JSON[flexibleValue]())},
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

func TestWithSchemaOverrideIsolatesMutableSchemas(t *testing.T) {
	override := routespec.Schema{
		Type:       "object",
		Properties: map[string]routespec.Schema{"name": {Type: "string"}},
		Required:   []string{"name"},
		Example:    map[string]any{"name": "original"},
	}
	api := routespec.New(
		http.NewServeMux(),
		routespec.Info{Title: "Overrides", Version: "1.0.0"},
		routespec.WithSchemaOverride[mutableOverrideValue](override),
	)
	api.RegisterFunc(http.MethodGet, "/value", func(http.ResponseWriter, *http.Request) {}, routespec.Operation{
		ID:        "getValue",
		Responses: routespec.Responses{http.StatusOK: routespec.Respond(routespec.JSON[mutableOverrideValue]())},
	})

	want := api.Document()
	override.Properties["name"] = routespec.Schema{Type: "integer"}
	override.Required[0] = "changed"
	override.Example.(map[string]any)["name"] = "changed"
	if got := api.Document(); !reflect.DeepEqual(got, want) {
		t.Fatalf("document changed after override mutation\nwant: %#v\ngot:  %#v", want, got)
	}

	returned := api.Document()
	schema := returned.Paths["/value"].Get.Responses["200"].Content["application/json"].Schema
	schema.Properties["name"] = routespec.Schema{Type: "integer"}
	schema.Required[0] = "changed"
	schema.Example.(map[string]any)["name"] = "changed"
	if got := api.Document(); !reflect.DeepEqual(got, want) {
		t.Fatalf("document changed after returned-document mutation\nwant: %#v\ngot:  %#v", want, got)
	}
}

func TestWithSchemaOverridePreservesUnannotatedFields(t *testing.T) {
	minimum := 3
	override := routespec.Schema{Type: "string", MinLength: &minimum, ReadOnly: true}
	api := routespec.New(
		http.NewServeMux(),
		routespec.Info{Title: "Overrides", Version: "1.0.0"},
		routespec.WithSchemaOverride[constrainedOverrideValue](override),
	)
	api.RegisterFunc(http.MethodGet, "/value", func(http.ResponseWriter, *http.Request) {}, routespec.Operation{
		ID:        "getValue",
		Responses: routespec.Responses{http.StatusOK: routespec.Respond(routespec.JSON[constrainedOverrideHolder]())},
	})

	property := api.Document().Components.Schemas["constrainedOverrideHolder"].Properties["value"]
	if !reflect.DeepEqual(property, override) {
		t.Fatalf("property schema = %#v, want %#v", property, override)
	}
}
