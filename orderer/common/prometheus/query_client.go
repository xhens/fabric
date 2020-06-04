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
	base, _ := url.Parse(baseUrl)          // domain = localhost:9090
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

func retrieveQueryResponse(url *url.URL) (InstantVectorObject, error) {
	response, _ := doRequest(url.String())
	data := InstantVectorObject{}
	err := json.Unmarshal(response, &data)
	if data.Status == SUCCESS {
		if data.Data.ResultType == VECTOR {
			return data, nil
		} else {
			return InstantVectorObject{}, err
		}
	} else if data.Status == FAIL {
		log.Fatalf("Prometheus query to %s with %s failed with %s", url.Path, url.RawQuery, data.Status)
		return InstantVectorObject{}, err
	} else if err != nil {
		log.Fatalf("Query error %v", err)
		return InstantVectorObject{}, err
	}
	return InstantVectorObject{}, err
}

// TODO: refactor timePoint to float64
func query(queryString string, timePoint int64) (InstantVectorObject, error) {
	queryStringParams := url.Values{}
	queryStringParams.Set("query", queryString)
	queryStringParams.Set("time", strconv.FormatInt(timePoint, 10))
	query := queryStringParams.Encode()
	targetParams := retrieveTargetParameters(SINGLE, query)
	res, err := retrieveQueryResponse(targetParams)
	return res, err
}

func retrieveRangeQueryResponse(url *url.URL) (RangeVectorObject, error) {
	response, _ := doRequest(url.String())
	data := RangeVectorObject{}
	err := json.Unmarshal(response, &data)
	if data.Status == SUCCESS {
		if data.Data.ResultType == MATRIX {
			return data, nil
		} else {
			return RangeVectorObject{}, err
		}
	} else if data.Status == FAIL {
		log.Fatalf("Prometheus query to %s with %s failed with %s", url.Path, url.RawQuery, data.Status)
		return RangeVectorObject{}, err
	} else if err != nil {
		log.Fatalf("Query error %v", err)
		return RangeVectorObject{}, err
	}
	return RangeVectorObject{}, err
}

// @param step: interval (ex: every x minutes)
// TODO: refactor func args to float64
func rangeQuery(queryString string, startTime int64, endTime int64, step int) RangeVectorObject {
	qsParams := url.Values{}
	qsParams.Set("query", queryString)
	qsParams.Set("start", strconv.FormatInt(startTime, 10))
	qsParams.Set("end", strconv.FormatInt(endTime, 10))
	qsParams.Set("step", strconv.Itoa(step))
	query := qsParams.Encode()
	targetParams := retrieveTargetParameters(RANGE, query)
	res, _ := retrieveRangeQueryResponse(targetParams)
	return res
}

func doRequest(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil || resp.StatusCode != http.StatusOK {
		log.Fatalf("Error %v", err)
		return nil, err
	}
	resp.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	defer resp.Body.Close()

	body, err1 := ioutil.ReadAll(resp.Body)
	if err1 != nil {
		log.Fatalf("Error %v", err1)
		return nil, err1
	}
	return body, nil
}

func healthChecker() bool {
	_, err := http.Get("http://localhost:9090")
	if err != nil {
		fmt.Println(err)
		return false
	}
	return true
}

func heartBeat(interval time.Duration, timeout time.Duration) bool {
	ticker := time.NewTicker(interval * time.Second)
	done := make(chan bool)
	go func() {
		for {
			select {
			case <-done:
				fmt.Println("done")
				return
			case t := <-ticker.C:
				status := healthChecker()
				if status == true {
					fmt.Println("Tick at ", t.Local())
					ticker.Stop()
				} else {
					fmt.Println("false. tick at ", t)
				}

			}
		}
	}()
	time.Sleep(timeout * time.Minute)
	ticker.Stop()
	done <- true
	fmt.Println("ticker stopped")
	return false
}

func callSingleQuery(singleQueryOp chan float64, metric string) {
	timeNow := time.Now()
	endTimeUnix := timeNow.Unix()
	singleQuery := query(metric, endTimeUnix)
	firstValue, err := extractFirstValueFromSingleQuery(singleQuery)
	fmt.Println(err)
	fmt.Println("first val ", firstValue)
	singleQueryOp <- firstValue
}

func callRangeQuery(rangeQueryOp chan KvMap, metric string) {
	timeNow := time.Now()
	endTimeUnix := timeNow.Unix()
	startTimeUnix := timeNow.Add(-time.Minute * 30).Unix() // 30 minutes earlier
	matrixQuery := rangeQuery(metric, startTimeUnix, endTimeUnix,7)
	firstValQueryRange, _ := extractFirstValueFromQueryRange(matrixQuery)
	fmt.Println("first val query range ", firstValQueryRange)
	stats, _ := extractStatisticFromQueryRange(matrixQuery, MAX, INSTANCE)
	rangeQueryOp <- stats
}

func Main() int {
	if healthChecker() {
		vector := make(chan float64)

		matrix := make(chan KvMap)
		metric := "blockcutter_block_fill_duration"
		go callSingleQuery(vector, metric)
		go callRangeQuery(matrix, metric)
		singleQuery, rangeQuery := <-vector, <-matrix
		fmt.Println("Final output SINGLE query ", singleQuery)
		fmt.Println("Final output RANGE query ", rangeQuery)
		close(vector)
		for item := range vector {
			fmt.Println("vector ", item)
		}

		close(matrix)
		for item := range matrix {
			fmt.Println("matrix ", item)
		}
	}

	test := heartBeat(2, 1)
	fmt.Println(test)
}
