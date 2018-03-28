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
		"/submit-message",
		AcceptClientMessage,
		"AcceptClientMsg",
		"POST",
		false,
	},
	Route{
		MSG_REQ_PATH,
		AcceptMsgRequestHandler,
		"AcceptMsgRequest",
		"POST",
		false,
	},
	Route{
		MSG_ACK_PATH,
		AcceptMsgAckHandler,
		"AcceptMsgAck",
		"POST",
		false,
	},
}
