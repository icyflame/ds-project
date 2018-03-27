package project

import (
	"fmt"
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
