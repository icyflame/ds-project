package project

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"io/ioutil"
	"log"
	"net/http"
	"os"
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

	log.Printf("NODE %d STARTED", my_node_num)
	log.Printf("Loading configuration from %s", config_file_path)
	log.Print("Configuration: ", config)

	r := mux.NewRouter()

	for _, route := range routes {
		r.HandleFunc(route.Pattern, route.Handler)
	}

	log.Printf("Listening on port 8080")
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", port), r))
}
