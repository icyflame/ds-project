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
var timestamping, Qc_updating sync.Mutex
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

	InitDropReqs(&DropReqs)
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

	log.Printf("SEND ALL %d, %d", msg_req.Sender, msg_req.SenderSeq)

	for i := 0; i < len(peers); i++ {
		v := <-resps
		if v == 200 {
			stamps[my_node_num] += 1
			log.Printf("RECV REQ ACK %d, %d", msg_req.Sender, msg_req.SenderSeq)
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

	msg_ack := BuildMsgAck(
		msg_req.Sender,
		msg_req.SenderSeq,
		my_node_num,
		nts,
		msg_req.Tlv,
		next_token_site,
	)

	return msg_ack
}

// Adds the given message to the Priority Queue
// Removes the message from Qb if it exists there
func AddStampedMsgToQc(m MsgWithFinalTS) {
	Qc_updating.Lock()

	heap.Push(&Queue_c, &m)

	// Remove the msg_req if it exists in Qb
	index := FindMsgInQb(m.Sender, m.SenderSeq)
	if index >= 0 {
		RemoveMsgFromQb(m.Sender, index)
	}

	Qc_updating.Unlock()

	log.Printf("QC: %d; New msg: %d, %d", len(Queue_c), m.Sender, m.SenderSeq)
}

// Find the message with the given sender and sender_seq in Qb
// If found, returns the index of the message inside Qb[sender]
// If not found, returns -1
func FindMsgInQb(sender int64, sender_seq int64) int {
	if sender >= int64(len(Queue_b)) {
		log.Fatal("Index not found: ")
	}

	for index, msg := range Queue_b[sender] {
		if msg.SenderSeq == sender_seq {
			return index
		}
	}

	return -1
}

// Remove the given index message from the Qb
// If sender >= len(Qb) or index >= len(Qb[sender]), this function will fail
// silently
func RemoveMsgFromQb(sender int64, index int) {
	if sender >= int64(len(Queue_b)) || index >= len(Queue_b[sender]) {
		return
	}

	Queue_b[sender] = append(Queue_b[sender][:index], Queue_b[sender][index+1:]...)
}

// Use the given MsgAck to remove the message from Qb and add it in it's
// appropriate place in Qb
func SequenceMsg(msg_ack MsgAck) {
	if msg_ack.FinalTS < nts {
		// This msg_ack has been received for the second time
		// Ignore
		return
	}

	if msg_ack.FinalTS > nts {
		// Can't accept this message until I have all the previous messages
		// Send retransmission request to the token site for the missed messages
		SendRetransmitReq(nts)
		return
	}

	sender := msg_ack.Sender
	index := FindMsgInQb(sender, msg_ack.SenderSeq)

	if index >= 0 {
		msg_req := Queue_b[sender][index]
		m := BuildMsgWithFinalTS(msg_req, msg_ack)

		AddStampedMsgToQc(m)

		RemoveMsgFromQb(sender, index)

		timestamping.Lock()

		nts += 1

		timestamping.Unlock()

		return
	}

	// Msg not found! Need to send a retransmission request
	log.Printf("FAIL NOT FOUND %d, %d, TS %d", sender, msg_ack.SenderSeq, msg_ack.FinalTS)
	SendRetransmitReq(msg_ack.FinalTS)
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

func MyNodeNum() int64 {
	return my_node_num
}

func SendRetransmitReq(final_ts int64) {
	m_rtr := BuildMsgRetransmitReq(my_node_num, final_ts, tlv)
	resp := SendMsgToNode(m_rtr, MSG_RETRANSMIT_REQ_PATH, token_site)
	if resp != 200 {
		log.Printf("FAIL RTR %d -> %d for %d", my_node_num,
			token_site, final_ts)
	} else {
		log.Printf("SEND RTR %d -> %d for %d", my_node_num,
			token_site, final_ts)
	}
}

func RetransmitMsg(m_rtr MsgRetransmitReq) {
	for _, msg := range Queue_c {
		if msg.FinalTS == m_rtr.FinalTS {
			m_ack := GetMsgAckFromMWFTS(*msg)
			SendMsgToNode(m_ack, MSG_ACK_PATH, m_rtr.Sender)
			return
		}
	}

	// Msg not found in Queue (?! SHOULD NOT HAPPEN)
	log.Printf("PANIC NOT FOUND TS %d", m_rtr.FinalTS)
}
