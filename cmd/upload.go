// Copyright © 2019 Alexander Kiel <alexander.kiel@life.uni-leipzig.de>
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
	"errors"
	"fmt"
	"github.com/life-research/blazectl/fhir"
	"github.com/spf13/cobra"
	"github.com/vbauerster/mpb"
	"github.com/vbauerster/mpb/decor"
	"gonum.org/v1/gonum/floats"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptrace"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type uploadInfo struct {
	statusCode         int
	bytesOut, bytesIn  int64
	requestDuration    time.Duration
	processingDuration time.Duration
}

// Uploads file with name and returns either the status code of the response or
// an error.
func uploadFile(client *fhir.Client, filename string) (uploadInfo, error) {
	info, err := os.Stat(filename)
	if err != nil {
		return uploadInfo{}, err
	}
	fileSize := info.Size()

	file, err := os.Open(filename)
	if err != nil {
		return uploadInfo{}, err
	}
	defer file.Close()

	req, err := client.NewTransactionRequest(file)
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

	bodySize, _ := io.Copy(ioutil.Discard, resp.Body)

	return uploadInfo{
		statusCode:         resp.StatusCode,
		bytesOut:           fileSize,
		bytesIn:            bodySize,
		requestDuration:    time.Since(requestStart),
		processingDuration: processingDuration,
	}, nil
}

type uploadResult struct {
	filename   string
	uploadInfo uploadInfo
	err        error
}

type aggregatedUploadResults struct {
	requestDurations, processingDurations []float64
	totalBytesIn, totalBytesOut           int64
	errorResponses                        map[string]int
	errors                                map[string]error
}

func aggregateUploadResults(
	numFiles int,
	uploadResultCh chan uploadResult,
	aggregatedUploadResultsCh chan aggregatedUploadResults) {

	requestDurations := make([]float64, 0, numFiles)
	processingDurations := make([]float64, 0, numFiles)
	var totalBytesIn int64
	var totalBytesOut int64
	errorResponses := make(map[string]int)
	errs := make(map[string]error)

	for uploadResult := range uploadResultCh {
		if uploadResult.err != nil {
			errs[uploadResult.filename] = uploadResult.err
		} else {
			if uploadResult.uploadInfo.statusCode == http.StatusOK {
				processingDurations = append(processingDurations, uploadResult.uploadInfo.processingDuration.Seconds())
			} else {
				errorResponses[uploadResult.filename] = uploadResult.uploadInfo.statusCode
			}
			totalBytesIn += uploadResult.uploadInfo.bytesIn
			totalBytesOut += uploadResult.uploadInfo.bytesOut
			requestDurations = append(requestDurations, uploadResult.uploadInfo.requestDuration.Seconds())
		}
	}

	aggregatedUploadResultsCh <- aggregatedUploadResults{
		requestDurations:    requestDurations,
		processingDurations: processingDurations,
		totalBytesIn:        totalBytesIn,
		totalBytesOut:       totalBytesOut,
		errorResponses:      errorResponses,
		errors:              errs,
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

		files, err := ioutil.ReadDir(dir)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		fmt.Printf("Starting Upload to %s ...\n", server)

		progress := mpb.New()
		bar := progress.AddBar(int64(len(files)),
			mpb.BarRemoveOnComplete(),
			mpb.PrependDecorators(
				decor.Name("upload", decor.WC{W: 7, C: decor.DidentRight}),
				decor.OnComplete(decor.EwmaETA(decor.ET_STYLE_GO, 60, decor.WC{W: 4}), "done"),
			),
			mpb.AppendDecorators(decor.Percentage()),
		)

		// Aggregate results in one single goroutine
		uploadResultCh := make(chan uploadResult)
		aggregatedUploadResultsCh := make(chan aggregatedUploadResults)
		go aggregateUploadResults(len(files), uploadResultCh, aggregatedUploadResultsCh)

		// Loop through files and upload
		sem := make(chan bool, concurrency)
		client := &fhir.Client{Base: server}
		start := time.Now()
		for _, file := range files {
			sem <- true
			go func(filename string) {
				defer func() { <-sem }()
				start := time.Now()
				if uploadInfo, err := uploadFile(client, filename); err != nil {
					uploadResultCh <- uploadResult{filename: filename, err: err}
					bar.Increment()
				} else {
					uploadResultCh <- uploadResult{filename: filename, uploadInfo: uploadInfo}
					bar.Increment(time.Duration(time.Since(start).Nanoseconds() / int64(concurrency)))
				}
			}(filepath.Join(dir, file.Name()))
		}

		// Wait for all uploads to finish
		for i := 0; i < cap(sem); i++ {
			sem <- true
		}
		close(uploadResultCh)
		client.CloseIdleConnections()

		aggResults := <-aggregatedUploadResultsCh

		fmt.Printf("Uploads          [total, concurrency]     %d, %d\n",
			len(files), concurrency)
		fmt.Printf("Success          [ratio]                  %.2f %%\n",
			float32(len(files)-len(aggResults.errors)-len(aggResults.errorResponses))/float32(len(files))*100)
		fmt.Printf("Duration         [total]                  %s\n",
			time.Since(start).Round(time.Second))

		requestStats := genStats(aggResults.requestDurations)
		fmt.Printf("Requ. Latencies  [mean, 50, 95, 99, max]  %s, %s, %s, %s %s\n",
			requestStats.mean, requestStats.q50, requestStats.q95, requestStats.q99, requestStats.max)

		processingStats := genStats(aggResults.processingDurations)
		fmt.Printf("Proc. Latencies  [mean, 50, 95, 99, max]  %s, %s, %s, %s %s\n",
			processingStats.mean, processingStats.q50, processingStats.q95, processingStats.q99, processingStats.max)

		totalTransfers := len(aggResults.requestDurations)
		fmt.Printf("Bytes In         [total, mean]            %s, %s\n", fmtBytes(float32(aggResults.totalBytesIn), 0), fmtBytes(float32(aggResults.totalBytesIn)/float32(totalTransfers), 0))
		fmt.Printf("Bytes Out        [total, mean]            %s, %s\n", fmtBytes(float32(aggResults.totalBytesOut), 0), fmtBytes(float32(aggResults.totalBytesOut)/float32(totalTransfers), 0))

		errorFrequencies := make(map[int]int)
		for _, statusCode := range aggResults.errorResponses {
			errorFrequencies[statusCode]++
		}
		statusCodes := make([]string, 1, len(errorFrequencies)+1)
		statusCodes[0] = fmt.Sprintf("200:%d", len(aggResults.processingDurations))
		for statusCode, freq := range errorFrequencies {
			statusCodes = append(statusCodes, fmt.Sprintf("%d:%d", statusCode, freq))
		}
		fmt.Printf("Status Codes     [code:count]             %s\n", strings.Join(statusCodes, ", "))

		if len(aggResults.errorResponses) > 0 {
			fmt.Println("\nNon-OK Status Codes:")
			for filename, statusCode := range aggResults.errorResponses {
				fmt.Println(filename, ":", statusCode)
			}
		}
		if len(aggResults.errors) > 0 {
			fmt.Println("\nErrors:")
			for filename, err := range aggResults.errors {
				fmt.Println(filename, ":", err.Error())
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(uploadCmd)

	uploadCmd.Flags().IntVarP(&concurrency, "concurrency", "c", 2, "number of parallel uploads")
}
