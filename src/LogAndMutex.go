package project

import (
	"log"
	"net/http"
	"sync"
	"time"
)

// This function takes a normal handler, a lock variable and the name of a route
// and ensures that no other function has locked this variable before executing
// the handler. Once the handler is executed, the function will print out the
// time that the execution took.
func LogAndMutex(inner http.Handler, name string, lock sync.Mutex) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		lock.Lock()

		inner.ServeHTTP(w, r)

		log.Printf(
			"%s\t%s\t%s\t%s",
			r.Method,
			r.RequestURI,
			name,
			time.Since(start),
		)

		lock.Unlock()
	})
}
