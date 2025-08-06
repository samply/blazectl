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
)

// CreateOutputFileOrDie creates the output file at the given filepath if it does not already exist
// and returns the file handle.
// This is a non-destructive operation. Hence, if a file already exists at the given filepath then
// the command exits with a non-success error code. If any other error case the command exits with
// a non-success error code as well.
//
// Note: The callee has to make sure that the file handle is closed properly.
func CreateOutputFileOrDie(filepath string) *os.File {
	outputFile, err := os.OpenFile(filepath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		if os.IsExist(err) {
			fmt.Printf("The output file %s does already exist.\n", filepath)
			os.Exit(3)
		} else {
			fmt.Printf("could not open/create the output file %s: %v\n", filepath, err)
			os.Exit(4)
		}
	}
	return outputFile
}
