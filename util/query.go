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
	"net/url"
	"os"
	"strings"
)

// ReadQueryFromFile reads a file and parses the content as URL query values.
//
// The filename is expected to start with a `@` which is stripped of.
func ReadQueryFromFile(filename string) (url.Values, error) {
	b, err := os.ReadFile(strings.TrimPrefix(filename, "@"))
	if err != nil {
		return nil, fmt.Errorf("error while reading file: %s: %w", filename, err)
	}
	q, err := url.ParseQuery(strings.TrimSpace(string(b)))
	if err != nil {
		return nil, fmt.Errorf("error while parsing query: %w", err)
	}
	return q, nil
}
