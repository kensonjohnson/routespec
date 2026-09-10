# routespec

`routespec` registers routes with an existing Go router and generates an OpenAPI 3.1 document from explicit operation metadata and Go DTOs.

It requires Go 1.22 or later. It does not parse or validate requests, provide middleware, discover existing routes, infer handler behavior, or generate clients.

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

mux.Handle("GET /openapi.json", api.Handler())
```

Use `Register` with an `http.Handler`. Use `RegisterFunc` with an `http.HandlerFunc`. Both panic during startup for invalid metadata, a duplicate operation, or an invalid route. This matches `http.ServeMux.Handle`.

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

Supported `openapi` directives are `required`, `name`, `description`, `format`, `minLength`, `maxLength`, `pattern`, `minimum`, `maximum`, `multipleOf`, `minItems`, `maxItems`, `minProperties`, `maxProperties`, `enum`, `default`, `example`, `readOnly`, `writeOnly`, and `deprecated`.

Separate directives with commas. Escape commas, pipes, equals signs, and backslashes in values with a backslash. `enum` values use `|` separators. `name` only applies to `Path` and `Query` models. Body and response fields always use their `json` name.

Tags change only the generated document. They do not validate HTTP input. Conditional and cross-field rules remain application code.

Routespec supports structs, nested and recursive named structs, booleans, numbers, strings, `time.Time`, slices, arrays, maps with string keys, and pointers. Pointers are dereferenced for schema generation. They do not currently advertise JSON `null`; use an override when clients must see that contract.

## Schema overrides

Use `WithSchemaOverride` for a shape tags cannot describe, such as a union:

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
