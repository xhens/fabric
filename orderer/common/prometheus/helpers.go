package prometheus

import (
	"math"
)

func findMaxOfFloatSlice(values []float64) float64 {
	max := -math.MaxFloat64
	for _, value := range values {
		if max < value {
			max = value
		}
	}
	return max
}

func findMinOfFloatSlice(values []float64) float64 {
	min := math.MaxFloat64
	for _, value := range values {
		if min > value {
			min = value
		}
	}
	return min
}

func findAvgOfFloatSlice(values []float64) float64 {
	total := 0.0
	for _, value := range values {
		total += value
	}
	avg := total / float64(len(values))
	return avg
}

func sumOfFloatSlice(values []float64) float64 {
	sum := 0.0
	for _, value := range values {
		sum += value
	}
	return sum
}
