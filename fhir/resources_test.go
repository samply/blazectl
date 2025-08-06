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

package fhir

import (
	"encoding/json"
	"io"
	"testing"

	. "github.com/samply/golang-fhir-models/fhir-models/fhir"
	"github.com/stretchr/testify/assert"
)

func TestUnmarshalBundleEntryResource(t *testing.T) {
	var bundle Bundle
	if err := json.Unmarshal([]byte(`{
"resourceType": "Bundle",
"type": "batch-response",
"entry": [{
  "resource": {
    "resourceType": "Bundle",
    "type": "searchset",
    "total": 23
}}]}`), &bundle); err != nil {
		t.Error(err)
	}
	if err := json.Unmarshal(bundle.Entry[0].Resource, &bundle); err != nil {
		t.Error(err)
	}
	assert.Equal(t, 23, *bundle.Total)
}

func TestWriteResource(t *testing.T) {
	t.Run("EmptyData", func(t *testing.T) {
		resources, outcomes, err := WriteResources([]byte{}, io.Discard)

		assert.Nil(t, err)
		assert.Equal(t, 0, resources)
		assert.Empty(t, outcomes)
	})

	t.Run("EmptyBundleEntry", func(t *testing.T) {
		data := []byte(`{"entry":[{}]}`)
		resources, outcomes, err := WriteResources(data, io.Discard)

		assert.Nil(t, err)
		assert.Equal(t, 0, resources)
		assert.Empty(t, outcomes)
	})

	t.Run("SingleBundleEntry", func(t *testing.T) {
		data := []byte(`{"entry": [{"resource": {}, "search": {"mode": "match"}}]}`)
		resources, outcomes, err := WriteResources(data, io.Discard)

		assert.Nil(t, err)
		assert.Equal(t, 1, resources)
		assert.Empty(t, outcomes)
	})

	t.Run("SingleBundleEntryWithInlineOutcome", func(t *testing.T) {
		outcome := OperationOutcome{
			Issue: []OperationOutcomeIssue{{
				Severity: IssueSeverityWarning,
				Code:     IssueTypeTooLong,
			}},
		}

		outcomeRawJSON, _ := json.Marshal(outcome)

		searchMode := SearchEntryModeOutcome

		var bundleEntry BundleEntry
		bundleEntry.Resource = outcomeRawJSON
		bundleEntry.Search = &BundleEntrySearch{
			Mode: &searchMode,
		}
		var bundle Bundle
		bundle.Entry = []BundleEntry{bundleEntry}

		bundleRawJSON, _ := json.Marshal(bundle)
		resources, outcomes, err := WriteResources(bundleRawJSON, io.Discard)

		assert.Nil(t, err)
		assert.Equal(t, 0, resources)
		assert.NotEmpty(t, outcomes)
	})

	t.Run("MultipleBundleEntries", func(t *testing.T) {
		searchMode := SearchEntryModeMatch

		var bundleEntryA BundleEntry
		bundleEntryA.Resource = []byte("{}")
		bundleEntryA.Search = &BundleEntrySearch{
			Mode: &searchMode,
		}
		var bundleEntryB BundleEntry
		bundleEntryB.Resource = []byte("{}")
		bundleEntryB.Search = &BundleEntrySearch{
			Mode: &searchMode,
		}
		var bundle Bundle
		bundle.Entry = []BundleEntry{bundleEntryA, bundleEntryB}

		bundleRawJSON, _ := json.Marshal(bundle)
		resources, outcomes, err := WriteResources(bundleRawJSON, io.Discard)

		assert.Nil(t, err)
		assert.Equal(t, 2, resources)
		assert.Empty(t, outcomes)
	})

	t.Run("MultipleBundleEntriesWithSingleInlineOutcome", func(t *testing.T) {
		searchModeA := SearchEntryModeMatch
		searchModeB := SearchEntryModeOutcome

		outcome := OperationOutcome{
			Issue: []OperationOutcomeIssue{{
				Severity: IssueSeverityWarning,
				Code:     IssueTypeTooLong,
			}},
		}
		outcomeRawJSON, _ := json.Marshal(outcome)

		var bundleEntryA BundleEntry
		bundleEntryA.Resource = []byte("{}")
		bundleEntryA.Search = &BundleEntrySearch{
			Mode: &searchModeA,
		}
		var bundleEntryB BundleEntry
		bundleEntryB.Resource = outcomeRawJSON
		bundleEntryB.Search = &BundleEntrySearch{
			Mode: &searchModeB,
		}
		var bundle Bundle
		bundle.Entry = []BundleEntry{bundleEntryA, bundleEntryB}

		bundleRawJSON, _ := json.Marshal(bundle)
		resources, outcomes, err := WriteResources(bundleRawJSON, io.Discard)

		assert.Nil(t, err)
		assert.Equal(t, 1, resources)
		assert.NotEmpty(t, outcomes)
	})
}
