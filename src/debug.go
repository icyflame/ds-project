package project

import (
	"encoding/json"
	"log"
	"net/http"
	"net/url"
)

func LogCommit(
	committed []int,
	node int64,
) {
	type CommitLog struct {
		Committed []int
		Node      int64
	}

	c := CommitLog{
		committed,
		node,
	}

	dat, _ := json.Marshal(c)
	dat_vals := url.Values{}
	dat_vals.Add("data", string(dat))

	_, err := http.PostForm(
		"http://localhost:8079",
		dat_vals,
	)

	if err != nil {
		log.Fatal(err)
	}

	return
}
