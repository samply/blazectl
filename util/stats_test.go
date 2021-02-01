package util

import (
	"github.com/stretchr/testify/assert"
	"math"
	"strings"
	"testing"
	"time"
)

func TestCalculateDurationStatistics_EmptyDurationSet(t *testing.T) {
	statistics := CalculateDurationStatistics([]float64{})
	assert.Equal(t, time.Duration(0), statistics.Mean)
	assert.Equal(t, time.Duration(0), statistics.Max)
	assert.Equal(t, time.Duration(0), statistics.Q50)
	assert.Equal(t, time.Duration(0), statistics.Q95)
	assert.Equal(t, time.Duration(0), statistics.Q99)
}

func TestCalculateDurationStatistics(t *testing.T) {
	statistics := CalculateDurationStatistics([]float64{1.0})
	assert.Equal(t, 1.0*time.Second, statistics.Mean)
	assert.Equal(t, 1.0*time.Second, statistics.Max)
	assert.Equal(t, 1.0*time.Second, statistics.Q50)
	assert.Equal(t, 1.0*time.Second, statistics.Q95)
	assert.Equal(t, 1.0*time.Second, statistics.Q99)
}

func TestFmtBytesHumanReadable(t *testing.T) {
	byteUnitMappings := map[float32]string{
		1:                               "B",
		float32(10 * math.Pow(1024, 1)): "KiB",
		float32(10 * math.Pow(1024, 2)): "MiB",
		float32(10 * math.Pow(1024, 3)): "GiB",
		float32(10 * math.Pow(1024, 4)): "TiB",
		float32(10 * math.Pow(1024, 5)): "PiB",
		float32(10 * math.Pow(1024, 6)): "PiB",
	}

	for bytes, unit := range byteUnitMappings {
		t.Run(unit, func(t *testing.T) {
			humanReadableResult := FmtBytesHumanReadable(bytes)
			assert.True(t, strings.HasSuffix(humanReadableResult, unit))
		})
	}
}
