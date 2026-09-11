package routespec

import (
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"
)

type annotation struct {
	required    bool
	name        string
	description string
	format      string
	pattern     string
	readOnly    bool
	writeOnly   bool
	deprecated  bool

	minLength        *int
	maxLength        *int
	minimum          *float64
	maximum          *float64
	exclusiveMinimum *float64
	exclusiveMaximum *float64
	multipleOf       *float64
	minItems         *int
	maxItems         *int
	uniqueItems      bool
	minProperties    *int
	maxProperties    *int
	closed           bool

	enum       []string
	constRaw   *string
	defaultRaw *string
	exampleRaw *string
}

func parseAnnotation(field reflect.StructField) annotation {
	raw, exists := field.Tag.Lookup("openapi")
	if !exists || raw == "" {
		return annotation{}
	}

	result := annotation{}
	seen := make(map[string]bool)
	for _, directive := range splitTag(raw, ',') {
		key, value, hasValue := splitDirective(directive)
		if key == "" {
			annotationPanic(field, "empty directive")
		}
		if seen[key] {
			annotationPanic(field, fmt.Sprintf("duplicate directive %q", key))
		}
		seen[key] = true

		switch key {
		case "required":
			requireNoValue(field, key, hasValue)
			result.required = true
		case "readOnly":
			requireNoValue(field, key, hasValue)
			result.readOnly = true
		case "writeOnly":
			requireNoValue(field, key, hasValue)
			result.writeOnly = true
		case "deprecated":
			requireNoValue(field, key, hasValue)
			result.deprecated = true
		case "name":
			result.name = requireValue(field, key, value, hasValue)
		case "description":
			result.description = requireValue(field, key, value, hasValue)
		case "format":
			result.format = requireValue(field, key, value, hasValue)
		case "pattern":
			result.pattern = requireValue(field, key, value, hasValue)
		case "minLength":
			result.minLength = annotationInt(field, key, value, hasValue)
		case "maxLength":
			result.maxLength = annotationInt(field, key, value, hasValue)
		case "minimum":
			result.minimum = annotationFloat(field, key, value, hasValue)
		case "maximum":
			result.maximum = annotationFloat(field, key, value, hasValue)
		case "exclusiveMinimum":
			result.exclusiveMinimum = annotationFloat(field, key, value, hasValue)
		case "exclusiveMaximum":
			result.exclusiveMaximum = annotationFloat(field, key, value, hasValue)
		case "multipleOf":
			result.multipleOf = annotationFloat(field, key, value, hasValue)
		case "minItems":
			result.minItems = annotationInt(field, key, value, hasValue)
		case "maxItems":
			result.maxItems = annotationInt(field, key, value, hasValue)
		case "uniqueItems":
			requireNoValue(field, key, hasValue)
			result.uniqueItems = true
		case "minProperties":
			result.minProperties = annotationInt(field, key, value, hasValue)
		case "maxProperties":
			result.maxProperties = annotationInt(field, key, value, hasValue)
		case "additionalProperties":
			if requireValue(field, key, value, hasValue) != "false" {
				annotationPanic(field, "additionalProperties only supports false")
			}
			result.closed = true
		case "enum":
			result.enum = splitTag(requireValue(field, key, value, hasValue), '|')
		case "const":
			constValue := requireValue(field, key, value, hasValue)
			result.constRaw = &constValue
		case "default":
			defaultValue := requireValue(field, key, value, hasValue)
			result.defaultRaw = &defaultValue
		case "example":
			exampleValue := requireValue(field, key, value, hasValue)
			result.exampleRaw = &exampleValue
		default:
			annotationPanic(field, fmt.Sprintf("unsupported directive %q", key))
		}
	}
	if result.readOnly && result.writeOnly {
		annotationPanic(field, "readOnly and writeOnly cannot be combined")
	}
	return result
}

func requireNoValue(field reflect.StructField, key string, hasValue bool) {
	if hasValue {
		annotationPanic(field, fmt.Sprintf("%s does not accept a value", key))
	}
}

func requireValue(field reflect.StructField, key, value string, hasValue bool) string {
	if !hasValue || value == "" {
		annotationPanic(field, fmt.Sprintf("%s requires a value", key))
	}
	return value
}

func annotationInt(field reflect.StructField, key, value string, hasValue bool) *int {
	parsed, err := strconv.Atoi(requireValue(field, key, value, hasValue))
	if err != nil || parsed < 0 {
		annotationPanic(field, fmt.Sprintf("%s must be a non-negative integer", key))
	}
	return &parsed
}

func annotationFloat(field reflect.StructField, key, value string, hasValue bool) *float64 {
	parsed, err := strconv.ParseFloat(requireValue(field, key, value, hasValue), 64)
	if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
		annotationPanic(field, fmt.Sprintf("%s must be a finite number", key))
	}
	if key == "multipleOf" && parsed <= 0 {
		annotationPanic(field, "multipleOf must be greater than zero")
	}
	return &parsed
}

func annotationPanic(field reflect.StructField, message string) {
	panic(fmt.Sprintf("routespec: invalid openapi tag on %s.%s: %s", field.Type.PkgPath(), field.Name, message))
}

func splitDirective(directive string) (string, string, bool) {
	for index := 0; index < len(directive); index++ {
		if directive[index] == '\\' {
			index++
			continue
		}
		if directive[index] == '=' {
			return directive[:index], unescapeTagValue(directive[index+1:]), true
		}
	}
	return directive, "", false
}

func splitTag(value string, separator byte) []string {
	parts := make([]string, 0, 1)
	var part strings.Builder
	for index := 0; index < len(value); index++ {
		character := value[index]
		if character == '\\' && index+1 < len(value) {
			next := value[index+1]
			if next == separator || next == '\\' || next == '=' || next == '|' {
				part.WriteByte(next)
				index++
				continue
			}
		}
		if character == separator {
			parts = append(parts, part.String())
			part.Reset()
			continue
		}
		part.WriteByte(character)
	}
	parts = append(parts, part.String())
	return parts
}

func unescapeTagValue(value string) string {
	var result strings.Builder
	for index := 0; index < len(value); index++ {
		if value[index] == '\\' && index+1 < len(value) {
			next := value[index+1]
			if next == ',' || next == '\\' || next == '=' || next == '|' {
				result.WriteByte(next)
				index++
				continue
			}
		}
		result.WriteByte(value[index])
	}
	return result.String()
}

func (annotation annotation) hasTypeSpecificValues() bool {
	return annotation.minLength != nil || annotation.maxLength != nil || annotation.pattern != "" ||
		annotation.minimum != nil || annotation.maximum != nil || annotation.exclusiveMinimum != nil || annotation.exclusiveMaximum != nil || annotation.multipleOf != nil ||
		annotation.minItems != nil || annotation.maxItems != nil || annotation.uniqueItems ||
		annotation.minProperties != nil || annotation.maxProperties != nil || annotation.closed ||
		len(annotation.enum) > 0 || annotation.constRaw != nil || annotation.defaultRaw != nil || annotation.exampleRaw != nil
}

func (annotation annotation) apply(schema Schema, field reflect.StructField, allowName bool) Schema {
	kind := deref(field.Type).Kind()
	if annotation.name != "" && !allowName {
		annotationPanic(field, "name is only valid on a Path or Query model")
	}
	if annotation.minLength != nil || annotation.maxLength != nil || annotation.pattern != "" {
		if kind != reflect.String {
			annotationPanic(field, "string constraints require a string field")
		}
	}
	if annotation.minimum != nil || annotation.maximum != nil || annotation.exclusiveMinimum != nil || annotation.exclusiveMaximum != nil || annotation.multipleOf != nil {
		if !isNumber(kind) {
			annotationPanic(field, "numeric constraints require a numeric field")
		}
	}
	if annotation.minItems != nil || annotation.maxItems != nil || annotation.uniqueItems {
		if kind != reflect.Slice && kind != reflect.Array {
			annotationPanic(field, "item constraints require an array or slice field")
		}
	}
	if annotation.minProperties != nil || annotation.maxProperties != nil || annotation.closed {
		if kind != reflect.Map && kind != reflect.Struct {
			annotationPanic(field, "property constraints require an object field")
		}
	}
	validateAnnotationRanges(field, annotation)

	if annotation.description != "" {
		schema.Description = annotation.description
	}
	if annotation.format != "" {
		schema.Format = annotation.format
	}
	if annotation.readOnly {
		schema.ReadOnly = true
	}
	if annotation.writeOnly {
		schema.WriteOnly = true
	}
	if annotation.deprecated {
		schema.Deprecated = true
	}
	if annotation.minLength != nil {
		schema.MinLength = annotation.minLength
	}
	if annotation.maxLength != nil {
		schema.MaxLength = annotation.maxLength
	}
	if annotation.pattern != "" {
		schema.Pattern = annotation.pattern
	}
	if annotation.minimum != nil {
		schema.Minimum = annotation.minimum
	}
	if annotation.maximum != nil {
		schema.Maximum = annotation.maximum
	}
	if annotation.exclusiveMinimum != nil {
		schema.ExclusiveMinimum = annotation.exclusiveMinimum
	}
	if annotation.exclusiveMaximum != nil {
		schema.ExclusiveMaximum = annotation.exclusiveMaximum
	}
	if annotation.multipleOf != nil {
		schema.MultipleOf = annotation.multipleOf
	}
	if annotation.minItems != nil {
		schema.MinItems = annotation.minItems
	}
	if annotation.maxItems != nil {
		schema.MaxItems = annotation.maxItems
	}
	if annotation.uniqueItems {
		schema.UniqueItems = true
	}
	if annotation.minProperties != nil {
		schema.MinProperties = annotation.minProperties
	}
	if annotation.maxProperties != nil {
		schema.MaxProperties = annotation.maxProperties
	}
	if annotation.closed {
		schema.Closed = true
	}

	if len(annotation.enum) > 0 {
		schema.Enum = make([]any, len(annotation.enum))
		for index, raw := range annotation.enum {
			schema.Enum[index] = parseTaggedValue(field, raw)
		}
	}
	if annotation.constRaw != nil {
		schema.Const = parseTaggedValue(field, *annotation.constRaw)
	}
	if annotation.defaultRaw != nil {
		schema.Default = parseTaggedValue(field, *annotation.defaultRaw)
	}
	if annotation.exampleRaw != nil {
		schema.Example = parseTaggedValue(field, *annotation.exampleRaw)
	}
	return schema
}

func validateAnnotationRanges(field reflect.StructField, annotation annotation) {
	for _, limits := range [][2]*int{
		{annotation.minLength, annotation.maxLength},
		{annotation.minItems, annotation.maxItems},
		{annotation.minProperties, annotation.maxProperties},
	} {
		if limits[0] != nil && limits[1] != nil && *limits[0] > *limits[1] {
			annotationPanic(field, "minimum constraint exceeds maximum constraint")
		}
	}
	for _, limits := range [][2]*float64{
		{annotation.minimum, annotation.maximum},
		{annotation.exclusiveMinimum, annotation.exclusiveMaximum},
	} {
		if limits[0] != nil && limits[1] != nil && *limits[0] > *limits[1] {
			annotationPanic(field, "minimum constraint exceeds maximum constraint")
		}
	}
}

func isNumber(kind reflect.Kind) bool {
	switch kind {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return true
	default:
		return false
	}
}

func parseTaggedValue(field reflect.StructField, raw string) any {
	t := deref(field.Type)
	value := reflect.New(t).Elem()
	switch t.Kind() {
	case reflect.String:
		value.SetString(raw)
	case reflect.Bool:
		parsed, err := strconv.ParseBool(raw)
		if err != nil {
			annotationPanic(field, fmt.Sprintf("%q is not a boolean", raw))
		}
		value.SetBool(parsed)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		parsed, err := strconv.ParseInt(raw, 10, t.Bits())
		if err != nil {
			annotationPanic(field, fmt.Sprintf("%q is not an integer", raw))
		}
		value.SetInt(parsed)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		parsed, err := strconv.ParseUint(raw, 10, t.Bits())
		if err != nil {
			annotationPanic(field, fmt.Sprintf("%q is not an unsigned integer", raw))
		}
		value.SetUint(parsed)
	case reflect.Float32, reflect.Float64:
		parsed, err := strconv.ParseFloat(raw, t.Bits())
		if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
			annotationPanic(field, fmt.Sprintf("%q is not a finite number", raw))
		}
		value.SetFloat(parsed)
	default:
		annotationPanic(field, "enum, default, and example require a scalar field")
	}
	return value.Interface()
}
