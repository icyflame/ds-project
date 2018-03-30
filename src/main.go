package project

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"sync"
)

type Peer struct {
	IP string
}

type Config struct {
	Peers       map[int64]Peer
	InitTokSite int64
	LVal        int64
}

func Main() {

	if len(os.Args) < 4 {
		log.Fatal("Provide two arguments: ./executable config_file_path node_num port")
	}

	config_file_path := os.Args[1]

	var my_node_num int64
	fmt.Sscanf(os.Args[2], "%d", &my_node_num)

	port := os.Args[3]

	_, err := os.Stat(config_file_path)
	if os.IsNotExist(err) {
		log.Fatal("Config file must exist: ", err)
	}

	dat, err := ioutil.ReadFile(config_file_path)
	if err != nil {
		log.Fatal("Couldn't read config file: ", err)
	}
	config := Config{}

	err = json.Unmarshal(dat, &config)
	if err != nil {
		log.Fatal("Couldn't parse config file: ", err)
	}

	if my_node_num >= int64(len(config.Peers)) {
		log.Fatal("Node number must be between 0 and number of peers at the beginning of operation")
	}

	log.Printf("Node %d/%d", my_node_num, len(config.Peers))
	log.Printf("Configuration: %s", config_file_path)

	InitFromConfig(config, my_node_num)

	if AmTokenSite() {
		log.Print("I am the TokSite")
	}

	MutexVars := map[string]sync.Mutex{}

	r := mux.NewRouter()

	for _, route := range routes {
		MutexVars[route.Name] = sync.Mutex{}
	}

	for _, route := range routes {
		r.
			Methods(route.Method).
			Path(route.Pattern).
			Name(route.Name).
			Handler(LogAndMutex(route.Handler, route.Name, MutexVars[route.Name], route.SingleHandler))
	}

	log.Printf("Listen %s", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", port), r))
}
