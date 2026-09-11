package routespec_test

import (
	"net/http"
	"reflect"
	"testing"

	"github.com/kensonjohnson/routespec"
)

type EmbeddedJSONSchemaRepeatedFields struct {
	Value string
}

type EmbeddedJSONSchemaRepeatedLeft struct {
	EmbeddedJSONSchemaRepeatedFields
}

type EmbeddedJSONSchemaRepeatedRight struct {
	EmbeddedJSONSchemaRepeatedFields
}

type embeddedJSONSchemaAmbiguousBody struct {
	EmbeddedJSONSchemaRepeatedLeft
	EmbeddedJSONSchemaRepeatedRight
	Kept string `json:"kept"`
}

type embeddedJSONSchemaMapFallbackBody struct {
	Known    string            `json:"known"`
	Fallback map[string]string `json:",embed"`
}

func TestEmbeddedJSONSchemaOmitsRepeatedEmbeddedFieldAmbiguity(t *testing.T) {
	api := routespec.New(http.NewServeMux(), routespec.Info{Title: "Embedded schemas", Version: "1.0.0"})
	api.RegisterFunc(http.MethodGet, "/repeated-embedded", func(http.ResponseWriter, *http.Request) {}, routespec.Operation{
		ID:        "repeatedEmbedded",
		Responses: routespec.Responses{http.StatusOK: routespec.Respond(routespec.JSON[embeddedJSONSchemaAmbiguousBody]())},
	})

	schema := api.Document().Components.Schemas["embeddedJSONSchemaAmbiguousBody"]
	if _, exists := schema.Properties["Value"]; exists {
		t.Fatalf("ambiguous embedded property Value = %#v, want omitted", schema.Properties["Value"])
	}
	if got, want := schema.Properties["kept"], (routespec.Schema{Type: "string"}); !reflect.DeepEqual(got, want) {
		t.Fatalf("kept property = %#v, want %#v", got, want)
	}
}

func TestEmbeddedJSONSchemaUsesMapFallbackAsAdditionalProperties(t *testing.T) {
	api := routespec.New(http.NewServeMux(), routespec.Info{Title: "Embedded schemas", Version: "1.0.0"})
	api.RegisterFunc(http.MethodGet, "/map-fallback", func(http.ResponseWriter, *http.Request) {}, routespec.Operation{
		ID:        "mapFallback",
		Responses: routespec.Responses{http.StatusOK: routespec.Respond(routespec.JSON[embeddedJSONSchemaMapFallbackBody]())},
	})

	schema := api.Document().Components.Schemas["embeddedJSONSchemaMapFallbackBody"]
	if got, want := schema.Properties["known"], (routespec.Schema{Type: "string"}); !reflect.DeepEqual(got, want) {
		t.Fatalf("known property = %#v, want %#v", got, want)
	}
	wantFallback := &routespec.Schema{Type: "string"}
	if got := schema.AdditionalProperties; !reflect.DeepEqual(got, wantFallback) {
		t.Fatalf("additional properties = %#v, want %#v", got, wantFallback)
	}
}
