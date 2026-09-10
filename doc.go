// Package routespec registers HTTP routes with an application's existing router
// and generates an OpenAPI document from explicit operation metadata and Go DTOs.
//
// Routespec does not parse or validate requests, provide middleware, infer handler
// behavior, or implement a router. Applications retain those responsibilities.
package routespec
