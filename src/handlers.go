package project

import (
	"fmt"
	"net/http"
)

func Handler1(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "handling 1")
}

func Handler2(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "handling 2")
}
