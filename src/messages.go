package project

// Type of data that the messages in this atomic broadcast implementation are
// exchanging
type Data string

// Message as constructed by the client, this needs to be converted into a
// MsgRequest
type MsgClient struct {
	Content Data
}

func BuildMsgClient(data Data) MsgClient {
	return MsgClient{
		data,
	}
}

// Message request that is broadcasted from the source to all receivers
type MsgRequest struct {
	Sender    int64
	SenderSeq int64
	Tlv       int64
	Content   Data
}

// Function to build a message request
func BuildMsgRequest(
	sender int64,
	sender_seq int64,
	tlv int64,
	data Data,
) MsgRequest {
	return MsgRequest{
		sender,
		sender_seq,
		tlv,
		data,
	}
}

// Message acknowledgement broadcasted from the token site to all receivers
// after it has been timestamped
type MsgAck struct {
	Sender      int64
	SenderSeq   int64
	TokSite     int64
	FinalTS     int64
	Tlv         int64
	NextTokSite int64
}

// Function to build the acknowledgement message
func BuildMsgAck(
	sender int64,
	sender_seq int64,
	tok_site int64,
	final_ts int64,
	tlv int64,
	next_tok_site int64,
) MsgAck {
	return MsgAck{
		sender,
		sender_seq,
		tok_site,
		final_ts,
		tlv,
		next_tok_site,
	}
}

type MsgWithFinalTS struct {
	Sender      int64
	SenderSeq   int64
	TokSite     int64
	Tlv         int64
	FinalTS     int64
	NextTokSite int64
	Content     Data
}

func BuildMsgWithFinalTS(
	msg_req MsgRequest,
	msg_ack MsgAck,
) MsgWithFinalTS {
	return MsgWithFinalTS{
		msg_req.Sender,
		msg_req.SenderSeq,
		msg_ack.TokSite,
		msg_req.Tlv,
		msg_ack.FinalTS,
		msg_ack.NextTokSite,
		msg_req.Content,
	}
}

func GetMsgAckFromMWFTS(
	m_wfts MsgWithFinalTS,
) MsgAck {
	return BuildMsgAck(
		m_wfts.Sender,
		m_wfts.SenderSeq,
		m_wfts.TokSite,
		m_wfts.FinalTS,
		m_wfts.Tlv,
		m_wfts.NextTokSite,
	)
}

type MsgPriorityQueue []*MsgWithFinalTS

func (a MsgPriorityQueue) Len() int {
	return len(a)
}

func (a MsgPriorityQueue) Less(i, j int) bool {
	return a[i].FinalTS < a[j].FinalTS
}

func (a MsgPriorityQueue) Swap(i, j int) {
	t := a[i]
	a[i] = a[j]
	a[j] = t
}

func (a *MsgPriorityQueue) Push(x interface{}) {
	*a = append(*a, x.(*MsgWithFinalTS))
}

func (a *MsgPriorityQueue) Pop() interface{} {
	old := *a
	n := len(old)

	popped := old[n-1]

	*a = old[:n-1]

	return popped
}

type MsgRetransmitReq struct {
	Sender  int64
	FinalTS int64
	Tlv     int64
}

func BuildMsgRetransmitReq(
	sender int64,
	final_ts int64,
	tlv int64,
) MsgRetransmitReq {
	return MsgRetransmitReq{
		sender,
		final_ts,
		tlv,
	}
}
