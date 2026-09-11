package routespec

import (
	"testing"

	json "encoding/json/v2"
)

func TestSchemaUnmarshalRejectsUnsupportedTypeUnions(t *testing.T) {
	var schema Schema
	if err := json.Unmarshal([]byte(`{"type":["string","number","null"]}`), &schema); err == nil {
		t.Fatal("unmarshal succeeded for an unsupported multi-type schema")
	}
}

func TestSchemaUnmarshalAcceptsOpenAdditionalProperties(t *testing.T) {
	var schema Schema
	if err := json.Unmarshal([]byte(`{"type":"object","additionalProperties":true}`), &schema); err != nil {
		t.Fatalf("unmarshal schema: %v", err)
	}
	if schema.Closed || schema.AdditionalProperties != nil {
		t.Fatalf("schema = %#v, want open object", schema)
	}
}

func TestSchemaUnmarshalRejectsArbitraryAnyOf(t *testing.T) {
	var schema Schema
	if err := json.Unmarshal([]byte(`{"anyOf":[{"type":"string"},{"type":"number"}]}`), &schema); err == nil {
		t.Fatal("unmarshal succeeded for unsupported anyOf")
	}
}
