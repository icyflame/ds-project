package project

import (
	"fmt"
	"github.com/gorilla/mux"
	"log"
	"net/http"
)

func Main() {
	fmt.Println("hello world")

	r := mux.NewRouter()

	for _, route := range routes {
		r.HandleFunc(route.Pattern, route.Handler)
	}

	log.Fatal(http.ListenAndServe(":8080", r))
}
