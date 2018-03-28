package project

import (
	"net/http"
)

type Route struct {
	Pattern       string
	Handler       http.HandlerFunc
	Name          string
	Method        string
	SingleHandler bool
}
type Routes []Route

var routes = Routes{
	Route{
		"/",
		Handler1,
		"root",
		"GET",
		true,
	},
	Route{
		"/test",
		Handler2,
		"test",
		"GET",
		false,
	},
	Route{
		"/submit-message",
		AcceptClientMessage,
		"AcceptClientMsg",
		"POST",
		false,
	},
}
