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
	"errors"
	"io"
	"strings"
	"testing"
	"testing/iotest"
	"time"

	"github.com/stretchr/testify/assert"
)

// collectChunkResults reads res until it is closed. Fails the test on timeout
// or if more than 100 results arrive.
func collectChunkResults(t *testing.T, res <-chan FileChunkCalculationResult) []FileChunkCalculationResult {
	t.Helper()
	var out []FileChunkCalculationResult
	deadline := time.After(time.Second)
	for {
		select {
		case r, ok := <-res:
			if !ok {
				return out
			}
			out = append(out, r)
			if len(out) > 100 {
				t.Fatal("too many results; channel flooded")
			}
		case <-deadline:
			t.Fatal("timeout waiting for result channel to close")
		}
	}
}

type dataWithErrReader struct {
	data string
	err  error
	done bool
}

func (r *dataWithErrReader) Read(p []byte) (int, error) {
	if r.done {
		return 0, r.err
	}
	r.done = true
	return copy(p, r.data), r.err
}

func assertChunkThenReadError(t *testing.T, r io.Reader, errX error) {
	t.Helper()
	res := make(chan FileChunkCalculationResult)
	go CalculateFileChunks(r, byte('\n'), res)

	got := collectChunkResults(t, res)

	if assert.Len(t, got, 2) {
		assert.NoError(t, got[0].Err)
		assert.Equal(t, FileChunk{ChunkNumber: 1, StartBytes: 0, EndBytes: 1}, got[0].FileChunk)
		assert.Equal(t, errX, got[1].Err)
		assert.Equal(t, 2, got[1].FileChunk.ChunkNumber)
	}
}

func TestCalculateFileChunksStopsAfterReadError(t *testing.T) {
	errX := errors.New("read failed")
	assertChunkThenReadError(t, io.MultiReader(strings.NewReader("a\nb"), iotest.ErrReader(errX)), errX)
}

func TestCalculateFileChunksReadErrorWithData(t *testing.T) {
	errX := errors.New("read failed")
	assertChunkThenReadError(t, &dataWithErrReader{data: "a\nb", err: errX}, errX)
}

func TestCalculateFileChunksDataWithEOF(t *testing.T) {
	res := make(chan FileChunkCalculationResult)
	go CalculateFileChunks(iotest.DataErrReader(strings.NewReader("a\nb")), byte('\n'), res)

	got := collectChunkResults(t, res)

	if assert.Len(t, got, 2) {
		assert.Equal(t, FileChunk{ChunkNumber: 1, StartBytes: 0, EndBytes: 1}, got[0].FileChunk)
		assert.Equal(t, FileChunk{ChunkNumber: 2, StartBytes: 2, EndBytes: 3}, got[1].FileChunk)
		assert.NoError(t, got[0].Err)
		assert.NoError(t, got[1].Err)
	}
}

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
