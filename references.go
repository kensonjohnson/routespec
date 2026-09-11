package routespec

import (
	"fmt"
	"strings"
)

func validateDocumentReferences(document Document) {
	for name, schema := range document.Components.Schemas {
		if !validComponentName(name) {
			panic(fmt.Sprintf("routespec: invalid schema component name %q", name))
		}
		validateSchemaReferences(schema, document.Components.Schemas)
	}

	operationIDs := make(map[string]struct{})
	operationReferences := make(map[string]struct{})
	for path, pathItem := range document.Paths {
		for method, operation := range pathItemOperations(pathItem) {
			if operation == nil {
				continue
			}
			operationIDs[operation.OperationID] = struct{}{}
			operationReferences["#/paths/"+escapeJSONPointer(path)+"/"+method] = struct{}{}
		}
	}
	for _, pathItem := range document.Paths {
		for _, operation := range pathItemOperations(pathItem) {
			if operation == nil {
				continue
			}
			for _, parameter := range operation.Parameters {
				validateSchemaReferences(parameter.Schema, document.Components.Schemas)
			}
			if operation.RequestBody != nil {
				for _, content := range operation.RequestBody.Content {
					validateSchemaReferences(content.Schema, document.Components.Schemas)
				}
			}
			for _, response := range operation.Responses {
				for _, content := range response.Content {
					validateSchemaReferences(content.Schema, document.Components.Schemas)
				}
				for _, header := range response.Headers {
					validateSchemaReferences(header.Schema, document.Components.Schemas)
				}
				for name, link := range response.Links {
					validateLinkReference(name, link, operationIDs, operationReferences)
				}
			}
		}
	}
}

func pathItemOperations(pathItem PathItem) map[string]*DocumentOperation {
	return map[string]*DocumentOperation{
		"get":     pathItem.Get,
		"put":     pathItem.Put,
		"post":    pathItem.Post,
		"delete":  pathItem.Delete,
		"patch":   pathItem.Patch,
		"head":    pathItem.Head,
		"options": pathItem.Options,
		"trace":   pathItem.Trace,
	}
}

func validateSchemaReferences(schema Schema, components map[string]Schema) {
	validateSchemaReference(schema.Ref, components, "schema")
	for _, alternative := range schema.AllOf {
		validateSchemaReferences(alternative, components)
	}
	for _, alternative := range schema.AnyOf {
		validateSchemaReferences(alternative, components)
	}
	for _, alternative := range schema.OneOf {
		validateSchemaReferences(alternative, components)
	}
	if schema.Not != nil {
		validateSchemaReferences(*schema.Not, components)
	}
	for _, property := range schema.Properties {
		validateSchemaReferences(property, components)
	}
	if schema.Items != nil {
		validateSchemaReferences(*schema.Items, components)
	}
	if schema.AdditionalProperties != nil {
		validateSchemaReferences(*schema.AdditionalProperties, components)
	}
	if schema.Discriminator != nil {
		for _, reference := range schema.Discriminator.Mapping {
			validateSchemaReference(reference, components, "discriminator mapping")
		}
	}
}

func validateSchemaReference(reference string, components map[string]Schema, label string) {
	if !strings.HasPrefix(reference, "#/") {
		return
	}
	const schemaPrefix = "#/components/schemas/"
	if !strings.HasPrefix(reference, schemaPrefix) {
		panic(fmt.Sprintf("routespec: %s reference %q is not a schema component", label, reference))
	}
	name := strings.TrimPrefix(reference, schemaPrefix)
	if name == "" {
		panic(fmt.Sprintf("routespec: %s reference %q is missing a component name", label, reference))
	}
	if _, exists := components[name]; !exists {
		panic(fmt.Sprintf("routespec: %s reference %q does not exist", label, reference))
	}
}

func validateLinkReference(name string, link Link, operationIDs, operationReferences map[string]struct{}) {
	if link.OperationID != "" {
		if _, exists := operationIDs[link.OperationID]; !exists {
			panic(fmt.Sprintf("routespec: link %q operation ID %q does not exist", name, link.OperationID))
		}
	}
	if strings.HasPrefix(link.OperationRef, "#/") {
		if _, exists := operationReferences[link.OperationRef]; !exists {
			panic(fmt.Sprintf("routespec: link %q operation reference %q does not exist", name, link.OperationRef))
		}
	}
}

func escapeJSONPointer(value string) string {
	value = strings.ReplaceAll(value, "~", "~0")
	return strings.ReplaceAll(value, "/", "~1")
}
