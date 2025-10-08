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
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReadQueryFromFile(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("test query", func(t *testing.T) {
		queryFile := filepath.Join(tmpDir, "test.query")
		assert.NoError(t, os.WriteFile(queryFile, []byte("foo=bar"), 0644))

		q, err := ReadQueryFromFile("@" + queryFile)

		assert.NoError(t, err)
		assert.Equal(t, "bar", q.Get("foo"))
	})

	t.Run("test query with trailing newline in the file", func(t *testing.T) {
		queryFile := filepath.Join(tmpDir, "test.query")
		assert.NoError(t, os.WriteFile(queryFile, []byte("foo=bar\n"), 0644))

		q, err := ReadQueryFromFile("@" + queryFile)

		assert.NoError(t, err)
		assert.Equal(t, "bar", q.Get("foo"))
	})
}
