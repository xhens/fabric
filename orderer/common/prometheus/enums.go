package prometheus

// Response from Prometheus for a GET or POST request
const (
	Success = "success"
	Fail    = "fail"
)

// 	https://prometheus.io/docs/prometheus/latest/querying/api/#range-vectors
// https://prometheus.io/docs/prometheus/latest/querying/api/#instant-vectors
// Instant Vectors are returned as type vector. Range vectors are returned as type matrix
const (
	Vector = "vector"
	Matrix = "matrix"
)

// https://prometheus.io/docs/prometheus/latest/querying/operators/#aggregation-operators
// Prometheus supports the following built-in aggregation operators that can be used to aggregate the elements of a
// single instant vector, resulting in a new vector of fewer elements with aggregated values:
const (
	Min         = "min"
	Max         = "max"
	Avg         = "avg"
	Sum         = "sum"
	Count       = "count"
	CountValues = "count_values"
)

// Labels exposed by Fabric when exposing Metric data
const (
	Name            = "__name__"
	Channel         = "channel"
	Instance        = "instance"
	Job             = "job"
	Chaincode       = "chaincode"
	TransactionType = "transaction_type"
	ValidationCode  = "validation_code"
	Le              = "le"
	Status          = "status"
)

// Useful metrics defined as constants
const (
	LedgerTransactionCount     = "ledger_transaction_count"
	LedgerTransactionCountRate = "irate(ledger_transaction_count[1m])"
	DeliverBLocksSent          = "deliver_blocks_sent"
)

const (
	ContainerHost  = "http://prometheus:9090"
	HealthEndpoint = ContainerHost + "/-/healthy"
)

// Constants for deliver_blocks_sent metric
const (
	BlockDataType = "block"
	FilteredBlock = "filtered_block"
)

const (
	PreferredMaxBytes = "PreferredMaxBytes"
	AbsoluteMaxBytes = "AbsoluteMaxBytes"
	MaxMessageCount = "MaxMessageCount"
)