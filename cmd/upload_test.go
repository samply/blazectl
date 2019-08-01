package cmd

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestGenStats(t *testing.T) {
	stats := genStats([]float64{1.0})
	assert.Equal(t, 1.0*time.Second, stats.mean)
	assert.Equal(t, 1.0*time.Second, stats.q50)
	assert.Equal(t, 1.0*time.Second, stats.q95)
	assert.Equal(t, 1.0*time.Second, stats.q99)
	assert.Equal(t, 1.0*time.Second, stats.max)
}
