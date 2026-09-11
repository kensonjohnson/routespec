package routespec_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"testing"

	"github.com/kensonjohnson/routespec"
)

type registrationIntegrityResponse struct {
	Name string `json:"name"`
}

func TestRegisterRejectsDuplicateOperationIDBeforeRegisteringRoute(t *testing.T) {
	mux := http.NewServeMux()
	api := routespec.New(mux, routespec.Info{Title: "Example", Version: "1.0.0"})
	api.RegisterFunc(http.MethodGet, "/first", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}, routespec.Operation{
		ID:        "sharedOperation",
		Responses: routespec.Responses{http.StatusNoContent: {}},
	})

	panicked := false
	func() {
		defer func() {
			panicked = recover() != nil
		}()
		api.RegisterFunc(http.MethodPost, "/second", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusCreated)
		}, routespec.Operation{
			ID:        "sharedOperation",
			Responses: routespec.Responses{http.StatusCreated: {}},
		})
	}()
	if !panicked {
		t.Fatal("registration with a duplicate operation ID did not panic")
	}

	response := httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/second", nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("second route status = %d, want %d", response.Code, http.StatusNotFound)
	}
}

func TestRegisterCopiesCallerOwnedOperationData(t *testing.T) {
	mux := http.NewServeMux()
	api := routespec.New(mux, routespec.Info{
		Title:   "Example",
		Version: "1.0.0",
		SecuritySchemes: map[string]routespec.SecurityScheme{
			"auth": {
				Type: "oauth2",
				Flows: &routespec.OAuthFlows{ClientCredentials: &routespec.OAuthFlow{
					TokenURL: "https://example.com/token",
					Scopes:   map[string]string{"read": "Read", "write": "Write"},
				}},
			},
			"secondary": {Type: "http", Scheme: "bearer"},
		},
	})

	tags := []string{"stable"}
	security := routespec.SecurityRequirement{"auth": {"read"}}
	responses := routespec.Responses{http.StatusOK: routespec.Respond(routespec.JSON[registrationIntegrityResponse]())}
	operation := routespec.Operation{
		ID:        "immutableOperation",
		Tags:      tags,
		Security:  []routespec.SecurityRequirement{security},
		Responses: responses,
	}
	api.RegisterFunc(http.MethodGet, "/immutable", func(http.ResponseWriter, *http.Request) {}, operation)

	wantDocument := api.Document()
	wantJSON, err := api.JSON()
	if err != nil {
		t.Fatalf("JSON: %v", err)
	}

	tags[0] = "mutated"
	security["auth"][0] = "write"
	security["auth"] = append(security["auth"], "admin")
	security["secondary"] = []string{"admin"}
	responses[http.StatusCreated] = routespec.Respond(routespec.JSON[registrationIntegrityResponse]())
	delete(responses, http.StatusOK)
	assertRegisteredOperationUnchanged(t, api, wantDocument, wantJSON)

	start := make(chan struct{})
	var workers sync.WaitGroup
	workers.Add(2)
	jsonErrors := make(chan error, 1)
	go func() {
		defer workers.Done()
		<-start
		for i := 0; i < 1_000; i++ {
			if i%2 == 0 {
				tags[0] = "mutated"
				security["auth"][0] = "write"
				security["secondary"] = []string{"admin"}
				responses[http.StatusCreated] = routespec.Respond(routespec.JSON[registrationIntegrityResponse]())
				delete(responses, http.StatusOK)
			} else {
				tags[0] = "changed"
				security["auth"][0] = "read"
				delete(security, "secondary")
				responses[http.StatusOK] = routespec.Respond(routespec.JSON[registrationIntegrityResponse]())
				delete(responses, http.StatusCreated)
			}
		}
	}()
	go func() {
		defer workers.Done()
		<-start
		for i := 0; i < 1_000; i++ {
			_ = api.Document()
			if _, err := api.JSON(); err != nil {
				select {
				case jsonErrors <- err:
				default:
				}
				return
			}
		}
	}()
	close(start)
	workers.Wait()
	select {
	case err := <-jsonErrors:
		t.Fatalf("JSON during caller mutation: %v", err)
	default:
	}

	assertRegisteredOperationUnchanged(t, api, wantDocument, wantJSON)
}

func assertRegisteredOperationUnchanged(t *testing.T, api *routespec.API, wantDocument routespec.Document, wantJSON []byte) {
	t.Helper()
	if got := api.Document(); !reflect.DeepEqual(got, wantDocument) {
		t.Fatalf("Document changed after caller mutation\nwant: %#v\ngot:  %#v", wantDocument, got)
	}
	gotJSON, err := api.JSON()
	if err != nil {
		t.Fatalf("JSON: %v", err)
	}
	if !bytes.Equal(gotJSON, wantJSON) {
		t.Fatalf("JSON changed after caller mutation\nwant: %s\ngot:  %s", wantJSON, gotJSON)
	}
}
