package prometheus

import (
	"fmt"
	"github.com/hyperledger/fabric-protos-go/orderer"
)

// create the Controller interface. Use methods to get the values from the struct, etc,.
type Controller interface {
	Run(batchSize *orderer.BatchSize, changingAttribute string, size uint32) (*ControllerBlock, bool)
}

// Rename to ControllerStruct and export it because it is being used in Blockcutter and other components
type ControllerStruct struct {
	FirstMetric         *MetricMonitor
	SaturationPoint     uint32
	CurrentValue        float64
	LastValue           float64
	SuggestedBlockValue *ControllerBlock
}

type ControllerBlock struct {
	PreferredMaxBytes uint32
	AbsoluteMaxBytes  uint32
	MaxMessageCount   uint32
}

func NewController(metric *MetricMonitor, SaturationPoint uint32) *ControllerStruct {
	return &ControllerStruct{
		FirstMetric:         metric,
		SaturationPoint:     SaturationPoint,
		CurrentValue:        0,
		LastValue:           0,
		SuggestedBlockValue: nil,
	}
}

func (c *ControllerStruct) Run(batchSize *orderer.BatchSize, changingAttribute string, size uint32) (*ControllerBlock, bool) {
	fmt.Println("ControllerStruct running")
	if c.FirstMetric != nil {
		fmt.Println(float64(c.SaturationPoint))
		fmt.Println("First metric: ", c.FirstMetric.Value, " Saturation point: ", c.SaturationPoint)
		if changingAttribute == PreferredMaxBytes {
			if c.FirstMetric.Value < float64(c.SaturationPoint) {
				newSize := size
				return &ControllerBlock{PreferredMaxBytes: newSize}, true
			} else if c.FirstMetric.Value > float64(c.SaturationPoint) {
				newSize := batchSize.PreferredMaxBytes + (batchSize.PreferredMaxBytes * 10 / 100)
				return &ControllerBlock{PreferredMaxBytes: newSize}, true
			}
		} else if changingAttribute == AbsoluteMaxBytes {
			fmt.Println("second condition")
			newSize := batchSize.AbsoluteMaxBytes + (batchSize.AbsoluteMaxBytes * 5 / 100)
			return &ControllerBlock{AbsoluteMaxBytes: newSize}, true
		} else if changingAttribute == MaxMessageCount {
			newSize := batchSize.MaxMessageCount + 1
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
