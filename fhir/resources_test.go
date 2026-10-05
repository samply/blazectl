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
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"testing"
	"testing/iotest"
	"time"

	. "github.com/samply/golang-fhir-models/fhir-models/fhir"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

// writeResourcesOf writes the resources of the bundle in data to sink. Reads
// data one byte at a time, so that values used after they became invalid show
// up as corrupted output.
func writeResourcesOf(data []byte, sink io.Writer) (int, []*OperationOutcome, error) {
	return writeResources(iotest.OneByteReader(bytes.NewReader(data)), sink, func(*url.URL) {})
}

func TestWriteResource(t *testing.T) {
	t.Run("EmptyData", func(t *testing.T) {
		resources, outcomes, err := writeResourcesOf([]byte{}, io.Discard)

		assert.EqualError(t, err, "could not parse the bundle entries from JSON: EOF")
		assert.Equal(t, 0, resources)
		assert.Empty(t, outcomes)
	})

	t.Run("EmptyBundleEntry", func(t *testing.T) {
		data := []byte(`{"entry":[{}]}`)
		resources, outcomes, err := writeResourcesOf(data, io.Discard)

		assert.Nil(t, err)
		assert.Equal(t, 0, resources)
		assert.Empty(t, outcomes)
	})

	t.Run("SingleBundleEntry", func(t *testing.T) {
		data := []byte(`{"entry": [{"resource": {}, "search": {"mode": "match"}}]}`)
		resources, outcomes, err := writeResourcesOf(data, io.Discard)

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
		resources, outcomes, err := writeResourcesOf(bundleRawJSON, io.Discard)

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
		resources, outcomes, err := writeResourcesOf(bundleRawJSON, io.Discard)

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
		resources, outcomes, err := writeResourcesOf(bundleRawJSON, io.Discard)

		assert.Nil(t, err)
		assert.Equal(t, 1, resources)
		assert.NotEmpty(t, outcomes)
	})

	t.Run("WritesCompactNdjson", func(t *testing.T) {
		data := []byte(`{
  "resourceType": "Bundle",
  "link": [{"relation": "self", "url": "http://localhost:8080/fhir/Patient"}],
  "entry": [
    {
      "fullUrl": "http://localhost:8080/fhir/Patient/0",
      "resource": {
        "resourceType": "Patient",
        "id": "0",
        "name": [ { "family": "Müller" } ]
      },
      "search": { "mode": "match" }
    },
    {
      "resource": { "resourceType": "Patient", "id": "1" }
    }
  ]
}`)
		var sink bytes.Buffer
		resources, outcomes, err := writeResourcesOf(data, &sink)

		assert.Nil(t, err)
		assert.Equal(t, 2, resources)
		assert.Empty(t, outcomes)
		assert.Equal(t, `{"resourceType":"Patient","id":"0","name":[{"family":"Müller"}]}
{"resourceType":"Patient","id":"1"}
`, sink.String())
	})

	t.Run("SearchBeforeResource", func(t *testing.T) {
		data := []byte(`{"entry":[
{"search":{"mode":"outcome"},"resource":{"resourceType":"OperationOutcome","issue":[{"severity":"warning","code":"too-long"}]}},
{"search":{"mode":"match"},"resource":{"resourceType":"Patient","id":"0"}}]}`)
		var sink bytes.Buffer
		resources, outcomes, err := writeResourcesOf(data, &sink)

		assert.Nil(t, err)
		assert.Equal(t, 1, resources)
		assert.Len(t, outcomes, 1)
		assert.Equal(t, "{\"resourceType\":\"Patient\",\"id\":\"0\"}\n", sink.String())
	})

	t.Run("NullResource", func(t *testing.T) {
		data := []byte(`{"entry":[{"resource":null}]}`)
		var sink bytes.Buffer
		resources, outcomes, err := writeResourcesOf(data, &sink)

		assert.Nil(t, err)
		assert.Equal(t, 0, resources)
		assert.Empty(t, outcomes)
		assert.Empty(t, sink.String())
	})

	t.Run("DuplicateNames", func(t *testing.T) {
		data := []byte(`{"entry":[{"resource":{"resourceType":"Patient","id":"0","id":"1"}}]}`)
		var sink bytes.Buffer
		resources, _, err := writeResourcesOf(data, &sink)

		assert.Nil(t, err)
		assert.Equal(t, 1, resources)
		assert.Equal(t, "{\"resourceType\":\"Patient\",\"id\":\"0\",\"id\":\"1\"}\n", sink.String())
	})

	t.Run("InvalidJson", func(t *testing.T) {
		data := []byte(`{"entry":[{"resource":{"resourceType":"Patient"}}`)
		_, _, err := writeResourcesOf(data, io.Discard)

		assert.NotNil(t, err)
	})

	t.Run("SearchModeNumberInsteadOfString", func(t *testing.T) {
		data := []byte(`{"entry":[{"resource":{"resourceType":"Patient"},"search":{"mode":1}}]}`)
		_, _, err := writeResourcesOf(data, io.Discard)

		assert.EqualError(t, err, "could not parse the bundle entries from JSON: expected a JSON string but got a JSON number")
	})

	t.Run("ResourceStringInsteadOfObject", func(t *testing.T) {
		data := []byte(`{"entry":[{"resource":"Patient"}]}`)
		var sink bytes.Buffer
		resources, _, err := writeResourcesOf(data, &sink)

		assert.EqualError(t, err, "could not parse the bundle entries from JSON: expected a JSON object but got a JSON string")
		assert.Equal(t, 0, resources)
		assert.Empty(t, sink.String())
	})

	t.Run("NullSearchMode", func(t *testing.T) {
		data := []byte(`{"entry":[{"resource":{"resourceType":"Patient","id":"0"},"search":{"mode":null}}]}`)
		var sink bytes.Buffer
		resources, outcomes, err := writeResourcesOf(data, &sink)

		assert.Nil(t, err)
		assert.Equal(t, 1, resources)
		assert.Empty(t, outcomes)
		assert.Equal(t, "{\"resourceType\":\"Patient\",\"id\":\"0\"}\n", sink.String())
	})
}

// nextLinkOf writes the resources of the bundle in data and returns the
// reported next links alongside the error.
func nextLinkOf(data []byte) ([]*url.URL, error) {
	var nextLinks []*url.URL
	_, _, err := writeResources(bytes.NewReader(data), io.Discard, func(nextLink *url.URL) {
		nextLinks = append(nextLinks, nextLink)
	})
	return nextLinks, err
}

func TestWriteResourcesNextLink(t *testing.T) {
	t.Run("NextLinkBeforeEntries", func(t *testing.T) {
		nextLinks, err := nextLinkOf(searchsetBundle(1))

		assert.Nil(t, err)
		require.Len(t, nextLinks, 1)
		assert.Equal(t, "http://localhost:8080/fhir/__page/0", nextLinks[0].String())
	})

	t.Run("NextLinkAfterEntries", func(t *testing.T) {
		body := []byte(`{"entry":[{"resource":{"resourceType":"Patient","link":[{"type":"seealso"}]}}],
"link":[{"relation":"self","url":"http://localhost:8080/fhir/Patient"},{"relation":"next","url":"http://localhost:8080/fhir/__page/1"}]}`)
		nextLinks, err := nextLinkOf(body)

		assert.Nil(t, err)
		require.Len(t, nextLinks, 1)
		assert.Equal(t, "http://localhost:8080/fhir/__page/1", nextLinks[0].String())
	})

	t.Run("NextLinkNotLast", func(t *testing.T) {
		body := []byte(`{"link":[{"relation":"self","url":"http://localhost:8080/fhir/Patient"},
{"relation":"next","url":"http://localhost:8080/fhir/__page/1"},
{"relation":"previous","url":"http://localhost:8080/fhir/__page/0"}],
"entry":[{"resource":{"resourceType":"Patient"}}]}`)
		var sink bytes.Buffer
		var nextLinks []*url.URL
		resources, _, err := writeResources(bytes.NewReader(body), &sink, func(nextLink *url.URL) {
			nextLinks = append(nextLinks, nextLink)
		})

		assert.Nil(t, err)
		require.Len(t, nextLinks, 1)
		assert.Equal(t, "http://localhost:8080/fhir/__page/1", nextLinks[0].String())
		assert.Equal(t, 1, resources)
		assert.Equal(t, "{\"resourceType\":\"Patient\"}\n", sink.String())
	})

	t.Run("NoNextLink", func(t *testing.T) {
		body := []byte(`{"link":[{"relation":"self","url":"http://localhost:8080/fhir/Patient"}],"entry":[]}`)
		nextLinks, err := nextLinkOf(body)

		assert.Nil(t, err)
		assert.Empty(t, nextLinks)
	})

	t.Run("NullRelationAndUrl", func(t *testing.T) {
		body := []byte(`{"link":[{"relation":null,"url":"http://localhost:8080/fhir/Patient"},
{"relation":"self","url":null},
{"relation":"next","url":"http://localhost:8080/fhir/__page/1"}]}`)
		nextLinks, err := nextLinkOf(body)

		assert.Nil(t, err)
		require.Len(t, nextLinks, 1)
		assert.Equal(t, "http://localhost:8080/fhir/__page/1", nextLinks[0].String())
	})

	t.Run("NullUrlOfNextLink", func(t *testing.T) {
		nextLinks, err := nextLinkOf([]byte(`{"link":[{"relation":"next","url":null}]}`))

		assert.NotNil(t, err)
		assert.Empty(t, nextLinks)
	})

	t.Run("NoLinks", func(t *testing.T) {
		nextLinks, err := nextLinkOf([]byte(`{"entry":[]}`))

		assert.Nil(t, err)
		assert.Empty(t, nextLinks)
	})

	t.Run("InvalidUrl", func(t *testing.T) {
		nextLinks, err := nextLinkOf([]byte(`{"link":[{"relation":"next","url":"__page"}]}`))

		assert.NotNil(t, err)
		assert.Empty(t, nextLinks)
	})

	t.Run("LinkObjectInsteadOfArray", func(t *testing.T) {
		_, err := nextLinkOf([]byte(`{"link":{"relation":"next","url":"http://localhost:8080/fhir/__page/1"}}`))

		assert.EqualError(t, err, "could not parse the bundle entries from JSON: expected a JSON array but got a JSON object")
	})

	t.Run("InvalidJson", func(t *testing.T) {
		nextLinks, err := nextLinkOf([]byte(`{"link":[{"relation":"next"`))

		assert.NotNil(t, err)
		assert.Empty(t, nextLinks)
	})

	t.Run("ReadError", func(t *testing.T) {
		readErr := errors.New("connection reset")
		r := io.MultiReader(bytes.NewReader([]byte(`{"entry":[`)), iotest.ErrReader(readErr))
		_, _, err := writeResources(r, io.Discard, func(*url.URL) {})

		assert.ErrorIs(t, err, readErr)
	})

	t.Run("ReportsNextLinkBeforeEntriesAreRead", func(t *testing.T) {
		r, w := io.Pipe()
		reported := make(chan *url.URL, 1)
		done := make(chan error, 1)
		go func() {
			_, _, err := writeResources(r, io.Discard, func(nextLink *url.URL) {
				reported <- nextLink
			})
			done <- err
		}()

		_, _ = w.Write([]byte(`{"link":[{"relation":"next","url":"http://localhost:8080/fhir/__page/1"}],"entry":[`))
		select {
		case nextLink := <-reported:
			assert.Equal(t, "http://localhost:8080/fhir/__page/1", nextLink.String())
		case <-time.After(5 * time.Second):
			t.Fatal("next link wasn't reported before the entries were read")
		}
		_, _ = w.Write([]byte(`]}`))
		_ = w.Close()
		assert.Nil(t, <-done)
	})
}

// searchsetBundle returns the JSON of a searchset bundle with the given number
// of Observation entries.
func searchsetBundle(entries int) []byte {
	var buf bytes.Buffer
	buf.WriteString(`{"resourceType":"Bundle","type":"searchset","link":[{"relation":"self","url":"http://localhost:8080/fhir/Observation"},{"relation":"next","url":"http://localhost:8080/fhir/__page/0"}],"entry":[`)
	for i := range entries {
		if i > 0 {
			buf.WriteByte(',')
		}
		fmt.Fprintf(&buf, `{"fullUrl":"http://localhost:8080/fhir/Observation/%[1]d","resource":{"resourceType":"Observation","id":"%[1]d","meta":{"versionId":"1","lastUpdated":"2026-01-01T00:00:00Z"},"status":"final","code":{"coding":[{"system":"http://loinc.org","code":"718-7","display":"Hemoglobin [Mass/volume] in Blood"}]},"subject":{"reference":"Patient/%[1]d"},"effectiveDateTime":"2025-05-01T10:00:00+02:00","valueQuantity":{"value":13.4,"unit":"g/dL","system":"http://unitsofmeasure.org","code":"g/dL"}},"search":{"mode":"match"}}`, i)
	}
	buf.WriteString(`]}`)
	return buf.Bytes()
}

func BenchmarkWriteResources(b *testing.B) {
	data := searchsetBundle(1000)
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	for b.Loop() {
		if _, _, err := writeResources(bytes.NewReader(data), io.Discard, func(*url.URL) {}); err != nil {
			b.Fatal(err)
		}
	}
}

func TestDoesSupportSystemOperation(t *testing.T) {
	t.Run("empty capability statement", func(t *testing.T) {
		assert.False(t, DoesSupportSystemOperation(CapabilityStatement{}, "disk-perf"))
	})

	t.Run("server rest without operations", func(t *testing.T) {
		capabilityStatement := CapabilityStatement{
			Rest: []CapabilityStatementRest{{Mode: RestfulCapabilityModeServer}},
		}

		assert.False(t, DoesSupportSystemOperation(capabilityStatement, "disk-perf"))
	})

	t.Run("server rest with other operation", func(t *testing.T) {
		capabilityStatement := CapabilityStatement{
			Rest: []CapabilityStatementRest{{
				Mode:      RestfulCapabilityModeServer,
				Operation: []CapabilityStatementRestResourceOperation{{Name: "compact"}},
			}},
		}

		assert.False(t, DoesSupportSystemOperation(capabilityStatement, "disk-perf"))
	})

	t.Run("server rest with matching operation", func(t *testing.T) {
		capabilityStatement := CapabilityStatement{
			Rest: []CapabilityStatementRest{{
				Mode: RestfulCapabilityModeServer,
				Operation: []CapabilityStatementRestResourceOperation{
					{Name: "compact"},
					{Name: "disk-perf"},
				},
			}},
		}

		assert.True(t, DoesSupportSystemOperation(capabilityStatement, "disk-perf"))
	})

	t.Run("client rest with matching operation", func(t *testing.T) {
		capabilityStatement := CapabilityStatement{
			Rest: []CapabilityStatementRest{{
				Mode:      RestfulCapabilityModeClient,
				Operation: []CapabilityStatementRestResourceOperation{{Name: "disk-perf"}},
			}},
		}

		assert.False(t, DoesSupportSystemOperation(capabilityStatement, "disk-perf"))
	})
}
