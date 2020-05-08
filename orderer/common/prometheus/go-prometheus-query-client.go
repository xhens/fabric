package prometheus

import (
	"io/ioutil"
	"log"
	"net/http"
)

var url = "http://localhost:9090/"
var apiVersion = "api/v1/"
var query = "query?query="

func prometheusGet() {

	var resp, err = http.Get(url + apiVersion + query + "ledger_blockchain_height")
	if err != nil {
		log.Fatalln(err)
	}

	defer resp.Body.Close()

	body, error := ioutil.ReadAll(resp.Body)
	if error != nil {
		log.Fatalln(error)
	}

	log.Println(string(body))
}

func Main() {
	prometheusGet()
}