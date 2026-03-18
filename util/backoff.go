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
	"math/rand"
	"sync"
	"time"
)

type Backoff struct {
	sync.Mutex
	currentDelay time.Duration
	maxDelay     time.Duration
	lastIncrease time.Time
}

func NewBackoff(maxDelay time.Duration) *Backoff {
	return &Backoff{
		maxDelay: maxDelay,
	}
}

func (b *Backoff) Wait() {
	b.Lock()
	delay := b.currentDelay
	b.Unlock()
	if delay > 0 {
		// add some jitter
		time.Sleep(delay + time.Duration(rand.Int63n(int64(delay/10))))
	}
}

func (b *Backoff) Increase() {
	b.Lock()
	defer b.Unlock()
	if b.currentDelay > 0 && time.Since(b.lastIncrease) < b.currentDelay {
		return
	}
	if b.currentDelay == 0 {
		b.currentDelay = 1 * time.Second
	} else {
		b.currentDelay *= 2
		if b.currentDelay > b.maxDelay {
			b.currentDelay = b.maxDelay
		}
	}
	b.lastIncrease = time.Now()
}

func (b *Backoff) Reset() {
	b.Lock()
	defer b.Unlock()
	b.currentDelay = 0
}
