# Changelog

## v1.5.0

### Enhancements

* Implement Evaluating an Existing Measure ([#104](https://github.com/samply/blazectl/issues/104))
* Support the parameters input of $evaluate-measure ([#175](https://github.com/samply/blazectl/issues/175))
* Add Disk Performance Measurement Command ([#194](https://github.com/samply/blazectl/issues/194))

### Bugfixes

* Fix Error Message Output Sink ([#161](https://github.com/samply/blazectl/issues/161))

### Maintenance

* Add CVE checking ([#148](https://github.com/samply/blazectl/issues/148))

The full changelog can be found [here](https://github.com/samply/blazectl/milestone/11?closed=1).

## v1.4.0

### Enhancement

* Backoff on 503 and 504 While Uploading ([#134](https://github.com/samply/blazectl/issues/134))

### Performance

* Enable Go 1.25 JSONv2 Experiment ([#75](https://github.com/samply/blazectl/pull/75))

The full changelog can be found [here](https://github.com/samply/blazectl/milestone/9?closed=1).

## v1.3.1

### Bugfixes

* Fix Rendering of Stratum without Text ([#119](https://github.com/samply/blazectl/issues/119))

## v1.3.0

### Enhancements

* Add Render-Report Command ([#105](https://github.com/samply/blazectl/issues/105))

## v1.1.0

### Notes

This is the first version of blazectl with [GitHub attestations](https://docs.github.com/en/actions/concepts/security/artifact-attestations). Users of blazectl are now able to verify that the downloaded binary was build on GitHub by using the following [GitHub CLI](https://github.com/cli/cli) command:

```sh
gh attestation verify --repo samply/blazectl blazectl
```

Additionally SBOMs are available either as release asset or by calling:

```sh
gh attestation verify --repo samply/blazectl blazectl --predicate-type "https://spdx.dev/Document/v2.3" --format json  --jq '.[].verificationResult.statement.predicate' > blazectl-sbom.json
```

The resulting SBOM can be viewed at a [Web Viewer provided by SUSE](https://apps.rancher.io/sbom-viewer).

### Enhancements

* Use Link Headers to Improve Download Speed ([#37](https://github.com/samply/blazectl/pull/37))

### Bugfixes

* Fix Limit of 100 Polls ([#35](https://github.com/samply/blazectl/pull/35))

The full changelog can be found [here](https://github.com/samply/blazectl/milestone/6?closed=1).

## v0.15.1

* Remove Dependency on glibc ([#31](https://github.com/samply/blazectl/issues/31))

## v0.15.0

* Make Measure Evaluation Async per Default ([#29](https://github.com/samply/blazectl/issues/29))

## v0.14.0

* Add OAuth 2 bearer token authentication.

## v0.9.0

* Accept Self Signed Certificates ([#22](https://github.com/samply/blazectl/issues/22))
* Find Processable Files in Sub Directories

## v0.8.6

* Fix Freeze on no Bundles to Upload ([#21](https://github.com/samply/blazectl/issues/21))
* Output Non-FHIR (OperationOutcome) Error Responses

## v0.8.5

* Allow to Disable the Progress Bar

## v0.8.4

* Allow up to 100 Concurrent Connections per Host

## v0.8.3

* Fix Concurrent Progress Bar Increments
* Add Context to Error Messages

## v0.8.2

* Update Dependencies

## v0.8.1

* Improve FHIR Error Response Parsing Error Message
* Fix Panic in MPB

## v0.8.0

* update dependencies
* implement POST searches in download

## v0.7.0

* Exit with One on Upload Errors ([#19](https://github.com/samply/blazectl/issues/19))

## v0.6.0

* Allow Uploading of gzip and bzip2 Files ([#15](https://github.com/samply/blazectl/issues/15))
* Buffer File Writes ([#14](https://github.com/samply/blazectl/issues/14))

## v0.5.0

* Add Download Command ([#12](https://github.com/samply/blazectl/issues/12))

## v0.4.0

* Allow Multibundle Uploads Using ndjson Files ([#9](https://github.com/samply/blazectl/issues/9))
* Allow basic auth ([#10](https://github.com/samply/blazectl/issues/10))

## v0.3.0

### Improved Upload Error Reporting

This release outputs detailed error reports in case something goes wrong at upload. It assumes that a FHIR server responds with a conforming [OperationOutcome](https://www.hl7.org/fhir/operationoutcome.html) resource and prints it's contents in a human readable manner.

## v0.2.2

* Only read JSON files from the Directory at Upload ([#4](https://github.com/samply/blazectl/issues/4))

## v0.2.1

* Fix Issues on Empty Processing Durations

## v0.2.0

### Improvements

* **Batch Count Resources** — Uses FHIR batch instead of individual requests to perform the count-resources sub command.
* **Add Processing Stats to Upload** — The processing stats show the duration of processing on the server, which can be much less than the whole request duration if the network connection to the server isn't very fast.

## v0.1.0

This is the first version of blazectl. The upload command was tested with [Blaze](https://github.com/life-research/blaze) and [HAPI FHIR](https://github.com/jamesagnew/hapi-fhir).
