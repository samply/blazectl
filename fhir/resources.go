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
	"encoding/json/jsontext"
	"fmt"
	"io"
	"net/url"

	fm "github.com/samply/golang-fhir-models/fhir-models/fhir"
)

var ResourceTypes = []string{
	"Account",
	"ActivityDefinition",
	"AdverseEvent",
	"AllergyIntolerance",
	"Appointment",
	"AppointmentResponse",
	"AuditEvent",
	"Basic",
	"Binary",
	"BiologicallyDerivedProduct",
	"BodyStructure",
	"Bundle",
	"CapabilityStatement",
	"CarePlan",
	"CareTeam",
	"CatalogEntry",
	"ChargeItem",
	"ChargeItemDefinition",
	"Claim",
	"ClaimResponse",
	"ClinicalImpression",
	"CodeSystem",
	"Communication",
	"CommunicationRequest",
	"CompartmentDefinition",
	"Composition",
	"ConceptMap",
	"Condition",
	"Consent",
	"Contract",
	"Coverage",
	"CoverageEligibilityRequest",
	"CoverageEligibilityResponse",
	"DetectedIssue",
	"Device",
	"DeviceDefinition",
	"DeviceMetric",
	"DeviceRequest",
	"DeviceUseStatement",
	"DiagnosticReport",
	"DocumentManifest",
	"DocumentReference",
	"EffectEvidenceSynthesis",
	"Encounter",
	"Endpoint",
	"EnrollmentRequest",
	"EnrollmentResponse",
	"EpisodeOfCare",
	"EventDefinition",
	"Evidence",
	"EvidenceVariable",
	"ExampleScenario",
	"ExplanationOfBenefit",
	"FamilyMemberHistory",
	"Flag",
	"Goal",
	"GraphDefinition",
	"Group",
	"GuidanceResponse",
	"HealthcareService",
	"ImagingStudy",
	"Immunization",
	"ImmunizationEvaluation",
	"ImmunizationRecommendation",
	"ImplementationGuide",
	"InsurancePlan",
	"Invoice",
	"Library",
	"Linkage",
	"List",
	"Location",
	"Measure",
	"MeasureReport",
	"Media",
	"Medication",
	"MedicationAdministration",
	"MedicationDispense",
	"MedicationKnowledge",
	"MedicationRequest",
	"MedicationStatement",
	"MedicinalProduct",
	"MedicinalProductAuthorization",
	"MedicinalProductContraindication",
	"MedicinalProductIndication",
	"MedicinalProductIngredient",
	"MedicinalProductInteraction",
	"MedicinalProductManufactured",
	"MedicinalProductPackaged",
	"MedicinalProductPharmaceutical",
	"MedicinalProductUndesirableEffect",
	"MessageDefinition",
	"MessageHeader",
	"MolecularSequence",
	"NamingSystem",
	"NutritionOrder",
	"Observation",
	"ObservationDefinition",
	"OperationDefinition",
	"OperationOutcome",
	"Organization",
	"OrganizationAffiliation",
	"Patient",
	"PaymentNotice",
	"PaymentReconciliation",
	"Person",
	"PlanDefinition",
	"Practitioner",
	"PractitionerRole",
	"Procedure",
	"Provenance",
	"Questionnaire",
	"QuestionnaireResponse",
	"RelatedPerson",
	"RequestGroup",
	"ResearchDefinition",
	"ResearchElementDefinition",
	"ResearchStudy",
	"ResearchSubject",
	"RiskAssessment",
	"RiskEvidenceSynthesis",
	"Schedule",
	"SearchParameter",
	"ServiceRequest",
	"Slot",
	"Specimen",
	"SpecimenDefinition",
	"StructureDefinition",
	"StructureMap",
	"Subscription",
	"Substance",
	"SubstanceNucleicAcid",
	"SubstancePolymer",
	"SubstanceProtein",
	"SubstanceReferenceInformation",
	"SubstanceSourceMaterial",
	"SubstanceSpecification",
	"SupplyDelivery",
	"SupplyRequest",
	"Task",
	"TerminologyCapabilities",
	"TestReport",
	"TestScript",
	"ValueSet",
	"VerificationResult",
	"VisionPrescription",
}

// DoesSupportsInteraction returns true if the resource supports the given
// interaction. Possible interactions are defined in
// https://www.hl7.org/fhir/valueset-type-restful-interaction.html
func DoesSupportsInteraction(r fm.CapabilityStatementRestResource, code fm.TypeRestfulInteraction) bool {
	for _, interaction := range r.Interaction {
		if interaction.Code == code {
			return true
		}
	}
	return false
}

// DoesSupportSystemOperation returns true if the capability statement declares
// a system-level operation with the given name.
func DoesSupportSystemOperation(capabilityStatement fm.CapabilityStatement, name string) bool {
	for _, rest := range capabilityStatement.Rest {
		if rest.Mode == fm.RestfulCapabilityModeServer {
			for _, operation := range rest.Operation {
				if operation.Name == name {
					return true
				}
			}
		}
	}
	return false
}

// writeResources reads a bundle from r and writes the resource of each of its
// entries to sink, so that all information resembles a valid NDJSON stream.
// Calls onNextLink with the next link of the bundle as soon as the links are
// read, so that the next page can be requested while the entries are still
// being read. onNextLink isn't called if the bundle has no next link.
//
// Always returns the number of written resources alongside all encountered
// inline operation outcomes, also if there is an error. An error can only occur
// if reading the bundle or writing to sink fails or the bundle is invalid.
func writeResources(r io.Reader, sink io.Writer, onNextLink func(*url.URL)) (int, []*fm.OperationOutcome, error) {
	var resources int
	var inlineOutcomes []*fm.OperationOutcome

	dec := newDecoder(r)
	enc := newEncoder(sink)
	// holds the resource of the current entry and is reused for all entries
	var resource []byte

	for name, err := range members(dec) {
		if err != nil {
			return resources, inlineOutcomes, entriesParseError(err)
		}
		if string(name) == "link" {
			nextLink, err := readNextLinks(dec)
			if err != nil {
				return resources, inlineOutcomes, entriesParseError(err)
			}
			if nextLink != nil {
				onNextLink(nextLink)
			}
			continue
		}
		if string(name) != "entry" {
			if err := dec.SkipValue(); err != nil {
				return resources, inlineOutcomes, entriesParseError(err)
			}
			continue
		}
		for err := range elements(dec) {
			if err != nil {
				return resources, inlineOutcomes, entriesParseError(err)
			}
			var isOutcome bool
			resource, isOutcome, err = readEntry(dec, resource[:0])
			if err != nil {
				return resources, inlineOutcomes, entriesParseError(err)
			}

			if len(resource) == 0 {
				continue
			}

			if isOutcome {
				outcome, err := fm.UnmarshalOperationOutcome(resource)
				if err != nil {
					return resources, inlineOutcomes, fmt.Errorf("could not parse an encountered inline outcome from JSON: %v", err)
				}
				inlineOutcomes = append(inlineOutcomes, &outcome)
				continue
			}

			if err := enc.WriteValue(resource); err != nil {
				return resources, inlineOutcomes, fmt.Errorf("could not write resource to output file: %w", err)
			}
			resources++
		}
	}

	return resources, inlineOutcomes, nil
}

func entriesParseError(err error) error {
	return fmt.Errorf("could not parse the bundle entries from JSON: %w", err)
}

// readEntry reads a bundle entry from dec. Returns the resource of the entry
// appended to buf and whether the entry is an inline outcome.
func readEntry(dec *jsontext.Decoder, buf []byte) ([]byte, bool, error) {
	resource := buf
	var isOutcome bool
	for name, err := range members(dec) {
		if err != nil {
			return nil, false, err
		}
		switch string(name) {
		case "resource":
			var value jsontext.Value
			value, err = dec.ReadValue()
			if err != nil {
				break
			}
			switch value.Kind() {
			case jsontext.KindNull:
			case jsontext.KindBeginObject:
				// value is only valid until the next read
				resource = append(buf, value...)
			default:
				err = kindError(jsontext.KindBeginObject, value.Kind())
			}
		case "search":
			isOutcome, err = readIsOutcome(dec)
		default:
			err = dec.SkipValue()
		}
		if err != nil {
			return nil, false, err
		}
	}
	return resource, isOutcome, nil
}

// readIsOutcome reads the search part of a bundle entry from dec and returns
// whether its mode is outcome.
func readIsOutcome(dec *jsontext.Decoder) (bool, error) {
	var isOutcome bool
	for name, err := range members(dec) {
		if err != nil {
			return false, err
		}
		if string(name) != "mode" {
			err = dec.SkipValue()
		} else {
			var mode []byte
			mode, err = readString(dec)
			isOutcome = string(mode) == fm.SearchEntryModeOutcome.Code()
		}
		if err != nil {
			return false, err
		}
	}
	return isOutcome, nil
}
