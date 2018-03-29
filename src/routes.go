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
	// CLIENT-SIDE
	Route{
		"/sm",
		AcceptClientMessage,
		"AcceptClientMsg",
		"POST",
		false,
	},

	// PROTOCOL-SIDE
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
	Route{
		MSG_RETRANSMIT_REQ_PATH,
		RetransmissionReqHandler,
		"RetransmissionReq",
		"POST",
		false,
	},
	Route{
		MSG_RETRANSMITTED_REQ_PATH,
		ReceiveRetransmittedMsg,
		"RecvRetransmittedMsg",
		"POST",
		false,
	},

	// MAINTENANCE
	Route{
		"/health",
		HealthReqHandler,
		"HealthReq",
		"GET",
		false,
	},
}
