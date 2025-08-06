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
	"fmt"
	"io"

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

type entryBundle struct {
	Entry []fm.BundleEntry `bson:"entry,omitempty" json:"entry,omitempty"`
}

// WriteResources takes a raw set of FHIR bundle entries and writes the resource part of each of them to the given
// sink. The data is written to the sink so that all information resembles a valid NDJSON stream.
//
// Always returns the number of written resources alongside all inline encountered operation outcomes.
// This is also true for when there is an error. An error is returned alongside the other information
// and can only occur if there is an actual issue writing to the file or the given resource bundle is
// invalid in regard to the FHIR specification.
func WriteResources(data []byte, sink io.Writer) (int, []*fm.OperationOutcome, error) {
	var resources int
	var inlineOutcomes []*fm.OperationOutcome

	if len(data) == 0 {
		return resources, inlineOutcomes, nil
	}

	var bundle entryBundle
	if err := json.Unmarshal(data, &bundle); err != nil {
		return resources, inlineOutcomes, fmt.Errorf("could not parse the bundle entries from JSON: %v", err)
	}

	var buf bytes.Buffer
	for _, e := range bundle.Entry {
		if e.Resource == nil {
			continue
		}

		if e.Search != nil && *e.Search.Mode == fm.SearchEntryModeOutcome {
			outcome, err := fm.UnmarshalOperationOutcome(e.Resource)
			if err != nil {
				return resources, inlineOutcomes, fmt.Errorf("could not parse an encountered inline outcome from JSON: %v", err)
			}

			inlineOutcomes = append(inlineOutcomes, &outcome)
			continue
		}

		buf.Reset()
		err := json.Compact(&buf, e.Resource)
		if err != nil {
			return resources, inlineOutcomes, fmt.Errorf("could not compact JSON representation for write operation: %v", err)
		}

		_, err = sink.Write(buf.Bytes())
		if err != nil {
			return resources, inlineOutcomes, fmt.Errorf("could not write resource to output file: %v", err)
		}

		_, err = sink.Write([]byte{'\n'})
		if err != nil {
			return resources, inlineOutcomes, fmt.Errorf("could not write resource separator to output file: %v", err)
		}
		resources++
	}

	return resources, inlineOutcomes, nil
}
