package routespec_test

import (
	"encoding/json"
	"net/http"
	"reflect"
	"testing"

	"github.com/kensonjohnson/routespec"
)

type annotatedBody struct {
	Name   string            `json:"name,omitempty" openapi:"required,description=Display\\, name \\\\ alias,format=email,minLength=1,maxLength=100,pattern=^[a-z]+$,enum=admin|member,default=member,example=admin,readOnly"`
	Secret string            `json:"secret" openapi:"writeOnly"`
	Old    string            `json:"old" openapi:"deprecated"`
	Count  int               `json:"count" openapi:"required,minimum=1,maximum=10,multipleOf=2,enum=2|4|6,default=2,example=4"`
	Tags   []string          `json:"tags" openapi:"minItems=1,maxItems=3"`
	Meta   map[string]string `json:"meta" openapi:"minProperties=1,maxProperties=2"`
	Live   bool              `json:"live" openapi:"default=true,example=false"`
}

type renamedPath struct {
	ID int64 `json:"id" openapi:"name=user_id,description=Public user ID"`
}

type renamedQuery struct {
	Page int `json:"page" openapi:"name=page_number,required,description=One-based page number,minimum=1"`
}

type unknownAnnotation struct {
	Value string `json:"value" openapi:"unknown=value"`
}

type stringConstraintOnInteger struct {
	Value int `json:"value" openapi:"minLength=1"`
}

type numericConstraintOnString struct {
	Value string `json:"value" openapi:"minimum=1"`
}

func TestOpenAPIAnnotationsPopulateSchema(t *testing.T) {
	api := annotationAPI(t)
	api.RegisterFunc(http.MethodPost, "/annotated", annotationHandler, routespec.Operation{
		ID:          "annotatedBody",
		RequestBody: routespec.JSON[annotatedBody](),
		Responses:   routespec.Responses{http.StatusOK: routespec.JSON[annotatedBody]()},
	})

	schema := annotationSchema(t, api, "annotatedBody")
	if got, want := schema["required"], []any{"count", "name"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("required = %#v, want %#v", got, want)
	}

	name := annotationProperty(t, schema, "name")
	for key, want := range map[string]any{
		"description": "Display, name \\ alias",
		"format":      "email",
		"minLength":   float64(1),
		"maxLength":   float64(100),
		"pattern":     "^[a-z]+$",
		"enum":        []any{"admin", "member"},
		"default":     "member",
		"example":     "admin",
		"readOnly":    true,
	} {
		if got := name[key]; !reflect.DeepEqual(got, want) {
			t.Errorf("name.%s = %#v, want %#v", key, got, want)
		}
	}

	if got, want := annotationProperty(t, schema, "secret")["writeOnly"], true; got != want {
		t.Errorf("secret.writeOnly = %#v, want %#v", got, want)
	}
	if got, want := annotationProperty(t, schema, "old")["deprecated"], true; got != want {
		t.Errorf("old.deprecated = %#v, want %#v", got, want)
	}

	count := annotationProperty(t, schema, "count")
	for key, want := range map[string]any{
		"minimum":    float64(1),
		"maximum":    float64(10),
		"multipleOf": float64(2),
		"enum":       []any{float64(2), float64(4), float64(6)},
		"default":    float64(2),
		"example":    float64(4),
	} {
		if got := count[key]; !reflect.DeepEqual(got, want) {
			t.Errorf("count.%s = %#v, want %#v", key, got, want)
		}
	}

	tags := annotationProperty(t, schema, "tags")
	if got, want := tags["minItems"], float64(1); got != want {
		t.Errorf("tags.minItems = %#v, want %#v", got, want)
	}
	if got, want := tags["maxItems"], float64(3); got != want {
		t.Errorf("tags.maxItems = %#v, want %#v", got, want)
	}

	meta := annotationProperty(t, schema, "meta")
	if got, want := meta["minProperties"], float64(1); got != want {
		t.Errorf("meta.minProperties = %#v, want %#v", got, want)
	}
	if got, want := meta["maxProperties"], float64(2); got != want {
		t.Errorf("meta.maxProperties = %#v, want %#v", got, want)
	}

	live := annotationProperty(t, schema, "live")
	if got, want := live["default"], true; got != want {
		t.Errorf("live.default = %#v, want %#v", got, want)
	}
	if got, want := live["example"], false; got != want {
		t.Errorf("live.example = %#v, want %#v", got, want)
	}
}

func TestOpenAPIAnnotationNameRenamesParameters(t *testing.T) {
	api := annotationAPI(t)
	api.RegisterFunc(http.MethodGet, "/users/{user_id}", annotationHandler, routespec.Operation{
		ID:        "getUser",
		Path:      routespec.Path[renamedPath](),
		Query:     routespec.Query[renamedQuery](),
		Responses: routespec.Responses{http.StatusOK: {}},
	})

	parameters := api.Document().Paths["/users/{user_id}"].Get.Parameters
	if len(parameters) != 2 {
		t.Fatalf("parameter count = %d, want 2", len(parameters))
	}
	if got, want := parameters[0].Name, "user_id"; got != want {
		t.Errorf("path parameter name = %q, want %q", got, want)
	}
	if parameters[0].In != "path" || !parameters[0].Required || parameters[0].Description != "Public user ID" {
		t.Errorf("path parameter = %#v", parameters[0])
	}
	if got, want := parameters[1].Name, "page_number"; got != want {
		t.Errorf("query parameter name = %q, want %q", got, want)
	}
	if parameters[1].In != "query" || !parameters[1].Required || parameters[1].Description != "One-based page number" {
		t.Errorf("query parameter = %#v", parameters[1])
	}

	body, err := api.JSON()
	if err != nil {
		t.Fatalf("marshal document: %v", err)
	}
	var document map[string]any
	if err := json.Unmarshal(body, &document); err != nil {
		t.Fatalf("unmarshal document: %v", err)
	}
	paths := document["paths"].(map[string]any)
	operation := paths["/users/{user_id}"].(map[string]any)["get"].(map[string]any)
	query := operation["parameters"].([]any)[1].(map[string]any)
	if got, want := query["schema"].(map[string]any)["minimum"], float64(1); got != want {
		t.Errorf("query minimum = %#v, want %#v", got, want)
	}
}

func TestOpenAPIAnnotationNameIsRejectedForJSONFields(t *testing.T) {
	tests := []struct {
		name      string
		operation routespec.Operation
	}{
		{
			name: "request body",
			operation: routespec.Operation{
				ID: "invalidRequestName",
				RequestBody: routespec.JSON[struct {
					Value string `json:"value" openapi:"name=renamed"`
				}](),
				Responses: routespec.Responses{http.StatusOK: {}},
			},
		},
		{
			name: "response body",
			operation: routespec.Operation{
				ID: "invalidResponseName",
				Responses: routespec.Responses{http.StatusOK: routespec.JSON[struct {
					Value string `json:"value" openapi:"name=renamed"`
				}]()},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			api := annotationAPI(t)
			mustAnnotationPanic(t, func() {
				api.RegisterFunc(http.MethodPost, "/invalid-name", annotationHandler, test.operation)
			})
		})
	}
}

func TestOpenAPIInvalidAnnotationsPanicDuringRegistration(t *testing.T) {
	tests := []struct {
		name     string
		register func(*routespec.API)
	}{
		{
			name: "unknown directive",
			register: func(api *routespec.API) {
				api.RegisterFunc(http.MethodPost, "/invalid-annotation", annotationHandler, routespec.Operation{
					ID:          "unknownAnnotation",
					RequestBody: routespec.JSON[unknownAnnotation](),
					Responses:   routespec.Responses{http.StatusOK: {}},
				})
			},
		},
		{
			name: "string constraint on integer",
			register: func(api *routespec.API) {
				api.RegisterFunc(http.MethodPost, "/invalid-annotation", annotationHandler, routespec.Operation{
					ID:          "stringConstraintOnInteger",
					RequestBody: routespec.JSON[stringConstraintOnInteger](),
					Responses:   routespec.Responses{http.StatusOK: {}},
				})
			},
		},
		{
			name: "numeric constraint on string",
			register: func(api *routespec.API) {
				api.RegisterFunc(http.MethodPost, "/invalid-annotation", annotationHandler, routespec.Operation{
					ID:          "numericConstraintOnString",
					RequestBody: routespec.JSON[numericConstraintOnString](),
					Responses:   routespec.Responses{http.StatusOK: {}},
				})
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mustAnnotationPanic(t, func() {
				test.register(annotationAPI(t))
			})
		})
	}
}

func annotationAPI(t *testing.T) *routespec.API {
	t.Helper()
	return routespec.New(http.NewServeMux(), routespec.Info{Title: "Annotations", Version: "1.0.0"})
}

func annotationHandler(http.ResponseWriter, *http.Request) {}

func annotationSchema(t *testing.T, api *routespec.API, name string) map[string]any {
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

func annotationProperty(t *testing.T, schema map[string]any, name string) map[string]any {
	t.Helper()
	properties := schema["properties"].(map[string]any)
	property, ok := properties[name].(map[string]any)
	if !ok {
		t.Fatalf("property %q = %#v", name, properties[name])
	}
	return property
}

func mustAnnotationPanic(t *testing.T, fn func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatal("function did not panic")
		}
	}()
	fn()
}
