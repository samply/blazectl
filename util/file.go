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

// CreateOutputFile creates the output file at the given filepath if it does not
// already exist and returns the file handle.
//
// This is a non-destructive operation. Hence, if a file already exists at the
// given filepath, an ExitCodeError with code 3 is returned. Any other error is
// returned as ExitCodeError with code 4.
//
// Note: The callee has to make sure that the file handle is closed properly.
func CreateOutputFile(filepath string) (*os.File, error) {
	outputFile, err := os.OpenFile(filepath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		if os.IsExist(err) {
			return nil, &ExitCodeError{Code: 3, Err: fmt.Errorf("the output file %s does already exist", filepath)}
		}
		return nil, &ExitCodeError{Code: 4, Err: fmt.Errorf("could not open/create the output file %s: %w", filepath, err)}
	}
	return outputFile, nil
}
