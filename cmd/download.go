// Copyright 2019 - 2022 The Samply Community
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

package cmd

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/samply/blazectl/fhir"
	"github.com/samply/blazectl/util"
	fm "github.com/samply/golang-fhir-models/fhir-models/fhir"
	"github.com/spf13/cobra"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"
)

var outputFile string
var fhirSearchQuery string
var usePost bool

type commandStats struct {
	totalPages                            int
	resourcesPerPage                      []int
	requestDurations, processingDurations []float64
	totalBytesIn                          int64
	totalDuration                         time.Duration
	inlineOperationOutcomes               []*fm.OperationOutcome
	error                                 *util.ErrorResponse
}

func (cs *commandStats) String() string {

	builder := strings.Builder{}
	builder.WriteString(fmt.Sprintf("Pages		[total]			%d\n", cs.totalPages))

	var resourcesTotal int
	for _, res := range cs.resourcesPerPage {
		resourcesTotal += res
	}
	builder.WriteString(fmt.Sprintf("Resources 	[total]			%d\n", resourcesTotal))

	if len(cs.resourcesPerPage) > 0 {
		sort.Ints(cs.resourcesPerPage)
		var totalResources int
		for _, v := range cs.resourcesPerPage {
			totalResources += v
		}

		builder.WriteString(fmt.Sprintf("Resources/Page	[min, mean, max]	%d, %d, %d\n", cs.resourcesPerPage[0], totalResources/len(cs.resourcesPerPage), cs.resourcesPerPage[len(cs.resourcesPerPage)-1]))
	}

	builder.WriteString(fmt.Sprintf("Duration	[total]			%s\n", util.FmtDurationHumanReadable(cs.totalDuration)))

	if len(cs.requestDurations) > 0 {
		p := util.CalculateDurationStatistics(cs.requestDurations)
		builder.WriteString(fmt.Sprintf("Requ. Latencies	[mean, 50, 95, 99, max]	%s, %s, %s, %s, %s\n", p.Mean, p.Q50, p.Q95, p.Q99, p.Max))
	}

	if len(cs.processingDurations) > 0 {
		p := util.CalculateDurationStatistics(cs.processingDurations)
		builder.WriteString(fmt.Sprintf("Proc. Latencies	[mean, 50, 95, 99, max]	%s, %s, %s, %s, %s\n", p.Mean, p.Q50, p.Q95, p.Q99, p.Max))
	}

	totalRequests := len(cs.requestDurations)
	builder.WriteString(fmt.Sprintf("Bytes In	[total, mean]		%s, %s\n", util.FmtBytesHumanReadable(float32(cs.totalBytesIn)), util.FmtBytesHumanReadable(float32(cs.totalBytesIn)/float32(totalRequests))))

	if len(cs.inlineOperationOutcomes) > 0 {
		builder.WriteString("\nServer Warnings & Information:\n")
		builder.WriteString(util.Indent(2, util.FmtOperationOutcomes(cs.inlineOperationOutcomes)))
	}

	if cs.error != nil {
		builder.WriteString("\nServer Error:\n")
		builder.WriteString(util.Indent(2, cs.error.String()))
	}

	return builder.String()
}

// networkStats describes network statistics that arise when downloading resources from
// a FHIR server.
type networkStats struct {
	requestDuration, processingDuration float64
	totalBytesIn                        int64
}

// downloadBundle describes the result of downloading a single page of resources from a FHIR server.
type downloadBundle struct {
	associatedRequestURL url.URL
	rawEntries           []byte
	err                  error
	stats                *networkStats
	errResponse          *util.ErrorResponse
}

// downloadBundleError creates a downloadResource instance with an error attached to it.
// The error is formatted using the given format with all potential substitutions.
func downloadBundleError(format string, a ...interface{}) downloadBundle {
	return downloadBundle{
		err: fmt.Errorf(format, a...),
	}
}

var resourceTypes = []string{
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

var downloadCmd = &cobra.Command{
	Use:   "download [resource-type]",
	Short: "Download FHIR resources into an NDJSON file",
	Long: `Downloads FHIR resources and puts them into an NDJSON file.
	
Potential FHIR resources that will be downloaded can be limited by a mandatory -t/--type flag
and an optional -q/--query flag. The query flag has to be a valid FHIR search query.

Downloaded resources will be stored within a file denoted by the -o/--output-file flag.

Example:
	
	blazectl download --server http://localhost:8080/fhir Patient
	blazectl download --server http://localhost:8080/fhir Patient -q "gender=female" -o ~/Downloads/patient.ndjson`,
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return resourceTypes, cobra.ShellCompDirectiveNoFileComp
	},
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) < 1 {
			return errors.New("requires a resource type argument like Patient")
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		err := createClient()
		if err != nil {
			return err
		}
		var stats commandStats
		startTime := time.Now()

		file := createOutputFileOrDie(outputFile)
		sink := bufio.NewWriter(file)
		defer file.Close()
		defer file.Sync()
		defer sink.Flush()

		bundleChannel := make(chan downloadBundle, 2)

		go downloadResources(client, args[0], fhirSearchQuery, usePost, bundleChannel)

		for bundle := range bundleChannel {
			stats.totalPages++

			if bundle.err != nil || bundle.errResponse != nil {
				fmt.Printf("Failed to download resources: %v\n", bundle.err)

				stats.error = bundle.errResponse
				stats.totalDuration = time.Since(startTime)
				fmt.Println(stats.String())
				os.Exit(1)
			} else {
				stats.requestDurations = append(stats.requestDurations, bundle.stats.requestDuration)
				stats.processingDurations = append(stats.processingDurations, bundle.stats.processingDuration)
				stats.totalBytesIn += bundle.stats.totalBytesIn

				resources, inlineOutcomes, err := writeResources(&bundle.rawEntries, sink)
				stats.resourcesPerPage = append(stats.resourcesPerPage, resources)
				stats.inlineOperationOutcomes = append(stats.inlineOperationOutcomes, inlineOutcomes...)

				if err != nil {
					fmt.Printf("Failed to write downloaded resources received from request to URL %s: %v\n", bundle.associatedRequestURL.String(), err)
					os.Exit(2)
				}
			}
		}

		stats.totalDuration = time.Since(startTime)
		fmt.Println(stats.String())
		return nil
	},
}

// downloadResources tries to download all resources of a given resource type from a FHIR server using
// the given client. Resources that are downloaded can optionally be limited by a given FHIR search query.
// The download respects pagination, i.e. it follows pagination links until there is no other next link.
//
// Downloaded resources as well as errors are sent to a given result channel.
// As soon as an error occurs it is written to the channel and the channel is closed thereafter.
func downloadResources(client *fhir.Client, resourceType string, fhirSearchQuery string, usePost bool,
	resChannel chan<- downloadBundle) {
	defer close(resChannel)

	query, err := url.ParseQuery(fhirSearchQuery)
	if err != nil {
		resChannel <- downloadBundleError("could not parse the FHIR search query: %v\n", err)
		return
	}

	var requestStart time.Time
	var processingStart time.Time
	var request *http.Request
	var nextPageURL *url.URL
	for ok := true; ok; ok = nextPageURL != nil {
		var stats networkStats

		if request == nil {
			if usePost {
				request, err = client.NewPostSearchTypeRequest(resourceType, query)
			} else {
				request, err = client.NewSearchTypeRequest(resourceType, query)
			}
		} else {
			request, err = client.NewPaginatedResourceRequest(nextPageURL)
		}
		if err != nil {
			resChannel <- downloadBundleError("could not create FHIR server request: %v\n", err)
			return
		}

		trace := &httptrace.ClientTrace{
			GotConn: func(_ httptrace.GotConnInfo) {
				requestStart = time.Now()
			},
			WroteRequest: func(_ httptrace.WroteRequestInfo) {
				processingStart = time.Now()
			},
			GotFirstResponseByte: func() {
				stats.processingDuration = time.Since(processingStart).Seconds()
			},
		}
		request = request.WithContext(httptrace.WithClientTrace(request.Context(), trace))

		response, err := client.Do(request)
		if err != nil {
			resChannel <- downloadBundleError("could not request the FHIR server with URL %s: %v\n", request.URL, err)
			return
		}

		if response.StatusCode != http.StatusOK {
			responseBody, err := ioutil.ReadAll(response.Body)
			if err != nil {
				resChannel <- downloadBundleError("request to FHIR server with URL %s had a non-ok response status (%d) but its body could not be read: %v",
					request.URL, response.StatusCode, err)
				return
			}
			response.Body.Close()
			stats.requestDuration = time.Since(requestStart).Seconds()
			stats.totalBytesIn += int64(len(responseBody))

			outcome, err := fm.UnmarshalOperationOutcome(responseBody)
			if err != nil {
				bundle := downloadBundleError("request to FHIR server with URL %s had a non-ok response status (%d) but the expected operation outcome could not be parsed: %v", request.URL, response.StatusCode, err)
				bundle.stats = &stats
				resChannel <- bundle
				return
			}

			bundle := downloadBundleError("request to FHIR server with URL %s had a non-ok response status (%d)", request.URL, response.StatusCode)
			bundle.errResponse = &util.ErrorResponse{
				StatusCode:       response.StatusCode,
				OperationOutcome: &outcome,
			}
			bundle.stats = &stats
			resChannel <- bundle
			return
		}

		responseBody, err := ioutil.ReadAll(response.Body)
		if err != nil {
			resChannel <- downloadBundleError("could not read FHIR server response after request to URL %s: %v\n", request.URL, err)
			return
		}
		response.Body.Close()
		stats.requestDuration = time.Since(requestStart).Seconds()
		stats.totalBytesIn += int64(len(responseBody))

		essentialResource := struct {
			Entries json.RawMessage `bson:"entry,omitempty" json:"entry,omitempty"`
			Links   []fm.BundleLink `bson:"link,omitempty" json:"link,omitempty"`
		}{}
		err = json.Unmarshal(responseBody, &essentialResource)
		if err != nil {
			resChannel <- downloadBundleError("could not parse FHIR server response after request to URL %s: %v\n", request.URL, err)
			return
		}
		resChannel <- downloadBundle{
			associatedRequestURL: *request.URL,
			rawEntries:           essentialResource.Entries,
			stats:                &stats,
		}

		nextPageURL, err = getNextPageURL(essentialResource.Links)
		if err != nil {
			resChannel <- downloadBundleError("could not parse the next page link within the FHIR server response after request to URL %s: %v\n", request.URL, err)
			return
		}
	}
}

// createOutputFileOrDie creates the output file at the given filepath if it does not already exist
// and returns the file handle.
// This is a non-destructive operation. Hence, if a file already exists at the given filepath then
// the command exits with a non-success error code. If any other error case the command exits with
// a non-success error code as well.
//
// Note: The callee has to make sure that the file handle is closed properly.
func createOutputFileOrDie(filepath string) *os.File {
	outputFile, err := os.OpenFile(filepath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		if os.IsExist(err) {
			fmt.Printf("The output file %s does already exist.\n", filepath)
			os.Exit(3)
		} else {
			fmt.Printf("could not open/create the output file %s: %v\n", filepath, err)
			os.Exit(4)
		}
	}
	return outputFile
}

// writeOutResources takes a raw set of FHIR bundle entries and writes the resource part of each of them to the given
// sink. The data is written to the sink so that all information resemble a valid NDJSON stream.
//
// Always returns the number of written resources alongside all inline encountered operation outcomes.
// This is also true for when there is an error. An error is returned alongside the other information
// and can only occur if there is an actual issue writing to the file or the given resource bundle is
// invalid in regard to the FHIR specification.
func writeResources(data *[]byte, sink io.Writer) (int, []*fm.OperationOutcome, error) {
	var resources int
	var inlineOutcomes []*fm.OperationOutcome

	if len(*data) == 0 {
		return resources, inlineOutcomes, nil
	}

	var entries []fm.BundleEntry
	if err := json.Unmarshal(*data, &entries); err != nil {
		return resources, inlineOutcomes, fmt.Errorf("could not parse the bundle entries from JSON: %v\n", err)
	}

	var buf bytes.Buffer
	for _, e := range entries {
		if *e.Search.Mode == fm.SearchEntryModeOutcome {
			outcome, err := fm.UnmarshalOperationOutcome(e.Resource)
			if err != nil {
				return resources, inlineOutcomes, fmt.Errorf("could not parse an encountered inline outcome from JSON: %v\n", err)
			}

			inlineOutcomes = append(inlineOutcomes, &outcome)
			continue
		}

		buf.Reset()
		err := json.Compact(&buf, e.Resource)
		if err != nil {
			return resources, inlineOutcomes, fmt.Errorf("could not compact JSON representation for write operation: %v\n", err)
		}

		_, err = sink.Write(buf.Bytes())
		if err != nil {
			return resources, inlineOutcomes, fmt.Errorf("could not write resource to output file: %v\n", err)
		}

		_, err = sink.Write([]byte{'\n'})
		if err != nil {
			return resources, inlineOutcomes, fmt.Errorf("could not write resource separator to output file: %v\n", err)
		}
		resources++
	}

	return resources, inlineOutcomes, nil
}

// getNextPageURL extracts the URL to the next resource bundle page from a given
// set of links.
// The extraction respects the FHIR specification with regard to how links are
// defined: https://www.iana.org/assignments/link-relations/link-relations.xhtml#link-relations-1
//
// Returns the URL to the next resource bundle page if there is any or nil.
// An error is returned if there is a URL, but it can not be parsed.
func getNextPageURL(links []fm.BundleLink) (*url.URL, error) {
	if len(links) == 0 {
		return nil, nil
	}

	for _, link := range links {
		if link.Relation == "next" {
			return url.ParseRequestURI(link.Url)
		}
	}

	return nil, nil
}

func init() {
	rootCmd.AddCommand(downloadCmd)

	downloadCmd.Flags().StringVar(&server, "server", "", "the base URL of the server to use")
	downloadCmd.Flags().StringVarP(&outputFile, "output-file", "o", "", "path to the NDJSON file downloaded resources get written to")
	downloadCmd.Flags().StringVarP(&fhirSearchQuery, "query", "q", "", "FHIR search query")
	downloadCmd.Flags().BoolVarP(&usePost, "use-post", "p", false, "use POST to execute the search")

	_ = downloadCmd.MarkFlagRequired("server")
	_ = downloadCmd.MarkFlagRequired("output-file")
	_ = downloadCmd.MarkFlagFilename("output-file", "ndjson")
}
