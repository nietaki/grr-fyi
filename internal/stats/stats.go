package stats

import (
	"fmt"
	"runtime/metrics"
)

func MemoryUsage() (uint64, error) {
	// Name of the metric we want to read.
	const myMetric = "/memory/classes/total:bytes"

	// Create a sample for the metric.
	sample := make([]metrics.Sample, 1)
	sample[0].Name = myMetric

	// Sample the metric.
	metrics.Read(sample)

	// Check if the metric is actually supported.
	// If it's not, the resulting value will always have
	// kind KindBad.
	if sample[0].Value.Kind() == metrics.KindBad {
		return 0, fmt.Errorf("metric %q no longer supported", myMetric)
	}

	// Handle the result.
	//
	// It's OK to assume a particular Kind for a metric;
	// they're guaranteed not to change.
	return sample[0].Value.Uint64(), nil
}
