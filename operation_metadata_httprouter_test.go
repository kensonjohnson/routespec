package routespec_test

import (
	"net/http"
	"testing"

	"github.com/julienschmidt/httprouter"
	"github.com/kensonjohnson/routespec"
)

type operationMetadataRouterPath struct {
	ID string `json:"id"`
}

type operationMetadataRouterQuery struct {
	Fields []string `json:"fields"`
}

type operationMetadataForm struct {
	Name string `json:"name"`
}

func TestHTTPRouterOperationMetadata(t *testing.T) {
	router := httprouter.New()
	api := routespec.NewHTTPRouter(router, routespec.Info{Title: "Example", Version: "1.0.0"})
	api.RegisterFunc(http.MethodPost, "/uploads/:id", func(http.ResponseWriter, *http.Request, httprouter.Params) {}, routespec.Operation{
		ID: "routerUpload",
		RequestBody: routespec.Request(
			routespec.Form[operationMetadataForm](),
			routespec.Multipart[operationMetadataForm](),
			routespec.Binary[[]byte](),
		),
		Path:  routespec.Path[operationMetadataRouterPath]().Style("simple").Explode(false),
		Query: routespec.Query[operationMetadataRouterQuery]().Style("pipeDelimited").AllowReserved(),
		Responses: routespec.Responses{
			http.StatusNoContent: {Description: "Uploaded."},
		},
	})

	operation := api.Document().Paths["/uploads/{id}"].Post
	if operation == nil || operation.RequestBody == nil {
		t.Fatal("POST operation request body is missing")
	}
	for _, mediaType := range []string{"application/x-www-form-urlencoded", "multipart/form-data", "application/octet-stream"} {
		if _, exists := operation.RequestBody.Content[mediaType]; !exists {
			t.Fatalf("request content is missing %q", mediaType)
		}
	}
	if got := operation.RequestBody.Content["application/octet-stream"].Schema; got.Type != "string" || got.Format != "binary" {
		t.Fatalf("binary schema = %#v, want string/binary", got)
	}

	parameters := make(map[string]routespec.Parameter, len(operation.Parameters))
	for _, parameter := range operation.Parameters {
		parameters[parameter.In+":"+parameter.Name] = parameter
	}
	if got := parameters["path:id"]; got.Style != "simple" || got.Explode == nil || *got.Explode {
		t.Fatalf("path serialization = %#v, want simple with explode=false", got)
	}
	if got := parameters["query:fields"]; got.Style != "pipeDelimited" || !got.AllowReserved {
		t.Fatalf("query serialization = %#v, want pipeDelimited with allowReserved", got)
	}
}
