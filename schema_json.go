package routespec

import (
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"fmt"
)

// MarshalJSON emits OpenAPI 3.1 JSON Schema nullability and closed-object
// semantics while keeping Schema convenient to construct in Go.
func (schema Schema) MarshalJSON() ([]byte, error) {
	type wireSchema Schema
	if !schema.Nullable && !schema.Closed {
		return json.Marshal(wireSchema(schema), json.Deterministic(true))
	}

	base := schema
	base.Nullable = false
	base.Closed = false
	baseJSON, err := json.Marshal(wireSchema(base), json.Deterministic(true))
	if err != nil {
		return nil, err
	}

	var object map[string]jsontext.Value
	if err := json.Unmarshal(baseJSON, &object); err != nil {
		return nil, err
	}
	if schema.Closed {
		object["additionalProperties"] = jsontext.Value("false")
	}
	if schema.Nullable {
		if schema.Ref == "" && schema.Type != "" {
			types, err := json.Marshal([]string{schema.Type, "null"})
			if err != nil {
				return nil, err
			}
			object["type"] = types
		} else {
			nullSchema := jsontext.Value(`{"type":"null"}`)
			base, err := json.Marshal(object, json.Deterministic(true))
			if err != nil {
				return nil, err
			}
			return json.Marshal(map[string]jsontext.Value{
				"anyOf": jsontext.Value("[" + string(base) + "," + string(nullSchema) + "]"),
			}, json.Deterministic(true))
		}
	}
	return json.Marshal(object, json.Deterministic(true))
}

// UnmarshalJSON accepts the OpenAPI 3.1 representation produced by
// MarshalJSON and restores Schema's Go-friendly nullable and closed fields.
func (schema *Schema) UnmarshalJSON(data []byte) error {
	type wireSchema Schema
	var object map[string]jsontext.Value
	if err := json.Unmarshal(data, &object); err != nil {
		return err
	}

	nullable := false
	if rawType, exists := object["type"]; exists {
		var types []string
		if err := json.Unmarshal(rawType, &types); err == nil {
			concreteType := ""
			for _, typeName := range types {
				if typeName == "null" {
					nullable = true
					continue
				}
				if concreteType != "" {
					return fmt.Errorf("routespec: Schema cannot represent multiple non-null JSON types")
				}
				concreteType = typeName
			}
			if !nullable || concreteType == "" {
				return fmt.Errorf("routespec: Schema cannot represent JSON type array %s", rawType)
			}
			object["type"], _ = json.Marshal(concreteType)
		}
	}

	closed := false
	if rawAdditionalProperties, exists := object["additionalProperties"]; exists {
		switch string(rawAdditionalProperties) {
		case "false":
			closed = true
			delete(object, "additionalProperties")
		case "true":
			delete(object, "additionalProperties")
		}
	}

	if rawAnyOf, exists := object["anyOf"]; exists {
		var alternatives []jsontext.Value
		if err := json.Unmarshal(rawAnyOf, &alternatives); err == nil && len(object) == 1 && len(alternatives) == 2 && isNullSchema(alternatives[1]) {
			var base Schema
			if err := json.Unmarshal(alternatives[0], &base); err != nil {
				return err
			}
			base.Nullable = true
			*schema = base
			return nil
		}
	}

	transformed, err := json.Marshal(object, json.Deterministic(true))
	if err != nil {
		return err
	}
	var wire wireSchema
	if err := json.Unmarshal(transformed, &wire); err != nil {
		return err
	}
	*schema = Schema(wire)
	schema.Nullable = nullable
	schema.Closed = closed
	return nil
}

func isNullSchema(value jsontext.Value) bool {
	var object map[string]jsontext.Value
	if err := json.Unmarshal(value, &object); err != nil || len(object) != 1 {
		return false
	}
	rawType, exists := object["type"]
	if !exists {
		return false
	}
	var typeName string
	return json.Unmarshal(rawType, &typeName) == nil && typeName == "null"
}
