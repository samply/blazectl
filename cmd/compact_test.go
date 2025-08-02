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

package cmd

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestCreateParameters(t *testing.T) {
	parameters := createParameters("index", "resource-as-of-index")

	assert.Equal(t, "database", parameters.Parameter[0].Name)
	assert.Equal(t, "index", *parameters.Parameter[0].ValueCode)

	assert.Equal(t, "column-family", parameters.Parameter[1].Name)
	assert.Equal(t, "resource-as-of-index", *parameters.Parameter[1].ValueCode)
}
