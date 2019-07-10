package fhir

type CapabilityStatementRestResource struct {
	Type string
}

type CapabilityStatementRest struct {
	Mode     string
	Resource []CapabilityStatementRestResource
}

type CapabilityStatement struct {
	FhirVersion string
	Rest        []CapabilityStatementRest
}

type Bundle struct {
	Type  string
	Total int
}
