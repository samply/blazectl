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
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

// mustCreateOutputFile creates the output file at filepath, failing the test on
// errors.
func mustCreateOutputFile(t testing.TB, filepath string) *os.File {
	file, err := CreateOutputFile(filepath)
	if err != nil {
		t.Fatal(err)
	}
	return file
}

func TestCreateOutputFile(t *testing.T) {
	// Create a temporary directory for our tests
	tempDir := t.TempDir()

	t.Run("Successfully create new file", func(t *testing.T) {
		filepath := filepath.Join(tempDir, "test_new_file.txt")

		// Ensure file doesn't exist
		_, err := os.Stat(filepath)
		assert.True(t, os.IsNotExist(err))

		file := mustCreateOutputFile(t, filepath)
		defer file.Close()

		// Verify file was created
		assert.NotNil(t, file)

		// Verify file exists and has correct permissions
		info, err := file.Stat()
		assert.NoError(t, err)
		assert.Equal(t, os.FileMode(0644), info.Mode().Perm())

		// Verify we can write to the file
		_, err = file.WriteString("test content")
		assert.NoError(t, err)
	})

	t.Run("File creation with nested directory path", func(t *testing.T) {
		nestedDir := filepath.Join(tempDir, "nested", "directory")
		err := os.MkdirAll(nestedDir, 0755)
		assert.NoError(t, err)

		filepath := filepath.Join(nestedDir, "nested_file.txt")

		file := mustCreateOutputFile(t, filepath)
		defer file.Close()

		assert.NotNil(t, file)

		// Verify file exists
		_, err = os.Stat(filepath)
		assert.NoError(t, err)
	})

	t.Run("File already exists - fails with exit code 3", func(t *testing.T) {
		filepath := filepath.Join(tempDir, "existing_file.txt")
		assert.NoError(t, os.WriteFile(filepath, []byte("content"), 0644))

		file, err := CreateOutputFile(filepath)

		assert.Nil(t, file)
		var exitCodeErr *ExitCodeError
		if assert.ErrorAs(t, err, &exitCodeErr) {
			assert.Equal(t, 3, exitCodeErr.Code)
		}
		assert.EqualError(t, err, "the output file "+filepath+" does already exist")
		content, _ := os.ReadFile(filepath)
		assert.Equal(t, "content", string(content))
	})

	t.Run("Missing directory - fails with exit code 4", func(t *testing.T) {
		filepath := filepath.Join(tempDir, "missing", "file.txt")

		file, err := CreateOutputFile(filepath)

		assert.Nil(t, file)
		var exitCodeErr *ExitCodeError
		if assert.ErrorAs(t, err, &exitCodeErr) {
			assert.Equal(t, 4, exitCodeErr.Code)
		}
		assert.ErrorContains(t, err, "could not open/create the output file "+filepath)
		assert.ErrorIs(t, err, os.ErrNotExist)
	})

	t.Run("Create file in current directory", func(t *testing.T) {
		// Change to temp directory
		originalDir, err := os.Getwd()
		assert.NoError(t, err)
		defer os.Chdir(originalDir)

		err = os.Chdir(tempDir)
		assert.NoError(t, err)

		filename := "current_dir_file.txt"

		file := mustCreateOutputFile(t, filename)
		defer file.Close()

		assert.NotNil(t, file)

		// Verify file was created in current directory
		_, err = os.Stat(filename)
		assert.NoError(t, err)
	})

	t.Run("Create file with absolute path", func(t *testing.T) {
		absolutePath := filepath.Join(tempDir, "absolute_path_file.txt")

		file := mustCreateOutputFile(t, absolutePath)
		defer file.Close()

		assert.NotNil(t, file)

		// Verify file was created at absolute path
		_, err := os.Stat(absolutePath)
		assert.NoError(t, err)
	})

	t.Run("File permissions are correct", func(t *testing.T) {
		filepath := filepath.Join(tempDir, "permissions_test.txt")

		file := mustCreateOutputFile(t, filepath)
		defer file.Close()

		info, err := file.Stat()
		assert.NoError(t, err)

		// Check that permissions are 0644 (owner: rw-, group: r--, others: r--)
		expectedPerm := os.FileMode(0644)
		actualPerm := info.Mode().Perm()
		assert.Equal(t, expectedPerm, actualPerm)
	})

	t.Run("Can write to created file", func(t *testing.T) {
		filepath := filepath.Join(tempDir, "writable_test.txt")

		file := mustCreateOutputFile(t, filepath)
		defer file.Close()

		testContent := "This is test content for the file"
		n, err := file.WriteString(testContent)
		assert.NoError(t, err)
		assert.Equal(t, len(testContent), n)

		// Sync to ensure content is written
		err = file.Sync()
		assert.NoError(t, err)

		// Close the file first
		file.Close()

		// Read back the content to verify by opening the file again
		content, err := os.ReadFile(filepath)
		assert.NoError(t, err)
		assert.Equal(t, testContent, string(content))
	})
}

// Benchmark tests to ensure the function performs well
func BenchmarkCreateOutputFile(b *testing.B) {
	tempDir := b.TempDir()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		filepath := filepath.Join(tempDir, "bench_file_"+fmt.Sprintf("%d", i)+".txt")
		file := mustCreateOutputFile(b, filepath)
		file.Close()
	}
}
