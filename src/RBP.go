package project

import (
	"container/heap"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"sort"
	"sync"
	"time"
)

// Time between getting the token and attempting a token transfer
var TOKEN_TRANSFER_TIME = 15 * (time.Second)

// Time to wait for an "I will accept the token" response from the intended
// future token site
var wait_time_for_re_tti = 5 * (time.Second)

// Function to clean up Qb will be run periodically at this rate
var clean_up_time = 30 * (time.Second)

// Time to wait for a heartbeat before probing to see if the token site is alive
// or not
var tok_site_probe_time = 2 * 60 * (time.Second)

// The timeout will be selected to be the base timeout added to a random timeout
// to avoid two nodes starting a TLV change at the same time (which will lead to
// each partitioning the other out!)
var tok_site_probe_range = 10 * (time.Second)

// Time allowed between the last message from the token site (last_seen of the
// token site) and current time. If more than this time elapses and we haven't
// received anything from the token site, a heartbeat will be broadcasted
var tok_site_probe_timeout = 2 * (time.Second)

// Timeout to retry network requests after
var NETWORK_TIMEOUT = 3 * (time.Second)

// Time to wait after broadcasting a TLV change to all the nodes for their
// respective replies
var TLV_TIMEOUT = 20 * (time.Second)

var HB_UNREPLIED_ALLOWED = 3

var TokenTransferring = false
var TLVChanging = false

var nts int64
var tlv int64
var new_tlv int64
var token_site int64
var next_token_site int64
var my_node_num int64
var hbs_not_replied_to int
var LVal int64
var RVal int
var Commitment = map[int64]int64{}
var stamps = []int64{}
var peers = map[int64]Peer{}

var NewPeers = map[int64]Peer{}
var Mapper = []Changed{}
var TentativeTokSite int64
var HighestNTSSeen int64

var TLVInitiator int64
var timestamping, Qc_updating, TokenTransfer, TLVChange, NewTLChanging sync.Mutex
var last_seen time.Time

var RetriesPerMsg = map[string]int{}

var Queue_c = MsgPriorityQueue{}

// Map from the string "sender-sender_seq" to the request and the ack of a
// particular message
var Queue_b_1 = map[string]MsgPreReq{}

// Function to initiate the protocol using the given config object
func InitFromConfig(config Config, my_num int64) {
	rand.Seed(time.Now().UnixNano())
	peers = config.Peers
	token_site = config.InitTokSite
	next_token_site = token_site
	nts = 1
	tlv = 1

	LVal = config.LVal
	if LVal <= 0 {
		LVal = 2
	}

	RVal = int(config.RVal)
	if RVal <= 0 {
		RVal = 3
	}

	last_seen = time.Now()

	for i := 0; i < len(config.Peers); i++ {
		stamps = append(stamps, 1)
	}

	if config.TTI != 0 &&
		config.ReTTI != 0 &&
		config.CleanUp != 0 &&
		config.TokSiteProbe != 0 &&
		config.TokSiteElapsed != 0 &&
		config.Network != 0 &&
		config.TLV != 0 {

		TOKEN_TRANSFER_TIME = time.Duration(config.TTI) * (time.Second)
		wait_time_for_re_tti = time.Duration(config.ReTTI) * (time.Second)
		clean_up_time = time.Duration(config.CleanUp) * (time.Second)
		tok_site_probe_time = time.Duration(config.TokSiteProbe) * (time.Second)
		tok_site_probe_range = time.Duration(config.TokSiteRange) * (time.Second)
		tok_site_probe_timeout = time.Duration(config.TokSiteElapsed) * (time.Second)
		NETWORK_TIMEOUT = time.Duration(config.Network) * (time.Second)
		TLV_TIMEOUT = time.Duration(config.TLV) * (time.Second)
	}

	my_node_num = my_num
	heap.Init(&Queue_c)

	InitDropReqs(&DropReqs)

	go RegularQbCleanUp(1)

	go StartTokSiteProbeTimer()

	if AmTokenSite() {
		BecomeTokenSite()
	}
}

func StartTokSiteProbeTimer() {
	rand_timeout := tok_site_probe_time + time.Duration(rand.Float32())*tok_site_probe_range
	c := time.After(rand_timeout)
	<-c

	t := time.Now()
	elapsed := t.Sub(last_seen)

	if elapsed > tok_site_probe_timeout {
		// Tok site might not be alive! Broadcast a Heartbeat now!
		BroadcastHeartbeat()
		log.Println("SEND HEARTBEAT NOW")
		log.Printf("NOT REPLIED TO %d", hbs_not_replied_to)
		hbs_not_replied_to += 1

		if hbs_not_replied_to > HB_UNREPLIED_ALLOWED {
			InitTLVChange(token_site)
		}
	}

	if !TokenTransferring && !TLVChanging {
		go StartTokSiteProbeTimer()
	}

	return
}

func ResetTimerTokSiteAlive(
	node int64,
) {
	if node == token_site {
		last_seen = time.Now()
		hbs_not_replied_to = 0
	}
}

func BroadcastHeartbeat() {
	BroadcastMsg(MsgHeartbeat{
		my_node_num,
	}, MSG_HEARTBEAT)
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

// Start the reformation phase to remove `site` from the existing token list and
// eventually start accepting messages again
func InitTLVChange(site int64) {
	if TLVChanging {
		return
	}

	log.Printf("INIT TLV CHANGE: %d failed", site)
	new_tlv = tlv + 1
	t := BuildMsgTLVChange(my_node_num, site, tlv, new_tlv)

	PrepareForTlv(my_node_num)
	PrepTLVAsInit()

	my_acc := MsgAcceptTLV{my_node_num, nts}
	AddToNewTLV(my_acc)

	BroadcastMsg(t, MSG_TLV_CHANGE_PATH)

	c := time.After(TLV_TIMEOUT)
	<-c

	CompleteTLVChange()
}

// Prepare this node for a TLV change. Gain the TLV Change lock which ensures
// that two separate TLV changes can't occur at the same time
func PrepareForTlv(initiator int64) {
	TLVChange.Lock()
	TLVChanging = true
	TLVInitiator = initiator
	hbs_not_replied_to = 0
}

// Terminate the TLV change routine, this will put this node in a state to
// accept new messages
func TerminateTLVChange() {
	TLVChange.Unlock()
	TLVChanging = false
}

type Changed struct {
	Old int64
	New int64
}

type TLVChangeComplete struct {
	Peers     map[int64]Peer
	Mapping   map[int64]int64
	Tlv       int64
	TokenSite int64
}

// Prepare the initiator node for a TLV change. this mainly sets all the
// variables to their default values
func PrepTLVAsInit() {
	Mapper = []Changed{}
	NewPeers = map[int64]Peer{}
	HighestNTSSeen = -1
	TentativeTokSite = 0
}

// When the TLV timeout expires, we will be ready with a new list of peers and
// IPs and the new token site. This function will construct the new token list
// and send it to all the other nodes
func CompleteTLVChange() {
	TLVChanging = false
	tlv = new_tlv
	new_tlv = 0

	// Send the peers and old to new number mappings to all the nodes in the new
	// peers list

	mapping := map[int64]int64{}
	for _, v := range Mapper {
		mapping[v.Old] = v.New
	}

	m := TLVChangeComplete{
		NewPeers,
		mapping,
		tlv,
		TentativeTokSite,
	}

	FixForChangedTLV(m)

	BroadcastMsg(m, MSG_TLV_COMPLETED)

	TerminateTLVChange()
}

// When the TLV changes, this function will be called and it will put the new
// list of peers, the tlv value and the token site in the config for this node
func FixForChangedTLV(m TLVChangeComplete) {
	peers = m.Peers
	tlv = m.Tlv
	my_node_num = m.Mapping[my_node_num]

	token_site = m.Mapping[m.TokenSite]
	next_token_site = m.TokenSite

	log.Printf("I am Node %d NOW", my_node_num)
	if AmTokenSite() {
		BecomeTokenSite()
		log.Println("I am the token site!")
	}

	log.Println("OK TLV CHANGE")
}

// When a site receives a "TLV Change" message from an initiator, it will accept
// the change by sending the initiator it's newest timestamp value and it's node
// ID
func AcceptTLVChange(m MsgTLVChange) {
	if !TLVChanging {
		return
	}

	tlv = m.NewTLV

	m_acc_tlv := MsgAcceptTLV{my_node_num, nts}
	SendMsgToNode(m_acc_tlv, MSG_TLV_ACCEPTED, m.Initiator)
}

// When an initiator receives an "Accept TLV" change from another node, it will
// run this function to add that new node to the New list of peers.
func AddToNewTLV(m MsgAcceptTLV) {
	node := m.Node

	NewTLChanging.Lock()

	new_id := int64(len(NewPeers))
	Mapper = append(Mapper, Changed{
		node,
		new_id,
	})
	NewPeers[new_id] = Peer{peers[node].IP}

	if HighestNTSSeen < m.NTS {
		HighestNTSSeen = m.NTS
		TentativeTokSite = m.Node
		log.Printf("POSSIBLE TOK SITE: %d", m.Node)
	}

	NewTLChanging.Unlock()
}

// If this node has the token, then it will transfer it to the next site AFTER d
// duration of time.
func InitTokenTransferTimeout(d time.Duration, retry_count int) {
	if !AmTokenSite() {
		return
	}

	if retry_count >= RVal {
		log.Printf("DETECT %d FAILURE", next_token_site)
		InitTLVChange(next_token_site)
		return
	}

	c := time.After(d)

	<-c

	next_token_site = int64((my_node_num + 1) % int64(len(peers)))
	log.Printf("INIT TOK TRANSFER %d -> %d", my_node_num, next_token_site)

	// Send a NULL message indicating to everyone that the token site is going
	// to change
	AcceptMessage(MsgClient{})

	log.Printf("RETRY TTI %d -> %d AFTER %v", my_node_num, next_token_site, wait_time_for_re_tti)
	InitTokenTransferTimeout(wait_time_for_re_tti, retry_count+1)
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

	return TransmitMsgReq(msg_req)
}

func TransmitMsgReq(msg_req MsgRequest) MsgRequest {
	rep := Represent(msg_req.Sender, msg_req.SenderSeq)

	RetriesPerMsg[rep] += 1

	log.Printf("Inside transmit msg req")

	if RetriesPerMsg[rep] >= RVal {
		log.Printf("DETECT TOKEN SITE %d FAIL", token_site)
		delete(RetriesPerMsg, rep)
		InitTLVChange(token_site)
	} else {

		log.Printf("broadcasting to everyone")

		// Send to everyone else
		resps := BroadcastMsg(msg_req, MSG_REQ_PATH)

		// Accept the message yourself
		AcceptMsgRequest(msg_req)

		log.Printf("SEND ALL %d, %d. TRY %d", msg_req.Sender, msg_req.SenderSeq, RetriesPerMsg[rep])

		msg_accepted := false

		// If I am not the token site, then I have to ensure that the token site
		// accepts this msg req. If not, then I have to keep retrying! If I am
		// the token site, then discard all this, and consider the message to be
		// accepted! (Push a 200 in the resps channel to simulate acceptance)

		// Resps is a channel which will have ONLY the response code of the
		// response from the token site. All the other responses will be
		// discarded because we don't care about them

		// If I am the token site, then there will be nothing in resps. We
		// have to populate it here so that rest of this will work properly

		select {
		case <-time.After(NETWORK_TIMEOUT):
			log.Printf("NETWORK TIMEOUT")
		case v := <-resps:
			if v == 200 {
				stamps[my_node_num] += 1
				log.Printf("OK ACK DELIVER %d, %d", msg_req.Sender, msg_req.SenderSeq)
				msg_accepted = true
			}
		}

		if !msg_accepted {
			log.Printf("FAIL ACK DELIVER %d, %d", msg_req.Sender, msg_req.SenderSeq)
			TransmitMsgReq(msg_req)
			return MsgRequest{}
		} else {
			return msg_req
		}
	}

	log.Printf("exiting transmit msg req")

	return MsgRequest{}
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
			go func() {
				if AmTokenSite() {
					resps <- 200
				}
			}()
			continue
		}
		go func(j int) {
			r := SendMsgToNode(msg, path, int64(j))
			if int64(j) == token_site {
				resps <- r
			}
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

	// If this particular msg ack is not stamped by the token site that I know
	// of, that implies that a token transfer was completed successfully
	// somewhere else in the network. Set my token site to the new token site
	// and recognize that a token transfer has been complete => commit any
	// messages that have been in the network long enough (i.e messages that
	// have seen LVal number of token transfers)
	if token_site != msg_ack.TokSite &&
		my_node_num != msg_ack.TokSite &&
		my_node_num != msg_ack.NextTokSite {
		token_site = msg_ack.TokSite
		next_token_site = token_site
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
	Commitment  map[int64]int64
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
		Commitment,
	}
}

func MyNodeNum() int64 {
	return my_node_num
}

func SendRetransmitReq(final_ts int64, have_req, have_ack bool) {
	m_rtr := BuildMsgRetransmitReq(my_node_num, final_ts, tlv, have_req, have_ack)
	resp := SendMsgToNode(m_rtr, MSG_RETRANSMIT_REQ_PATH, token_site)
	if resp != 200 {
		log.Printf("FAIL RTR DELIVER %d -> %d for %d", my_node_num,
			token_site, final_ts)
	} else {
		log.Printf("OK RTR DELIVER %d -> %d for %d", my_node_num,
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
	go InitTokenTransferTimeout(TOKEN_TRANSFER_TIME, 0)
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
