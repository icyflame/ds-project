package project

import (
	"fmt"
	"log"
	"net/http"
	"time"
)

func Handler1(w http.ResponseWriter, r *http.Request) {
	time.Sleep(5 * time.Second)
	fmt.Fprint(w, "handling 1")
}

func Handler2(w http.ResponseWriter, r *http.Request) {
	time.Sleep(7 * time.Second)
	fmt.Fprint(w, "handling 2")
}

func AcceptClientMessage(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	data := r.PostFormValue("data")

	log.Print("Sending msg from client for acceptance: ", data)

	stamped, msg_ack := AcceptMessage(BuildMsgClient(Data(data)))

	if stamped {
		fmt.Fprintf(w, "Message accepted, final timestamp: %d", msg_ack.FinalTS)
	} else {
		fmt.Fprint(w, "Message accepted and forwarded to token site")
	}
}
