package prometheus

import (
	"fmt"
	"log"
	"net/http"
	"time"
)

const (
	numPollers     = 3                // number of Poller goroutines to launch
	pollInterval   = 2 * time.Second // how often to poll each URL
	statusInterval = 1 * time.Second // how often to log status to stdout
	errTimeout     = 1 * time.Second // back-off timeout on error
	successTimeout = 10
)

var urls = []string{
	"http://localhost:9090/-/healthy",
	"http://localhost:9090/-/unhealthy",
}

// State represents the last-known state of a URL
type State struct {
	url string
	status string
	healthy bool
}

// StateMonitor maintains a map that stores the state of the URLs being
// polled, and prints the current state every updateInterval nanoseconds.
// It returns a chan State to which resource state should be sent.
func StateMonitor(updateInterval time.Duration, successTimeout time.Duration) chan<- State {
	updates := make(chan State)
	urlStatus := make(map[string]bool)
	ticker := time.NewTicker(updateInterval)
	succTimeout := time.After(successTimeout * time.Second)
	start := time.Now()
	go func() {
		for {
			select {
			case <-ticker.C:
				logState(urlStatus)
			case state := <-updates:
				urlStatus[state.url] = state.healthy
			case <-succTimeout: // TODO: should return only if successful
				elapsedTime := time.Since(start)
				fmt.Println("Elapsed time: ", elapsedTime)
				return
			}
		}
	}()
	return updates
}

// logState prints a state map
func logState(s map[string]bool) {
	for k, v := range s {
		log.Printf("Current state: %v %v", k, v)
	}
}

// Resource represents an HTTP URL to be polled by this program
type Resource struct {
	url string
	errCount int
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
	fmt.Println(r.successCount)
	return resp.Status, resp.StatusCode
}

func (r *Resource) Sleep(done chan<- *Resource) {
	time.Sleep(pollInterval +  errTimeout*time.Duration(r.errCount))
	done <- r
}

func Poller(in <-chan *Resource, out chan<- *Resource, status chan<- State) {
	for r := range in {
		statusMsg, statusCode := r.Poll()
		healthy := false
		if statusCode == http.StatusOK {
			healthy = true
			fmt.Println(healthy)
		} else {
			healthy = false
		}
		url := r.url
		fmt.Println(url)
		status <- State{url, statusMsg, healthy}
		out <- r
	}
}

func Run(url string) {
	// Create our input and output channels
	pending, complete := make(chan *Resource), make(chan *Resource)

	go func() {
		ticker := time.NewTicker(pollInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				fmt.Println(ticker)
				fmt.Println("test ticker c")
				fmt.Println("here we should get the status")
				fmt.Println("and check with the checker")
			case <-pending:
				fmt.Println("pending")
				return
			}
		}
	}()

	// Launch the StateMonitor
	status := StateMonitor(statusInterval, successTimeout)

	// Launch same Poller goroutines
	for i := 0; i < numPollers; i++ {
		go Poller(pending, complete, status)
	}

	pending <- &Resource{url: url}
	go func() {
		fmt.Println("test")
	}()

	for r := range complete {
		go r.Sleep(pending)
	}
}