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

// Package cmd contains all commands of blazectl
package cmd

import (
	"errors"
	"fmt"
	"github.com/vbauerster/mpb"
	"github.com/vbauerster/mpb/decor"
	"gonum.org/v1/gonum/floats"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type uploadResult struct {
	statusCode        int
	bytesOut, bytesIn int64
	duration          time.Duration
}

// Uploads file with name and returns either the status code of the response or
// an error.
func uploadFile(filename string) (uploadResult, error) {
	info, err := os.Stat(filename)
	if err != nil {
		return uploadResult{}, err
	}
	fileSize := info.Size()

	file, err := os.Open(filename)
	if err != nil {
		return uploadResult{}, err
	}
	defer file.Close()

	start := time.Now()
	resp, err := http.Post(server, "application/fhir+json", file)
	if err != nil {
		return uploadResult{}, err
	}
	defer resp.Body.Close()

	bodySize, _ := io.Copy(ioutil.Discard, resp.Body)

	return uploadResult{resp.StatusCode, fileSize, bodySize, time.Since(start)}, nil
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

		durations := make([]float64, 0, len(files))
		var totalBytesIn int64 = 0
		var totalBytesOut int64 = 0
		errorResponses := make(map[string]int)
		errors := make(map[string]error)

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

		// Loop through files and upload
		sem := make(chan bool, concurrency)
		start := time.Now()
		for _, file := range files {
			sem <- true
			go func(filename string) {
				defer func() { <-sem }()
				if uploadResult, err := uploadFile(filename); err != nil {
					errors[filename] = err
				} else {
					if uploadResult.statusCode != http.StatusOK {
						errorResponses[filename] = uploadResult.statusCode
					}
					totalBytesIn += uploadResult.bytesIn
					totalBytesOut += uploadResult.bytesOut
					durations = append(durations, uploadResult.duration.Seconds())
					bar.IncrBy(1, time.Duration(uploadResult.duration.Nanoseconds()/int64(concurrency)))
				}
			}(filepath.Join(dir, file.Name()))
		}

		// Wait for all uploads to finish
		for i := 0; i < cap(sem); i++ {
			sem <- true
		}

		progress.Wait()

		fmt.Printf("Uploads       [total, concurrency]     %d, %d\n",
			len(files), concurrency)
		fmt.Printf("Success       [ratio]                  %.2f %%\n",
			float32(len(files)-len(errors)-len(errorResponses))/float32(len(files))*100)
		fmt.Printf("Duration      [total]                  %s\n",
			time.Since(start).Round(time.Second))

		sort.Float64s(durations)
		mean := time.Duration(floats.Sum(durations)/float64(len(durations))*1000) * time.Millisecond
		q50 := time.Duration(durations[len(durations)/2]*1000) * time.Millisecond
		q95 := time.Duration(durations[int(float32(len(durations))*0.95)]*1000) * time.Millisecond
		q99 := time.Duration(durations[int(float32(len(durations))*0.99)]*1000) * time.Millisecond
		max := time.Duration(durations[len(durations)-1]*1000) * time.Millisecond
		fmt.Printf("Latencies     [mean, 50, 95, 99, max]  %s, %s, %s, %s %s\n", mean, q50, q95, q99, max)

		totalTransfers := len(durations) + len(errorResponses)
		fmt.Printf("Bytes In      [total, mean]            %s, %s\n", fmtBytes(float32(totalBytesIn), 0), fmtBytes(float32(totalBytesIn)/float32(totalTransfers), 0))
		fmt.Printf("Bytes Out     [total, mean]            %s, %s\n", fmtBytes(float32(totalBytesOut), 0), fmtBytes(float32(totalBytesOut)/float32(totalTransfers), 0))

		errorFrequencies := make(map[int]int)
		for _, statusCode := range errorResponses {
			errorFrequencies[statusCode]++
		}
		statusCodes := make([]string, 1, len(errorFrequencies)+1)
		statusCodes[0] = fmt.Sprintf("200:%d", len(durations))
		for statusCode, freq := range errorFrequencies {
			statusCodes = append(statusCodes, fmt.Sprintf("%d:%d", statusCode, freq))
		}
		fmt.Printf("Status Codes  [code:count]             %s\n\n", strings.Join(statusCodes, ", "))

		fmt.Println("Non-OK Status Codes:")
		for filename, statusCode := range errorResponses {
			fmt.Println(filename, ":", statusCode)
		}
		fmt.Println()
		fmt.Println("Errors:")
		for filename, err := range errors {
			fmt.Println(filename, ":", err.Error())
		}
	},
}

func init() {
	rootCmd.AddCommand(uploadCmd)

	uploadCmd.Flags().IntVarP(&concurrency, "concurrency", "c", 2, "number of parallel uploads")
}
