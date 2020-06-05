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

type KvPair struct {
	Name string
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
			Metric struct { // metric fields are not always the same. They depend on specific metrics
				Name     string `json:"__name__"`
				Channel  string `json:"channel"`
				Instance string `json:"instance"`
				Job      string `json:"job"`
			} `json:"metric"`
			Value []interface{} `json:"value"`
		} `json:"result"`
	} `json:"data"`
}

type RangeVectorObject struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Metric struct {
				Name     string `json:"__name__"`
				Channel  string `json:"channel"`
				Instance string `json:"instance"`
				Job      string `json:"job"`
				Le  	 string `json:"le"`
			} `json:"metric"`
			Values [][]interface{} `json:"values"`
		} `json:"result"`
	} `json:"data"`
}

type Monitor interface {
	MetricName() string
	GetStatus() bool
	startHeartBeat()
}

type MonitoringService struct {
	metricName string
	status     bool
	interval   time.Duration
	timeout    time.Duration
}

func New(metricName string, interval time.Duration, timeout time.Duration) Monitor {
	return &MonitoringService{
		metricName: metricName,
		status:     false,
		interval:   interval,
		timeout:    timeout,
	}
}

func (m *MonitoringService) MetricName() string {
	return m.metricName
}

func (m *MonitoringService) GetStatus() bool {
	fmt.Println("STATUS ", m.status)
	return m.status
}

func (m *MonitoringService) startHeartBeat() {
	m.heartBeat(m.interval, m.timeout)
}

func (m *MonitoringService) heartBeat(interval time.Duration, timeout time.Duration) *bool {
	ticker := time.NewTicker(interval * time.Second)
	done := make(chan bool)
	go func() {
		for {
			select {
			case <-done:
				fmt.Println("done")
				return
			case t := <-ticker.C:
				status := m.healthChecker()
				if status == true {
					fmt.Println("Tick at ", t.Local())
					m.status = true
					ticker.Stop()
				} else {
					m.status = false
					fmt.Println("False. Retrying... ", t)
				}

			}
		}
	}()
	time.Sleep(timeout * time.Second)
	ticker.Stop()
	done <- true
	fmt.Println("Ticker stopped. Stopping heart beating and continuing other tasks...")
	fmt.Println("Heartbeat status ", &m.status)
	return &m.status
}

func (m *MonitoringService) healthChecker() bool {
	_, err := http.Get("http://localhost:9090")
	if err != nil {
		fmt.Println(err)
		return false
	}
	return true
}

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
func query(queryString string, timePoint int64) InstantVectorObject {
	queryStringParams := url.Values{}
	queryStringParams.Set("query", queryString)
	queryStringParams.Set("time", strconv.FormatInt(timePoint, 10))
	query := queryStringParams.Encode()
	targetParams := retrieveTargetParameters(SINGLE, query)
	res, _ := retrieveQueryResponse(targetParams)
	return res
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
	res, err := retrieveRangeQueryResponse(targetParams)
	if err != nil {
		return res
	} else {
		fmt.Println("error range query", err)
		return RangeVectorObject{}
	}
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
	matrixQuery := rangeQuery(metric, startTimeUnix, endTimeUnix, 7)
	firstValQueryRange, _ := extractFirstValueFromQueryRange(matrixQuery)
	fmt.Println("first val query range ", firstValQueryRange)
	stats, _ := extractStatisticFromQueryRange(matrixQuery, MAX, INSTANCE)
	rangeQueryOp <- stats
}

func execProm(metric string) {
	p := New(metric, 2, 10)
	p.startHeartBeat()
	status := p.GetStatus()
	if status == true {
		fmt.Println("yayy")
		vector := make(chan float64)
		matrix := make(chan KvMap)
		go callSingleQuery(vector, p.MetricName())
		go callRangeQuery(matrix, p.MetricName())
		singleQuery, rangeQuery := <-vector, <-matrix
		singleQuery1, rangeQuery1 := <-vector, <-matrix
		fmt.Println("Final output SINGLE query ", singleQuery, singleQuery1)
		fmt.Println("Final output RANGE query ", rangeQuery, rangeQuery1)
	}
}

func Main() {
	p := New("blockcutter_block_fill_duration", 2, 10)
	p1 := New("ledger_blockchain_height", 1, 10)

	p.startHeartBeat()
	p1.startHeartBeat()

	status := p.GetStatus()
	statusP1 := p1.GetStatus()

	if status == true && statusP1 == true {
		fmt.Println("yayy")
		vector := make(chan float64)
		matrix := make(chan KvMap)
		go callSingleQuery(vector, p.MetricName())
		go callSingleQuery(vector, p1.MetricName())
		go callRangeQuery(matrix, p.MetricName())
		go callRangeQuery(matrix, p1.MetricName())
		singleQuery, rangeQuery := <-vector, <-matrix
		singleQuery1, rangeQuery1 := <-vector, <-matrix
		fmt.Println("Final output SINGLE query ", singleQuery, singleQuery1)
		fmt.Println("Final output RANGE query ", rangeQuery, rangeQuery1)
	}

	/*	if healthChecker() {
			vector := make(chan float64)
			matrix := make(chan KvMap)
			fmt.Println("metric name ", metric)
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
		fmt.Println(test)*/

}
