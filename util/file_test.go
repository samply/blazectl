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

func TestCreateOutputFileOrDie(t *testing.T) {
	// Create a temporary directory for our tests
	tempDir := t.TempDir()

	t.Run("Successfully create new file", func(t *testing.T) {
		filepath := filepath.Join(tempDir, "test_new_file.txt")

		// Ensure file doesn't exist
		_, err := os.Stat(filepath)
		assert.True(t, os.IsNotExist(err))

		file := CreateOutputFileOrDie(filepath)
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

		file := CreateOutputFileOrDie(filepath)
		defer file.Close()

		assert.NotNil(t, file)

		// Verify file exists
		_, err = os.Stat(filepath)
		assert.NoError(t, err)
	})

	t.Run("File already exists - should exit with code 3", func(t *testing.T) {
		filepath := filepath.Join(tempDir, "existing_file.txt")

		// Create the file first
		existingFile, err := os.Create(filepath)
		assert.NoError(t, err)
		existingFile.Close()

		// Verify file exists
		_, err = os.Stat(filepath)
		assert.NoError(t, err)

		// This test would normally cause os.Exit(3), but we can't test that directly
		// in a unit test without using a subprocess. Instead, we'll test the condition
		// that leads to the exit by checking if the file exists
		_, err = os.OpenFile(filepath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
		assert.True(t, os.IsExist(err))
	})

	t.Run("Invalid directory path - permission denied", func(t *testing.T) {
		// Create a directory with no write permissions
		restrictedDir := filepath.Join(tempDir, "restricted")
		err := os.Mkdir(restrictedDir, 0555) // read and execute only
		assert.NoError(t, err)
		defer os.Chmod(restrictedDir, 0755) // restore permissions for cleanup

		filepath := filepath.Join(restrictedDir, "restricted_file.txt")

		// This would normally cause os.Exit(4), but we can test the condition
		_, err = os.OpenFile(filepath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
		assert.Error(t, err)
		assert.False(t, os.IsExist(err)) // Should not be a "file exists" error
	})

	t.Run("Create file in current directory", func(t *testing.T) {
		// Change to temp directory
		originalDir, err := os.Getwd()
		assert.NoError(t, err)
		defer os.Chdir(originalDir)

		err = os.Chdir(tempDir)
		assert.NoError(t, err)

		filename := "current_dir_file.txt"

		file := CreateOutputFileOrDie(filename)
		defer file.Close()

		assert.NotNil(t, file)

		// Verify file was created in current directory
		_, err = os.Stat(filename)
		assert.NoError(t, err)
	})

	t.Run("Create file with absolute path", func(t *testing.T) {
		absolutePath := filepath.Join(tempDir, "absolute_path_file.txt")

		file := CreateOutputFileOrDie(absolutePath)
		defer file.Close()

		assert.NotNil(t, file)

		// Verify file was created at absolute path
		_, err := os.Stat(absolutePath)
		assert.NoError(t, err)
	})

	t.Run("File permissions are correct", func(t *testing.T) {
		filepath := filepath.Join(tempDir, "permissions_test.txt")

		file := CreateOutputFileOrDie(filepath)
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

		file := CreateOutputFileOrDie(filepath)
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

// TestCreateOutputFileOrDieErrorConditions tests the error conditions that would
// cause the function to call os.Exit(). We can't test os.Exit() directly in unit tests,
// but we can verify the conditions that lead to it.
func TestCreateOutputFileOrDieErrorConditions(t *testing.T) {
	tempDir := t.TempDir()

	t.Run("Verify os.IsExist error condition", func(t *testing.T) {
		filepath := filepath.Join(tempDir, "exist_check.txt")

		// Create file first
		file, err := os.Create(filepath)
		assert.NoError(t, err)
		file.Close()

		// Now try to create with O_EXCL flag (same as CreateOutputFileOrDie)
		_, err = os.OpenFile(filepath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
		assert.True(t, os.IsExist(err), "Should detect file exists")
	})

	t.Run("Verify other error condition", func(t *testing.T) {
		// Try to create file in non-existent directory
		invalidPath := filepath.Join(tempDir, "nonexistent", "path", "file.txt")

		_, err := os.OpenFile(invalidPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
		assert.Error(t, err)
		assert.False(t, os.IsExist(err), "Should not be a file exists error")
	})
}

// Benchmark tests to ensure the function performs well
func BenchmarkCreateOutputFileOrDie(b *testing.B) {
	tempDir := b.TempDir()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		filepath := filepath.Join(tempDir, "bench_file_"+fmt.Sprintf("%d", i)+".txt")
		file := CreateOutputFileOrDie(filepath)
		file.Close()
	}
}
