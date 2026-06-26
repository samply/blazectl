package data

import "github.com/goccy/go-yaml"

type Population struct {
	Code       string
	Expression string
}

type Stratifier struct {
	Code        string
	Description string
	Expression  string
}

type Group struct {
	Type        string
	Code        string
	Description string
	Population  []Population
	Stratifier  []Stratifier
}

// Parameter is a CQL parameter that is passed to the $evaluate-measure
// operation. Its Type is one of the supported FHIR primitive types (string,
// code, date, dateTime, boolean, integer, decimal). The Value can be overridden
// on the command line.
type Parameter struct {
	Name  string
	Type  string
	Value ParameterValue
}

// ParameterValue is the value of a Parameter. In the measure file it can be
// given either as a single scalar or as a sequence of scalars. A sequence is
// mapped to a CQL list.
type ParameterValue struct {
	Values []string
}

// UnmarshalYAML accepts both a single scalar and a sequence of scalars. Scalars
// are kept as their literal string so that the declared type drives the
// conversion. Values whose exact lexical form matters (e.g. codes with leading
// zeros) should be quoted in the YAML file.
func (v *ParameterValue) UnmarshalYAML(b []byte) error {
	var values []string
	if err := yaml.Unmarshal(b, &values); err == nil {
		v.Values = values
		return nil
	}
	var value string
	if err := yaml.Unmarshal(b, &value); err != nil {
		return err
	}
	v.Values = []string{value}
	return nil
}

type Measure struct {
	Library   string
	Parameter []Parameter
	Group     []Group
}
