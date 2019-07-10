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
