package project

import (
	"container/heap"
	"encoding/json"
	"log"
	"net/http"
	"net/url"
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
var Queue_b = map[int64][]MsgRequest{}
var Queue_c = MsgPriorityQueue{}

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
		Queue_b[int64(i)] = []MsgRequest{}
	}
	heap.Init(&Queue_c)
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
func AcceptMessage(min_msg MsgClient) MsgRequest {
	msg_req := BuildMsgRequest(
		my_node_num,
		stamps[my_node_num],
		tlv,
		min_msg.Content,
	)

	// Send to everyone else
	resps := BroadcastMsg(msg_req, MSG_REQ_PATH)

	// Accept the message yourself
	AcceptMsgRequest(msg_req)

	log.Printf("BROADCAST MSG %d, %d", msg_req.Sender, msg_req.SenderSeq)

	for i := 0; i < len(peers); i++ {
		v := <-resps
		if v == 200 {
			stamps[my_node_num] += 1
			log.Printf("ACK TOKEN SITE %d %d", msg_req.Sender, msg_req.SenderSeq)
			break
		}
	}

	return msg_req
}

func AddToMyQb(msg_req MsgRequest) {
	Queue_b[msg_req.Sender] = append(Queue_b[msg_req.Sender], msg_req)
}

func AcceptMsgRequest(msg_req MsgRequest) {
	AddToMyQb(msg_req)

	if AmTokenSite() {
		// Timestamp this message and broadcast ack to everyone else
		msg_ack := StampMsg(msg_req)
		BroadcastMsg(msg_ack, MSG_ACK_PATH)

		// Add msg_req to the priority queue
		SequenceMsg(msg_ack)
	}
}

// Broadcast the given msg object to all peers at {IP}/{path}
// Return a channel of status codes
func BroadcastMsg(
	msg interface{},
	path string,
) chan int {
	resps := make(chan int)
	for i := 0; i < len(peers); i++ {
		if int64(i) == my_node_num {
			continue
		}
		go func(j int) {
			resps <- SendMsgToNode(msg, path, int64(j))
		}(i)
	}

	return resps
}

// Communication function: sends this msg request to the appropriate IP for
// timestamping
func SendMsgToNode(
	msg interface{},
	path string,
	site_num int64,
) int {
	dest_ip := peers[site_num].IP
	// Post the data to the token site
	dat, _ := json.Marshal(msg)
	dat_vals := url.Values{}
	dat_vals.Add("data", string(dat))

	resp, err := http.PostForm(
		dest_ip+path,
		dat_vals,
	)

	if err != nil {
		return -1
	}

	if resp != nil {
		return resp.StatusCode
	} else {
		return 0
	}
}

// Function that provides the token site functionality
// Stamps the given msg request and returns the MsgAck that should be broadcast
// to all the nodes
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

// Use the given MsgAck to remove the message from Qb and add it in it's
// appropriate place in Qb
func SequenceMsg(msg_ack MsgAck) {
	for index, msg_req := range Queue_b[msg_ack.Sender] {
		if msg_ack.SenderSeq == msg_req.SenderSeq {
			m := BuildMsgWithFinalTS(msg_req, msg_ack)
			heap.Push(&Queue_c, &m)

			Queue_b[msg_ack.Sender] = append(Queue_b[msg_ack.Sender][:index], Queue_b[msg_ack.Sender][index+1:]...)

			log.Printf("QC: %d; New msg: %d, %d", len(Queue_c), m.Sender, m.SenderSeq)

			return
		}
	}

	// Msg not found! Need to send a retransmission request
	log.Printf("MSG NOT FOUND %d, %d", msg_ack.Sender, msg_ack.SenderSeq)
}

type Health struct {
	MyNum   int64
	Nts     int64
	Tlv     int64
	TokSite int64
	Qb      map[int64][]MsgRequest
	Qc      MsgPriorityQueue
}

func GetHealthInfo() Health {
	return Health{
		my_node_num,
		nts,
		tlv,
		token_site,
		Queue_b,
		Queue_c,
	}
}
