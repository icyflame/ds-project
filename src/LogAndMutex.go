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
func LogAndMutex(inner http.Handler, name string, lock sync.Mutex, to_lock bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		if to_lock {
			lock.Lock()
		}

		inner.ServeHTTP(w, r)

		log.Printf(
			"%c\t%s\t%s",
			r.Method[0],
			r.RequestURI,
			time.Since(start),
		)

		if to_lock {
			lock.Unlock()
		}
	})
}
