package routespec

import "reflect"

// Info describes an API document.
type Info struct {
	Title       string
	Version     string
	Description string
}

// Operation describes one HTTP operation. Request, Path, Query, and Responses
// declare the API contract explicitly because a handler cannot expose it.
type Operation struct {
	ID          string
	Summary     string
	Description string
	Tags        []string
	Security    []SecurityRequirement
	RequestBody Content
	Path        ParameterModel
	Query       ParameterModel
	Responses   Responses
}

// SecurityRequirement names a security scheme and its required scopes.
type SecurityRequirement map[string][]string

// Content describes a representation by media type and Go type.
type Content struct {
	mediaType string
	typeOf    reflect.Type
}

// JSON declares a JSON representation of T.
func JSON[T any]() Content {
	return Content{
		mediaType: "application/json",
		typeOf:    typeOf[T](),
	}
}

// Responses maps HTTP status codes to their response representations. A zero
// Content declares a response without a body, such as 204 No Content.
type Responses map[int]Content

type parameterLocation string

const (
	pathParameter  parameterLocation = "path"
	queryParameter parameterLocation = "query"
)

// ParameterModel describes a Go DTO used for one parameter location.
type ParameterModel struct {
	location parameterLocation
	typeOf   reflect.Type
}

// Path declares T as the model for path parameters.
func Path[T any]() ParameterModel {
	return ParameterModel{
		location: pathParameter,
		typeOf:   typeOf[T](),
	}
}

// Query declares T as the model for query parameters.
func Query[T any]() ParameterModel {
	return ParameterModel{
		location: queryParameter,
		typeOf:   typeOf[T](),
	}
}

func typeOf[T any]() reflect.Type {
	return reflect.TypeFor[T]()
}
