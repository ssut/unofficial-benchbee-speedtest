package tool

import (
	"math"
	"time"
)

func GetTime() int64 {
	now := time.Now()
	nanos := now.UnixNano()

	return nanos / 1e6
}

// CalculateJitter computes the jitter from a list of latencies
// RFC 1889 (https://www.ietf.org/rfc/rfc1889.txt):
// J=J+(|D(i-1,i)|-J)/16
func CalculateJitter(latencies []float64) float64 {
	jitter := 0.0
	for i := 1; i < len(latencies); i++ {
		latency := latencies[i]
		previousLatency := latencies[i-1]
		jitter = jitter + (math.Abs(previousLatency-latency)-jitter)/16.0
	}
	return jitter
}

func GetAverage(values []float64) float64 {
	var total float64 = 0
	for _, value := range values {
		total += value
	}

	return total / float64((len(values)))
}
