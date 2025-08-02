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
	"io"
)

// Size of the buffer used for calculating file chunks.
const chunksCalculationBufferSizeBytes = 4096

// FileChunk describes a chunk within a file with its starting position and end
// position in bytes. Both are given as bytes counted from the file's beginning.
// Also carries information about the chunk number (i.e. its order position)
// within the file.
type FileChunk struct {
	ChunkNumber int
	StartBytes  int64
	EndBytes    int64
}

// FileChunkCalculationResult represents a single result of a file chunk calculation.
// Carries information about a FileChunk and additional error information.
// A FileChunk can still hold valuable information even in the presence of an
// error such as the chunk number.
type FileChunkCalculationResult struct {
	FileChunk FileChunk
	Err       error
}

// CalculateFileChunks calculates all chunks of r that are delimited by delimiter.
// r is read in a streamed fashion.
// Results will be published on a res channel as they appear when reading r.
// Closes the result channel as soon as r is exhaustively read.
func CalculateFileChunks(r io.Reader, delimiter byte, res chan<- FileChunkCalculationResult) {
	var lastSeenDelimiterTokenOffsetBytes int64 = 0
	alreadyReadBytes := int64(0)
	chunkNumber := 0
	buf := make([]byte, 0, chunksCalculationBufferSizeBytes)
	for {
		n, err := r.Read(buf[:cap(buf)])
		buf = buf[:n]
		if n == 0 {
			if err == nil {
				continue
			}
			if err == io.EOF {
				// For when r does not end with the delimiter.
				if alreadyReadBytes > lastSeenDelimiterTokenOffsetBytes {
					res <- FileChunkCalculationResult{
						FileChunk: FileChunk{
							ChunkNumber: chunkNumber + 1,
							StartBytes:  lastSeenDelimiterTokenOffsetBytes,
							EndBytes:    alreadyReadBytes,
						},
					}
				}

				close(res)
				break
			}
			res <- FileChunkCalculationResult{
				FileChunk: FileChunk{
					ChunkNumber: chunkNumber,
				},
				Err: err,
			}
		}

		for idx, b := range buf {
			if b == delimiter {
				chunkNumber++
				res <- FileChunkCalculationResult{
					FileChunk: FileChunk{
						ChunkNumber: chunkNumber,
						StartBytes:  lastSeenDelimiterTokenOffsetBytes,
						EndBytes:    alreadyReadBytes + int64(idx),
					},
				}
				lastSeenDelimiterTokenOffsetBytes = alreadyReadBytes + int64(idx) + 1
			}
		}

		alreadyReadBytes += int64(n)
	}
}
