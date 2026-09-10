package routespec_test

import (
	"fmt"
	"net/http"

	"github.com/julienschmidt/httprouter"
	"github.com/kensonjohnson/routespec"
)

type httpRouterExampleUserPath struct {
	ID string `json:"id"`
}

func ExampleNewHTTPRouter() {
	router := httprouter.New()
	api := routespec.NewHTTPRouter(router, routespec.Info{
		Title:   "Users",
		Version: "1.0.0",
	})

	var getUser httprouter.Handle = func(http.ResponseWriter, *http.Request, httprouter.Params) {}
	api.RegisterFunc(http.MethodGet, "/users/:id", getUser, routespec.Operation{
		ID:        "getUser",
		Path:      routespec.Path[httpRouterExampleUserPath](),
		Responses: routespec.Responses{http.StatusOK: {}},
	})

	fmt.Println(api.Document().Paths["/users/{id}"].Get.OperationID)
	// Output: getUser
}
