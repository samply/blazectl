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

package cmd

import (
	"bufio"
	"compress/bzip2"
	"compress/gzip"
	"errors"
	"fmt"
	"github.com/samply/blazectl/fhir"
	"github.com/samply/blazectl/util"
	fm "github.com/samply/golang-fhir-models/fhir-models/fhir"
	"github.com/spf13/cobra"
	"github.com/vbauerster/mpb/v8"
	"github.com/vbauerster/mpb/v8/decor"
	"io"
	"net/http"
	"net/http/httptrace"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var reverse bool

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
	error              []byte
	bytesOut, bytesIn  int64
	requestDuration    time.Duration
	processingDuration time.Duration
}

type CountingReader struct {
	reader    io.Reader
	BytesRead int64
}

func (r *CountingReader) Read(p []byte) (n int, err error) {
	n, err = r.reader.Read(p)
	r.BytesRead += int64(n)
	return n, err
}

// Uploads a single bundle and returns either the status code of the response or
// an error.
func uploadBundle(client *fhir.Client, bundleId *bundleIdentifier) (uploadInfo, error) {
	file, err := os.Open(bundleId.filename)
	if err != nil {
		return uploadInfo{}, err
	}
	defer file.Close()

	var reader io.Reader
	var bundleSize func() int64
	if strings.HasSuffix(bundleId.filename, ".json") {
		reader = bufio.NewReader(file)
		bundleSize = func() int64 {
			return bundleId.endBytes - bundleId.startBytes
		}
	} else if strings.HasSuffix(bundleId.filename, ".json.gz") {
		rdr, err := gzip.NewReader(bufio.NewReader(file))
		if err != nil {
			return uploadInfo{}, err
		}
		reader = &CountingReader{reader: rdr}
		bundleSize = func() int64 {
			return reader.(*CountingReader).BytesRead
		}
	} else if strings.HasSuffix(bundleId.filename, ".json.bz2") {
		reader = &CountingReader{reader: bzip2.NewReader(bufio.NewReader(file))}
		bundleSize = func() int64 {
			return reader.(*CountingReader).BytesRead
		}
	} else {
		reader, err = NewFileChunkReader(file, bundleId.startBytes, bundleId.endBytes-bundleId.startBytes)
		if err != nil {
			return uploadInfo{}, err
		}
		bundleSize = func() int64 {
			return bundleId.endBytes - bundleId.startBytes
		}
	}

	req, err := client.NewTransactionRequest(reader)
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
		return uploadInfo{}, fmt.Errorf("error while uploading: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		bodySize, err := io.Copy(io.Discard, resp.Body)
		if err != nil {
			return uploadInfo{}, err
		}

		return uploadInfo{
			statusCode:         resp.StatusCode,
			bytesOut:           bundleSize(),
			bytesIn:            bodySize,
			requestDuration:    time.Since(requestStart),
			processingDuration: processingDuration,
		}, nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return uploadInfo{}, fmt.Errorf("error while reading the FHIR error response: %v", err)
	}

	return uploadInfo{
		statusCode:         resp.StatusCode,
		error:              body,
		bytesOut:           bundleSize(),
		bytesIn:            int64(len(body)),
		requestDuration:    time.Since(requestStart),
		processingDuration: processingDuration,
	}, nil
}

type bundleUploadResult struct {
	id         bundleIdentifier
	uploadInfo uploadInfo
	err        error
	duration   time.Duration
}

type aggregatedUploadResults struct {
	totalProcessedBundles                 int
	requestDurations, processingDurations []float64
	totalBytesIn, totalBytesOut           int64
	errorResponses                        map[bundleIdentifier]util.ErrorResponse
	errors                                map[bundleIdentifier]error
}

func aggregateUploadResults(
	uploadResultCh chan bundleUploadResult,
	aggregatedUploadResultsCh chan aggregatedUploadResults,
	progress progress) {

	var totalProcessedBundles int
	var requestDurations []float64
	var processingDurations []float64
	var totalBytesIn int64
	var totalBytesOut int64
	errorResponses := make(map[bundleIdentifier]util.ErrorResponse)
	errs := make(map[bundleIdentifier]error)

	for uploadResult := range uploadResultCh {
		progress.increment(uploadResult.duration)
		totalProcessedBundles += 1

		if uploadResult.err != nil {
			errs[uploadResult.id] = uploadResult.err
		} else {
			if uploadResult.uploadInfo.statusCode == http.StatusOK {
				processingDurations = append(processingDurations, uploadResult.uploadInfo.processingDuration.Seconds())
			} else {
				operationOutcome, err := fm.UnmarshalOperationOutcome(uploadResult.uploadInfo.error)
				if err != nil {
					errorResponses[uploadResult.id] = util.ErrorResponse{
						StatusCode: uploadResult.uploadInfo.statusCode,
						OtherError: string(uploadResult.uploadInfo.error),
					}
				} else {
					errorResponses[uploadResult.id] = util.ErrorResponse{
						StatusCode:       uploadResult.uploadInfo.statusCode,
						OperationOutcome: &operationOutcome,
					}
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

type processableFiles struct {
	singleBundleFiles []string
	multiBundleFiles  []string
}

func findProcessableFiles(dir string) (processableFiles, error) {
	dirEntries, err := os.ReadDir(dir)
	if err != nil {
		return processableFiles{}, err
	}

	var procFiles processableFiles

	for _, dirEntry := range dirEntries {
		name := dirEntry.Name()
		if dirEntry.IsDir() {
			subProcFiles, err := findProcessableFiles(filepath.Join(dir, name))
			if err != nil {
				return procFiles, err
			}
			procFiles.singleBundleFiles = append(procFiles.singleBundleFiles, subProcFiles.singleBundleFiles...)
			procFiles.multiBundleFiles = append(procFiles.multiBundleFiles, subProcFiles.multiBundleFiles...)
		} else {
			if isSingleBundleFile(name) {
				procFiles.singleBundleFiles = append(procFiles.singleBundleFiles, filepath.Join(dir, name))
			} else if isMultiBundleFile(name) {
				procFiles.multiBundleFiles = append(procFiles.multiBundleFiles, filepath.Join(dir, name))
			}
		}
	}

	return procFiles, nil
}

func isSingleBundleFile(name string) bool {
	return strings.HasSuffix(name, ".json") ||
		strings.HasSuffix(name, ".json.gz") ||
		strings.HasSuffix(name, ".json.bz2")
}

func isMultiBundleFile(name string) bool {
	return strings.HasSuffix(name, ".ndjson")
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
}

func newUploadBundleConsumer(client *fhir.Client, uploadResults chan<- bundleUploadResult) *uploadBundleConsumer {
	return &uploadBundleConsumer{
		client:        client,
		uploadResults: uploadResults,
	}
}

func (consumer *uploadBundleConsumer) uploadBundles(uploadBundles []bundle, concurrency int, wg *sync.WaitGroup) {
	limiter := make(chan bool, concurrency)

	for i := 0; i < len(uploadBundles); i++ {
		var queueItem bundle
		if reverse {
			queueItem = uploadBundles[len(uploadBundles)-(i+1)]
		} else {
			queueItem = uploadBundles[i]
		}

		limiter <- true
		wg.Add(1)
		go func(b bundle, limiter <-chan bool, wg *sync.WaitGroup) {
			defer func() { <-limiter }()
			if b.err != nil {
				consumer.uploadResults <- bundleUploadResult{id: b.id, err: b.err}
			} else {
				start := time.Now()
				if uploadInfo, err := uploadBundle(consumer.client, &b.id); err != nil {
					consumer.uploadResults <- bundleUploadResult{id: b.id, err: err, duration: time.Duration(time.Since(start).Nanoseconds() / int64(concurrency))}
				} else {
					consumer.uploadResults <- bundleUploadResult{id: b.id, uploadInfo: uploadInfo, duration: time.Duration(time.Since(start).Nanoseconds() / int64(concurrency))}
				}
			}
			wg.Done()
		}(queueItem, limiter, wg)
	}
}

type progress interface {
	increment(duration time.Duration)
	wait()
}

type realProgress struct {
	progress *mpb.Progress
	bar      *mpb.Bar
}

func (rP realProgress) increment(duration time.Duration) {
	rP.bar.EwmaIncrement(duration)
}

func (rP realProgress) wait() {
	rP.progress.Wait()
}

type noopProgress struct {
}

func (nP noopProgress) increment(_ time.Duration) {
	// nothing to do here
}

func (nP noopProgress) wait() {
	// nothing to do here
}

func createRealProgress(numBundles int) progress {
	p := mpb.New()
	return realProgress{progress: p,
		bar: p.AddBar(int64(numBundles),
			mpb.BarRemoveOnComplete(),
			mpb.PrependDecorators(
				decor.Name("upload", decor.WC{W: 7, C: decor.DindentRight}),
				decor.OnComplete(decor.EwmaETA(decor.ET_STYLE_GO, 60, decor.WC{W: 4}), "done"),
			),
			mpb.AppendDecorators(decor.Percentage()),
		),
	}
}

func createProgress(numBundles int) progress {
	if noProgress {
		return noopProgress{}
	} else {
		return createRealProgress(numBundles)
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
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return nil, cobra.ShellCompDirectiveFilterDirs
	},
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
	RunE: func(cmd *cobra.Command, args []string) error {
		err := createClient()
		if err != nil {
			return err
		}

		dir := args[0]

		files, err := findProcessableFiles(dir)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		fmt.Printf("Starting Upload to %s ...\n", server)

		// Aggregate results in one single goroutine
		uploadResultCh := make(chan bundleUploadResult)
		aggregatedUploadResultsCh := make(chan aggregatedUploadResults)

		fmt.Printf("Inspecting files eligible for upload from %s... ", dir)
		bundleProducer := newUploadBundleProducer()
		uploadBundlesSummary := bundleProducer.createUploadBundles(files)
		fmt.Println("DONE")

		if len(uploadBundlesSummary.bundles) == 0 {
			fmt.Println("Found no bundles to upload.")
			os.Exit(0)
		}

		fmt.Printf("Found %d bundles in total (from %d JSON files and from %d NDJSON files)\n",
			len(uploadBundlesSummary.bundles), uploadBundlesSummary.singleBundlesFiles, uploadBundlesSummary.multiBundlesFiles)

		progress := createProgress(len(uploadBundlesSummary.bundles))

		// Loop through bundles
		var consumerWg sync.WaitGroup
		start := time.Now()
		bundleConsumer := newUploadBundleConsumer(client, uploadResultCh)
		go aggregateUploadResults(uploadResultCh, aggregatedUploadResultsCh, progress)

		bundleConsumer.uploadBundles(uploadBundlesSummary.bundles, concurrency, &consumerWg)

		consumerWg.Wait()
		close(uploadResultCh)
		progress.wait()
		client.CloseIdleConnections()

		aggResults := <-aggregatedUploadResultsCh

		fmt.Printf("Uploads          [total, concurrency]     %d, %d\n",
			aggResults.totalProcessedBundles, concurrency)
		fmt.Printf("Success          [ratio]                  %.2f %%\n",
			float32(aggResults.totalProcessedBundles-len(aggResults.errors)-len(aggResults.errorResponses))/float32(aggResults.totalProcessedBundles)*100)
		fmt.Printf("Duration         [total]                  %s\n",
			util.FmtDurationHumanReadable(time.Since(start)))

		if len(aggResults.requestDurations) > 0 {
			requestStats := util.CalculateDurationStatistics(aggResults.requestDurations)
			fmt.Printf("Requ. Latencies  [mean, 50, 95, 99, max]  %s, %s, %s, %s %s\n",
				requestStats.Mean, requestStats.Q50, requestStats.Q95, requestStats.Q99, requestStats.Max)
		}

		if len(aggResults.processingDurations) > 0 {
			processingStats := util.CalculateDurationStatistics(aggResults.requestDurations)
			fmt.Printf("Proc. Latencies  [mean, 50, 95, 99, max]  %s, %s, %s, %s %s\n",
				processingStats.Mean, processingStats.Q50, processingStats.Q95, processingStats.Q99, processingStats.Max)
		}

		totalTransfers := len(aggResults.requestDurations)
		fmt.Printf("Bytes In         [total, mean]            %s, %s\n", util.FmtBytesHumanReadable(float32(aggResults.totalBytesIn)), util.FmtBytesHumanReadable(float32(aggResults.totalBytesIn)/float32(totalTransfers)))
		fmt.Printf("Bytes Out        [total, mean]            %s, %s\n", util.FmtBytesHumanReadable(float32(aggResults.totalBytesOut)), util.FmtBytesHumanReadable(float32(aggResults.totalBytesOut)/float32(totalTransfers)))

		errorFrequencies := make(map[int]int)
		for _, errorResponse := range aggResults.errorResponses {
			errorFrequencies[errorResponse.StatusCode]++
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
				fmt.Printf("%s", util.Indent(4, errorResponse.String()))
			}
		}
		if len(aggResults.errors) > 0 {
			fmt.Println("\nErrors:")
			for bundleId, err := range aggResults.errors {
				fmt.Printf("File: %s [Bundle: %d] : %v\n", bundleId.filename, bundleId.bundleNumber, err.Error())
			}
		}
		if len(aggResults.errorResponses) > 0 || len(aggResults.errors) > 0 {
			os.Exit(1)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(uploadCmd)

	uploadCmd.Flags().StringVar(&server, "server", "", "the base URL of the server to use")
	uploadCmd.Flags().IntVarP(&concurrency, "concurrency", "c", 2, "number of parallel uploads")
	uploadCmd.Flags().BoolVarP(&reverse, "reverse", "r", false, "upload data in reverse order")

	_ = uploadCmd.MarkFlagRequired("server")
}
