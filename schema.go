package routespec

import (
	"fmt"
	"net/http"
	"reflect"
	"sort"
	"strings"
	"time"
)

var timeType = reflect.TypeFor[time.Time]()

func buildDocument(info Info, operations map[routeKey]Operation, overrides map[reflect.Type]Schema) Document {
	builder := schemaBuilder{
		components: make(map[string]Schema),
		states:     make(map[reflect.Type]schemaState),
		names:      make(map[string]reflect.Type),
		overrides:  overrides,
	}
	document := Document{
		OpenAPI: "3.1.0",
		Info: DocumentInfo{
			Title:       info.Title,
			Version:     info.Version,
			Description: info.Description,
		},
		Paths: make(map[string]PathItem),
	}

	keys := make([]routeKey, 0, len(operations))
	for key := range operations {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].path == keys[j].path {
			return keys[i].method < keys[j].method
		}
		return keys[i].path < keys[j].path
	})

	for _, key := range keys {
		pathItem := document.Paths[key.path]
		operation := builder.operation(key.path, operations[key])
		switch key.method {
		case http.MethodGet:
			pathItem.Get = &operation
		case http.MethodPut:
			pathItem.Put = &operation
		case http.MethodPost:
			pathItem.Post = &operation
		case http.MethodDelete:
			pathItem.Delete = &operation
		case http.MethodPatch:
			pathItem.Patch = &operation
		case http.MethodHead:
			pathItem.Head = &operation
		case http.MethodOptions:
			pathItem.Options = &operation
		case http.MethodTrace:
			pathItem.Trace = &operation
		default:
			panic(fmt.Sprintf("routespec: register %s %s: unsupported OpenAPI method", key.method, key.path))
		}
		document.Paths[key.path] = pathItem
	}

	if len(builder.components) > 0 {
		document.Components.Schemas = builder.components
	}
	return document
}

type schemaState uint8

const (
	schemaBuilding schemaState = iota + 1
	schemaBuilt
)

type schemaBuilder struct {
	components map[string]Schema
	states     map[reflect.Type]schemaState
	names      map[string]reflect.Type
	overrides  map[reflect.Type]Schema
}

func (builder *schemaBuilder) operation(path string, source Operation) DocumentOperation {
	operation := DocumentOperation{
		OperationID: source.ID,
		Summary:     source.Summary,
		Description: source.Description,
		Tags:        append([]string(nil), source.Tags...),
		Security:    cloneSecurity(source.Security),
		Responses:   make(map[string]Response, len(source.Responses)),
	}
	if source.RequestBody.typeOf != nil {
		operation.RequestBody = &RequestBody{
			Required: true,
			Content: map[string]MediaType{
				source.RequestBody.mediaType: {Schema: builder.schema(source.RequestBody.typeOf)},
			},
		}
	}
	operation.Parameters = append(operation.Parameters, builder.parameters(source.Path)...)
	operation.Parameters = append(operation.Parameters, builder.parameters(source.Query)...)
	for status, content := range source.Responses {
		response := Response{Description: http.StatusText(status)}
		if response.Description == "" {
			response.Description = "Response"
		}
		if content.typeOf != nil {
			response.Content = map[string]MediaType{
				content.mediaType: {Schema: builder.schema(content.typeOf)},
			}
		}
		operation.Responses[fmt.Sprint(status)] = response
	}
	return operation
}

func cloneSecurity(source []SecurityRequirement) []SecurityRequirement {
	if len(source) == 0 {
		return nil
	}
	result := make([]SecurityRequirement, len(source))
	for i, requirement := range source {
		result[i] = make(SecurityRequirement, len(requirement))
		for name, scopes := range requirement {
			result[i][name] = append([]string(nil), scopes...)
		}
	}
	return result
}

func (builder *schemaBuilder) parameters(model ParameterModel) []Parameter {
	if model.typeOf == nil {
		return nil
	}
	fields := parameterFields(deref(model.typeOf))
	names := make([]string, 0, len(fields))
	for name := range fields {
		names = append(names, name)
	}
	sort.Strings(names)

	parameters := make([]Parameter, 0, len(names))
	for _, name := range names {
		field := fields[name]
		schema := field.Annotation.apply(builder.schema(field.Type), field.Field, true)
		parameters = append(parameters, Parameter{
			Name:        name,
			In:          string(model.location),
			Description: field.Annotation.description,
			Required:    model.location == pathParameter || field.Annotation.required,
			Schema:      schema,
		})
	}
	return parameters
}

func (builder *schemaBuilder) schema(t reflect.Type) Schema {
	t = deref(t)
	if override, exists := builder.overrides[t]; exists {
		return override
	}
	if t == timeType {
		return Schema{Type: "string", Format: "date-time"}
	}
	if t.Kind() == reflect.Struct && t.Name() != "" {
		name := componentName(t)
		if existing, exists := builder.names[name]; exists && existing != t {
			panic(fmt.Sprintf("routespec: schema component name %q is shared by %s and %s", name, existing, t))
		}
		builder.names[name] = t
		if builder.states[t] == schemaBuilt || builder.states[t] == schemaBuilding {
			return Schema{Ref: "#/components/schemas/" + name}
		}
		builder.states[t] = schemaBuilding
		builder.components[name] = Schema{}
		builder.components[name] = builder.objectSchema(t)
		builder.states[t] = schemaBuilt
		return Schema{Ref: "#/components/schemas/" + name}
	}

	switch t.Kind() {
	case reflect.Bool:
		return Schema{Type: "boolean"}
	case reflect.String:
		return Schema{Type: "string"}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return Schema{Type: "integer", Format: integerFormat(t)}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return Schema{Type: "integer", Format: integerFormat(t)}
	case reflect.Float32:
		return Schema{Type: "number", Format: "float"}
	case reflect.Float64:
		return Schema{Type: "number", Format: "double"}
	case reflect.Slice:
		if t.Elem().Kind() == reflect.Uint8 {
			return Schema{Type: "string", Format: "byte"}
		}
		items := builder.schema(t.Elem())
		return Schema{Type: "array", Items: &items}
	case reflect.Array:
		items := builder.schema(t.Elem())
		return Schema{Type: "array", Items: &items}
	case reflect.Map:
		if t.Key().Kind() != reflect.String {
			panic(fmt.Sprintf("routespec: unsupported map key type %s", t.Key()))
		}
		value := builder.schema(t.Elem())
		return Schema{Type: "object", AdditionalProperties: &value}
	case reflect.Interface:
		return Schema{}
	default:
		panic(fmt.Sprintf("routespec: unsupported schema type %s", t))
	}
}

func (builder *schemaBuilder) objectSchema(t reflect.Type) Schema {
	fields := jsonFields(t)
	properties := make(map[string]Schema, len(fields))
	required := make([]string, 0)
	for name, field := range fields {
		properties[name] = field.Annotation.apply(builder.schema(field.Type), field.Field, false)
		if field.Annotation.required {
			required = append(required, name)
		}
	}
	sort.Strings(required)
	return Schema{Type: "object", Properties: properties, Required: required}
}

func componentName(t reflect.Type) string {
	return t.Name()
}

func integerFormat(t reflect.Type) string {
	if t.Bits() <= 32 {
		return "int32"
	}
	return "int64"
}

func deref(t reflect.Type) reflect.Type {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t
}

type jsonField struct {
	Field      reflect.StructField
	Type       reflect.Type
	Annotation annotation
}

func jsonFields(t reflect.Type) map[string]jsonField {
	if t.Kind() != reflect.Struct {
		panic(fmt.Sprintf("routespec: expected struct, got %s", t))
	}
	fields := make(map[string]jsonField)
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}
		name, _ := jsonName(field)
		if name == "-" {
			continue
		}
		if name == "" {
			name = field.Name
		}
		if _, exists := fields[name]; exists {
			panic(fmt.Sprintf("routespec: duplicate JSON field name %q in %s", name, t))
		}
		fields[name] = jsonField{
			Field:      field,
			Type:       field.Type,
			Annotation: parseAnnotation(field),
		}
	}
	return fields
}

func parameterFields(t reflect.Type) map[string]jsonField {
	fields := jsonFields(t)
	parameters := make(map[string]jsonField, len(fields))
	for jsonName, field := range fields {
		name := jsonName
		if field.Annotation.name != "" {
			name = field.Annotation.name
		}
		if _, exists := parameters[name]; exists {
			panic(fmt.Sprintf("routespec: duplicate parameter name %q in %s", name, t))
		}
		parameters[name] = field
	}
	return parameters
}

func jsonName(field reflect.StructField) (string, []string) {
	tag := field.Tag.Get("json")
	if tag == "" {
		return "", nil
	}
	parts := strings.Split(tag, ",")
	return parts[0], parts[1:]
}

func serveMuxPathParameters(path string) []string {
	parts := strings.Split(path, "/")
	parameters := make([]string, 0)
	for _, part := range parts {
		if strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") {
			name := strings.TrimSuffix(strings.TrimPrefix(part, "{"), "}")
			if name == "$" || strings.HasSuffix(name, "...") {
				panic(fmt.Sprintf("routespec: unsupported ServeMux path wildcard %q", part))
			}
			parameters = append(parameters, name)
		}
	}
	return parameters
}
