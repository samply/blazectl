// Copyright 2019 The Samply Development Community
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
	"errors"
	"fmt"
	"github.com/samply/blazectl/fhir"
	"github.com/samply/blazectl/util"
	fm "github.com/samply/golang-fhir-models/fhir-models/fhir"
	"github.com/spf13/cobra"
	"github.com/vbauerster/mpb/v4"
	"github.com/vbauerster/mpb/v4/decor"
	"gonum.org/v1/gonum/floats"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptrace"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const MultiBundleFileBundleDelimiter = byte('\n')

func NewFileChunkReader(file *os.File, offsetBytes int64, limitBytes int64) (*io.LimitedReader, error) {
	if _, err := file.Seek(offsetBytes, io.SeekStart); err != nil {
		return nil, err
	}

	return &io.LimitedReader{R: file, N: limitBytes}, nil
}

type bundleIdentifier struct {
	filename     string
	bundleNumber int
	startBytes   int64
	endBytes     int64
}

type bundle struct {
	id  bundleIdentifier
	err error
}

type uploadInfo struct {
	statusCode         int
	error              *fm.OperationOutcome
	bytesOut, bytesIn  int64
	requestDuration    time.Duration
	processingDuration time.Duration
}

// Uploads a single bundle and returns either the status code of the response or
// an error.
func uploadBundle(client *fhir.Client, bundle *bundle) (uploadInfo, error) {
	bundleSize := bundle.id.endBytes - bundle.id.startBytes

	file, err := os.Open(bundle.id.filename)
	if err != nil {
		return uploadInfo{}, err
	}
	defer file.Close()

	fileChunkReader, err := NewFileChunkReader(file, bundle.id.startBytes, bundle.id.endBytes-bundle.id.startBytes)
	if err != nil {
		return uploadInfo{}, err
	}

	req, err := client.NewTransactionRequest(fileChunkReader)
	if err != nil {
		return uploadInfo{}, err
	}

	var requestStart time.Time
	var processingStart time.Time
	var processingDuration time.Duration
	trace := &httptrace.ClientTrace{
		GotConn: func(_ httptrace.GotConnInfo) {
			requestStart = time.Now()
		},
		WroteRequest: func(_ httptrace.WroteRequestInfo) {
			processingStart = time.Now()
		},
		GotFirstResponseByte: func() {
			processingDuration = time.Since(processingStart)
		},
	}
	req = req.WithContext(httptrace.WithClientTrace(req.Context(), trace))

	resp, err := client.Do(req)
	if err != nil {
		return uploadInfo{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		bodySize, err := io.Copy(ioutil.Discard, resp.Body)
		if err != nil {
			return uploadInfo{}, err
		}

		return uploadInfo{
			statusCode:         resp.StatusCode,
			bytesOut:           bundleSize,
			bytesIn:            bodySize,
			requestDuration:    time.Since(requestStart),
			processingDuration: processingDuration,
		}, nil
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return uploadInfo{}, err
	}
	operationOutcome, err := fm.UnmarshalOperationOutcome(body)
	if err != nil {
		return uploadInfo{}, err
	}
	return uploadInfo{
		statusCode:         resp.StatusCode,
		error:              &operationOutcome,
		bytesOut:           bundleSize,
		bytesIn:            int64(len(body)),
		requestDuration:    time.Since(requestStart),
		processingDuration: processingDuration,
	}, nil
}

type bundleUploadResult struct {
	id         bundleIdentifier
	uploadInfo uploadInfo
	err        error
}

type errorResponse struct {
	statusCode int
	error      *fm.OperationOutcome
}

type aggregatedUploadResults struct {
	totalProcessedBundles                 int
	requestDurations, processingDurations []float64
	totalBytesIn, totalBytesOut           int64
	errorResponses                        map[bundleIdentifier]errorResponse
	errors                                map[bundleIdentifier]error
}

func aggregateUploadResults(
	uploadResultCh chan bundleUploadResult,
	aggregatedUploadResultsCh chan aggregatedUploadResults) {

	var totalProcessedBundles int
	var requestDurations []float64
	var processingDurations []float64
	var totalBytesIn int64
	var totalBytesOut int64
	errorResponses := make(map[bundleIdentifier]errorResponse)
	errs := make(map[bundleIdentifier]error)

	for uploadResult := range uploadResultCh {
		totalProcessedBundles += 1
		if uploadResult.err != nil {
			errs[uploadResult.id] = uploadResult.err
		} else {
			if uploadResult.uploadInfo.statusCode == http.StatusOK {
				processingDurations = append(processingDurations, uploadResult.uploadInfo.processingDuration.Seconds())
			} else {
				errorResponses[uploadResult.id] = errorResponse{
					statusCode: uploadResult.uploadInfo.statusCode,
					error:      uploadResult.uploadInfo.error,
				}
			}
			totalBytesIn += uploadResult.uploadInfo.bytesIn
			totalBytesOut += uploadResult.uploadInfo.bytesOut
			requestDurations = append(requestDurations, uploadResult.uploadInfo.requestDuration.Seconds())
		}
	}

	aggregatedUploadResultsCh <- aggregatedUploadResults{
		totalProcessedBundles: totalProcessedBundles,
		requestDurations:      requestDurations,
		processingDurations:   processingDurations,
		totalBytesIn:          totalBytesIn,
		totalBytesOut:         totalBytesOut,
		errorResponses:        errorResponses,
		errors:                errs,
	}
}

func fmtBytes(count float32, level int) string {
	if count > 1024 {
		return fmtBytes(count/1024, level+1)
	}
	unit := "B"
	switch level {
	case 1:
		unit = "KiB"
	case 2:
		unit = "MiB"
	case 3:
		unit = "GiB"
	case 4:
		unit = "TiB"
	case 5:
		unit = "PiB"
	}
	return fmt.Sprintf("%.2f %s", count, unit)
}

type stats struct {
	mean, q50, q95, q99, max time.Duration
}

func genStats(durations []float64) stats {
	sort.Float64s(durations)
	return stats{
		mean: time.Duration(floats.Sum(durations)/float64(len(durations))*1000) * time.Millisecond,
		q50:  time.Duration(durations[len(durations)/2]*1000) * time.Millisecond,
		q95:  time.Duration(durations[int(float32(len(durations))*0.95)]*1000) * time.Millisecond,
		q99:  time.Duration(durations[int(float32(len(durations))*0.99)]*1000) * time.Millisecond,
		max:  time.Duration(durations[len(durations)-1]*1000) * time.Millisecond,
	}
}

type processableFiles struct {
	singleBundleFiles []string
	multiBundleFiles  []string
}

func filterProcessableFiles(dir string) (processableFiles, error) {
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return processableFiles{}, err
	}

	var procFiles processableFiles

	for _, file := range files {
		if !file.IsDir() {
			if strings.HasSuffix(file.Name(), ".json") {
				procFiles.singleBundleFiles = append(procFiles.singleBundleFiles, filepath.Join(dir, file.Name()))
			} else if strings.HasSuffix(file.Name(), ".ndjson") {
				procFiles.multiBundleFiles = append(procFiles.multiBundleFiles, filepath.Join(dir, file.Name()))
			}
		}
	}

	return procFiles, nil
}

type uploadBundleProductionSummary struct {
	singleBundlesFiles int
	multiBundlesFiles  int
	bundles            []bundle
}

type uploadBundleProducer struct {
	res chan bundle
}

func newUploadBundleProducer() *uploadBundleProducer {
	return &uploadBundleProducer{
		res: make(chan bundle),
	}
}

func (ubp *uploadBundleProducer) createUploadBundles(f processableFiles) *uploadBundleProductionSummary {
	var producerWg sync.WaitGroup
	producerWg.Add(2)
	go ubp.createUploadBundlesFromSingleBundleFiles(f.singleBundleFiles, &producerWg)
	go ubp.createUploadBundlesFromMultiBundleFiles(f.multiBundleFiles, &producerWg)

	go func(wg *sync.WaitGroup, bundleCh chan<- bundle) {
		wg.Wait()
		close(bundleCh)
	}(&producerWg, ubp.res)

	var bundles []bundle
	for bundle := range ubp.res {
		bundles = append(bundles, bundle)
	}

	return &uploadBundleProductionSummary{
		singleBundlesFiles: len(f.singleBundleFiles),
		multiBundlesFiles:  len(f.multiBundleFiles),
		bundles:            bundles,
	}
}

func (ubp *uploadBundleProducer) createUploadBundlesFromSingleBundleFiles(files []string, wg *sync.WaitGroup) {
	for _, file := range files {
		func() {
			f, err := os.Open(file)
			if err != nil {
				ubp.res <- bundle{id: bundleIdentifier{filename: file}, err: err}
				return
			}
			defer f.Close()

			fInfo, err := f.Stat()
			if err != nil {
				ubp.res <- bundle{
					id: bundleIdentifier{
						filename:     file,
						bundleNumber: 1,
					},
					err: err,
				}
				return
			}

			ubp.res <- bundle{
				id: bundleIdentifier{
					filename:     file,
					bundleNumber: 1,
					startBytes:   0,
					endBytes:     fInfo.Size(),
				}}
		}()
	}
	wg.Done()
}

func (ubp *uploadBundleProducer) createUploadBundlesFromMultiBundleFiles(files []string, wg *sync.WaitGroup) {
	for _, file := range files {
		func() {
			f, err := os.Open(file)
			if err != nil {
				ubp.res <- bundle{id: bundleIdentifier{filename: file}, err: err}
				return
			}
			defer f.Close()

			reader := bufio.NewReader(f)
			calcRes := make(chan util.FileChunkCalculationResult)

			go util.CalculateFileChunks(reader, MultiBundleFileBundleDelimiter, calcRes)

			for res := range calcRes {
				if res.Err != nil {
					ubp.res <- bundle{
						id: bundleIdentifier{
							filename:     file,
							bundleNumber: res.FileChunk.ChunkNumber,
						},
						err: res.Err,
					}
				} else {
					if res.FileChunk.StartBytes == res.FileChunk.EndBytes {
						continue
					}
					ubp.res <- bundle{
						id: bundleIdentifier{
							filename:     file,
							bundleNumber: res.FileChunk.ChunkNumber,
							startBytes:   res.FileChunk.StartBytes,
							endBytes:     res.FileChunk.EndBytes,
						},
					}
				}
			}
		}()
	}
	wg.Done()
}

type uploadBundleConsumer struct {
	client        *fhir.Client
	uploadResults chan<- bundleUploadResult
	progressBar   *mpb.Bar
}

func newUploadBundleConsumer(client *fhir.Client, uploadResults chan<- bundleUploadResult, progressBar *mpb.Bar) *uploadBundleConsumer {
	return &uploadBundleConsumer{
		client:        client,
		uploadResults: uploadResults,
		progressBar:   progressBar,
	}
}

func (consumer *uploadBundleConsumer) uploadBundles(uploadBundles []bundle, concurrency int, wg *sync.WaitGroup) {
	limiter := make(chan bool, concurrency)

	for _, queueItem := range uploadBundles {
		limiter <- true
		wg.Add(1)
		go func(b bundle, limiter <-chan bool, wg *sync.WaitGroup) {
			defer func() { <-limiter }()
			start := time.Now()
			if b.err != nil {
				consumer.uploadResults <- bundleUploadResult{id: b.id, err: b.err}
				consumer.progressBar.Increment()
			} else {
				if uploadInfo, err := uploadBundle(consumer.client, &b); err != nil {
					consumer.uploadResults <- bundleUploadResult{id: b.id, err: err}
					consumer.progressBar.Increment()
				} else {
					consumer.uploadResults <- bundleUploadResult{id: b.id, uploadInfo: uploadInfo}
					consumer.progressBar.Increment(time.Duration(time.Since(start).Nanoseconds() / int64(concurrency)))
				}
			}
			wg.Done()
		}(queueItem, limiter, wg)
	}
}

var concurrency int

// uploadCmd represents the upload command
var uploadCmd = &cobra.Command{
	Use:   "upload [directory]",
	Short: "Upload transaction bundles",
	Long: `You can upload transaction bundles from JSON files inside a directory.

The upload will be parallel according to the --concurrency flag. A upload 
statistic will be printed after the upload.

Example:

  blazectl upload my/bundles`,
	ValidArgs: []string{"directory"},
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) < 1 {
			return errors.New("requires a directory argument")
		}
		if info, err := os.Stat(args[0]); os.IsNotExist(err) {
			return fmt.Errorf("directory `%s` doesn't exist", args[0])
		} else if !info.IsDir() {
			return fmt.Errorf("`%s` isn't a directory", args[0])
		} else {
			return nil
		}
	},
	Run: func(cmd *cobra.Command, args []string) {
		dir := args[0]

		files, err := filterProcessableFiles(dir)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		fmt.Printf("Starting Upload to %s ...\n", server)

		// Aggregate results in one single goroutine
		uploadResultCh := make(chan bundleUploadResult)
		aggregatedUploadResultsCh := make(chan aggregatedUploadResults)
		go aggregateUploadResults(uploadResultCh, aggregatedUploadResultsCh)

		fmt.Printf("Inspecting files eligible for upload from %s... ", dir)
		bundleProducer := newUploadBundleProducer()
		uploadBundlesSummary := bundleProducer.createUploadBundles(files)
		fmt.Println("DONE")

		fmt.Printf("Found %d bundles in total (from %d JSON files and from %d NDJSON files)\n",
			len(uploadBundlesSummary.bundles), uploadBundlesSummary.singleBundlesFiles, uploadBundlesSummary.multiBundlesFiles)

		progress := mpb.New()
		bar := progress.AddBar(int64(len(uploadBundlesSummary.bundles)),
			mpb.BarRemoveOnComplete(),
			mpb.PrependDecorators(
				decor.Name("upload", decor.WC{W: 7, C: decor.DidentRight}),
				decor.OnComplete(decor.EwmaETA(decor.ET_STYLE_GO, 60, decor.WC{W: 4}), "done"),
			),
			mpb.AppendDecorators(decor.Percentage()),
		)

		// Loop through bundles
		var consumerWg sync.WaitGroup
		start := time.Now()
		bundleConsumer := newUploadBundleConsumer(client, uploadResultCh, bar)
		bundleConsumer.uploadBundles(uploadBundlesSummary.bundles, concurrency, &consumerWg)

		consumerWg.Wait()
		close(uploadResultCh)
		progress.Wait()
		client.CloseIdleConnections()

		aggResults := <-aggregatedUploadResultsCh

		fmt.Printf("Uploads          [total, concurrency]     %d, %d\n",
			aggResults.totalProcessedBundles, concurrency)
		fmt.Printf("Success          [ratio]                  %.2f %%\n",
			float32(aggResults.totalProcessedBundles-len(aggResults.errors)-len(aggResults.errorResponses))/float32(aggResults.totalProcessedBundles)*100)
		fmt.Printf("Duration         [total]                  %s\n",
			time.Since(start).Round(time.Second))

		if len(aggResults.requestDurations) > 0 {
			requestStats := genStats(aggResults.requestDurations)
			fmt.Printf("Requ. Latencies  [mean, 50, 95, 99, max]  %s, %s, %s, %s %s\n",
				requestStats.mean, requestStats.q50, requestStats.q95, requestStats.q99, requestStats.max)
		}

		if len(aggResults.processingDurations) > 0 {
			processingStats := genStats(aggResults.processingDurations)
			fmt.Printf("Proc. Latencies  [mean, 50, 95, 99, max]  %s, %s, %s, %s %s\n",
				processingStats.mean, processingStats.q50, processingStats.q95, processingStats.q99, processingStats.max)
		}

		totalTransfers := len(aggResults.requestDurations)
		fmt.Printf("Bytes In         [total, mean]            %s, %s\n", fmtBytes(float32(aggResults.totalBytesIn), 0), fmtBytes(float32(aggResults.totalBytesIn)/float32(totalTransfers), 0))
		fmt.Printf("Bytes Out        [total, mean]            %s, %s\n", fmtBytes(float32(aggResults.totalBytesOut), 0), fmtBytes(float32(aggResults.totalBytesOut)/float32(totalTransfers), 0))

		errorFrequencies := make(map[int]int)
		for _, errorResponse := range aggResults.errorResponses {
			errorFrequencies[errorResponse.statusCode]++
		}
		statusCodes := make([]string, 1, len(errorFrequencies)+1)
		statusCodes[0] = fmt.Sprintf("200:%d", len(aggResults.processingDurations))
		for statusCode, freq := range errorFrequencies {
			statusCodes = append(statusCodes, fmt.Sprintf("%d:%d", statusCode, freq))
		}
		fmt.Printf("Status Codes     [code:count]             %s\n", strings.Join(statusCodes, ", "))

		if len(aggResults.errorResponses) > 0 {
			fmt.Println()
			fmt.Println("Non-OK Responses:")
			fmt.Println()
			for bundleId, errorResponse := range aggResults.errorResponses {
				fmt.Printf("File: %s [Bundle: %d]\n", bundleId.filename, bundleId.bundleNumber)
				fmt.Printf("    Status Code : %d\n", errorResponse.statusCode)
				if issues := errorResponse.error.Issue; len(issues) > 0 {
					fmt.Printf("    Severity    : %s\n", issues[0].Severity.Display())
					fmt.Printf("    Code        : %s\n", issues[0].Code.Definition())
					if details := issues[0].Details; details != nil {
						if text := details.Text; text != nil {
							fmt.Printf("    Details     : %s\n", *text)
						} else if codings := details.Coding; len(codings) > 0 {
							if code := codings[0].Code; code != nil {
								fmt.Printf("    Details     : %s\n", *code)
							}
						}
					}
					if diagnostics := issues[0].Diagnostics; diagnostics != nil {
						fmt.Printf("    Diagnostics : %s\n", *diagnostics)
					}
					if expressions := issues[0].Expression; len(expressions) > 0 {
						fmt.Printf("    Expression  : %s\n", strings.Join(expressions, ", "))
					}
				}
			}
		}
		if len(aggResults.errors) > 0 {
			fmt.Println("\nErrors:")
			for bundleId, err := range aggResults.errors {
				fmt.Printf("File: %s [Bundle: %d] : %v\n", bundleId.filename, bundleId.bundleNumber, err.Error())
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(uploadCmd)

	uploadCmd.Flags().IntVarP(&concurrency, "concurrency", "c", 2, "number of parallel uploads")
}
