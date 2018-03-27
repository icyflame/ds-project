package project

// Type of data that the messages in this atomic broadcast implementation are
// exchanging
type Data string

// Message request that is broadcasted from the source to all receivers
type MsgRequest struct {
	sender     int64
	sender_seq int64
	tlv        int64
	data       Data
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
	sender        int64
	tok_site      int64
	final_ts      int64
	tlv           int64
	next_tok_site int64
}

// Function to build the acknowledgement message
func BuildMsgAck(
	sender int64,
	tok_site int64,
	final_ts int64,
	tlv int64,
	next_tok_site int64,
) MsgAck {
	return MsgAck{
		sender,
		tok_site,
		final_ts,
		tlv,
		next_tok_site,
	}
}

// Retransmit request that is unicast from the receiver who has missed a message
// to the current token site
type RetransmitReq struct {
	tlv           int64
	final_ts_reqd int64
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
