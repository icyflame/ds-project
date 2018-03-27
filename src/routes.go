package project

import (
	"fmt"
	"net/http"
)

type Route struct {
	Pattern string
	Handler http.HandlerFunc
}
type Routes []Route

var routes = Routes{
	Route{
		"/",
		Handler1,
	},
	Route{
		"/test",
		Handler2,
	},
}

func Handler1(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "handling 1")
}

func Handler2(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "handling 2")
}
