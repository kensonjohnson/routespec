package routespec

import (
	"encoding"
	json "encoding/json/v2"
	"fmt"
	"net/http"
	"reflect"
	"sort"
	"strings"
	"time"
	"unicode"
)

var (
	timeType            = reflect.TypeFor[time.Time]()
	jsonMarshalerType   = reflect.TypeFor[json.Marshaler]()
	jsonMarshalerToType = reflect.TypeFor[json.MarshalerTo]()
	textMarshalerType   = reflect.TypeFor[encoding.TextMarshaler]()
)

// buildDocument materializes the current explicit route declarations.
func buildDocument(info Info, operations map[routeKey]Operation, overrides map[reflect.Type]Schema) Document {
	builder := schemaBuilder{
		components: make(map[string]Schema),
		states:     make(map[reflect.Type]schemaState),
		names:      make(map[string]reflect.Type),
		overrides:  overrides,
	}
	validateSecurityRequirements(info, operations)
	document := Document{
		OpenAPI: "3.1.0",
		Info: DocumentInfo{
			Title:          info.Title,
			Version:        info.Version,
			Description:    info.Description,
			TermsOfService: info.TermsOfService,
			Contact:        cloneContact(info.Contact),
			License:        cloneLicense(info.License),
		},
		Servers:      cloneServers(info.Servers),
		Tags:         cloneTags(info.Tags),
		ExternalDocs: cloneExternalDocs(info.ExternalDocs),
		Paths:        make(map[string]PathItem),
		Components: Components{
			SecuritySchemes: cloneSecuritySchemes(info.SecuritySchemes),
		},
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

func validateSchemaCompositions(document Document) {
	components := document.Components.Schemas
	var validateSchema func(Schema)
	validateSchema = func(schema Schema) {
		validateCompositionAlternatives("allOf", schema.AllOf, validateSchema)
		validateCompositionAlternatives("anyOf", schema.AnyOf, validateSchema)
		validateCompositionAlternatives("oneOf", schema.OneOf, validateSchema)
		if schema.Not != nil {
			validateSchema(*schema.Not)
		}
		for _, property := range schema.Properties {
			validateSchema(property)
		}
		if schema.Items != nil {
			validateSchema(*schema.Items)
		}
		if schema.AdditionalProperties != nil {
			validateSchema(*schema.AdditionalProperties)
		}
		if schema.Discriminator != nil {
			validateDiscriminator(schema, components)
		}
	}

	for _, schema := range components {
		validateSchema(schema)
	}
	for _, pathItem := range document.Paths {
		for _, operation := range []*DocumentOperation{pathItem.Get, pathItem.Put, pathItem.Post, pathItem.Delete, pathItem.Patch, pathItem.Head, pathItem.Options, pathItem.Trace} {
			if operation == nil {
				continue
			}
			for _, parameter := range operation.Parameters {
				validateSchema(parameter.Schema)
			}
			if operation.RequestBody != nil {
				for _, content := range operation.RequestBody.Content {
					validateSchema(content.Schema)
				}
			}
			for _, response := range operation.Responses {
				for _, content := range response.Content {
					validateSchema(content.Schema)
				}
				for _, header := range response.Headers {
					validateSchema(header.Schema)
				}
			}
		}
	}
}

func validateCompositionAlternatives(name string, alternatives []Schema, validate func(Schema)) {
	if alternatives == nil {
		return
	}
	if len(alternatives) == 0 {
		panic(fmt.Sprintf("routespec: %s requires at least one schema", name))
	}
	for _, alternative := range alternatives {
		validate(alternative)
	}
}

func validateDiscriminator(schema Schema, components map[string]Schema) {
	discriminator := schema.Discriminator
	if discriminator.PropertyName == "" {
		panic("routespec: discriminator propertyName is required")
	}
	if len(schema.AllOf) == 0 && len(schema.AnyOf) == 0 && len(schema.OneOf) == 0 {
		panic("routespec: discriminator requires allOf, anyOf, or oneOf")
	}
	for value, reference := range discriminator.Mapping {
		if value == "" || reference == "" {
			panic("routespec: discriminator mappings require non-empty values and references")
		}
	}
	if !schemaRequiresProperty(schema, discriminator.PropertyName, components, make(map[string]bool)) {
		panic(fmt.Sprintf("routespec: discriminator property %q must be required", discriminator.PropertyName))
	}
}

func schemaRequiresProperty(schema Schema, property string, components map[string]Schema, visited map[string]bool) bool {
	for _, required := range schema.Required {
		if required == property {
			return true
		}
	}
	if strings.HasPrefix(schema.Ref, "#/components/schemas/") {
		name := strings.TrimPrefix(schema.Ref, "#/components/schemas/")
		if visited[name] {
			return false
		}
		if component, exists := components[name]; exists {
			visited[name] = true
			return schemaRequiresProperty(component, property, components, visited)
		}
	}
	for _, alternative := range schema.AllOf {
		if schemaRequiresProperty(alternative, property, components, visited) {
			return true
		}
	}
	return false
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
	if source.RequestBody.Content != nil {
		operation.RequestBody = &RequestBody{
			Description: source.RequestBody.Description,
			Required:    source.RequestBody.Required,
			Content:     builder.content(source.RequestBody.Content),
		}
	}
	operation.Parameters = append(operation.Parameters, builder.parameters(source.Path)...)
	operation.Parameters = append(operation.Parameters, builder.parameters(source.Query)...)
	operation.Parameters = append(operation.Parameters, builder.parameters(source.Header)...)
	operation.Parameters = append(operation.Parameters, builder.parameters(source.Cookie)...)
	for status, sourceResponse := range source.Responses {
		response := Response{
			Description: sourceResponse.Description,
			Content:     builder.content(sourceResponse.Content),
			Headers:     builder.headers(sourceResponse.Headers),
			Links:       documentLinks(sourceResponse.Links),
		}
		if response.Description == "" {
			response.Description = http.StatusText(status)
		}
		if response.Description == "" {
			response.Description = "Response"
		}
		operation.Responses[fmt.Sprint(status)] = response
	}
	return operation
}

func (builder *schemaBuilder) content(contents []Content) map[string]MediaType {
	if len(contents) == 0 {
		return nil
	}
	result := make(map[string]MediaType, len(contents))
	for _, content := range contents {
		schema := builder.schema(content.typeOf)
		if content.binary {
			schema = Schema{Type: "string", Format: "binary"}
		}
		result[content.mediaType] = MediaType{Schema: schema}
	}
	return result
}

func (builder *schemaBuilder) headers(headers map[string]HeaderSpec) map[string]DocumentHeader {
	if len(headers) == 0 {
		return nil
	}
	result := make(map[string]DocumentHeader, len(headers))
	for name, header := range headers {
		schema := cloneSchema(header.Schema)
		if header.typeOf != nil {
			schema = builder.schema(header.typeOf)
		}
		result[name] = DocumentHeader{
			Description: header.Description,
			Required:    header.Required,
			Deprecated:  header.Deprecated,
			Schema:      schema,
		}
	}
	return result
}

func documentLinks(links map[string]LinkSpec) map[string]Link {
	if len(links) == 0 {
		return nil
	}
	result := make(map[string]Link, len(links))
	for name, link := range links {
		parameters := make(map[string]any, len(link.Parameters))
		for parameter, value := range link.Parameters {
			parameters[parameter] = cloneSchemaValue(value)
		}
		result[name] = Link{
			OperationID:  link.OperationID,
			OperationRef: link.OperationRef,
			Description:  link.Description,
			Parameters:   parameters,
			RequestBody:  cloneSchemaValue(link.RequestBody),
		}
	}
	return result
}

func validateSecurityRequirements(info Info, operations map[routeKey]Operation) {
	for key, operation := range operations {
		for _, requirement := range operation.Security {
			for name, scopes := range requirement {
				scheme, exists := info.SecuritySchemes[name]
				if !exists {
					panic(fmt.Sprintf("routespec: register %s %s: security scheme %q is not configured", key.method, key.path, name))
				}
				if scheme.Type != "oauth2" {
					if len(scopes) > 0 {
						panic(fmt.Sprintf("routespec: register %s %s: security scheme %q does not support scopes", key.method, key.path, name))
					}
					continue
				}
				available := oauthScopes(scheme.Flows)
				for _, scope := range scopes {
					if _, exists := available[scope]; !exists {
						panic(fmt.Sprintf("routespec: register %s %s: security scheme %q does not define scope %q", key.method, key.path, name, scope))
					}
				}
			}
		}
	}
}

func oauthScopes(flows *OAuthFlows) map[string]struct{} {
	scopes := make(map[string]struct{})
	if flows == nil {
		return scopes
	}
	for _, flow := range []*OAuthFlow{flows.Implicit, flows.Password, flows.ClientCredentials, flows.AuthorizationCode} {
		if flow == nil {
			continue
		}
		for scope := range flow.Scopes {
			scopes[scope] = struct{}{}
		}
	}
	return scopes
}

func cloneSecuritySchemes(source map[string]SecurityScheme) map[string]SecurityScheme {
	if len(source) == 0 {
		return nil
	}
	result := make(map[string]SecurityScheme, len(source))
	for name, scheme := range source {
		result[name] = cloneSecurityScheme(scheme)
	}
	return result
}

func cloneSecurity(source []SecurityRequirement) []SecurityRequirement {
	if len(source) == 0 {
		return nil
	}
	result := make([]SecurityRequirement, len(source))
	for i, requirement := range source {
		result[i] = make(SecurityRequirement, len(requirement))
		for name, scopes := range requirement {
			result[i][name] = make([]string, len(scopes))
			copy(result[i][name], scopes)
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
		schema := builder.fieldSchema(field, true)
		parameters = append(parameters, Parameter{
			Name:          name,
			In:            string(model.location),
			Description:   field.Annotation.description,
			Required:      model.location == pathParameter || field.Annotation.required,
			Style:         model.style,
			Explode:       cloneBool(model.explode),
			AllowReserved: model.allowReserved,
			Schema:        schema,
		})
	}
	return parameters
}

func cloneBool(value *bool) *bool {
	if value == nil {
		return nil
	}
	clone := *value
	return &clone
}

func (builder *schemaBuilder) fieldSchema(field jsonField, allowName bool) Schema {
	schema := builder.schema(field.Type)
	if field.Stringified {
		if !isNumber(deref(field.Type).Kind()) {
			annotationPanic(field.Field, "json string requires a numeric field")
		}
		if field.Annotation.hasTypeSpecificValues() {
			annotationPanic(field.Field, "json string cannot be combined with schema constraints or values")
		}
		schema.Ref = ""
		schema.Type = "string"
		schema.Format = ""
	}
	if field.OmitNull {
		schema.Nullable = false
	}
	return field.Annotation.apply(schema, field.Field, allowName)
}

func (builder *schemaBuilder) schema(t reflect.Type) Schema {
	nullable := false
	for t.Kind() == reflect.Pointer {
		nullable = true
		t = t.Elem()
	}
	if _, overridden := builder.overrides[t]; !overridden && t != timeType && hasCustomJSONEncoding(t) {
		panic(fmt.Sprintf("routespec: schema type %s has custom JSON encoding; use WithSchemaOverride", t))
	}
	schema := builder.schemaValue(t)
	schema.Nullable = schema.Nullable || nullable
	return schema
}

func hasCustomJSONEncoding(t reflect.Type) bool {
	for _, candidate := range []reflect.Type{t, reflect.PointerTo(t)} {
		if candidate.Implements(jsonMarshalerType) || candidate.Implements(jsonMarshalerToType) || candidate.Implements(textMarshalerType) {
			return true
		}
	}
	return false
}

func (builder *schemaBuilder) schemaValue(t reflect.Type) Schema {
	if override, exists := builder.overrides[t]; exists {
		return cloneSchema(override)
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
	fieldSet := jsonFields(t)
	properties := make(map[string]Schema, len(fieldSet.properties))
	required := make([]string, 0)
	for name, field := range fieldSet.properties {
		properties[name] = builder.fieldSchema(field, false)
		if field.Annotation.required {
			required = append(required, name)
		}
	}
	var additionalProperties *Schema
	if fieldSet.fallback != nil {
		fallback := builder.fieldSchema(*fieldSet.fallback, false)
		additionalProperties = fallback.AdditionalProperties
	}
	sort.Strings(required)
	return Schema{Type: "object", Properties: properties, Required: required, AdditionalProperties: additionalProperties}
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
	Field       reflect.StructField
	Type        reflect.Type
	Annotation  annotation
	Stringified bool
	OmitNull    bool
}

type jsonFieldCandidate struct {
	field       reflect.StructField
	name        string
	tagged      bool
	index       []int
	stringified bool
	omitNull    bool
}

type jsonTag struct {
	name        string
	tagged      bool
	ignored     bool
	embedded    bool
	stringified bool
	omitNull    bool
}

type jsonFieldLevel struct {
	typeOf    reflect.Type
	index     []int
	ancestors map[reflect.Type]bool
}

type jsonFieldSet struct {
	properties map[string]jsonField
	fallback   *jsonField
}

func jsonFields(t reflect.Type) jsonFieldSet {
	if t.Kind() != reflect.Struct {
		panic(fmt.Sprintf("routespec: expected struct, got %s", t))
	}

	candidates := make([]jsonFieldCandidate, 0)
	current := []jsonFieldLevel{{typeOf: t, ancestors: map[reflect.Type]bool{t: true}}}
	var fallback *jsonField
	for len(current) > 0 {
		next := make([]jsonFieldLevel, 0)
		for _, level := range current {
			for i := 0; i < level.typeOf.NumField(); i++ {
				field := level.typeOf.Field(i)
				if !field.IsExported() {
					if tag := field.Tag.Get("json"); tag != "" && tag != "-" {
						panic(fmt.Sprintf("routespec: unexported field %s in %s has a json tag", field.Name, level.typeOf))
					}
					continue
				}
				fieldType := deref(field.Type)

				tag := parseJSONTag(field)
				if tag.ignored {
					continue
				}
				index := append(append([]int(nil), level.index...), i)
				if tag.embedded {
					switch fieldType.Kind() {
					case reflect.Struct:
						if level.ancestors[fieldType] {
							continue
						}
						ancestors := make(map[reflect.Type]bool, len(level.ancestors)+1)
						for ancestor := range level.ancestors {
							ancestors[ancestor] = true
						}
						ancestors[fieldType] = true
						next = append(next, jsonFieldLevel{typeOf: fieldType, index: index, ancestors: ancestors})
						continue
					case reflect.Map:
						if fieldType.Key().Kind() != reflect.String {
							panic(fmt.Sprintf("routespec: json embed map on %s.%s requires string keys", level.typeOf, field.Name))
						}
						if fallback != nil {
							panic(fmt.Sprintf("routespec: %s has multiple json embed map fallbacks", t))
						}
						field := jsonField{Field: field, Type: field.Type, Annotation: parseAnnotation(field)}
						fallback = &field
						continue
					default:
						panic(fmt.Sprintf("routespec: json embed on %s.%s requires a struct or map", level.typeOf, field.Name))
					}
				}
				if tag.name == "" && field.Anonymous {
					if fieldType.Kind() != reflect.Struct {
						panic(fmt.Sprintf("routespec: anonymous JSON field %s.%s requires a struct", level.typeOf, field.Name))
					}
					if level.ancestors[fieldType] {
						continue
					}
					ancestors := make(map[reflect.Type]bool, len(level.ancestors)+1)
					for ancestor := range level.ancestors {
						ancestors[ancestor] = true
					}
					ancestors[fieldType] = true
					next = append(next, jsonFieldLevel{typeOf: fieldType, index: index, ancestors: ancestors})
					continue
				}
				if tag.name == "" {
					tag.name = field.Name
				}
				candidates = append(candidates, jsonFieldCandidate{
					field:       field,
					name:        tag.name,
					tagged:      tag.tagged,
					index:       index,
					stringified: tag.stringified,
					omitNull:    tag.omitNull,
				})
			}
		}
		current = next
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].name != candidates[j].name {
			return candidates[i].name < candidates[j].name
		}
		if len(candidates[i].index) != len(candidates[j].index) {
			return len(candidates[i].index) < len(candidates[j].index)
		}
		if candidates[i].tagged != candidates[j].tagged {
			return candidates[i].tagged
		}
		for index := range candidates[i].index {
			if candidates[i].index[index] != candidates[j].index[index] {
				return candidates[i].index[index] < candidates[j].index[index]
			}
		}
		return false
	})

	fields := make(map[string]jsonField)
	for start := 0; start < len(candidates); {
		end := start + 1
		for end < len(candidates) && candidates[end].name == candidates[start].name {
			end++
		}
		candidate, ok := dominantJSONField(candidates[start:end])
		if ok {
			fields[candidate.name] = jsonField{
				Field:       candidate.field,
				Type:        candidate.field.Type,
				Annotation:  parseAnnotation(candidate.field),
				Stringified: candidate.stringified,
				OmitNull:    candidate.omitNull,
			}
		}
		start = end
	}
	if len(fields) == 0 && fallback == nil && t.NumField() > 0 {
		panic(fmt.Sprintf("routespec: %s has no JSON-representable fields", t))
	}
	return jsonFieldSet{properties: fields, fallback: fallback}
}

func dominantJSONField(candidates []jsonFieldCandidate) (jsonFieldCandidate, bool) {
	if len(candidates) == 0 {
		return jsonFieldCandidate{}, false
	}
	if len(candidates) > 1 && len(candidates[0].index) == len(candidates[1].index) && candidates[0].tagged == candidates[1].tagged {
		return jsonFieldCandidate{}, false
	}
	return candidates[0], true
}

func parseJSONTag(field reflect.StructField) jsonTag {
	raw := field.Tag.Get("json")
	if raw == "-" {
		return jsonTag{ignored: true}
	}
	if raw == "" {
		return jsonTag{}
	}

	parts := strings.Split(raw, ",")
	tag := jsonTag{name: parts[0]}
	if !isValidJSONTag(tag.name) {
		tag.name = ""
	}
	tag.tagged = tag.name != ""
	for _, option := range parts[1:] {
		switch {
		case option == "omitempty" || option == "omitzero":
			tag.omitNull = true
		case option == "string":
			tag.stringified = true
		case option == "embed":
			tag.embedded = true
		case option == "case:ignore" || option == "case:strict":
			// Name matching only affects unmarshaling.
		case strings.HasPrefix(option, "format:") && len(option) > len("format:"):
			// Format affects runtime marshaling, not field selection.
		default:
			panic(fmt.Sprintf("routespec: unsupported json tag option %q on %s.%s", option, field.Type, field.Name))
		}
	}
	if tag.embedded && (tag.name != "" || len(parts) != 2) {
		panic(fmt.Sprintf("routespec: json embed on %s.%s cannot be combined with a name or other options", field.Type, field.Name))
	}
	return tag
}

func isValidJSONTag(name string) bool {
	if name == "" {
		return false
	}
	for _, character := range name {
		if strings.ContainsRune("!#$%&()*+-./:<=>?@^_|~ ", character) {
			continue
		}
		if !unicode.IsLetter(character) && !unicode.IsDigit(character) {
			return false
		}
	}
	return true
}

func parameterFields(t reflect.Type) map[string]jsonField {
	fieldSet := jsonFields(t)
	if fieldSet.fallback != nil {
		panic(fmt.Sprintf("routespec: parameter model %s cannot use a json embed map fallback", t))
	}
	parameters := make(map[string]jsonField, len(fieldSet.properties))
	for jsonName, field := range fieldSet.properties {
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
