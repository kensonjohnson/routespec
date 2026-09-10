package routespec_test

import (
	"fmt"
	"net/http"

	"github.com/kensonjohnson/routespec"
)

type createWidgetRequest struct {
	Name string `json:"name" openapi:"required,minLength=1,maxLength=100,description=Widget name"`
}

func ExampleJSON_annotations() {
	api := routespec.New(http.NewServeMux(), routespec.Info{
		Title:   "Widgets",
		Version: "1.0.0",
	})
	api.RegisterFunc(http.MethodPost, "/widgets", func(http.ResponseWriter, *http.Request) {}, routespec.Operation{
		ID:          "createWidget",
		RequestBody: routespec.JSON[createWidgetRequest](),
		Responses:   routespec.Responses{http.StatusCreated: routespec.JSON[createWidgetRequest]()},
	})

	fmt.Println(api.Document().Paths["/widgets"].Post.OperationID)
	// Output: createWidget
}
