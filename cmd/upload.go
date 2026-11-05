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

// bundle is either a bundle to upload or, if endOfFile is set, the marker that
// the producer emitted bundleCount bundles for id.filename.
type bundle struct {
	id          bundleIdentifier
	err         error
	endOfFile   bool
	bundleCount int
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

// bundleUploadResult is the result of a bundle upload or, if endOfFile is set,
// the forwarded end-of-file marker of id.filename with its bundleCount.
type bundleUploadResult struct {
	id          bundleIdentifier
	uploadInfo  uploadInfo
	err         error
	duration    time.Duration
	endOfFile   bool
	bundleCount int
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
	fileProgress *fileProgress) {

	var totalProcessedBundles int
	var requestDurations []float64
	var processingDurations []float64
	var totalBytesIn int64
	var totalBytesOut int64
	errorResponses := make(map[bundleIdentifier]util.ErrorResponse)
	errs := make(map[bundleIdentifier]error)

	for uploadResult := range uploadResultCh {
		if uploadResult.endOfFile {
			fileProgress.fileEnd(uploadResult.id.filename, uploadResult.bundleCount)
			continue
		}
		fileProgress.bundleDone(uploadResult.id.filename, uploadResult.duration)
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

type uploadBundleProducer struct {
	res chan bundle
}

func newUploadBundleProducer() *uploadBundleProducer {
	return &uploadBundleProducer{
		res: make(chan bundle),
	}
}

// createUploadBundles starts producing the bundles of the files and returns the
// channel they are sent to. The channel is closed after all files are done.
// After the last bundle of each file, the producer sends one end-of-file marker
// carrying the number of bundles it emitted for that file.
func (ubp *uploadBundleProducer) createUploadBundles(f processableFiles) <-chan bundle {
	var producerWg sync.WaitGroup
	producerWg.Add(2)
	go ubp.createUploadBundlesFromSingleBundleFiles(f.singleBundleFiles, &producerWg)
	go ubp.createUploadBundlesFromMultiBundleFiles(f.multiBundleFiles, &producerWg)

	go func(wg *sync.WaitGroup, bundleCh chan<- bundle) {
		wg.Wait()
		close(bundleCh)
	}(&producerWg, ubp.res)

	return ubp.res
}

// forEachFile calls produce for each file and sends the end-of-file marker
// after produce returns, so every exit path of produce is covered. produce has
// to emit all bundles of the file through emit.
func (ubp *uploadBundleProducer) forEachFile(files []string, wg *sync.WaitGroup, produce func(file string, emit func(bundle))) {
	defer wg.Done()
	for _, file := range files {
		count := 0
		produce(file, func(b bundle) {
			count++
			ubp.res <- b
		})
		ubp.res <- bundle{id: bundleIdentifier{filename: file}, endOfFile: true, bundleCount: count}
	}
}

func (ubp *uploadBundleProducer) createUploadBundlesFromSingleBundleFiles(files []string, wg *sync.WaitGroup) {
	ubp.forEachFile(files, wg, func(file string, emit func(bundle)) {
		f, err := os.Open(file)
		if err != nil {
			emit(bundle{id: bundleIdentifier{filename: file}, err: err})
			return
		}
		defer f.Close()

		fInfo, err := f.Stat()
		if err != nil {
			emit(bundle{
				id: bundleIdentifier{
					filename:     file,
					bundleNumber: 1,
				},
				err: err,
			})
			return
		}

		emit(bundle{
			id: bundleIdentifier{
				filename:     file,
				bundleNumber: 1,
				startBytes:   0,
				endBytes:     fInfo.Size(),
			}})
	})
}

func (ubp *uploadBundleProducer) createUploadBundlesFromMultiBundleFiles(files []string, wg *sync.WaitGroup) {
	ubp.forEachFile(files, wg, func(file string, emit func(bundle)) {
		f, err := os.Open(file)
		if err != nil {
			emit(bundle{id: bundleIdentifier{filename: file}, err: err})
			return
		}
		defer f.Close()

		reader := bufio.NewReader(f)
		calcRes := make(chan util.FileChunkCalculationResult)

		go util.CalculateFileChunks(reader, MultiBundleFileBundleDelimiter, calcRes)

		for res := range calcRes {
			if res.Err != nil {
				emit(bundle{
					id: bundleIdentifier{
						filename:     file,
						bundleNumber: res.FileChunk.ChunkNumber,
					},
					err: res.Err,
				})
			} else {
				if res.FileChunk.StartBytes == res.FileChunk.EndBytes {
					continue
				}
				emit(bundle{
					id: bundleIdentifier{
						filename:     file,
						bundleNumber: res.FileChunk.ChunkNumber,
						startBytes:   res.FileChunk.StartBytes,
						endBytes:     res.FileChunk.EndBytes,
					},
				})
			}
		}
	})
}

type uploadBundleConsumer struct {
	client        *fhir.Client
	uploadResults chan<- bundleUploadResult
	backoff       *util.Backoff
}

func newUploadBundleConsumer(client *fhir.Client, uploadResults chan<- bundleUploadResult) *uploadBundleConsumer {
	return &uploadBundleConsumer{
		client:        client,
		uploadResults: uploadResults,
		backoff:       util.NewBackoff(1 * time.Minute),
	}
}

// uploadBundles uploads the bundles from uploadBundles until the channel is
// closed. End-of-file markers are forwarded as results without upload.
func (consumer *uploadBundleConsumer) uploadBundles(uploadBundles <-chan bundle, concurrency int, wg *sync.WaitGroup) {
	limiter := make(chan bool, concurrency)

	for queueItem := range uploadBundles {
		if queueItem.endOfFile {
			// Sent synchronously, so it neither needs a limiter slot nor
			// wg accounting. The aggregator always keeps receiving.
			consumer.uploadResults <- bundleUploadResult{id: queueItem.id, endOfFile: true, bundleCount: queueItem.bundleCount}
			continue
		}
		limiter <- true
		wg.Add(1)
		go func(b bundle, limiter <-chan bool, wg *sync.WaitGroup) {
			defer func() { <-limiter }()
			consumer.backoff.Wait()
			if b.err != nil {
				consumer.uploadResults <- bundleUploadResult{id: b.id, err: b.err}
			} else {
				start := time.Now()
				if uploadInfo, err := uploadBundle(consumer.client, &b.id); err != nil {
					consumer.uploadResults <- bundleUploadResult{id: b.id, err: err, duration: time.Duration(time.Since(start).Nanoseconds() / int64(concurrency))}
				} else {
					switch uploadInfo.statusCode {
					case http.StatusServiceUnavailable, http.StatusGatewayTimeout:
						consumer.backoff.Increase()
					case http.StatusOK:
						consumer.backoff.Reset()
					}
					consumer.uploadResults <- bundleUploadResult{id: b.id, uploadInfo: uploadInfo, duration: time.Duration(time.Since(start).Nanoseconds() / int64(concurrency))}
				}
			}
			wg.Done()
		}(queueItem, limiter, wg)
	}
}

// fileProgress translates per-bundle results and end-of-file markers into
// per-file progress. It is not safe for concurrent use.
type fileProgress struct {
	files    map[string]*fileState
	progress progress
}

// fileState is the progress of a single file. expected is only valid once
// ended is true.
type fileState struct {
	done     int
	duration time.Duration
	expected int
	ended    bool
}

func newFileProgress(p progress) *fileProgress {
	return &fileProgress{files: make(map[string]*fileState), progress: p}
}

// bundleDone records the result of one bundle of the file.
func (fp *fileProgress) bundleDone(filename string, duration time.Duration) {
	state := fp.state(filename)
	state.done++
	state.duration += duration
	fp.completeIfDone(filename, state)
}

// fileEnd records that the producer emitted count bundles for the file. The
// marker may arrive before or after the results of the bundles.
func (fp *fileProgress) fileEnd(filename string, count int) {
	state := fp.state(filename)
	state.ended = true
	state.expected = count
	fp.completeIfDone(filename, state)
}

func (fp *fileProgress) state(filename string) *fileState {
	state, ok := fp.files[filename]
	if !ok {
		state = &fileState{}
		fp.files[filename] = state
	}
	return state
}

// completeIfDone increments the progress once by the summed duration, if the
// marker has arrived and all bundles of the file are done, and drops the state
// of the file.
func (fp *fileProgress) completeIfDone(filename string, state *fileState) {
	if state.ended && state.done == state.expected {
		fp.progress.increment(state.duration)
		delete(fp.files, filename)
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

func createRealProgress(numFiles int, out io.Writer) progress {
	p := mpb.New(mpb.WithOutput(out))
	return realProgress{progress: p,
		bar: p.AddBar(int64(numFiles),
			mpb.BarRemoveOnComplete(),
			mpb.PrependDecorators(
				decor.Name("upload", decor.WC{W: 7, C: decor.DindentRight}),
				decor.OnComplete(decor.EwmaETA(decor.ET_STYLE_GO, 60, decor.WC{W: 4}), "done"),
			),
			mpb.AppendDecorators(decor.Percentage()),
		),
	}
}

// createProgress creates a progress bar rendered to out, unless disabled by
// --no-progress or --silent.
func createProgress(numFiles int, out io.Writer) progress {
	if noProgress || silent {
		return noopProgress{}
	} else {
		return createRealProgress(numFiles, out)
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

		// All inputs are validated here, so printing the usage would only
		// distract from errors.
		cmd.SilenceUsage = true
		meta := metaOut(cmd)

		dir := args[0]

		files, err := findProcessableFiles(dir)
		if err != nil {
			return err
		}

		fmt.Fprintf(meta, "Starting Upload to %s ...\n", server)

		numFiles := len(files.singleBundleFiles) + len(files.multiBundleFiles)
		if numFiles == 0 {
			fmt.Fprintln(meta, "Found no files to upload.")
			return nil
		}

		fmt.Fprintf(meta, "Uploading %d files (%d JSON files and %d NDJSON files) ...\n",
			numFiles, len(files.singleBundleFiles), len(files.multiBundleFiles))

		progress := createProgress(numFiles, meta)

		// Aggregate results in one single goroutine
		uploadResultCh := make(chan bundleUploadResult)
		aggregatedUploadResultsCh := make(chan aggregatedUploadResults)

		var consumerWg sync.WaitGroup
		start := time.Now()
		bundleConsumer := newUploadBundleConsumer(client, uploadResultCh)
		go aggregateUploadResults(uploadResultCh, aggregatedUploadResultsCh, newFileProgress(progress))

		bundleConsumer.uploadBundles(newUploadBundleProducer().createUploadBundles(files), concurrency, &consumerWg)

		consumerWg.Wait()
		close(uploadResultCh)
		progress.wait()
		client.CloseIdleConnections()

		aggResults := <-aggregatedUploadResultsCh

		if aggResults.totalProcessedBundles == 0 {
			fmt.Fprintln(meta, "Found no bundles to upload.")
			return nil
		}

		fmt.Fprintf(meta, "Uploads          [total, concurrency]     %d, %d\n",
			aggResults.totalProcessedBundles, concurrency)
		fmt.Fprintf(meta, "Success          [ratio]                  %.2f %%\n",
			float32(aggResults.totalProcessedBundles-len(aggResults.errors)-len(aggResults.errorResponses))/float32(aggResults.totalProcessedBundles)*100)
		fmt.Fprintf(meta, "Duration         [total]                  %s\n",
			util.FmtDurationHumanReadable(time.Since(start)))

		if len(aggResults.requestDurations) > 0 {
			requestStats := util.CalculateDurationStatistics(aggResults.requestDurations)
			fmt.Fprintf(meta, "Requ. Latencies  [mean, 50, 95, 99, max]  %s, %s, %s, %s %s\n",
				requestStats.Mean, requestStats.Q50, requestStats.Q95, requestStats.Q99, requestStats.Max)
		}

		if len(aggResults.processingDurations) > 0 {
			processingStats := util.CalculateDurationStatistics(aggResults.requestDurations)
			fmt.Fprintf(meta, "Proc. Latencies  [mean, 50, 95, 99, max]  %s, %s, %s, %s %s\n",
				processingStats.Mean, processingStats.Q50, processingStats.Q95, processingStats.Q99, processingStats.Max)
		}

		totalTransfers := len(aggResults.requestDurations)
		fmt.Fprintf(meta, "Bytes In         [total, mean]            %s, %s\n", util.FmtBytesHumanReadable(float32(aggResults.totalBytesIn)), util.FmtBytesHumanReadable(float32(aggResults.totalBytesIn)/float32(totalTransfers)))
		fmt.Fprintf(meta, "Bytes Out        [total, mean]            %s, %s\n", util.FmtBytesHumanReadable(float32(aggResults.totalBytesOut)), util.FmtBytesHumanReadable(float32(aggResults.totalBytesOut)/float32(totalTransfers)))

		errorFrequencies := make(map[int]int)
		for _, errorResponse := range aggResults.errorResponses {
			errorFrequencies[errorResponse.StatusCode]++
		}
		statusCodes := make([]string, 1, len(errorFrequencies)+1)
		statusCodes[0] = fmt.Sprintf("200:%d", len(aggResults.processingDurations))
		for statusCode, freq := range errorFrequencies {
			statusCodes = append(statusCodes, fmt.Sprintf("%d:%d", statusCode, freq))
		}
		fmt.Fprintf(meta, "Status Codes     [code:count]             %s\n", strings.Join(statusCodes, ", "))

		errOut := errOut(cmd)
		if len(aggResults.errorResponses) > 0 {
			fmt.Fprintln(errOut)
			fmt.Fprintln(errOut, "Non-OK Responses:")
			fmt.Fprintln(errOut)
			for bundleId, errorResponse := range aggResults.errorResponses {
				fmt.Fprintf(errOut, "File: %s [Bundle: %d]\n", bundleId.filename, bundleId.bundleNumber)
				fmt.Fprintf(errOut, "%s", util.Indent(4, errorResponse.String()))
			}
		}
		if len(aggResults.errors) > 0 {
			fmt.Fprintln(errOut, "\nErrors:")
			for bundleId, err := range aggResults.errors {
				fmt.Fprintf(errOut, "File: %s [Bundle: %d] : %v\n", bundleId.filename, bundleId.bundleNumber, err.Error())
			}
		}
		if failed := len(aggResults.errorResponses) + len(aggResults.errors); failed > 0 {
			return fmt.Errorf("failed to upload %d of %d bundles", failed, aggResults.totalProcessedBundles)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(uploadCmd)

	uploadCmd.Flags().StringVar(&server, "server", "", "the base URL of the server to use")
	uploadCmd.Flags().IntVarP(&concurrency, "concurrency", "c", 2, "number of parallel uploads")

	_ = uploadCmd.MarkFlagRequired("server")
}
