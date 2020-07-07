package prometheus

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

type KvPair struct {
	Name  string
	Value float64
}

type KvMap struct {
	Items []KvPair
}

func (valuesMap *KvMap) AddItem(item KvPair) []KvPair {
	valuesMap.Items = append(valuesMap.Items, item)
	return valuesMap.Items
}

type InstantVectorObject struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Metric struct {
				Name     string `json:"__name__"`
				Channel  string `json:"channel"`
				Instance string `json:"instance"`
				Job      string `json:"job"`
			} `json:"metric"`
			Value []interface{} `json:"value"`
		} `json:"result"`
	} `json:"data"`
}

type GenericRangeVector struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Metric struct {
				Name            string `json:"__name__"`
				Chaincode       string `json:"chaincode"`
				Channel         string `json:"channel"`
				DataType        string `json:"data_type"`
				Filtered        string `json:"filtered"`
				Instance        string `json:"instance"`
				Job             string `json:"job"`
				TransactionType string `json:"transaction_type"`
				ValidationCode  string `json:"validation_code"`
				Status          string `json:"status"`
				Le              string `json:"le"`
			} `json:"metric"`
			Values [][]interface{} `json:"values"`
		}
	} `json:"data"`
}

type MetricError struct {
	status    string
	errorType string
	error     string
}

// TODO: testing locally can be localhost. testing on fabric should be prometheus.
var baseUrl = ContainerHost
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
	logger.Debug("Generated URL: ", fullUrl)
	return fullUrl
}

func retrieveInstantQueryResponse(url *url.URL) (InstantVectorObject, error) {
	response, _ := doRequest(url.String())
	data := InstantVectorObject{}
	err := json.Unmarshal(response, &data)
	if data.Status == Success {
		if data.Data.ResultType == Vector {
			return data, nil
		} else {
			return InstantVectorObject{}, err
		}
	} else if data.Status == Fail {
		log.Fatalf("Prometheus query to %s with %s failed with %s", url.Path, url.RawQuery, data.Status)
		return InstantVectorObject{}, err
	} else if err != nil {
		log.Fatalf("Query error %v", err)
		return InstantVectorObject{}, err
	}
	return InstantVectorObject{}, err
}

func instantQuery(queryString string, timePoint float64) InstantVectorObject {
	queryStringParams := url.Values{}
	queryStringParams.Set("query", queryString)
	queryStringParams.Set("time", strconv.FormatFloat(timePoint, 'f', -1, 64))
	query := queryStringParams.Encode()
	targetParams := retrieveTargetParameters(SINGLE, query)
	res, _ := retrieveInstantQueryResponse(targetParams)
	return res
}

func retrieveGenericRangeQueryResponse(url *url.URL) (GenericRangeVector, error) {
	response, _ := doRequest(url.String())
	var data GenericRangeVector
	err := json.Unmarshal(response, &data)
	if data.Status == Success {
		if data.Data.ResultType == Matrix {
			return data, nil
		} else {
			return GenericRangeVector{}, err
		}
	} else if data.Status == Fail {
		log.Fatalf("Prometheus query to %s with %s failed with %s", url.Path, url.RawQuery, data.Status)
		return GenericRangeVector{}, err
	} else if err != nil {
		log.Fatalf("Query error %v", err)
		return GenericRangeVector{}, err
	}
	return GenericRangeVector{}, err
}

func rangeGenericQuery(queryString string, startTime int64, endTime int64, step int) GenericRangeVector {
	qsParams := url.Values{}
	qsParams.Set("query", queryString)
	qsParams.Set("start", strconv.FormatInt(startTime, 10))
	qsParams.Set("end", strconv.FormatInt(endTime, 10))
	qsParams.Set("step", strconv.Itoa(step))
	query := qsParams.Encode()
	targetParams := retrieveTargetParameters(RANGE, query)
	// TODO: Be careful of error handling here. it returned empty response.
	res, _ := retrieveGenericRangeQueryResponse(targetParams)
	return res
}

// TODO: Improve error handling
func doRequest(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		// set logger to Printf because using Panic or Fatal will cause the Docker Container to stop
		log.Printf("Req Error %v", err)
		return nil, err
	} else if resp.StatusCode != http.StatusOK {
		logger.Debug("Bad request. Status code ", resp.StatusCode)
	}
	resp.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	defer resp.Body.Close()

	body, readErr := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("ioutil read error %v", readErr)
		return nil, err
	}
	return body, nil
}

func callSingleQuery(singleQueryOp chan float64, metric string) {
	timeNow := time.Now()
	endTimeUnix := timeNow.Unix()
	singleQuery := instantQuery(metric, float64(endTimeUnix))
	firstValue, err := extractFirstValueFromSingleQuery(singleQuery)
	if err != nil {
		logger.Debug(err)
	}
	singleQueryOp <- firstValue
}

func callGenericRangeQuery(rangeQueryOp chan KvMap, metric string, timeRange time.Duration, statType string, label string, firstValue bool) {
	timeNow := time.Now()
	endTimeUnix := timeNow.Unix()
	startTimeUnix := timeNow.Add(-time.Minute * timeRange).Unix() // x minutes earlier
	matrixQuery := rangeGenericQuery(metric, startTimeUnix, endTimeUnix, 7)
	if firstValue == true {
		firstValQueryRange, _ := extractFirstValueFromGenericQueryRange(matrixQuery)
		log.Print("First value of query range: ", firstValQueryRange)
	}
	stats, _ := extractStatisticFromGenericQueryRange(matrixQuery, statType, label)
	rangeQueryOp <- stats
}
