[![Build](https://github.com/samply/blazectl/actions/workflows/build.yml/badge.svg)](https://github.com/samply/blazectl/actions/workflows/build.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/samply/blazectl)](https://goreportcard.com/report/github.com/samply/blazectl)
[![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/samply/blazectl/badge)](https://scorecard.dev/viewer/?uri=github.com/samply/blazectl)
[![SLSA 2](https://slsa.dev/images/gh-badge-level2.svg)](https://slsa.dev)

# blazectl

blazectl is a command line tool to control your FHIR® server. blazectl also works with [Blaze][4].

Currently, you can do the following:

* upload transaction bundles from a directory
* download resources in NDJSON format
* count all resources by type
* evaluate a measure

## Installation

blazectl is available as binary for Linux, macOS and Windows.

For Linux and macOS an `install.sh` script is provided. It will download a tar file, extract it, and verify GitHub attestations using the [GitHub CLI][10] tool.  

```sh
curl -sSfL https://raw.githubusercontent.com/samply/blazectl/main/install.sh | sh
```

If you prefer a manual installation or need the Windows variant, please download the latest release [here][3]. The attestation verification is described below.

## Usage

```
blazectl is a command line tool to control your FHIR® server.

Currently you can upload transaction bundles from a directory, download
and count resources and evaluate measures.

Usage:
  blazectl [command]

Available Commands:
  completion       Generate the autocompletion script for the specified shell
  count-resources  Counts all resources by type
  download         Download FHIR resources in NDJSON format
  evaluate-measure Evaluates a Measure
  help             Help about any command
  upload           Upload transaction bundles

Flags:
      --certificate-authority string   path to a cert file for the certificate authority
  -h, --help                           help for blazectl
  -k, --insecure                       allow insecure server connections when using SSL
      --no-progress                    don't show progress bar
      --password string                password information for basic authentication
      --token string                   bearer token for authentication
      --user string                    user information for basic authentication
  -v, --version                        version for blazectl

Use "blazectl [command] --help" for more information about a command.
```

### Upload

You can use the upload command to upload transaction bundles to your server. Currently, JSON (*.json), [gzip compressed][7] JSON (*.json.gz), [bzip2 compressed][8] JSON (*.json.bz2) and NDJSON (*.ndjson) files are supported. If you don't have any transaction bundles, you can generate some with [SyntheaTM][5].

Assuming the URL of your FHIR server is `http://localhost:8080/fhir`, in order to upload run:

```sh
blazectl upload --server http://localhost:8080/fhir my/bundles
```

You will see a progress bar with an estimated ETA during upload. After the upload, a statistic inspired by [vegeta][6] will be printed:

```
Starting Upload to http://localhost:8080/fhir ...
Uploads          [total, concurrency]     362, 4
Success          [ratio]                  100 %
Duration         [total]                  1m42s
Requ. Latencies  [mean, 50, 95, 99, max]  826ms, 534ms, 2.71s, 3.85s 6.467s
Proc. Latencies  [mean, 50, 95, 99, max]  710ms, 526ms, 2.041s, 2.739s 4.133s
Bytes In         [total, mean]            5.10 MiB, 14.59 KiB
Bytes Out        [total, mean]            61.74 MiB, 176.59 KiB
Status Codes     [code:count]             200:362
```

The statistics have the following meaning:

* Uploads — the total number of files uploaded with the given concurrency
* Success — the success rate (possible errors will be printed under the statistics)
* Duration – the total duration of the upload
* Requ. Latencies – mean, max and percentiles of the duration of whole requests including network transfers 
* Proc. Latencies – mean, max and percentiles of the duration of the server processing time excluding networks transfers 
* Bytes In – total and mean number of bytes returned by the server
* Bytes Out – total and mean number of bytes send by blazectl
* Status Codes – a list of status code frequencies. Will show non-200 status codes if they happen.

### Download

You can use the download command to download bundles from the server. Downloaded bundles are stored within an NDJSON file. This operation is non-destructive on your site, i.e., if the specified NDJSON file already exists, then it won't be overwritten.

Use the download command as follows:

```sh
blazectl download --server http://localhost:8080/fhir Patient \
         --query "gender=female" \
         --output-file ~/Downloads/Patients.ndjson
```

If the optional resource-type is given, the corresponding type-level search will be used. Otherwise, the system-level search will be used and all resources of the whole system will be downloaded.

The --query flag will take an optional FHIR search query that will be used to constrain the resources to download. If the query starts with an `@`, the rest is interpreted as filename to read the query from. Using this filename syntax, it's possible to supply very large query strings.

With the flag --use-post, you can ensure that the FHIR search query specified with --query is send as POST request in the body.

Using POST can have two benefits, first if the query string is too large for URL's, it will still fine in the body. Second if the query string contains sensitive information like IDAT's it will be less likely end up in log files, because URL's are often logged but bodies not.

The next links are still traversed with GET. The FHIR server is supposed to not expose any sensitive query params in the URL and also keep the URL short enough.

Resources will be either streamed to STDOUT, delimited by newline, or stored in a file if the --output-file flag is given.

As soon as the download has finished, you will be shown a download statistics overview that looks something like this:

```
Pages           [total]                 184
Resources       [total]                 1835
Resources/Page  [min, mean, max]        5, 9, 10
Duration        [total]                 371ms
Requ. Latencies	[mean, 50, 95, 99, max]	1ms, 1ms, 2ms, 2ms, 3ms
Proc. Latencies	[mean, 50, 95, 99, max]	1ms, 1ms, 1ms, 2ms, 3ms
Bytes In        [total, mean]           1.22 MiB, 6.82 KiB
```

The statistics have the following meaning:

* Pages - total number of pages requested from the server to retrieve resources
* Resources - total number of downloaded resources
* Resources/Page - minimum, mean and maximum number of resources over all pages 
* Duration - total duration of the download
* Requ. Latencies - mean, max and percentiles of the duration of whole requests including networks transfers
* Proc. Latencies - mean, max and percentiles of the duration of the server processing time excluding network transfers
* Bytes In - total and mean number of bytes returned by the server

### Count Resources

The count-resources command is useful to see how many resources a FHIR server stores by resource type. The resource counting is done by first fetching the capability statement of the server. After that blazectl will perform a search-type interaction with query parameter `_summary` set to `count` on every resource type which supports that interaction using one batch request. Bundle.total will be used as resource count.

You can run:
 
```sh
blazectl count-resources --server http://localhost:8080/fhir
```

It will return:

```
Count all resources on http://localhost:8080/fhir ...

AllergyIntolerance       :    7297
CarePlan                 :   49818
Claim                    :  689111
Condition                :  116688
DiagnosticReport         :  193141
Encounter                :  540542
ExplanationOfBenefit     :  540542
Goal                     :   39857
ImagingStudy             :   11212
Immunization             :  187987
MedicationAdministration :    6400
MedicationRequest        :  148569
Observation              : 2689215
Organization             :   52645
Patient                  :   16875
Practitioner             :   52647
Procedure                :  418310
```

### Evaluate Measure

Given a measure in YAML form, creates the required FHIR resources, evaluates that measure and returns the measure report.

You can run:

```sh
blazectl evaluate-measure --server "http://localhost:8080/fhir" stratifier-condition-code.yml
```

More comprehensive documentation can be found in the [Blaze CQL Queries Documentation][9].

## GitHub Attestations

To ensure trust and security in the software supply chain, GitHub [attestations][11] are available for all `blazectl` binaries. To verify the attestations, please install the [GitHub CLI][10] tool and run:

```sh
gh attestation verify --repo samply/blazectl blazectl
```

The `install.sh` script already verifies the attestations.

### SBOM Viewer

The SBOM can be generated by the GitHub CLI:

```sh
gh attestation verify --repo samply/blazectl blazectl --predicate-type "https://spdx.dev/Document/v2.3" --format json  --jq '.[].verificationResult.statement.predicate' > blazectl-sbom.json
```

The resulting SBOM can be viewed at a [Web Viewer provided by SUSE][12].

## Similar Software

* [VonkLoader][1] - can also upload transaction bundles but needs .NET SDK
* [Synthea Uploader][2] - no parallel uploads

## License

Copyright 2019 - 2025 The Samply Community

Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License. You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

[1]: <http://docs.simplifier.net/vonkloader/>
[2]: <https://github.com/synthetichealth/uploader>
[3]: <https://github.com/samply/blazectl/releases/latest>
[4]: <https://github.com/samply/blaze>
[5]: <https://github.com/synthetichealth/synthea>
[6]: <https://github.com/tsenart/vegeta>
[7]: <https://en.wikipedia.org/wiki/Gzip>
[8]: <https://en.wikipedia.org/wiki/Bzip2>
[9]: <https://github.com/samply/blaze/blob/main/docs/cql-queries/blazectl.md>
[10]: <https://github.com/cli/cli>
[11]: <https://docs.github.com/en/actions/concepts/security/artifact-attestations>
[12]: <https://apps.rancher.io/sbom-viewer>
