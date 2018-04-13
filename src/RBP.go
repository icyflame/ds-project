package project

import (
	"container/heap"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"sort"
	"sync"
	"time"
)

var TOKEN_TRANSFER_TIME = 10 * (time.Second)
var TokenTransferring = false

var wait_time_for_re_tti = 5 * (time.Second)
var clean_up_time = 30 * (time.Second)

var nts int64
var tlv int64
var token_site int64
var next_token_site int64
var my_node_num int64
var LVal int64
var Commitment = map[int64]int64{}
var stamps = []int64{}
var peers = map[int64]Peer{}
var timestamping, Qc_updating, TokenTransfer sync.Mutex

var Queue_c = MsgPriorityQueue{}

// Map from the string "sender-sender_seq" to the request and the ack of a
// particular message
var Queue_b_1 = map[string]MsgPreReq{}

// Function to initiate the protocol using the given config object
func InitFromConfig(config Config, my_num int64) {
	peers = config.Peers
	token_site = config.InitTokSite
	next_token_site = token_site
	nts = 1
	tlv = 1
	LVal = config.LVal

	for i := 0; i < len(config.Peers); i++ {
		stamps = append(stamps, 1)
	}

	my_node_num = my_num
	heap.Init(&Queue_c)

	InitDropReqs(&DropReqs)

	go RegularQbCleanUp(1)

	if AmTokenSite() {
		BecomeTokenSite()
	}
}

func RegularQbCleanUp(check_from int64) {
	c := time.After(clean_up_time)
	<-c

	log.Printf("CLEAN UP >= TS %d", check_from)

	ncf := check_from

	if len(Queue_c) > 0 {

		sort.Sort(Queue_c)

		for i := len(Queue_c) - 1; i >= 0 && Queue_c[i].FinalTS >= check_from; i-- {
			t := *Queue_c[i]
			RemoveMsgFromQb(t.Sender, t.SenderSeq)
		}

		ncf = (*Queue_c[len(Queue_c)-1]).FinalTS
	}

	go RegularQbCleanUp(ncf)
	return
}

// If this node has the token, then it will transfer it to the next site AFTER d
// duration of time.
func InitTokenTransferTimeout(d time.Duration) {
	if !AmTokenSite() {
		return
	}

	c := time.After(d)

	<-c

	next_token_site = int64((my_node_num + 1) % int64(len(peers)))
	log.Printf("INIT TOK TRANSFER %d -> %d", my_node_num, next_token_site)

	// Send a NULL message indicating to everyone that the token site is going
	// to change
	AcceptMessage(MsgClient{})

	c = time.After(wait_time_for_re_tti)
	<-c

	if AmTokenSite() &&
		!TokenTransferring {
		log.Printf("RETRY TTI %d -> %d", my_node_num, next_token_site)
		InitTokenTransferTimeout(d)
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

	msg_accepted := false

	for i := 0; i < len(peers); i++ {
		v := <-resps
		if v == 200 {
			stamps[my_node_num] += 1
			log.Printf("RECV REQ ACK %d, %d", msg_req.Sender, msg_req.SenderSeq)
			msg_accepted = true
			break
		}
	}

	if !msg_accepted {
		return MsgRequest{}
	} else {
		return msg_req
	}
}

// Return a string that acts as the unique identifier for a message (stamped and
// unstamped)
func Represent(sender, sender_seq int64) string {
	return fmt.Sprintf("%d-%d", sender, sender_seq)
}

// Adds the msg request to the Qb
func AddMsgReqToQb(msg_req MsgRequest) {
	Queue_b_1[Represent(msg_req.Sender, msg_req.SenderSeq)] = MsgPreReq{
		msg_req,
		MsgAck{},
	}
}

// Adds a msg ack to the Qb (request if in Qb will be over-written)
func AddMsgAckToQb(msg_ack MsgAck) {
	Queue_b_1[Represent(msg_ack.Sender, msg_ack.SenderSeq)] = MsgPreReq{
		MsgRequest{},
		msg_ack,
	}
}

// Add a msg request and ack both to Qb. Ready to be stamped and added to Qc
// (probably waiting because this message still has time until it comes to the
// top)
func AddMsgPreReqToQb(msg_req MsgRequest, ack MsgAck) {
	Queue_b_1[Represent(msg_req.Sender, msg_req.SenderSeq)] = MsgPreReq{
		msg_req,
		ack,
	}
}

// An ack is empty if FinalTS <= 0. Minimum possible FinalTS is 1
func NotEmptyAck(ack MsgAck) bool {
	return ack.FinalTS > 0
}

// A request is empty if Tlv <= 0. Minimum possible Tlv is 1
func NotEmptyReq(req MsgRequest) bool {
	return req.Tlv > 0
}

// Accept the message request passed as parameter. If the ack is already in Qb,
// then use that ack and stamp the message.
// Otherwise, just add the msg request to Qb.
// If this is also the token site, try and sequence the message
func AcceptMsgRequest(msg_req MsgRequest) {
	// Check if the ack for the req has already been received. If yes, then we
	// can directly sequence it!
	if val, ok := FindMsgInQb(msg_req.Sender, msg_req.SenderSeq); ok && NotEmptyAck(val.Ack) {
		AddMsgPreReqToQb(msg_req, val.Ack)
		SequenceMsg(val.Ack)
		return
	} else {
		AddMsgReqToQb(msg_req)
	}

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
	RemoveMsgFromQb(m.Sender, m.SenderSeq)

	Qc_updating.Unlock()

	log.Printf("QC: %d; New msg: %d, %d", len(Queue_c), m.Sender, m.SenderSeq)

	Commitment[m.FinalTS] = LVal
}

// Find the message with the given sender and sender_seq in Qb
// If found, returns the index of the message inside Qb[sender]
// If not found, returns -1
func FindMsgInQb(sender int64, sender_seq int64) (MsgPreReq, bool) {
	val, ok := Queue_b_1[Represent(sender, sender_seq)]
	return val, ok
}

// Remove the given index message from the Qb
// If sender >= len(Qb) or index >= len(Qb[sender]), this function will fail
// silently
func RemoveMsgFromQb(sender int64, sender_seq int64) {
	delete(Queue_b_1, Represent(sender, sender_seq))
}

// Use the given MsgAck to remove the message from Qb and add it in it's
// appropriate place in Qb
func SequenceMsg(msg_ack MsgAck) {
	if msg_ack.FinalTS < nts {
		// This msg_ack has been received for the second time
		// Ignore
		return
	}

	// If I am the NextTokSite, then I need to bring myself up to date and get
	// ready to accept the token. This happens irrespective of how behind I am.
	if msg_ack.TokSite != my_node_num &&
		msg_ack.NextTokSite == my_node_num &&
		msg_ack.TokSite != msg_ack.NextTokSite &&
		!TokenTransferring {
		InitRecvTokenMode()
		IndicateAcceptingToken(msg_ack.TokSite)
		return
	}

	if msg_ack.FinalTS > nts {
		// Can't accept this message until I have all the previous messages
		// Send retransmission request to the token site for the missed messages
		SendRetransmitReq(nts, false, false)
		return
	}

	sender := msg_ack.Sender
	msg_pre_req, ok := FindMsgInQb(sender, msg_ack.SenderSeq)

	if ok && NotEmptyReq(msg_pre_req.Req) {
		// We already have the request
		msg_req := msg_pre_req.Req

		m := BuildMsgWithFinalTS(msg_req, msg_ack)

		AddStampedMsgToQc(m)

		RemoveMsgFromQb(sender, msg_ack.SenderSeq)

		timestamping.Lock()

		nts += 1

		timestamping.Unlock()
	} else {
		AddMsgAckToQb(msg_ack)

		// Msg not found! Need to send a retransmission request
		log.Printf("FAIL NOT FOUND %d, %d, TS %d", sender, msg_ack.SenderSeq, msg_ack.FinalTS)
		SendRetransmitReq(msg_ack.FinalTS, false, true)
	}

	if msg_ack.TokSite != my_node_num &&
		msg_ack.NextTokSite != my_node_num &&
		token_site != msg_ack.TokSite {
		token_site = msg_ack.NextTokSite
		next_token_site = msg_ack.NextTokSite
		log.Printf("RECOGNIZE %d as TOK SITE", token_site)
		SuccessfulTokenTransfer()
	}
}

// This node indicates to the current token site that it is ready to accept the
// token. It sends it's identity and the last message it has seen till now to
// the current token site.
// The current token site will stop accepting new messages, send all messages to
// this node to bring it up to date and then send it a special indicator message
// to tell it that it is up to date.
// Then, the token transfer will be considered to be complete
func IndicateAcceptingToken(tok_site int64) {
	log.Printf("RECV TTI %d -> %d, TS %d", my_node_num, tok_site, nts)
	m_tti := BuildTokenTransferInit(my_node_num, nts, tlv)
	resp := SendMsgToNode(m_tti, MSG_INIT_TOKEN_TRANSFER, tok_site)
	if resp != 200 {
		log.Printf("FAIL TTI OK %d -> %d, LATER", my_node_num, tok_site)
	} else {
		log.Printf("SENT TTI OK %d -> %d, TS %d", my_node_num, tok_site, nts)
	}
}

type Health struct {
	MyNum       int64
	Nts         int64
	Tlv         int64
	TokSite     int64
	NextTokSite int64
	Qb          map[string]MsgPreReq
	Qc          MsgPriorityQueue
}

func GetHealthInfo() Health {
	return Health{
		my_node_num,
		nts,
		tlv,
		token_site,
		next_token_site,
		Queue_b_1,
		Queue_c,
	}
}

func MyNodeNum() int64 {
	return my_node_num
}

func SendRetransmitReq(final_ts int64, have_req, have_ack bool) {
	m_rtr := BuildMsgRetransmitReq(my_node_num, final_ts, tlv, have_req, have_ack)
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
			if !m_rtr.HaveReq {
				m_req := GetMsgReqFromMWFTS(*msg)
				SendMsgToNode(m_req, MSG_REQ_PATH, m_rtr.Sender)
			}

			if !m_rtr.HaveAck {
				m_ack := GetMsgAckFromMWFTS(*msg)
				SendMsgToNode(m_ack, MSG_ACK_PATH, m_rtr.Sender)
			}

			return
		}
	}

	// Msg not found in Queue (?! SHOULD NOT HAPPEN)
	log.Printf("PANIC NOT FOUND TS %d", m_rtr.FinalTS)
}

func RelinquishTokSite() {
	token_site = next_token_site
	next_token_site = token_site
	timestamping.Unlock()
	Qc_updating.Unlock()
	TokenTransfer.Lock()
	TokenTransferring = false
	TokenTransfer.Unlock()
	SuccessfulTokenTransfer()
}

func BecomeTokenSite() {
	token_site = my_node_num
	next_token_site = my_node_num
	// timestamping.Unlock()
	// Qc_updating.Unlock()
	TokenTransfer.Lock()
	TokenTransferring = false
	TokenTransfer.Unlock()
	go InitTokenTransferTimeout(TOKEN_TRANSFER_TIME)
	SuccessfulTokenTransfer()
}

func SuccessfulTokenTransfer() {
	to_commit := []int{}
	for k, v := range Commitment {
		Commitment[k] = v - 1

		if Commitment[k] <= 0 {
			to_commit = append(to_commit, int(k))
			delete(Commitment, k)
		}
	}

	sort.Ints(to_commit)
	log.Print("COMMIT: ", to_commit)

	LogCommit(to_commit, my_node_num)
}

func InitSendTokenMode() {
	timestamping.Lock()
	Qc_updating.Lock()
	TokenTransfer.Lock()
	TokenTransferring = true
	TokenTransfer.Unlock()
}

func InitRecvTokenMode() {
	TokenTransfer.Lock()
	TokenTransferring = true
	TokenTransfer.Unlock()
}

func BringNodeUpToDate(
	node_num int64,
	nts int64,
) int {
	// Find the latest message on this node
	// Send everything to `node_num`
	sort.Sort(Queue_c)
	latest := Queue_c[len(Queue_c)-1]

	diff := int((*latest).FinalTS + 1 - nts)

	// Send all the messages that the future token site doesn't have to it
	for i := 0; i < diff; i++ {
		msg := Queue_c[len(Queue_c)-diff+i]

		m_req := GetMsgReqFromMWFTS(*msg)
		SendMsgToNode(m_req, MSG_REQ_PATH, node_num)

		m_ack := GetMsgAckFromMWFTS(*msg)
		SendMsgToNode(m_ack, MSG_ACK_PATH, node_num)

		log.Printf("RTR REQ-ACK %d, %d, TS %d -> %d", m_req.Sender, m_req.SenderSeq, m_ack.FinalTS, node_num)
	}

	return diff
}

func IndicateTokTransferComplete(
	dest int64,
) {
	m_ttc := BuildTokenTransferComplete(my_node_num, dest, nts, tlv)
	SendMsgToNode(m_ttc, MSG_COMPLETE_TOK_TRANSFER, dest)
}

func EnsureConsistency(
	old int64,
	old_nts int64,
) {
	if nts < old_nts {
		// need to get all the messages from Old
	}
}

func GetMyNts() int64 {
	return nts
}
