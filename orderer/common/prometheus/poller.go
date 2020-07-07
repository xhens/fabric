package prometheus

import (
	"fmt"
	"github.com/hyperledger/fabric/common/flogging"
	"log"
	"net/http"
	"time"
)

// Used only in Fabric (not in this repo)
var logger = flogging.MustGetLogger("orderer.common.prometheus")

// TODO: Some of these constants might need to be variables
const (
	numPollers     = 2                // number of Poller goroutines to launch
	pollInterval   = 4 * time.Second  // how often to poll each URL
	statusInterval = 2 * time.Second  // how often to log status to stdout
	errTimeout     = 10 * time.Second // back-off timeout on error
	successTimeout = 600
)

type MetricMonitor struct {
	Metric     string
	MetricType string
	Label      string
	StatType   string
	Status     string
	Healthy    bool
	Value      float64
}

// StateMonitor maintains a map that stores the state of the URLs being
// polled, and prints the current state every updateInterval nanoseconds.
// It returns a chan MetricMonitor to which resource state should be sent.
func (metricMonitor *MetricMonitor) stateMonitor(updateInterval time.Duration, successTimeout time.Duration) chan<- MetricMonitor {
	updates := make(chan MetricMonitor)
	urlStatus := make(map[string]bool)
	ticker := time.NewTicker(updateInterval)
	succTimeout := time.After(successTimeout * time.Second)
	start := time.Now()
	go func() {
		for {
			select {
			case <-ticker.C:
				metricMonitor.logState(urlStatus)
			case monitoringState := <-updates:
				urlStatus[HealthEndpoint] = metricMonitor.Healthy
				metricMonitor.Healthy = monitoringState.Healthy
				// metricMonitor.HealthEndpoint = monitoringState.HealthEndpoint
				metricMonitor.Status = monitoringState.Status
			case <-succTimeout: // TODO: should return only if successful
				elapsedTime := time.Since(start)
				logger.Debug("Elapsed time: ", elapsedTime)
				return
			}
		}
	}()
	return updates
}

// logState prints a state map
func (metricMonitor *MetricMonitor) logState(s map[string]bool) {
	for k, v := range s {
		log.Printf("Current state: %v %v", k, v)
	}
}

// Resource represents an HTTP URL to be polled
type Resource struct {
	url          string
	errCount     int
	successCount int
}

func (r *Resource) poll() (string, int) {
	resp, err := http.Get(r.url)
	if err != nil {
		logger.Debug("Error", r.url, err)
		r.errCount++
		fmt.Println(r.errCount)
		return err.Error(), 0
	}
	r.errCount = 0
	r.successCount++
	log.Print("Fetch nr: ", r.successCount)
	return resp.Status, resp.StatusCode
}

func (r *Resource) sleep(done chan<- *Resource) {
	time.Sleep(pollInterval + errTimeout*time.Duration(r.errCount))
	done <- r
}

func (metricMonitor *MetricMonitor) poller(in <-chan *Resource, out chan<- *Resource, status chan<- MetricMonitor) {
	for r := range in {
		statusMsg, statusCode := r.poll()
		if statusCode == http.StatusOK {
			metricMonitor.Healthy = true
		} else {
			metricMonitor.Healthy = false
		}
		status <- MetricMonitor{Status: statusMsg, Healthy: metricMonitor.Healthy}
		out <- r
	}
}


func (metricMonitor *MetricMonitor) updateMetricValue(values KvMap) float64 {
	var valSlice []float64
	for _, pair := range values.Items {
		valSlice = append(valSlice, pair.Value)
	}
	max := findMaxOfFloatSlice(valSlice)
	metricMonitor.Value = max
	return metricMonitor.Value
}

func (metricMonitor *MetricMonitor) executeQuery() {
	if metricMonitor.MetricType == Vector {
		vector := make(chan float64)
		go callSingleQuery(vector, metricMonitor.Metric)
		queryValue := <-vector
		log.Println("Metric: ", metricMonitor.Metric, "| Metric Type: ", metricMonitor.MetricType, "| Query Values: ", queryValue)
		// return queryValue
	} else if metricMonitor.MetricType == Matrix {
		matrix := make(chan KvMap)
		go callGenericRangeQuery(matrix, metricMonitor.Metric, 10, metricMonitor.StatType, metricMonitor.Label, false)
		queryValue := <-matrix
		metricMonitor.updateMetricValue(queryValue)
		log.Println("Metric: ", metricMonitor.Metric, "| Metric Type: ", metricMonitor.MetricType, "| Query Values: ", queryValue)
	} else if metricMonitor.MetricType != Vector && metricMonitor.MetricType != Matrix {
		log.Println("Unknown structure type")
	}
	// return 0
}

func (metricMonitor *MetricMonitor) RunOnce() float64 {
	pending, complete := make(chan *Resource), make(chan *Resource)
	var metricValue float64
	if metricMonitor.Healthy == true {
		metricMonitor.executeQuery()
		metricValue = metricMonitor.Value
		fmt.Println(metricMonitor.Value)
		return metricValue
	} else {
		fmt.Println("Waiting to recover...")
	}
	status := metricMonitor.stateMonitor(statusInterval, successTimeout)
	go metricMonitor.poller(pending, complete, status)
	pending <- &Resource{url: HealthEndpoint}
	for r := range complete {
		go r.sleep(pending)
	}
	return metricValue
}

func (metricMonitor *MetricMonitor) Run() float64{
	// Create our input and output channels
	pending, complete := make(chan *Resource), make(chan *Resource)
	var metricVal float64

	go func() {
		ticker := time.NewTicker(pollInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if metricMonitor.Healthy == true {
					metricMonitor.executeQuery()
					metricVal = metricMonitor.Value

				} else {
					log.Println("Waiting to recover...")
				}
			case <-pending:
				log.Println("pending", pending)
				return
			}
		}
	}()

	status := metricMonitor.stateMonitor(statusInterval, successTimeout)
	go metricMonitor.poller(pending, complete, status)
	pending <- &Resource{url: HealthEndpoint}

	for r := range complete {
		go r.sleep(pending)
	}
	return metricVal
}
