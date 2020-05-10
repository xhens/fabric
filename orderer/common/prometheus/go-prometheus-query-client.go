package prometheus

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

var baseUrl = "http://localhost:9090"
var SINGLE = "/api/v1/query"
var RANGE = "/api/v1/query_range"

func retrieveTargetParameters(queryType string, query string) *url.URL {
	base, _ := url.Parse(baseUrl) // domain = localhost:9090
	relativeUrl, _ := url.Parse(queryType) // api/v1/query or api/v1/query_range
	queryStringParams := relativeUrl.Query()
	params, _ := url.ParseQuery(query)
	for key, value := range params {
		queryStringParams.Set(key, value[0])
	}
	relativeUrl.RawQuery = queryStringParams.Encode()
	fullUrl := base.ResolveReference(relativeUrl)
	fmt.Println(fullUrl)
	return fullUrl
}

func query(queryString string, timePoint int64) interface{} {
	queryStringParams := url.Values{}
	queryStringParams.Set("query", queryString)
	queryStringParams.Set("time", strconv.FormatInt(timePoint, 10))
	encodedQueryStringParams := queryStringParams.Encode()
	targetParams := retrieveTargetParameters(SINGLE, encodedQueryStringParams)
	retrievedResponse := retrieveResponse(targetParams)
	return retrievedResponse
}

func rangeQuery(queryString string, startTime int64, endTime int64, step int) interface{} {
	// url.Values will map values by sorting keys alphabetically
	qsParams := url.Values{}
	qsParams.Set("query", queryString)
	qsParams.Set("start", strconv.FormatInt(startTime, 10))
	qsParams.Set("end", strconv.FormatInt(endTime, 10))
	qsParams.Set("step", strconv.Itoa(step))
	encodedQueryStringParams := qsParams.Encode()
	targetParams := retrieveTargetParameters(RANGE, encodedQueryStringParams)
	retrievedResponse := retrieveResponse(targetParams)
	return retrievedResponse
}

func doRequest(url string) []byte {
	method := "GET"
	client := &http.Client{}
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		log.Fatalf("Error %s ", err)
	}
	res, _ := client.Do(req)
	defer res.Body.Close()
	body, _ := ioutil.ReadAll(res.Body)
	return body
}

func retrieveResponse(url *url.URL) interface{} {
	response := doRequest(url.String())
	var data interface{}  // generic JSON with interface{}
	_ = json.Unmarshal(response, &data)
	result := data.(map[string]interface{})
	for key, value := range result {
		if key == "status" && value == "success" {
			log.Println(value)
		} else if key == "data" {
			fmt.Println(value)
			return value
		} else {
			log.Fatalf("Prometheus query to %s failed with %s", value, response)
			return nil
		}
	}
	return nil
}

func Main() {
	timeNow := time.Now()
	endTimeUnix := timeNow.Unix()
	startTimeUnix := timeNow.Add(-time.Minute * 30).Unix() // 30 minutes earlier
	query("ledger_blockchain_height", startTimeUnix)
	rangeQuery("ledger_blockchain_height", startTimeUnix, endTimeUnix,7)
}
