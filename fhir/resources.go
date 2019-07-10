/*
Copyright © 2019 Alexander Kiel <alexander.kiel@life.uni-leipzig.de>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package fhir contains structs for FHIR resources usable with JSON marshalling.
package fhir

// CapabilityStatementRestResource represents the CapabilityStatement.rest.resource
// BackboneElement
type CapabilityStatementRestResource struct {
	Type string
}

// CapabilityStatementRest represents the CapabilityStatement.rest BackboneElement
type CapabilityStatementRest struct {
	Mode     string
	Resource []CapabilityStatementRestResource
}

// CapabilityStatement is documented here
// https://www.hl7.org/fhir/capabilitystatement.html
type CapabilityStatement struct {
	FhirVersion string
	Rest        []CapabilityStatementRest
}

// Bundle is documented here https://www.hl7.org/fhir/bundle.html
type Bundle struct {
	Type  string
	Total int
}
