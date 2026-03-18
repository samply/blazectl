// Copyright 2019 - 2025 The Samply Community
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package util

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestBackoff(t *testing.T) {
	b := NewBackoff(10 * time.Second)

	// Initial delay is 0
	start := time.Now()
	b.Wait()
	assert.True(t, time.Since(start) < 100*time.Millisecond)

	// Increase delay
	b.Increase()
	assert.Equal(t, 1*time.Second, b.currentDelay)

	// Wait should take at least 1 second (minus some jitter)
	start = time.Now()
	b.Wait()
	assert.True(t, time.Since(start) >= 900*time.Millisecond)

	// Increase delay again
	b.lastIncrease = time.Time{}
	b.Increase()
	assert.Equal(t, 2*time.Second, b.currentDelay)

	// Wait should take at least 2 seconds (minus some jitter)
	start = time.Now()
	b.Wait()
	assert.True(t, time.Since(start) >= 1800*time.Millisecond)

	// Reset delay
	b.Reset()
	assert.Equal(t, 0*time.Duration(0), b.currentDelay)

	// Wait should be fast again
	start = time.Now()
	b.Wait()
	assert.True(t, time.Since(start) < 100*time.Millisecond)
}

func TestBackoff_MaxDelay(t *testing.T) {
	b := NewBackoff(3 * time.Second)

	b.Increase() // 1s
	b.lastIncrease = time.Time{}
	b.Increase() // 2s
	b.lastIncrease = time.Time{}
	b.Increase() // 4s -> capped at 3s
	assert.Equal(t, 3*time.Second, b.currentDelay)
}

func TestBackoff_IncreaseWait(t *testing.T) {
	b := NewBackoff(10 * time.Second)

	b.Increase() // 1s
	firstIncrease := b.lastIncrease

	b.Increase() // Should not increase because lastIncrease is too recent (0 < 1s)
	assert.Equal(t, 1*time.Second, b.currentDelay)
	assert.Equal(t, firstIncrease, b.lastIncrease)

	// Simulate time passing by manually setting lastIncrease
	b.lastIncrease = time.Now().Add(-2 * time.Second)
	b.Increase() // Should increase because lastIncrease is old enough (2s > 1s)
	assert.Equal(t, 2*time.Second, b.currentDelay)
}
