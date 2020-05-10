package prometheus

import (
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

func getPrometheusData() string {

	var resp, err = http.Get(baseUrl + SINGLE + "ledger_blockchain_height")
	if err != nil {
		log.Fatalln(err)
	}

	// get data from the response body
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatalln(err)
	}
	return string(body)
}

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

func query(queryString string, timePoint int64) *url.URL {
	queryStringParams := url.Values{}
	queryStringParams.Set("query", queryString)
	queryStringParams.Set("time", strconv.FormatInt(timePoint, 10))
	encodedQueryStringParams := queryStringParams.Encode()
	return retrieveTargetParameters(SINGLE, encodedQueryStringParams)
}

func rangeQuery(queryString string, startTime int64, endTime int64, step int) *url.URL {
	// url.Values will map values by sorting keys alphabetically
	qsParams := url.Values{}
	qsParams.Set("query", queryString)
	qsParams.Set("start", strconv.FormatInt(startTime, 10))
	qsParams.Set("end", strconv.FormatInt(endTime, 10))
	qsParams.Set("step", strconv.Itoa(step))
	encodedQueryStringParams := qsParams.Encode()
	return retrieveTargetParameters(RANGE, encodedQueryStringParams)
}


func readPrometheusData() {
	// data := getPrometheusData()
	// fmt.Println(data)
	timeNow := time.Now()
	endTimeUnix := timeNow.Unix()
	startTimeUnix := timeNow.Add(-time.Minute * 30).Unix() // 30 minutes earlier
	query("ledger_blockchain_height", startTimeUnix)
	rangeQuery("ledger_blockchain_height", startTimeUnix, endTimeUnix,7)
}
