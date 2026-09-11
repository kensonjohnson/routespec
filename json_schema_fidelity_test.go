package routespec_test

import (
	json "encoding/json/v2"
	"errors"
	"net/http"
	"reflect"
	"testing"

	"github.com/kensonjohnson/routespec"
)

type JSONSchemaFidelityEmbeddedDTO struct {
	Embedded string `json:"embedded"`
	Shared   int    `json:"shared"`
}

type JSONSchemaFidelitySettingsDTO struct {
	Mode string `json:"mode"`
}

type jsonSchemaFidelityDTO struct {
	JSONSchemaFidelityEmbeddedDTO
	Shared   string                        `json:"shared"`
	Optional *string                       `json:"optional"`
	Fixed    string                        `json:"fixed" openapi:"const=value"`
	Lower    int                           `json:"lower" openapi:"exclusiveMinimum=1"`
	Upper    int                           `json:"upper" openapi:"exclusiveMaximum=10"`
	IDs      []string                      `json:"ids" openapi:"uniqueItems"`
	Settings JSONSchemaFidelitySettingsDTO `json:"settings" openapi:"additionalProperties=false"`
}

type JSONSchemaFidelityAmbiguousLeft struct {
	Value string
}

type JSONSchemaFidelityAmbiguousRight struct {
	Value string
}

type jsonSchemaFidelityAmbiguousDTO struct {
	JSONSchemaFidelityAmbiguousLeft
	JSONSchemaFidelityAmbiguousRight
}

type JSONSchemaFidelityCustomDTO struct{}

func (JSONSchemaFidelityCustomDTO) MarshalJSON() ([]byte, error) {
	return nil, errors.New("not used")
}

type JSONSchemaFidelityEmbeddedTagDTO struct {
	Label string `json:"label"`
}

type jsonSchemaFidelityV2DTO struct {
	Metadata JSONSchemaFidelityEmbeddedTagDTO `json:",embed"`
	Quoted   int                              `json:"quoted,string"`
	Omitted  *string                          `json:"omitted,omitempty"`
	Nullable *string                          `json:"nullable"`
}

func TestJSONSchemaFidelity(t *testing.T) {
	api := routespec.New(http.NewServeMux(), routespec.Info{Title: "Schema fidelity", Version: "1.0.0"})
	api.RegisterFunc(http.MethodPost, "/schema-fidelity", func(http.ResponseWriter, *http.Request) {}, routespec.Operation{
		ID:          "schemaFidelity",
		RequestBody: routespec.Request(routespec.JSON[jsonSchemaFidelityDTO]()),
		Responses:   routespec.Responses{http.StatusOK: {}},
	})

	schema := jsonSchemaFidelitySchema(t, api, "jsonSchemaFidelityDTO")
	properties := jsonSchemaFidelityProperties(t, schema)
	if got, want := properties["embedded"], map[string]any{"type": "string"}; !reflect.DeepEqual(got, want) {
		t.Errorf("embedded property = %#v, want %#v", got, want)
	}
	if got, want := properties["shared"], map[string]any{"type": "string"}; !reflect.DeepEqual(got, want) {
		t.Errorf("shared property = %#v, want %#v", got, want)
	}

	optional := jsonSchemaFidelityProperty(t, properties, "optional")
	if got, want := optional["type"], []any{"string", "null"}; !reflect.DeepEqual(got, want) {
		t.Errorf("optional.type = %#v, want %#v", got, want)
	}

	for name, assertion := range map[string]struct {
		key  string
		want any
	}{
		"fixed": {key: "const", want: "value"},
		"lower": {key: "exclusiveMinimum", want: float64(1)},
		"upper": {key: "exclusiveMaximum", want: float64(10)},
		"ids":   {key: "uniqueItems", want: true},
	} {
		property := jsonSchemaFidelityProperty(t, properties, name)
		if got := property[assertion.key]; !reflect.DeepEqual(got, assertion.want) {
			t.Errorf("%s.%s = %#v, want %#v", name, assertion.key, got, assertion.want)
		}
	}

	settings := jsonSchemaFidelityProperty(t, properties, "settings")
	if got, want := settings["additionalProperties"], false; got != want {
		t.Errorf("settings.additionalProperties = %#v, want %#v", got, want)
	}
}

func TestJSONSchemaFidelityUsesJSONV2Tags(t *testing.T) {
	api := routespec.New(http.NewServeMux(), routespec.Info{Title: "Schema fidelity", Version: "1.0.0"})
	api.RegisterFunc(http.MethodPost, "/json-v2", jsonSchemaFidelityHandler, routespec.Operation{
		ID:          "jsonV2Tags",
		RequestBody: routespec.Request(routespec.JSON[jsonSchemaFidelityV2DTO]()),
		Responses:   routespec.Responses{http.StatusOK: {}},
	})

	properties := jsonSchemaFidelityProperties(t, jsonSchemaFidelitySchema(t, api, "jsonSchemaFidelityV2DTO"))
	if got, want := jsonSchemaFidelityProperty(t, properties, "label")["type"], "string"; got != want {
		t.Errorf("embedded label type = %#v, want %q", got, want)
	}
	if quoted := jsonSchemaFidelityProperty(t, properties, "quoted"); quoted["type"] != "string" || quoted["format"] != nil {
		t.Errorf("quoted schema = %#v, want string without format", quoted)
	}
	if got, want := jsonSchemaFidelityProperty(t, properties, "omitted")["type"], "string"; got != want {
		t.Errorf("omitempty pointer type = %#v, want %q", got, want)
	}
	if got, want := jsonSchemaFidelityProperty(t, properties, "nullable")["type"], []any{"string", "null"}; !reflect.DeepEqual(got, want) {
		t.Errorf("nullable pointer type = %#v, want %#v", got, want)
	}
}

func TestJSONSchemaFidelityRejectsAmbiguousEmbeddedFields(t *testing.T) {
	api := routespec.New(http.NewServeMux(), routespec.Info{Title: "Schema fidelity", Version: "1.0.0"})
	defer func() {
		if recover() == nil {
			t.Fatal("registration did not panic")
		}
	}()
	api.RegisterFunc(http.MethodPost, "/ambiguous", jsonSchemaFidelityHandler, routespec.Operation{
		ID:          "ambiguousEmbeddedFields",
		RequestBody: routespec.Request(routespec.JSON[jsonSchemaFidelityAmbiguousDTO]()),
		Responses:   routespec.Responses{http.StatusOK: {}},
	})
}

func TestJSONSchemaFidelityInvalidTagsPanicDuringRegistration(t *testing.T) {
	tests := []struct {
		name     string
		register func(*routespec.API)
	}{
		{
			name: "custom JSON type without override",
			register: func(api *routespec.API) {
				api.RegisterFunc(http.MethodPost, "/invalid-schema-fidelity", jsonSchemaFidelityHandler, routespec.Operation{
					ID: "invalidSchemaFidelity",
					RequestBody: routespec.Request(routespec.JSON[struct {
						Value JSONSchemaFidelityCustomDTO `json:"value"`
					}]()),
					Responses: routespec.Responses{http.StatusOK: {}},
				})
			},
		},
		{
			name: "const on object",
			register: func(api *routespec.API) {
				api.RegisterFunc(http.MethodPost, "/invalid-schema-fidelity", jsonSchemaFidelityHandler, routespec.Operation{
					ID: "invalidSchemaFidelity",
					RequestBody: routespec.Request(routespec.JSON[struct {
						Value struct{} `json:"value" openapi:"const=value"`
					}]()),
					Responses: routespec.Responses{http.StatusOK: {}},
				})
			},
		},
		{
			name: "exclusive minimum on string",
			register: func(api *routespec.API) {
				api.RegisterFunc(http.MethodPost, "/invalid-schema-fidelity", jsonSchemaFidelityHandler, routespec.Operation{
					ID: "invalidSchemaFidelity",
					RequestBody: routespec.Request(routespec.JSON[struct {
						Value string `json:"value" openapi:"exclusiveMinimum=1"`
					}]()),
					Responses: routespec.Responses{http.StatusOK: {}},
				})
			},
		},
		{
			name: "exclusive maximum on string",
			register: func(api *routespec.API) {
				api.RegisterFunc(http.MethodPost, "/invalid-schema-fidelity", jsonSchemaFidelityHandler, routespec.Operation{
					ID: "invalidSchemaFidelity",
					RequestBody: routespec.Request(routespec.JSON[struct {
						Value string `json:"value" openapi:"exclusiveMaximum=1"`
					}]()),
					Responses: routespec.Responses{http.StatusOK: {}},
				})
			},
		},
		{
			name: "unique items on string",
			register: func(api *routespec.API) {
				api.RegisterFunc(http.MethodPost, "/invalid-schema-fidelity", jsonSchemaFidelityHandler, routespec.Operation{
					ID: "invalidSchemaFidelity",
					RequestBody: routespec.Request(routespec.JSON[struct {
						Value string `json:"value" openapi:"uniqueItems"`
					}]()),
					Responses: routespec.Responses{http.StatusOK: {}},
				})
			},
		},
		{
			name: "additional properties on string",
			register: func(api *routespec.API) {
				api.RegisterFunc(http.MethodPost, "/invalid-schema-fidelity", jsonSchemaFidelityHandler, routespec.Operation{
					ID: "invalidSchemaFidelity",
					RequestBody: routespec.Request(routespec.JSON[struct {
						Value string `json:"value" openapi:"additionalProperties=false"`
					}]()),
					Responses: routespec.Responses{http.StatusOK: {}},
				})
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("registration did not panic")
				}
			}()
			test.register(routespec.New(http.NewServeMux(), routespec.Info{Title: "Schema fidelity", Version: "1.0.0"}))
		})
	}
}

func jsonSchemaFidelityHandler(http.ResponseWriter, *http.Request) {}

func jsonSchemaFidelitySchema(t *testing.T, api *routespec.API, name string) map[string]any {
	t.Helper()
	body, err := api.JSON()
	if err != nil {
		t.Fatalf("marshal document: %v", err)
	}
	var document map[string]any
	if err := json.Unmarshal(body, &document); err != nil {
		t.Fatalf("unmarshal document: %v", err)
	}
	components := document["components"].(map[string]any)
	schemas := components["schemas"].(map[string]any)
	schema, ok := schemas[name].(map[string]any)
	if !ok {
		t.Fatalf("schema %q = %#v", name, schemas[name])
	}
	return schema
}

func jsonSchemaFidelityProperties(t *testing.T, schema map[string]any) map[string]any {
	t.Helper()
	properties, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("properties = %#v", schema["properties"])
	}
	return properties
}

func jsonSchemaFidelityProperty(t *testing.T, properties map[string]any, name string) map[string]any {
	t.Helper()
	property, ok := properties[name].(map[string]any)
	if !ok {
		t.Fatalf("property %q = %#v", name, properties[name])
	}
	return property
}
