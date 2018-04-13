package main

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"log"
	"net/http"
	"os"
	"sort"
)

type CommitLog struct {
	Committed []int
	Node      int64
}

func PrinterHandler(w http.ResponseWriter, r *http.Request) {
	m_vals := r.PostFormValue("data")
	m := CommitLog{}
	err := json.Unmarshal([]byte(m_vals), &m)

	if err != nil {
		log.Fatal("Couldn't parse token transfer complete message: ", err)
	}

	sort.Ints(m.Committed)

	for _, v := range m.Committed {
		log.Printf("%d COMMIT TS %d", m.Node, v)
	}
}

func main() {

	if len(os.Args) < 2 {
		log.Fatal("Provide two arguments: ./exec port")
	}

	port := os.Args[1]

	r := mux.NewRouter()

	r.
		Methods("POST").
		Path("/").
		Name("commit_main_route").
		Handler(http.HandlerFunc(PrinterHandler))

	log.Printf("Listen %s", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", port), r))
}
