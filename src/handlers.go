package project

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

func AcceptClientMessage(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	data := r.PostFormValue("data")

	log.Print("Broadcasting msg from client: ", data)

	AcceptMessage(BuildMsgClient(Data(data)))

	log.Print("Message accepted. Broadcasted to everyone, waiting for token site's ack")

	fmt.Fprint(w, "Message accepted. Broadcasted to everyone, waiting for token site's ack")
}

func AcceptMsgRequestHandler(w http.ResponseWriter, r *http.Request) {
	msg_req_vals := r.PostFormValue("data")
	msg_req := MsgRequest{}
	err := json.Unmarshal([]byte(msg_req_vals), &msg_req)

	if err != nil {
		fmt.Fprint(w, "ERROR: ", err)
	}

	AcceptMsgRequest(msg_req)

	log.Println("Message request received")
	if AmTokenSite() {
		log.Println("I have timestamped this message and broadcasted ACKs to everyone")
		// Indicate ACK to the requester
		fmt.Fprint(w, "")
	}
}

func AcceptMsgAckHandler(w http.ResponseWriter, r *http.Request) {
	msg_ack_vals := r.PostFormValue("data")
	msg_ack := MsgAck{}
	err := json.Unmarshal([]byte(msg_ack_vals), &msg_ack)

	if err != nil {
		fmt.Fprint(w, "ERROR: ", err)
	}

	log.Printf("Message ack recd: %#v", msg_ack)

	SequenceMsg(msg_ack)
}
