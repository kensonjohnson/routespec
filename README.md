# routespec

`routespec` registers routes with an existing Go router and generates an OpenAPI 3.1 document from explicit operation metadata and Go DTOs.

It requires Go 1.27 or later. It does not parse or validate requests, provide middleware, discover existing routes, infer handler behavior, or generate clients.

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
    RequestBody: routespec.JSON[createWidgetRequest](),
    Responses: routespec.Responses{
        http.StatusCreated: routespec.JSON[widgetResponse](),
        http.StatusBadRequest: routespec.JSON[problemResponse](),
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
    Responses: routespec.Responses{http.StatusOK: routespec.JSON[widgetResponse]()},
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

## DTOs and annotations

`json` tags determine body and response property names. `Path[T]` and `Query[T]` use the same names by default.

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

Separate directives with commas. Escape commas, pipes, equals signs, and backslashes in values with a backslash. `enum` values use `|` separators. `name` only applies to `Path` and `Query` models. Body and response fields always use their `json` name.

Tags change only the generated document. They do not validate HTTP input. Conditional and cross-field rules remain application code.

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
