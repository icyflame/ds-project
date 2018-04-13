package project

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"log"
	"net/http"
)

func AcceptClientMessage(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	data := r.PostFormValue("data")

	log.Print("RECV CLIENT ", data)

	m := AcceptMessage(BuildMsgClient(Data(data)))

	if !NotEmptyReq(m) {
		// msg_req empty, token site didn't accept the message!
		http.Error(w, "Token site didn't accept the message, try again after some time", 503)
	} else {
		log.Printf("SEND ALL %d, %d", m.Sender, m.SenderSeq)

		fmt.Fprint(w, "Message accepted. Broadcasted to everyone, waiting for token site's ack.\n")
	}
}

func AcceptMsgRequestHandler(w http.ResponseWriter, r *http.Request) {
	msg_req_vals := r.PostFormValue("data")
	msg_req := MsgRequest{}
	err := json.Unmarshal([]byte(msg_req_vals), &msg_req)

	if err != nil {
		log.Fatal("Couldn't parse msg request: ", err)
	}

	log.Printf("RECV REQ %d, %d", msg_req.Sender, msg_req.SenderSeq)

	AcceptMsgRequest(msg_req)

	if AmTokenSite() {
		log.Println("STAMPED and BROADCASTED")

		// Indicate ACK to the requester
		fmt.Fprint(w, "")
	}
}

func AcceptMsgAckHandler(w http.ResponseWriter, r *http.Request) {
	msg_ack_vals := r.PostFormValue("data")
	msg_ack := MsgAck{}
	err := json.Unmarshal([]byte(msg_ack_vals), &msg_ack)

	if err != nil {
		log.Fatal("Couldn't parse Msg ack: ", err)
	}

	log.Printf("RECV ACK %d, %d, TS %d", msg_ack.Sender, msg_ack.SenderSeq, msg_ack.FinalTS)

	SequenceMsg(msg_ack)
}

func HealthReqHandler(w http.ResponseWriter, r *http.Request) {
	t := GetHealthInfo()

	fmt.Fprintf(w, "Node: %d\nToken site: %d\nNTS: %d\nTLV: %d\n",
		t.MyNum,
		t.TokSite,
		t.Nts,
		t.Tlv,
	)

	fmt.Fprintf(w, "Next Token Site: %d\n", t.NextTokSite)

	fmt.Fprintf(w, "Commitment: \n")
	for k, v := range t.Commitment {
		fmt.Fprintf(w, "%d = %d\n", k, v)
	}

	fmt.Fprint(w, "\n\nQB:\n")
	for k, v := range t.Qb {
		fmt.Fprintf(w, "\n\t%s: ", k)
		fmt.Fprintf(w, "\n\t\t%#v", v)
	}

	fmt.Fprintf(w, "\n\nQC: %d msgs\n", len(t.Qc))
	for _, msg := range t.Qc {
		fmt.Fprintf(w, "\n\t%#v", msg)
	}

	fmt.Fprint(w, "\n\nDROP REQUESTS: \n")
	for k, v := range GetDropReqs() {
		fmt.Fprintf(w, "\n\t%s\t=%d", k, v)
	}
}

func RetransmissionReqHandler(w http.ResponseWriter, r *http.Request) {
	if !AmTokenSite() {
		http.Error(w, "", 401)
		return
	}

	msg_rtr_vals := r.PostFormValue("data")
	msg_rtr := MsgRetransmitReq{}
	err := json.Unmarshal([]byte(msg_rtr_vals), &msg_rtr)

	if err != nil {
		log.Fatal("Couldn't parse retranmission request: ", err)
	}

	log.Printf("RECV RTR %d -> %d for %d", msg_rtr.Sender,
		MyNodeNum(), msg_rtr.FinalTS)

	fmt.Fprint(w, "")
	RetransmitMsg(msg_rtr)
}

func DropMsgsHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	count := 0
	fmt.Sscanf(vars["count"], "%d", &count)

	DropMsgs(vars["name"], count)
}

func TokenTransferInitHandler(w http.ResponseWriter, r *http.Request) {
	if !AmTokenSite() {
		// Only token sites can initiate token transfers
		http.Error(w, "", 400)
		return
	}

	msg_tti_vals := r.PostFormValue("data")
	msg_tti := MsgTokenTransferInit{}
	err := json.Unmarshal([]byte(msg_tti_vals), &msg_tti)

	if err != nil {
		log.Fatal("Couldn't parse token transfer init message: ", err)
	}

	InitSendTokenMode()

	count := BringNodeUpToDate(msg_tti.Destination, msg_tti.Nts)
	log.Printf("SEND %d msgs -> %d", count, msg_tti.Destination)

	IndicateTokTransferComplete(msg_tti.Destination)
	log.Printf("SEND TOK TRANSFER COMPLETE -> %d", msg_tti.Destination)

	RelinquishTokSite()
	log.Print("GIVING AWAY TOK SITE RESP")
}

func TokenTransferCompleteHandler(w http.ResponseWriter, r *http.Request) {
	msg_ttc_vals := r.PostFormValue("data")
	msg_ttc := MsgTokenTransferComplete{}
	err := json.Unmarshal([]byte(msg_ttc_vals), &msg_ttc)

	if err != nil {
		log.Fatal("Couldn't parse token transfer complete message: ", err)
	}

	BecomeTokenSite()
	log.Println("BECOME TOK SITE")
}
