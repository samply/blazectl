// Copyright 2019 - 2023 The Samply Community
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
	"strings"
	"testing"
)

func TestCalculateFileChunks(t *testing.T) {
	res := make(chan FileChunkCalculationResult)
	reader := strings.NewReader("A simple\ntest case\n")

	resultPool := make([]FileChunkCalculationResult, 0, 2)
	go CalculateFileChunks(reader, byte('\n'), res)

	for chunk := range res {
		resultPool = append(resultPool, chunk)
	}

	assert.Equal(t, 2, len(resultPool))
	assert.Equal(t, int64(0), resultPool[0].FileChunk.StartBytes)
	assert.Equal(t, int64(8), resultPool[0].FileChunk.EndBytes)
	assert.Equal(t, int64(9), resultPool[1].FileChunk.StartBytes)
	assert.Equal(t, int64(18), resultPool[1].FileChunk.EndBytes)
}

func TestCalculateFileChunksWithoutClosingDelimiter(t *testing.T) {
	res := make(chan FileChunkCalculationResult)
	reader := strings.NewReader("No closing\nnewline")

	resultPool := make([]FileChunkCalculationResult, 0, 2)
	go CalculateFileChunks(reader, byte('\n'), res)

	for chunk := range res {
		resultPool = append(resultPool, chunk)
	}

	assert.Equal(t, 2, len(resultPool))
	assert.Equal(t, int64(0), resultPool[0].FileChunk.StartBytes)
	assert.Equal(t, int64(10), resultPool[0].FileChunk.EndBytes)
	assert.Equal(t, int64(11), resultPool[1].FileChunk.StartBytes)
	assert.Equal(t, int64(18), resultPool[1].FileChunk.EndBytes)
}

func TestCalculateFileChunksWithSingleChunkWithClosingDelimiter(t *testing.T) {
	res := make(chan FileChunkCalculationResult)
	reader := strings.NewReader("Closing delimiter\n")

	resultPool := make([]FileChunkCalculationResult, 0, 1)
	go CalculateFileChunks(reader, byte('\n'), res)

	for chunk := range res {
		resultPool = append(resultPool, chunk)
	}

	assert.Equal(t, 1, len(resultPool))
	assert.Equal(t, int64(0), resultPool[0].FileChunk.StartBytes)
	assert.Equal(t, reader.Size()-1, resultPool[0].FileChunk.EndBytes)
}

func TestCalculateFileChunksWithSingleChunkWithoutClosingDelimiter(t *testing.T) {
	res := make(chan FileChunkCalculationResult)
	reader := strings.NewReader("No closing delimiter")

	resultPool := make([]FileChunkCalculationResult, 0, 1)
	go CalculateFileChunks(reader, byte('\n'), res)

	for chunk := range res {
		resultPool = append(resultPool, chunk)
	}

	assert.Equal(t, 1, len(resultPool))
	assert.Equal(t, int64(0), resultPool[0].FileChunk.StartBytes)
	assert.Equal(t, reader.Size(), resultPool[0].FileChunk.EndBytes)
}

func TestCalculateFileChunksMultipleConsecutiveDelimiters(t *testing.T) {
	res := make(chan FileChunkCalculationResult)
	reader := strings.NewReader("Multiple\n\n\nDelimiters")

	resultPool := make([]FileChunkCalculationResult, 0, 4)
	go CalculateFileChunks(reader, byte('\n'), res)

	for chunk := range res {
		resultPool = append(resultPool, chunk)
	}

	assert.Equal(t, 4, len(resultPool))
	assert.Equal(t, int64(0), resultPool[0].FileChunk.StartBytes)
	assert.Equal(t, int64(8), resultPool[0].FileChunk.EndBytes)
	assert.Equal(t, int64(9), resultPool[1].FileChunk.StartBytes)
	assert.Equal(t, int64(9), resultPool[1].FileChunk.EndBytes)
	assert.Equal(t, int64(10), resultPool[2].FileChunk.StartBytes)
	assert.Equal(t, int64(10), resultPool[2].FileChunk.EndBytes)
	assert.Equal(t, int64(11), resultPool[3].FileChunk.StartBytes)
	assert.Equal(t, reader.Size(), resultPool[3].FileChunk.EndBytes)
}
