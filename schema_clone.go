package routespec

import "reflect"

func cloneSchema(schema Schema) Schema {
	clone := schema
	clone.MinLength = clonePointer(schema.MinLength)
	clone.MaxLength = clonePointer(schema.MaxLength)
	clone.Minimum = clonePointer(schema.Minimum)
	clone.Maximum = clonePointer(schema.Maximum)
	clone.ExclusiveMinimum = clonePointer(schema.ExclusiveMinimum)
	clone.ExclusiveMaximum = clonePointer(schema.ExclusiveMaximum)
	clone.MultipleOf = clonePointer(schema.MultipleOf)
	clone.MinItems = clonePointer(schema.MinItems)
	clone.MaxItems = clonePointer(schema.MaxItems)
	clone.MinProperties = clonePointer(schema.MinProperties)
	clone.MaxProperties = clonePointer(schema.MaxProperties)
	clone.Enum = cloneSchemaValues(schema.Enum)
	clone.Const = cloneSchemaValue(schema.Const)
	clone.Default = cloneSchemaValue(schema.Default)
	clone.Example = cloneSchemaValue(schema.Example)
	clone.AllOf = cloneSchemas(schema.AllOf)
	clone.AnyOf = cloneSchemas(schema.AnyOf)
	clone.OneOf = cloneSchemas(schema.OneOf)
	if schema.Not != nil {
		not := cloneSchema(*schema.Not)
		clone.Not = &not
	}
	if schema.Discriminator != nil {
		clone.Discriminator = &Discriminator{
			PropertyName: schema.Discriminator.PropertyName,
			Mapping:      cloneStringMap(schema.Discriminator.Mapping),
		}
	}
	clone.Properties = cloneSchemaProperties(schema.Properties)
	clone.Required = append([]string(nil), schema.Required...)
	if schema.Items != nil {
		items := cloneSchema(*schema.Items)
		clone.Items = &items
	}
	if schema.AdditionalProperties != nil {
		additionalProperties := cloneSchema(*schema.AdditionalProperties)
		clone.AdditionalProperties = &additionalProperties
	}
	return clone
}

func clonePointer[T any](value *T) *T {
	if value == nil {
		return nil
	}
	clone := *value
	return &clone
}

func cloneSchemas(schemas []Schema) []Schema {
	if schemas == nil {
		return nil
	}
	clone := make([]Schema, len(schemas))
	for index, schema := range schemas {
		clone[index] = cloneSchema(schema)
	}
	return clone
}

func cloneStringMap(values map[string]string) map[string]string {
	if values == nil {
		return nil
	}
	clone := make(map[string]string, len(values))
	for key, value := range values {
		clone[key] = value
	}
	return clone
}

func cloneSchemaProperties(properties map[string]Schema) map[string]Schema {
	if properties == nil {
		return nil
	}
	clone := make(map[string]Schema, len(properties))
	for name, schema := range properties {
		clone[name] = cloneSchema(schema)
	}
	return clone
}

func cloneSchemaValues(values []any) []any {
	if values == nil {
		return nil
	}
	clone := make([]any, len(values))
	for index, value := range values {
		clone[index] = cloneSchemaValue(value)
	}
	return clone
}

func cloneSchemaValue(value any) any {
	if value == nil {
		return nil
	}
	return cloneSchemaValueReflect(reflect.ValueOf(value)).Interface()
}

func cloneSchemaValueReflect(value reflect.Value) reflect.Value {
	switch value.Kind() {
	case reflect.Interface:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		clone := reflect.New(value.Type()).Elem()
		clone.Set(cloneSchemaValueReflect(value.Elem()))
		return clone
	case reflect.Pointer:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		clone := reflect.New(value.Type().Elem())
		clone.Elem().Set(cloneSchemaValueReflect(value.Elem()))
		return clone
	case reflect.Map:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		clone := reflect.MakeMapWithSize(value.Type(), value.Len())
		iterator := value.MapRange()
		for iterator.Next() {
			clone.SetMapIndex(iterator.Key(), cloneSchemaValueReflect(iterator.Value()))
		}
		return clone
	case reflect.Slice:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		clone := reflect.MakeSlice(value.Type(), value.Len(), value.Len())
		for index := 0; index < value.Len(); index++ {
			clone.Index(index).Set(cloneSchemaValueReflect(value.Index(index)))
		}
		return clone
	case reflect.Array:
		clone := reflect.New(value.Type()).Elem()
		for index := 0; index < value.Len(); index++ {
			clone.Index(index).Set(cloneSchemaValueReflect(value.Index(index)))
		}
		return clone
	case reflect.Struct:
		clone := reflect.New(value.Type()).Elem()
		clone.Set(value)
		for index := 0; index < value.NumField(); index++ {
			if value.Type().Field(index).IsExported() && clone.Field(index).CanSet() {
				clone.Field(index).Set(cloneSchemaValueReflect(value.Field(index)))
			}
		}
		return clone
	default:
		return value
	}
}
