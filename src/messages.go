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

// Retransmit request that is unicast from the receiver who has missed a message
// to the current token site
type RetransmitReq struct {
	Tlv         int64
	FinalTSReqd int64
}

// Function to build the retransmit request message
func BuildRetransmitReq(
	tlv int64,
	final_ts int64,
) RetransmitReq {
	return RetransmitReq{
		tlv,
		final_ts,
	}
}
