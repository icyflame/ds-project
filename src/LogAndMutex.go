package project

import (
	"log"
	"net/http"
	"sync"
	// "time"
)

// Map to indicate to this layer to drop requests.
// Will drop the next value number of requests that are made to the route that
// the key refers to
var DropReqs = map[string]int{}
var CheckDrop sync.Mutex

func InitDropReqs(DropReqs *map[string]int) {
	for _, r := range routes {
		(*DropReqs)[r.Name] = 0
	}
}

func DropMsgs(name string, n int) {
	DropReqs[name] += n
	log.Printf("RECD DROP REQ %s, %d", name, DropReqs[name])
}

func GetDropReqs() map[string]int {
	return DropReqs
}

var ActCrashed bool = false

func SetCrashed(t bool) {
	ActCrashed = t
}

// This function takes a normal handler, a lock variable and the name of a route
// and ensures that no other function has locked this variable before executing
// the handler. Once the handler is executed, the function will print out the
// time that the execution took.
func LogAndMutex(inner http.Handler, name string, lock sync.Mutex, to_lock bool) http.Handler {

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// start := time.Now()

		CheckDrop.Lock()

		if DropReqs[name] > 0 {
			DropReqs[name] -= 1

			log.Printf("DROP %s; REMAIN %d", name, DropReqs[name])

			CheckDrop.Unlock()

			return
		} else {
			CheckDrop.Unlock()

			if to_lock {
				lock.Lock()
			}

			inner.ServeHTTP(w, r)

			/*
			 * log.Printf(
			 *     "%c\t%s\t%s",
			 *     r.Method[0],
			 *     r.RequestURI,
			 *     time.Since(start),
			 * )
			 */

			if to_lock {
				lock.Unlock()
			}
		}
	})
}
