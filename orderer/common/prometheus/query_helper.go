package prometheus

import (
	"fmt"
	"log"
	"strconv"
)

/*
Go does not support optional parameters nor does it support method overloading, therefore two different functions for
each query type must to be built.
*/

//Extract a single numeric value from a Prometheus query response.
//@param instantVector JSON response from the prometheus server
//@returns {Map<string, value> | value} Either a map of values or a single value depending on if response is matrix or vector.
//In this case, it returns a single value.
func extractFirstValueFromSingleQuery(instantVector InstantVectorObject) (float64, error) {
	if instantVector.Data.ResultType == VECTOR && len(instantVector.Data.Result) > 0 {
		val := instantVector.Data.Result[0].Value[1].(string)
		floatConversion, _ := strconv.ParseFloat(val, 64)
		return floatConversion, nil
	} else {
		log.Printf("Result type unknown %s", instantVector.Data.ResultType)
		return 0, fmt.Errorf("result type unknown %s", instantVector.Data.ResultType)
	}
}

func extractFirstValueFromQueryRange(rangeVector RangeVectorObject) (KvMap, error) {
	if rangeVector.Data.ResultType == MATRIX {
		values := rangeVector.Data.Result
		var items []KvPair
		kvMap := KvMap{items}
		for _, result := range values {
			name := result.Metric.Name
			stringVal := result.Values[0][1].(string)
			floatVal, _ := strconv.ParseFloat(stringVal, 64)
			item := KvPair{Name: name, Value: floatVal}
			kvMap.AddItem(item)
		}
		return kvMap, nil
	} else {
		log.Printf("Result type unknown %s", rangeVector.Data.ResultType)
		return KvMap{}, fmt.Errorf("result type unknown %s", rangeVector.Data.ResultType)
	}
}

// label: name, job, etc
func extractStatisticFromQueryRange(rangeVector RangeVectorObject, statType string, label string) (KvMap, error) {
	if rangeVector.Data.ResultType == MATRIX {
		var items []KvPair
		kvMap := KvMap{items}
		for _, result := range rangeVector.Data.Result {
			var name string
			switch label {
			case NAME:
				name = result.Metric.Name
			case CHANNEL:
				name = result.Metric.Channel
			case JOB:
				name = result.Metric.Job
			case INSTANCE:
				name = result.Metric.Instance
			}
			series := result.Values
			values := extractValuesFromTimeSeries(series)
			stat := retrieveStatisticsFromArray(values, statType)
			item := KvPair{Name: name, Value: stat}
			kvMap.AddItem(item)
		}
		return kvMap, nil
	} else {
		log.Printf("Unknown result type %s", rangeVector.Data.ResultType)
		return KvMap{}, fmt.Errorf("result type unknown %s", rangeVector.Data.ResultType)
	}
}


// Extract values from time series data
// @param Array series Array of the form [ [timeIndex, value], [], ..., [] ]
// @param isNumeric boolean to indicate if value should be cast to float or not
// @returns Array one dimensional array of values
func extractValuesFromTimeSeries(series [][]interface{}) []float64 {
	var arr []float64
	for _, value := range series {
		floatVal, _ := strconv.ParseFloat(value[1].(string), 64)
		arr = append(arr, floatVal)
	}
	return arr
}

func retrieveStatisticsFromArray(values []float64, statType string) float64 {
	switch statType {
	case MAX:
		max := findMaxOfFloatSlice(values)
		return max
	case MIN:
		min := findMinOfFloatSlice(values)
		return min
	case AVG:
		avg := findAvgOfFloatSlice(values)
		return avg
	case SUM:
		sum := sumOfFloatSlice(values)
		return sum
	default:
		log.Printf("Unknown stat type %s", statType)
		return 0.0
	}
}
