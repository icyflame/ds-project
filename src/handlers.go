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

	log.Printf("SEND ALL %d, %d", m.Sender, m.SenderSeq)

	fmt.Fprint(w, "Message accepted. Broadcasted to everyone, waiting for token site's ack.\n")
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
