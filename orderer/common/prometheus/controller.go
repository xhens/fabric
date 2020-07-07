package prometheus

import (
	"fmt"
	"github.com/hyperledger/fabric-protos-go/orderer"
	"github.com/hyperledger/fabric/common/metrics"
	"time"
)

type ControllerInterface interface {
	getMetricValue() float64
	Run()
	SuggestNewBlockSize()
	ControllerState() *MetricMonitor
}

type Controller struct {
	FirstMetric         *MetricMonitor
	SaturationPoint     uint32
	CurrentValue        float64
	LastValue           float64
	SuggestedBlockValue *ControllerBlock
	Metrics             *MetricsController
	MetricProvider      metrics.Provider
}

type ControllerBlock struct {
	PreferredMaxBytes uint32
	AbsoluteMaxBytes  uint32
	MaxMessageCount   uint32
}

func NewControllerWithSingleMetric(firstMetric *MetricMonitor, SaturationPoint uint32) *Controller {
	return &Controller{
		FirstMetric:         firstMetric,
		SaturationPoint:     SaturationPoint,
		CurrentValue:        0,
		LastValue:           0,
		SuggestedBlockValue: nil,
	}
}

func (c *Controller) ControllerState() *MetricMonitor {
	return c.FirstMetric
}

func (c *Controller) getMetricValue() float64 {
	fmt.Println("this ", c.FirstMetric.Value)
	return c.FirstMetric.Value
}

func (c *Controller) Run(batchSize *orderer.BatchSize, changingAttribute string, size uint32) (*ControllerBlock, bool) {
	fmt.Println("Controller running")
	if c.FirstMetric != nil {
		fmt.Println(float64(c.SaturationPoint))
		fmt.Println("First metric: ", c.FirstMetric.Value, " Saturation point: ", c.SaturationPoint)
		if changingAttribute == PreferredMaxBytes {
			if c.FirstMetric.Value < float64(c.SaturationPoint) {
				fmt.Println("first condition")
				newSize := size
				// c.Metrics.PreferredMaxBytes.Add(float64(newSize))
				return &ControllerBlock{PreferredMaxBytes: newSize}, true
			} else if c.FirstMetric.Value > float64(c.SaturationPoint) {
				/*labels := []string{
					"channel", changingAttribute,
				}*/
				newSize := batchSize.PreferredMaxBytes + (batchSize.PreferredMaxBytes * 10 / 100)
				// c.Metrics.PreferredMaxBytes.With(labels...).Add(float64(newSize))
				return &ControllerBlock{PreferredMaxBytes: newSize}, true
			}
		} else if changingAttribute == AbsoluteMaxBytes {
			fmt.Println("second condition")
			/*			labels := []string{
						"channel", changingAttribute,
					}*/
			newSize := batchSize.AbsoluteMaxBytes + (batchSize.AbsoluteMaxBytes * 5 / 100)
			//c.Metrics.PreferredMaxBytes.With(labels...).Add(float64(newSize))
			return &ControllerBlock{AbsoluteMaxBytes: newSize}, true
		} else if changingAttribute == MaxMessageCount {
			fmt.Println("third condition")
			/*			labels := []string{
						"channel", changingAttribute,
					}*/
			newSize := batchSize.MaxMessageCount + 1
			//c.Metrics.PreferredMaxBytes.With(labels...).Add(float64(newSize))
			return &ControllerBlock{MaxMessageCount: newSize}, true
		}
	}
	// If the batchSize doesn't change, return the original size and false.
	return &ControllerBlock{
		PreferredMaxBytes: batchSize.PreferredMaxBytes,
		AbsoluteMaxBytes:  batchSize.AbsoluteMaxBytes,
		MaxMessageCount:   batchSize.MaxMessageCount,
	}, false
}

func (c *Controller) SuggestNewBlockSize() {
	fmt.Println("Controller running... ")
	go func() {
		ticker := time.NewTicker(pollInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if c.FirstMetric != nil {
					if c.FirstMetric.Value < float64(c.SaturationPoint) {
						cb := ControllerBlock{PreferredMaxBytes: 24 * 1024}
						fmt.Println("CB: ", cb)
					}
				}
			}
		}
	}()
}
