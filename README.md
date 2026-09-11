# routespec

`routespec` registers routes with an existing Go router and generates an OpenAPI 3.1 document from explicit operation metadata and Go DTOs.

It requires Go 1.27 or later. The v0.1 release line supports Go 1.27. It does not parse or validate requests, provide middleware, discover existing routes, infer handler behavior, or generate clients.

## ServeMux

```go
mux := http.NewServeMux()
api := routespec.New(mux, routespec.Info{
    Title:   "Widgets API",
    Version: "1.0.0",
})

api.RegisterFunc(http.MethodPost, "/widgets/{id}", createWidget, routespec.Operation{
    ID:          "createWidget",
    Summary:     "Create a widget",
    Path:        routespec.Path[widgetPath](),
    RequestBody: routespec.Request(routespec.JSON[createWidgetRequest]()),
    Responses: routespec.Responses{
        http.StatusCreated: routespec.Respond(routespec.JSON[widgetResponse]()),
        http.StatusBadRequest: routespec.Respond(routespec.JSON[problemResponse]()),
    },
})
```

Use `Register` with an `http.Handler`. Use `RegisterFunc` with an `http.HandlerFunc`. Both panic during startup for invalid metadata, a duplicate operation, or an invalid route. This matches `http.ServeMux.Handle`.

If you want to expose a live document to a UI or other tooling, mount the optional handler where it fits the application:

```go
mux.Handle("GET /openapi.json", api.Handler())
```

The application chooses the route, access control, and caching policy. Do not mount it when the API description should stay private; use `api.JSON()` to export a file instead.

## httprouter

```go
router := httprouter.New()
api := routespec.NewHTTPRouter(router, routespec.Info{
    Title:   "Widgets API",
    Version: "1.0.0",
})

api.RegisterFunc(http.MethodGet, "/widgets/:id", getWidget, routespec.Operation{
    ID:        "getWidget",
    Path:      routespec.Path[widgetPath](),
    Responses: routespec.Responses{http.StatusOK: routespec.Respond(routespec.JSON[widgetResponse]())},
})
```

The adapter's `Register` accepts `http.Handler`. Its `RegisterFunc` accepts `httprouter.Handle`, including its `httprouter.Params` argument. It registers the native `:id` path with httprouter and emits `/widgets/{id}` in OpenAPI. Catch-all routes such as `/*path` are rejected because their multi-segment behavior cannot be represented precisely by an OpenAPI path template.

## Export a static spec

Keep route setup in an application function that both the server and exporter call. For a ServeMux application, that function can return the registry it creates:

```go
// package api
func NewAPI(mux *http.ServeMux) *routespec.API {
    api := routespec.New(mux, routespec.Info{
        Title:   "Widgets API",
        Version: "1.0.0",
    })
    api.RegisterFunc(http.MethodPost, "/widgets/{id}", createWidget, createWidgetOperation)
    return api
}
```

An application-owned `cmd/openapi` program writes the deterministic JSON to a checked-in file. Replace `example.com/widgets` with the application's module path.

```go
// cmd/openapi/main.go
package main

import (
    "flag"
    "log"
    "net/http"
    "os"

    "example.com/widgets/api"
)

func main() {
    output := flag.String("output", "openapi.json", "OpenAPI output path")
    flag.Parse()

    document, err := api.NewAPI(http.NewServeMux()).JSON()
    if err != nil {
        log.Fatal(err)
    }
    if err := os.WriteFile(*output, document, 0o644); err != nil {
        log.Fatal(err)
    }
}
```

For httprouter, make the shared setup function construct `routespec.NewHTTPRouter` instead. Both registries expose `JSON()`, so the exporter follows the same pattern.

`go generate` can make the command easy to find. Put this in a Go file under `api/`:

```go
//go:generate go run ../cmd/openapi -output ../openapi.json
```

Check the generated file in. CI can regenerate it and fail when the document changed:

```sh
go generate ./...
git diff --exit-code -- openapi.json
```

Routespec does not provide an export CLI. The application owns route construction and chooses where its checked-in document lives.

## Releases

Routespec's first public release is `v0.1.0` under Apache-2.0. Public APIs may change in later `v0.x` minor releases. Patch releases remain backward-compatible. When practical, a replacement stays deprecated for one minor release before removal.

Before releasing, move the notes out of `Unreleased` under a `## 0.1.0` heading in `CHANGELOG.md`, then run:

```sh
RELEASE_VERSION=v0.1.0 ./scripts/release-dry-run.sh
```

Set `RELEASE_VERSION` to the version you plan to release. From `main`, run the `Release` workflow in GitHub Actions and provide a `v0.MINOR.PATCH` version such as `v0.1.0`. The workflow reruns the release checks, creates and pushes the tag for the selected commit, verifies a clean temporary consumer module against that tag, and creates GitHub release notes. The dry run also verifies a temporary consumer against the local source at the intended module version. To rerun that local check:

```sh
./scripts/verify-consumer.sh v0.1.0 "$PWD"
```

After publication, omit the local source to fetch the tagged module directly:

```sh
./scripts/verify-consumer.sh v0.1.0
```

## Document metadata

`Info` declares OpenAPI document metadata: terms of service, contact, license, servers, tags, external docs, and security schemes. Supported security schemes are `apiKey`, `http`, `mutualTLS`, `oauth2`, and `openIdConnect`; use `OAuthFlows` and `OAuthFlow` for OAuth 2.0.

```go
routespec.Info{
    Title:   "Widgets API",
    Version: "1.0.0",
    Servers: []routespec.Server{{URL: "https://api.example.com/v1"}},
    Tags:    []routespec.Tag{{Name: "widgets"}},
    SecuritySchemes: map[string]routespec.SecurityScheme{
        "bearerAuth": {Type: "http", Scheme: "bearer"},
    },
}
```

Internal schema references, discriminator mappings, link operation IDs and local operation references, and security scheme names/scopes are checked during route registration.

## DTOs and annotations

`json` tags determine body and response property names. `Path[T]`, `Query[T]`, `Header[T]`, and `Cookie[T]` use the same names by default.

```go
type createWidgetRequest struct {
    Name  string `json:"name" openapi:"required,minLength=1,maxLength=100,description=Widget name"`
    Email string `json:"email" openapi:"format=email"`
}

type widgetQuery struct {
    Limit int `json:"limit" openapi:"minimum=1,maximum=100,default=20"`
}
```

Supported `openapi` directives are `required`, `name`, `description`, `format`, `minLength`, `maxLength`, `pattern`, `minimum`, `maximum`, `exclusiveMinimum`, `exclusiveMaximum`, `multipleOf`, `minItems`, `maxItems`, `uniqueItems`, `minProperties`, `maxProperties`, `additionalProperties=false`, `enum`, `const`, `default`, `example`, `readOnly`, `writeOnly`, and `deprecated`.

Separate directives with commas. Escape commas, pipes, equals signs, and backslashes in values with a backslash. `enum` values use `|` separators. `name` only applies to `Path`, `Query`, `Header`, and `Cookie` models. Body and response fields always use their `json` name.

Tags change only the generated document. They do not validate HTTP input. Conditional and cross-field rules remain application code.

## HTTP metadata

`RequestSpec` and `ResponseSpec` support multiple representations through `Content`. Use `JSON[T]()`, `Text[T]()`, `Binary[T]()`, `Form[T]()`, `Multipart[T]()`, or `Media[T](mediaType)`. `ResponseSpec` can also declare a description, typed response headers with `ResponseHeader[T]()`, and OpenAPI links.

```go
routespec.Operation{
    RequestBody: routespec.Request(
        routespec.Form[createWidgetRequest](),
        routespec.Multipart[createWidgetRequest](),
    ),
    Responses: routespec.Responses{
        http.StatusCreated: {
            Description: "Widget created.",
            Content: []routespec.Content{routespec.JSON[widgetResponse]()},
            Headers: map[string]routespec.HeaderSpec{
                "X-Request-ID": routespec.ResponseHeader[string](),
            },
        },
    },
}
```

`ParameterModel.Style`, `Explode`, and `AllowReserved` expose supported OpenAPI parameter serialization. Invalid locations, styles, media types, and duplicate representations panic during registration.

Routespec supports structs, nested and recursive named structs, booleans, numbers, strings, `time.Time`, slices, arrays, maps with string keys, and pointers. Pointer fields emit an OpenAPI 3.1 schema that permits `null` unless `omitempty` or `omitzero` omits a nil pointer. Use `additionalProperties=false` on a struct field to close that object schema.

## Schema overrides

Use `WithSchemaOverride` for a shape tags cannot describe, such as a union or composed schema:

```go
api := routespec.New(mux, info,
    routespec.WithSchemaOverride[flexibleValue](routespec.Schema{
        OneOf: []routespec.Schema{
            {Type: "string"},
            {Type: "integer", Format: "int64"},
        },
    }),
)
```

`Schema` also exposes `AllOf`, `AnyOf`, `Not`, and `Discriminator`. A discriminator requires `AllOf`, `AnyOf`, or `OneOf`, and its `PropertyName` must be a required property of the composed schema.

## Security

Declare reusable schemes in `Info`, then reference them from an operation:

```go
api := routespec.New(mux, routespec.Info{
    Title:   "Widgets API",
    Version: "1.0.0",
    SecuritySchemes: map[string]routespec.SecurityScheme{
        "bearerAuth": {Type: "http", Scheme: "bearer"},
    },
})

// Operation{Security: []routespec.SecurityRequirement{{"bearerAuth": {}}}}
```

A referenced scheme must be present in `Info.SecuritySchemes`.
