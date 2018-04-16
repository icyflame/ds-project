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
		"mreq",
		"POST",
		false,
	},
	Route{
		MSG_ACK_PATH,
		AcceptMsgAckHandler,
		"mack",
		"POST",
		false,
	},
	Route{
		MSG_RETRANSMIT_REQ_PATH,
		RetransmissionReqHandler,
		"rtreq",
		"POST",
		false,
	},
	Route{
		MSG_INIT_TOKEN_TRANSFER,
		TokenTransferInitHandler,
		"tti",
		"POST",
		false,
	},
	Route{
		MSG_COMPLETE_TOK_TRANSFER,
		TokenTransferCompleteHandler,
		"ttc",
		"POST",
		false,
	},
	Route{
		MSG_HEARTBEAT,
		MsgHeartbeatHandler,
		"hbh",
		"POST",
		false,
	},
	Route{
		MSG_TLV_CHANGE_PATH,
		MsgTlvHandler,
		"tlv",
		"POST",
		true,
	},
	Route{
		MSG_TLV_ACCEPTED,
		AcceptTlvHandler,
		"tlvacc",
		"POST",
		true,
	},
	Route{
		MSG_TLV_COMPLETED,
		TlvChangeCompleteHandler,
		"tlvdone",
		"POST",
		false,
	},

	// DEBUGGING
	Route{
		MSG_DROP_PATH,
		DropMsgsHandler,
		"dropreq",
		"GET",
		false,
	},

	// MAINTENANCE
	Route{
		"/health",
		HealthReqHandler,
		"health",
		"GET",
		false,
	},
}
