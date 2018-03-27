package project

import (
	"net/http"
)

type Route struct {
	Pattern string
	Handler http.HandlerFunc
	Name    string
	Method  string
}
type Routes []Route

var routes = Routes{
	Route{
		"/",
		Handler1,
		"root",
		"GET",
	},
	Route{
		"/test",
		Handler2,
		"test",
		"GET",
	},
}
