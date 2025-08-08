package util

import (
	"bytes"
	"testing"
)

func FuzzCalculateFileChunks(f *testing.F) {
	f.Add([]byte("a\nb\nc"), byte('\n'))

	f.Fuzz(func(t *testing.T, data []byte, delimiter byte) {
		res := make(chan FileChunkCalculationResult)
		go CalculateFileChunks(bytes.NewReader(data), delimiter, res)

		for result := range res {
			if result.Err != nil {
				t.Fail()
			}

			chunk := result.FileChunk
			if chunk.EndBytes > int64(len(data)) {
				t.Fatalf("chunk end %d is out of bounds for data length %d", chunk.EndBytes, len(data))
			}
			if chunk.StartBytes > chunk.EndBytes {
				t.Fatalf("chunk start %d is after chunk end %d", chunk.StartBytes, chunk.EndBytes)
			}

			chunkData := data[chunk.StartBytes:chunk.EndBytes]

			if bytes.Contains(chunkData, []byte{delimiter}) {
				t.Errorf("chunk %v contains delimiter %q", chunkData, delimiter)
			}
		}
	})
}
