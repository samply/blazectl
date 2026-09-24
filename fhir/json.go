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
	"encoding/json/jsontext"
	"fmt"
	"io"
	"iter"
)

// lenientOptions accept invalid UTF-8 and duplicate names like encoding/json
// v1 does, so that a single odd resource doesn't abort a whole download.
var lenientOptions = []jsontext.Options{
	jsontext.AllowInvalidUTF8(true),
	jsontext.AllowDuplicateNames(true),
	jsontext.PreserveRawStrings(true),
}

func newDecoder(data []byte) *jsontext.Decoder {
	// a bytes.Buffer is decoded in place without copying
	return jsontext.NewDecoder(bytes.NewBuffer(data), lenientOptions...)
}

func newEncoder(w io.Writer) *jsontext.Encoder {
	return jsontext.NewEncoder(w, lenientOptions...)
}

// readBegin reads the begin token of a JSON object or array of the given
// kind from dec. Returns false if a JSON null was read instead.
func readBegin(dec *jsontext.Decoder, kind jsontext.Kind) (bool, error) {
	tok, err := dec.ReadToken()
	if err != nil {
		return false, err
	}
	switch tok.Kind() {
	case jsontext.KindNull:
		return false, nil
	case kind:
		return true, nil
	default:
		return false, kindError(kind, tok.Kind())
	}
}

// members returns an iterator over the member names of a JSON object read
// from dec. Before continuing the iteration, the caller has to consume the
// value of the current member. A name is only valid until the next read from
// dec. If the iteration is stopped early, the rest of the object stays unread.
// A JSON null is treated as an empty object. Errors are yielded and end the
// iteration.
func members(dec *jsontext.Decoder) iter.Seq2[[]byte, error] {
	return func(yield func([]byte, error) bool) {
		ok, err := readBegin(dec, jsontext.KindBeginObject)
		if err != nil {
			yield(nil, err)
			return
		}
		if !ok {
			return
		}
		for dec.PeekKind() != jsontext.KindEndObject {
			name, err := readString(dec)
			if !yield(name, err) || err != nil {
				return
			}
		}
		if _, err := dec.ReadToken(); err != nil {
			yield(nil, err)
		}
	}
}

// elements returns an iterator over the elements of a JSON array read from
// dec. Before continuing the iteration, the caller has to consume the current
// element. If the iteration is stopped early, the rest of the array stays
// unread. A JSON null is treated as an empty array. Errors are yielded and end
// the iteration.
func elements(dec *jsontext.Decoder) iter.Seq[error] {
	return func(yield func(error) bool) {
		ok, err := readBegin(dec, jsontext.KindBeginArray)
		if err != nil {
			yield(err)
			return
		}
		if !ok {
			return
		}
		for dec.PeekKind() != jsontext.KindEndArray {
			if !yield(nil) {
				return
			}
		}
		if _, err := dec.ReadToken(); err != nil {
			yield(err)
		}
	}
}

// readString reads a JSON string from dec and returns it unquoted. The string
// is only valid until the next read from dec. A JSON null is treated as an
// absent string and returns nil.
func readString(dec *jsontext.Decoder) ([]byte, error) {
	value, err := dec.ReadValue()
	if err != nil {
		return nil, err
	}
	if value.Kind() == jsontext.KindNull {
		return nil, nil
	}
	if value.Kind() != jsontext.KindString {
		return nil, kindError(jsontext.KindString, value.Kind())
	}
	// strings without escape sequences need no allocation
	if bytes.IndexByte(value, '\\') < 0 {
		return value[1 : len(value)-1], nil
	}
	return jsontext.AppendUnquote(nil, value)
}

// kindError returns an error stating that a JSON value of the expected kind
// was read as the actual kind.
func kindError(expected, actual jsontext.Kind) error {
	return fmt.Errorf("expected %s but got %s", describeKind(expected), describeKind(actual))
}

// describeKind returns a human-readable description of the JSON value kind
// for use in error messages.
func describeKind(kind jsontext.Kind) string {
	switch kind {
	case jsontext.KindNull:
		return "JSON null"
	case jsontext.KindFalse, jsontext.KindTrue:
		return "a JSON boolean"
	case jsontext.KindString:
		return "a JSON string"
	case jsontext.KindNumber:
		return "a JSON number"
	case jsontext.KindBeginObject:
		return "a JSON object"
	case jsontext.KindBeginArray:
		return "a JSON array"
	default:
		return kind.String()
	}
}
