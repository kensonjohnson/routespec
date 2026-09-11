// Package routespec registers HTTP routes with an application's existing router
// and generates an OpenAPI document from explicit operation metadata and Go DTOs.
//
// Routespec does not parse or validate requests, provide middleware, infer handler
// behavior, or implement a router. Applications retain those responsibilities.
//
// DTO fields may use an openapi struct tag, for example
// `openapi:"required,minLength=1,maxLength=100"`. The supported directives are
// required, name, description, format, minLength, maxLength, pattern, minimum,
// maximum, exclusiveMinimum, exclusiveMaximum, multipleOf, minItems, maxItems,
// uniqueItems, minProperties, maxProperties, additionalProperties=false, enum,
// const, default, example, readOnly, writeOnly, and deprecated. Separate directives
// with commas. Escape commas, pipes, equals signs, and backslashes in values with
// a backslash. The name directive only applies to Path and Query models; JSON
// request and response fields always use their json tag name.
//
// WithSchemaOverride provides an explicit schema for types that tags cannot
// describe, such as union schemas.
package routespec
