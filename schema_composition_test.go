package routespec_test

import (
	json "encoding/json/v2"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/kensonjohnson/routespec"
)

type schemaCompositionValue struct{}

type discriminatedUnionValue struct{}

func TestWithSchemaOverrideSerializesSchemaComposition(t *testing.T) {
	override := routespec.Schema{
		AllOf: []routespec.Schema{
			{Type: "object", Properties: map[string]routespec.Schema{"id": {Type: "string"}}},
		},
		AnyOf: []routespec.Schema{
			{Type: "string"},
			{Type: "integer"},
		},
		OneOf: []routespec.Schema{
			{Type: "boolean"},
			{Type: "null"},
		},
		Not: &routespec.Schema{Type: "number"},
	}
	api := routespec.New(
		http.NewServeMux(),
		routespec.Info{Title: "Composition", Version: "1.0.0"},
		routespec.WithSchemaOverride[schemaCompositionValue](override),
	)
	api.RegisterFunc(http.MethodGet, "/value", func(http.ResponseWriter, *http.Request) {}, routespec.Operation{
		ID:        "getComposition",
		Responses: routespec.Responses{http.StatusOK: routespec.JSON[schemaCompositionValue]()},
	})

	encoded, err := api.JSON()
	if err != nil {
		t.Fatalf("JSON: %v", err)
	}
	var document map[string]any
	if err := json.Unmarshal(encoded, &document); err != nil {
		t.Fatalf("unmarshal document: %v", err)
	}
	got := document["paths"].(map[string]any)["/value"].(map[string]any)["get"].(map[string]any)["responses"].(map[string]any)["200"].(map[string]any)["content"].(map[string]any)["application/json"].(map[string]any)["schema"]
	want := map[string]any{
		"allOf": []any{
			map[string]any{"type": "object", "properties": map[string]any{"id": map[string]any{"type": "string"}}},
		},
		"anyOf": []any{
			map[string]any{"type": "string"},
			map[string]any{"type": "integer"},
		},
		"oneOf": []any{
			map[string]any{"type": "boolean"},
			map[string]any{"type": "null"},
		},
		"not": map[string]any{"type": "number"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("serialized schema = %#v, want %#v", got, want)
	}
}

func TestWithSchemaOverrideSupportsDiscriminatedUnion(t *testing.T) {
	override := discriminatedUnionSchema(true)
	api := routespec.New(
		http.NewServeMux(),
		routespec.Info{Title: "Composition", Version: "1.0.0"},
		routespec.WithSchemaOverride[discriminatedUnionValue](override),
	)
	api.RegisterFunc(http.MethodGet, "/pet", func(http.ResponseWriter, *http.Request) {}, routespec.Operation{
		ID:        "getPet",
		Responses: routespec.Responses{http.StatusOK: routespec.JSON[discriminatedUnionValue]()},
	})

	got := api.Document().Paths["/pet"].Get.Responses["200"].Content["application/json"].Schema
	if !reflect.DeepEqual(got, override) {
		t.Fatalf("response schema = %#v, want %#v", got, override)
	}

	encoded, err := api.JSON()
	if err != nil {
		t.Fatalf("JSON: %v", err)
	}
	var document map[string]any
	if err := json.Unmarshal(encoded, &document); err != nil {
		t.Fatalf("unmarshal document: %v", err)
	}
	schema := document["paths"].(map[string]any)["/pet"].(map[string]any)["get"].(map[string]any)["responses"].(map[string]any)["200"].(map[string]any)["content"].(map[string]any)["application/json"].(map[string]any)["schema"].(map[string]any)
	wantDiscriminator := map[string]any{
		"propertyName": "kind",
		"mapping": map[string]any{
			"cat": "#/components/schemas/Cat",
			"dog": "#/components/schemas/Dog",
		},
	}
	if got := schema["discriminator"]; !reflect.DeepEqual(got, wantDiscriminator) {
		t.Fatalf("serialized discriminator = %#v, want %#v", got, wantDiscriminator)
	}
}

func TestWithSchemaOverrideRejectsInvalidSchemaCompositionDuringRegistration(t *testing.T) {
	tests := []struct {
		name   string
		schema routespec.Schema
	}{
		{
			name:   "empty allOf",
			schema: routespec.Schema{AllOf: []routespec.Schema{}},
		},
		{
			name:   "empty anyOf",
			schema: routespec.Schema{AnyOf: []routespec.Schema{}},
		},
		{
			name:   "empty oneOf",
			schema: routespec.Schema{OneOf: []routespec.Schema{}},
		},
		{
			name: "discriminator without composition",
			schema: routespec.Schema{
				Type:          "object",
				Properties:    map[string]routespec.Schema{"kind": {Type: "string"}},
				Required:      []string{"kind"},
				Discriminator: &routespec.Discriminator{PropertyName: "kind"},
			},
		},
		{
			name:   "discriminator property is not required",
			schema: discriminatedUnionSchema(false),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assertSchemaCompositionRegistrationPanics(t, test.schema)
		})
	}
}

func discriminatedUnionSchema(requireKind bool) routespec.Schema {
	required := []string(nil)
	catRequired := []string{"lives"}
	dogRequired := []string{"barks"}
	if requireKind {
		required = []string{"kind"}
		catRequired = append([]string{"kind"}, catRequired...)
		dogRequired = append([]string{"kind"}, dogRequired...)
	}
	return routespec.Schema{
		Type:       "object",
		Properties: map[string]routespec.Schema{"kind": {Type: "string"}},
		Required:   required,
		OneOf: []routespec.Schema{
			{
				Type:       "object",
				Properties: map[string]routespec.Schema{"kind": {Const: "cat"}, "lives": {Type: "integer"}},
				Required:   catRequired,
			},
			{
				Type:       "object",
				Properties: map[string]routespec.Schema{"kind": {Const: "dog"}, "barks": {Type: "boolean"}},
				Required:   dogRequired,
			},
		},
		Discriminator: &routespec.Discriminator{
			PropertyName: "kind",
			Mapping: map[string]string{
				"cat": "#/components/schemas/Cat",
				"dog": "#/components/schemas/Dog",
			},
		},
	}
}

func assertSchemaCompositionRegistrationPanics(t *testing.T, schema routespec.Schema) {
	t.Helper()
	mux := http.NewServeMux()
	api := routespec.New(
		mux,
		routespec.Info{Title: "Composition", Version: "1.0.0"},
		routespec.WithSchemaOverride[schemaCompositionValue](schema),
	)

	panicked := false
	func() {
		defer func() {
			panicked = recover() != nil
		}()
		api.RegisterFunc(http.MethodGet, "/invalid", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, routespec.Operation{
			ID:        "invalidComposition",
			Responses: routespec.Responses{http.StatusNoContent: routespec.JSON[schemaCompositionValue]()},
		})
	}()
	if !panicked {
		t.Fatal("registration did not panic")
	}

	response := httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/invalid", nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("invalid route status = %d, want %d", response.Code, http.StatusNotFound)
	}
}
