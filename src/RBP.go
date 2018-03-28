package project

import (
	"sync"
)

var nts int64
var tlv int64
var token_site int64
var next_token_site int64
var my_node_num int64
var stamps = []int64{}
var peers = map[int64]Peer{}
var timestamping sync.Mutex

// Function to initiate the protocol using the given config object
func InitFromConfig(config Config, my_num int64) {
	peers = config.Peers
	token_site = config.InitTokSite
	next_token_site = (token_site + 1) % int64(len(peers))
	nts = 0
	tlv = 0
	my_node_num = my_num
	for i := 0; i < len(config.Peers); i++ {
		stamps = append(stamps, 0)
	}
}

// Getter function for token_site variable
func AmTokenSite() bool {
	return (token_site == my_node_num)
}

// Function that will accept the given "data" message
// If this is the token site, it will stamp the message and forward it to all
// peers. If not, it will stamp the message with it's sender_seq and send it to
// everyone else.
// Returns true with the MsgAck or false with nil
func AcceptMessage(min_msg MsgClient) (bool, MsgAck) {
	msg_req := BuildMsgRequest(
		my_node_num,
		stamps[my_node_num],
		tlv,
		min_msg.Content,
	)

	if AmTokenSite() {
		// No need to send this message anywhere. Timestamp it and broadcast to
		// everyone else
		msg_ack := StampMsg(msg_req)
		return true, msg_ack
	} else {
		// Need to send this message to the IP stored inside Peers
		SendMsgReqToTokSite(msg_req, token_site)
		return false, MsgAck{}
	}
}

// Communication function: sends this msg request to the appropriate IP for
// timestamping
func SendMsgReqToTokSite(
	msg_req MsgRequest,
	token_site int64,
) {
	// dest_ip := peers[token_site].IP
	// Post the data to the token site
}

// Function that provides the token site functionality
func StampMsg(msg_req MsgRequest) MsgAck {
	// stamp this message and move on

	timestamping.Lock()

	msg_ack := BuildMsgAck(
		msg_req.Sender,
		msg_req.SenderSeq,
		my_node_num,
		nts,
		msg_req.Tlv,
		next_token_site,
	)

	nts += 1

	timestamping.Unlock()

	return msg_ack
}
