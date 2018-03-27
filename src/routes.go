package project

import (
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
