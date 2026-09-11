package routespec

import "reflect"

// Info describes an API document and its top-level OpenAPI metadata.
type Info struct {
	Title           string
	Version         string
	Description     string
	TermsOfService  string
	Contact         *Contact
	License         *License
	Servers         []Server
	Tags            []Tag
	ExternalDocs    *ExternalDocs
	SecuritySchemes map[string]SecurityScheme
}

// Contact describes an API contact.
type Contact struct {
	Name  string `json:"name,omitempty"`
	URL   string `json:"url,omitempty"`
	Email string `json:"email,omitempty"`
}

// License describes an API license. Identifier and URL are mutually exclusive.
type License struct {
	Name       string `json:"name"`
	Identifier string `json:"identifier,omitempty"`
	URL        string `json:"url,omitempty"`
}

// ExternalDocs links to external API documentation.
type ExternalDocs struct {
	Description string `json:"description,omitempty"`
	URL         string `json:"url"`
}

// Server describes one API server.
type Server struct {
	URL         string                    `json:"url"`
	Description string                    `json:"description,omitempty"`
	Variables   map[string]ServerVariable `json:"variables,omitempty"`
}

// ServerVariable describes a templated server URL variable.
type ServerVariable struct {
	Enum        []string `json:"enum,omitempty"`
	Default     string   `json:"default"`
	Description string   `json:"description,omitempty"`
}

// Tag describes a reusable OpenAPI tag.
type Tag struct {
	Name         string        `json:"name"`
	Description  string        `json:"description,omitempty"`
	ExternalDocs *ExternalDocs `json:"externalDocs,omitempty"`
}

// SecurityScheme describes a reusable OpenAPI security scheme.
type SecurityScheme struct {
	Type             string      `json:"type"`
	Description      string      `json:"description,omitempty"`
	Name             string      `json:"name,omitempty"`
	In               string      `json:"in,omitempty"`
	Scheme           string      `json:"scheme,omitempty"`
	BearerFormat     string      `json:"bearerFormat,omitempty"`
	Flows            *OAuthFlows `json:"flows,omitempty"`
	OpenIDConnectURL string      `json:"openIdConnectUrl,omitempty"`
}

// OAuthFlows describes OAuth 2.0 security flows.
type OAuthFlows struct {
	Implicit          *OAuthFlow `json:"implicit,omitempty"`
	Password          *OAuthFlow `json:"password,omitempty"`
	ClientCredentials *OAuthFlow `json:"clientCredentials,omitempty"`
	AuthorizationCode *OAuthFlow `json:"authorizationCode,omitempty"`
}

// OAuthFlow describes one OAuth 2.0 flow.
type OAuthFlow struct {
	AuthorizationURL string            `json:"authorizationUrl,omitempty"`
	TokenURL         string            `json:"tokenUrl,omitempty"`
	RefreshURL       string            `json:"refreshUrl,omitempty"`
	Scopes           map[string]string `json:"scopes"`
}

// Operation describes one HTTP operation. Its request, parameters, and
// responses declare the API contract explicitly because a handler cannot.
type Operation struct {
	ID          string
	Summary     string
	Description string
	Tags        []string
	Security    []SecurityRequirement
	RequestBody RequestSpec
	Path        ParameterModel
	Query       ParameterModel
	Header      ParameterModel
	Cookie      ParameterModel
	Responses   Responses
}

// SecurityRequirement names a security scheme and its required scopes.
type SecurityRequirement map[string][]string

// Content describes a representation by media type and Go type.
type Content struct {
	mediaType string
	typeOf    reflect.Type
	binary    bool
}

// JSON declares an application/json representation of T.
func JSON[T any]() Content { return Media[T]("application/json") }

// Media declares a representation of T with mediaType.
func Media[T any](mediaType string) Content {
	return Content{mediaType: mediaType, typeOf: typeOf[T]()}
}

// Text declares a text/plain representation of T.
func Text[T any]() Content { return Media[T]("text/plain") }

// Binary declares an application/octet-stream representation.
func Binary[T any]() Content {
	content := Media[T]("application/octet-stream")
	content.binary = true
	return content
}

// Form declares an application/x-www-form-urlencoded representation of T.
func Form[T any]() Content { return Media[T]("application/x-www-form-urlencoded") }

// Multipart declares a multipart/form-data representation of T.
func Multipart[T any]() Content { return Media[T]("multipart/form-data") }

// RequestSpec describes a request body.
type RequestSpec struct {
	Description string
	Required    bool
	Content     []Content
}

// Request declares a required request body with the supplied representations.
func Request(content ...Content) RequestSpec {
	return RequestSpec{Required: true, Content: content}
}

// ResponseSpec describes one status response.
type ResponseSpec struct {
	Description string
	Content     []Content
	Headers     map[string]HeaderSpec
	Links       map[string]LinkSpec
}

// Respond declares a response with the supplied representations.
func Respond(content ...Content) ResponseSpec { return ResponseSpec{Content: content} }

// Responses maps HTTP status codes to their explicit response specifications.
type Responses map[int]ResponseSpec

// HeaderSpec describes a response header. Use ResponseHeader for a typed
// schema, or set Schema directly for an advanced declaration.
type HeaderSpec struct {
	Description string
	Required    bool
	Deprecated  bool
	Schema      Schema
	typeOf      reflect.Type
}

// ResponseHeader declares a typed response header.
func ResponseHeader[T any]() HeaderSpec { return HeaderSpec{typeOf: typeOf[T]()} }

// LinkSpec describes an OpenAPI response link.
type LinkSpec struct {
	OperationID  string
	OperationRef string
	Description  string
	Parameters   map[string]any
	RequestBody  any
}

type parameterLocation string

const (
	pathParameter   parameterLocation = "path"
	queryParameter  parameterLocation = "query"
	headerParameter parameterLocation = "header"
	cookieParameter parameterLocation = "cookie"
)

// ParameterModel describes a Go DTO used for one parameter location.
type ParameterModel struct {
	location      parameterLocation
	typeOf        reflect.Type
	style         string
	explode       *bool
	allowReserved bool
}

// Style sets the OpenAPI parameter serialization style.
func (model ParameterModel) Style(style string) ParameterModel {
	model.style = style
	return model
}

// Explode sets the OpenAPI parameter explode behavior.
func (model ParameterModel) Explode(explode bool) ParameterModel {
	model.explode = &explode
	return model
}

// AllowReserved enables reserved characters in a query parameter value.
func (model ParameterModel) AllowReserved() ParameterModel {
	model.allowReserved = true
	return model
}

// Path declares T as the model for path parameters.
func Path[T any]() ParameterModel { return parameterModel[T](pathParameter) }

// Query declares T as the model for query parameters.
func Query[T any]() ParameterModel { return parameterModel[T](queryParameter) }

// Header declares T as the model for header parameters.
func Header[T any]() ParameterModel { return parameterModel[T](headerParameter) }

// Cookie declares T as the model for cookie parameters.
func Cookie[T any]() ParameterModel { return parameterModel[T](cookieParameter) }

func parameterModel[T any](location parameterLocation) ParameterModel {
	return ParameterModel{location: location, typeOf: typeOf[T]()}
}

func typeOf[T any]() reflect.Type { return reflect.TypeFor[T]() }
