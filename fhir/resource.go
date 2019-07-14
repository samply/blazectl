package fhir

type Resource struct {
	Json []byte
}

func (b Resource) MarshalJSON() ([]byte, error) {
	json := make([]byte, len(b.Json))
	copy(json, b.Json)
	return json, nil
}

func (r *Resource) UnmarshalJSON(json []byte) error {
	r.Json = make([]byte, len(json))
	copy(r.Json, json)
	return nil
}
