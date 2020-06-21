package prometheus

import (
	"fmt"
	"github.com/hyperledger/fabric/common/flogging"
	"log"
	"net/http"
	"time"
)

var flogger = flogging.MustGetLogger("orderer.common.prometheus")

const (
	numPollers     = 2                // number of Poller goroutines to launch
	pollInterval   = 5 * time.Second  // how often to poll each URL
	statusInterval = 2 * time.Second  // how often to log status to stdout
	errTimeout     = 10 * time.Second // back-off timeout on error
	successTimeout = 60
)

// State represents the last-known state of a URL
type State struct {
	HealthEndpoint string
	Metric         string
	Status         string
	Healthy        bool
}

// StateMonitor maintains a map that stores the state of the URLs being
// polled, and prints the current state every updateInterval nanoseconds.
// It returns a chan State to which resource state should be sent.
func (state *State) StateMonitor(updateInterval time.Duration, successTimeout time.Duration) chan<- State {
	updates := make(chan State)
	urlStatus := make(map[string]bool)
	ticker := time.NewTicker(updateInterval)
	succTimeout := time.After(successTimeout * time.Second)
	start := time.Now()
	go func() {
		for {
			select {
			case <-ticker.C:
				state.logState(urlStatus)
			case monitoringState := <-updates:
				urlStatus[monitoringState.HealthEndpoint] = state.Healthy
				state.Healthy = monitoringState.Healthy
				state.HealthEndpoint = monitoringState.HealthEndpoint
				state.Status = monitoringState.Status
			case <-succTimeout: // TODO: should return only if successful
				elapsedTime := time.Since(start)
				log.Println("Elapsed time: ", elapsedTime)
				return
			}
		}
	}()
	return updates
}

// logState prints a state map
func (state *State) logState(s map[string]bool) {
	for k, v := range s {
		log.Printf("Current state: %v %v", k, v)
	}
}

// Resource represents an HTTP URL to be polled by this program
type Resource struct {
	url          string
	errCount     int
	successCount int
}

func (r *Resource) Poll() (string, int) {
	resp, err := http.Get(r.url)
	if err != nil {
		log.Println("Error", r.url, err)
		r.errCount++
		fmt.Println(r.errCount)
		return err.Error(), 0
	}
	r.errCount = 0
	r.successCount++
	log.Print("Fetch nr: ", r.successCount)
	return resp.Status, resp.StatusCode
}

func (r *Resource) Sleep(done chan<- *Resource) {
	time.Sleep(pollInterval + errTimeout*time.Duration(r.errCount))
	done <- r
}

func (state *State) Poller(in <-chan *Resource, out chan<- *Resource, status chan<- State) {
	for r := range in {
		statusMsg, statusCode := r.Poll()
		if statusCode == http.StatusOK {
			state.Healthy = true
		} else {
			state.Healthy = false
		}
		status <- State{HealthEndpoint: state.HealthEndpoint, Status: statusMsg, Healthy: state.Healthy}
		out <- r
	}
}

func (state *State) Run() {
	// Create our input and output channels
	pending, complete := make(chan *Resource), make(chan *Resource)

	go func() {
		ticker := time.NewTicker(pollInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if state.Healthy == true {
					vectorValue := make(chan float64)
					go callSingleQuery(vectorValue, state.Metric)
					singleQueryValue := <-vectorValue
					log.Println("Single query: ", singleQueryValue)
				} else {
					log.Println("Waiting to recover...")
				}
			case <-pending:
				log.Println("pending", pending)
				return
			}
		}
	}()

	// Launch the StateMonitor
	status := state.StateMonitor(statusInterval, successTimeout)

	// Launch same Poller goroutines
	for i := 0; i < numPollers; i++ {
		go state.Poller(pending, complete, status)
	}

	pending <- &Resource{url: state.HealthEndpoint}

	for r := range complete {
		go r.Sleep(pending)
	}
}
