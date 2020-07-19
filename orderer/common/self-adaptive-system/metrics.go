package self_adaptive_system

import "github.com/hyperledger/fabric/common/metrics"

var (
	preferredMaxBytesBatchSize = metrics.GaugeOpts{
		Namespace:    "ControllerStruct",
		Name:         "batch_size_preferred_max_bytes",
		Help:         "Size of the batch.",
		LabelNames:   []string{"channel"},
		StatsdFormat: "%{#fqname}.%{channel}",
	}

	maxMessageCountBatchSize = metrics.GaugeOpts{
		Namespace:    "ControllerStruct",
		Name:         "batch_size_max_message_count",
		Help:         "Size of the batch.",
		LabelNames:   []string{"channel"},
		StatsdFormat: "%{#fqname}.%{channel}",
	}

	absoluteMaxBytesBatchSize = metrics.GaugeOpts{
		Namespace:    "ControllerStruct",
		Name:         "batch_size_absolute_max_bytes",
		Help:         "Size of the batch.",
		LabelNames:   []string{"channel"},
		StatsdFormat: "%{#fqname}.%{channel}",
	}
)

type MetricsController struct {
	PreferredMaxBytes metrics.Gauge
	MaxMessageCount metrics.Gauge
	AbsoluteMaxBytes metrics.Gauge
}

func NewMetrics(p metrics.Provider) *MetricsController {
	return &MetricsController{
		PreferredMaxBytes: p.NewGauge(preferredMaxBytesBatchSize),
		MaxMessageCount: p.NewGauge(maxMessageCountBatchSize),
		AbsoluteMaxBytes: p.NewGauge(absoluteMaxBytesBatchSize),
	}
}