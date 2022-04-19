// Copyright 2019 - 2022 The Samply Community
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

package cmd

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"os"
	"path/filepath"
	"testing"
)

func TestFindProcessableFiles(t *testing.T) {

	for _, fileExt := range []string{"json", "json.gz", "json.bz2"} {

		t.Run("dir with one "+fileExt+" file", func(t *testing.T) {
			dir, err := os.MkdirTemp("", "bundles")
			if err != nil {
				t.Fatal("can't create a temp dir")
			}
			defer os.Remove(dir)
			bundlePath := filepath.Join(dir, "bundle."+fileExt)
			err = os.WriteFile(bundlePath, []byte("{}"), 0644)
			if err != nil {
				t.Fatal("can't create a temp " + fileExt + " file")
			}
			defer os.Remove(bundlePath)
			files, err := findProcessableFiles(dir)
			if err != nil {
				t.Fatalf("error file filtering processable files %v", err)
			}
			assert.Equal(t, bundlePath, files.singleBundleFiles[0])
		})

		t.Run("dir with two "+fileExt+" files", func(t *testing.T) {
			dir, err := os.MkdirTemp("", "bundles")
			if err != nil {
				t.Fatal("can't create a temp dir")
			}
			defer os.Remove(dir)
			bundlePath1 := filepath.Join(dir, "bundle1."+fileExt)
			err = os.WriteFile(bundlePath1, []byte("{}"), 0644)
			if err != nil {
				t.Fatal("can't create a temp " + fileExt + " file")
			}
			defer os.Remove(bundlePath1)
			bundlePath2 := filepath.Join(dir, "bundle2."+fileExt)
			err = os.WriteFile(bundlePath2, []byte("{}"), 0644)
			if err != nil {
				t.Fatal("can't create a temp " + fileExt + " file")
			}
			defer os.Remove(bundlePath2)
			files, err := findProcessableFiles(dir)
			if err != nil {
				t.Fatalf("error file filtering processable files %v", err)
			}
			assert.Equal(t, bundlePath1, files.singleBundleFiles[0])
			assert.Equal(t, bundlePath2, files.singleBundleFiles[1])
		})

		t.Run("dir with one "+fileExt+" file and one in a sub dir", func(t *testing.T) {
			dir, err := os.MkdirTemp("", "bundles")
			if err != nil {
				t.Fatal("can't create a temp dir")
			}
			defer os.Remove(dir)
			bundlePath1 := filepath.Join(dir, "bundle1."+fileExt)
			err = os.WriteFile(bundlePath1, []byte("{}"), 0644)
			if err != nil {
				t.Fatal("can't create a temp " + fileExt + " file")
			}
			defer os.Remove(bundlePath1)
			subDir, err := os.MkdirTemp(dir, "bundles")
			if err != nil {
				t.Fatal("can't create a temp dir")
			}
			fmt.Println("subDir:", subDir)
			defer os.Remove(subDir)
			bundlePath2 := filepath.Join(subDir, "bundle2."+fileExt)
			err = os.WriteFile(bundlePath2, []byte("{}"), 0644)
			if err != nil {
				t.Fatal("can't create a temp " + fileExt + " file")
			}
			defer os.Remove(bundlePath2)
			files, err := findProcessableFiles(dir)
			if err != nil {
				t.Fatalf("error file filtering processable files %v", err)
			}
			assert.Equal(t, bundlePath1, files.singleBundleFiles[0])
			assert.Equal(t, bundlePath2, files.singleBundleFiles[1])
		})
	}

	t.Run("dir with one ndjson file", func(t *testing.T) {
		dir, err := os.MkdirTemp("", "bundles")
		if err != nil {
			t.Fatal("can't create a temp dir")
		}
		defer os.Remove(dir)
		bundlePath := filepath.Join(dir, "bundle.ndjson")
		err = os.WriteFile(bundlePath, []byte("{}"), 0644)
		if err != nil {
			t.Fatal("can't create a temp ndjson file")
		}
		defer os.Remove(bundlePath)
		files, err := findProcessableFiles(dir)
		if err != nil {
			t.Fatalf("error file filtering processable files %v", err)
		}
		assert.Equal(t, bundlePath, files.multiBundleFiles[0])
	})

	t.Run("dir with two ndjson files", func(t *testing.T) {
		dir, err := os.MkdirTemp("", "bundles")
		if err != nil {
			t.Fatal("can't create a temp dir")
		}
		defer os.Remove(dir)
		bundlePath1 := filepath.Join(dir, "bundle1.ndjson")
		err = os.WriteFile(bundlePath1, []byte("{}"), 0644)
		if err != nil {
			t.Fatal("can't create a temp ndjson file")
		}
		defer os.Remove(bundlePath1)
		bundlePath2 := filepath.Join(dir, "bundle2.ndjson")
		err = os.WriteFile(bundlePath2, []byte("{}"), 0644)
		if err != nil {
			t.Fatal("can't create a temp ndjson file")
		}
		defer os.Remove(bundlePath2)
		files, err := findProcessableFiles(dir)
		if err != nil {
			t.Fatalf("error file filtering processable files %v", err)
		}
		assert.Equal(t, bundlePath1, files.multiBundleFiles[0])
		assert.Equal(t, bundlePath2, files.multiBundleFiles[1])
	})
}
