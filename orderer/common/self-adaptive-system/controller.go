package self_adaptive_system

import (
	"github.com/hyperledger/fabric-protos-go/orderer"
)

type Controller interface {
	Run(batchSize *orderer.BatchSize, changingAttribute string, size uint32) (*ControllerBlock, bool)
}

type ControllerStruct struct {
	Metric             *MetricMonitor
	MinSaturationPoint uint32
	MaxSaturationPoint uint32
	CurrentMetricValue float64
	LastMetricValue    float64
	SuggestedBlockSize *ControllerBlock
}

type ControllerBlock struct {
	PreferredMaxBytes uint32
	AbsoluteMaxBytes  uint32
	MaxMessageCount   uint32
}

func NewController(metric *MetricMonitor, SaturationPoint uint32, MaxSaturationPoint uint32) *ControllerStruct {
	return &ControllerStruct{
		Metric:             metric,
		MinSaturationPoint: SaturationPoint,
		MaxSaturationPoint: MaxSaturationPoint,
		CurrentMetricValue: 0,
		LastMetricValue:    0,
		SuggestedBlockSize: nil,
	}
}

func (c *ControllerStruct) Run(batchSize *orderer.BatchSize, changingAttribute string, size uint32) (*ControllerBlock, bool) {
	if c.Metric != nil {
		if changingAttribute == PreferredMaxBytes {
			if c.Metric.Value < float64(c.MinSaturationPoint) {
				newSize := batchSize.PreferredMaxBytes + (batchSize.PreferredMaxBytes * 20 / 100)
				logger.Debug("PreferredMaxBytes: ", newSize)
				return &ControllerBlock{PreferredMaxBytes: newSize}, true
			} else if c.Metric.Value > float64(c.MinSaturationPoint) && c.Metric.Value < float64(c.MaxSaturationPoint) {
				newSize := size
				logger.Debug("PreferredMaxBytes: ", newSize)
				return &ControllerBlock{PreferredMaxBytes: newSize}, true
			} else if c.Metric.Value > float64(c.MaxSaturationPoint) {
				newSize := batchSize.PreferredMaxBytes - (batchSize.PreferredMaxBytes * 10 / 100)
				logger.Debug("PreferredMaxBytes: ", newSize)
				return &ControllerBlock{PreferredMaxBytes: newSize}, true
			}
		} else if changingAttribute == AbsoluteMaxBytes {
			newSize := batchSize.AbsoluteMaxBytes + (batchSize.AbsoluteMaxBytes * 5 / 100)
			logger.Debug("AbsoluteMaxBytes: ", newSize)
			return &ControllerBlock{AbsoluteMaxBytes: newSize}, true
		} else if changingAttribute == MaxMessageCount {
			newSize := batchSize.MaxMessageCount + 1
			logger.Debug("MaxMessageCount: ", newSize)
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
