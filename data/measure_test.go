package data

import (
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/stretchr/testify/assert"
)

func TestUnmarshalMeasureParameters(t *testing.T) {
	t.Run("scalar values keep their literal form", func(t *testing.T) {
		src := `
library: foo.cql
parameter:
- name: MinAge
  type: integer
  value: 18
- name: Start
  type: date
  value: 2020-01-01
- name: Ratio
  type: decimal
  value: "1.50"
group:
- population:
  - expression: InInitialPopulation
`
		var m Measure
		assert.Nil(t, yaml.Unmarshal([]byte(src), &m))

		assert.Equal(t, 3, len(m.Parameter))
		assert.Equal(t, "MinAge", m.Parameter[0].Name)
		assert.Equal(t, "integer", m.Parameter[0].Type)
		assert.Equal(t, []string{"18"}, m.Parameter[0].Value.Values)
		assert.Equal(t, []string{"2020-01-01"}, m.Parameter[1].Value.Values)
		assert.Equal(t, []string{"1.50"}, m.Parameter[2].Value.Values)
	})

	t.Run("a sequence value is a list", func(t *testing.T) {
		src := `
parameter:
- name: Codes
  type: code
  value:
  - "38341003"
  - "73211009"
`
		var m Measure
		assert.Nil(t, yaml.Unmarshal([]byte(src), &m))

		assert.Equal(t, 1, len(m.Parameter))
		assert.Equal(t, []string{"38341003", "73211009"}, m.Parameter[0].Value.Values)
	})

	t.Run("a missing value yields no values", func(t *testing.T) {
		src := `
parameter:
- name: MinAge
  type: integer
`
		var m Measure
		assert.Nil(t, yaml.Unmarshal([]byte(src), &m))

		assert.Equal(t, 1, len(m.Parameter))
		assert.Empty(t, m.Parameter[0].Value.Values)
	})
}
