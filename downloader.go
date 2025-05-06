package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

func downloadImagesConcurrently(itemsToDownload []DownloadItem) int {
	var wg sync.WaitGroup
	var httpErrorCount int32
	var otherErrorCount int32
	var processedCount int32
	var successfulDownloadCount int32

	totalItems := len(itemsToDownload)
	if totalItems == 0 {
		fmt.Println("No new images require downloading.")
		return 0
	}

	semaphore := make(chan struct{}, DownloadConcurrency)

	customTransport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          DownloadConcurrency + 10,
		MaxIdleConnsPerHost:   DownloadConcurrency,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	client := &http.Client{
		Transport: customTransport,
		Timeout:   DefaultTimeout,
	}

	fmt.Printf("\nStarting download of %d images (Concurrency: %d)...\n", totalItems, DownloadConcurrency)
	downloadStartTime := time.Now()

	doneStats := make(chan struct{})
	go reportStats(doneStats, &processedCount, &successfulDownloadCount, totalItems, downloadStartTime)

	for i := range itemsToDownload {
		item := itemsToDownload[i]
		wg.Add(1)
		semaphore <- struct{}{}

		go func(item DownloadItem) {
			defer wg.Done()
			defer func() { <-semaphore }()

			err := downloadSingleImage(client, item)
			atomic.AddInt32(&processedCount, 1)

			if err != nil {
				if _, ok := err.(*statusCodeError); ok {
					atomic.AddInt32(&httpErrorCount, 1)
				} else {
					atomic.AddInt32(&otherErrorCount, 1)
				}
			} else {
				atomic.AddInt32(&successfulDownloadCount, 1)
				idStr := strconv.Itoa(item.ID)
				var cachePath string
				var mutex *sync.Mutex
				if item.IsField {
					cachePath = FieldCachePath
					mutex = &fieldCacheMutex
				} else {
					cachePath = CardCachePath
					mutex = &cardCacheMutex
				}
				err := appendIDToCacheFile(cachePath, idStr, mutex)
				if err != nil {
					log.Printf("\nCache Append Error for ID %s to %s: %v\n", idStr, cachePath, err)
				}
			}
		}(item)
	}

	wg.Wait()
	close(doneStats)
	close(semaphore)

	finalProcessed := atomic.LoadInt32(&processedCount)
	finalSuccessful := atomic.LoadInt32(&successfulDownloadCount)
	finalHttpErrors := int(atomic.LoadInt32(&httpErrorCount))
	finalOtherErrors := int(atomic.LoadInt32(&otherErrorCount))

	fmt.Printf("\nDownload process finished. Processed: %d/%d. Successful: %d.           \n",
		finalProcessed, totalItems, finalSuccessful)
	if finalOtherErrors > 0 {
		fmt.Printf("%d downloads failed due to non-HTTP errors.\n", finalOtherErrors)
	}
	if finalHttpErrors > 0 && int(finalSuccessful) < totalItems-finalOtherErrors {
		fmt.Printf("%d downloads likely failed due to HTTP errors.\n", finalHttpErrors)
	}

	return finalHttpErrors
}

func reportStats(done <-chan struct{}, processedCounter, successfulCounter *int32, total int, startTime time.Time) {
	ticker := time.NewTicker(StatsUpdateInterval)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			processed := atomic.LoadInt32(processedCounter)
			successful := atomic.LoadInt32(successfulCounter)
			elapsed := time.Since(startTime).Seconds()

			if processed == 0 || elapsed < 0.1 {
				continue
			}

			percentage := float64(processed) * 100.0 / float64(total)
			ips := float64(successful) / elapsed

			progressLine := fmt.Sprintf("Processed: %d/%d (%.2f%%) - Successful: %d - Speed: %.2f img/s",
				processed, total, percentage, successful, ips)

			fmt.Printf("%-75s\r", progressLine)
		}
	}
}

type statusCodeError struct {
	StatusCode int
	URL        string
}

func (e *statusCodeError) Error() string {
	return fmt.Sprintf("bad status code %d fetching %s", e.StatusCode, e.URL)
}

func downloadSingleImage(client *http.Client, item DownloadItem) error {
	imgURL := ImagesBaseURL
	destDir := PicsDir
	if item.IsField {
		imgURL += "_cropped"
		destDir = FieldPicsDir
	}
	imgURL += fmt.Sprintf("/%d.jpg", item.ID)

	convertedID := convertID(item.ID)
	fileName := fmt.Sprintf("%d.jpg", convertedID)
	filePath := filepath.Join(destDir, fileName)

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", destDir, err)
	}

	req, err := http.NewRequest("GET", imgURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request for %s: %w", imgURL, err)
	}
	for key, value := range RequestHeaders {
		req.Header.Set(key, value)
	}

	resp, err := client.Do(req)
	if err != nil {
		if os.IsTimeout(err) {
			return fmt.Errorf("http request timeout for %s: %w", imgURL, err)
		}
		return fmt.Errorf("http request failed for %s: %w", imgURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &statusCodeError{StatusCode: resp.StatusCode, URL: imgURL}
	}

	outFile, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create output file %s: %w", filePath, err)
	}
	defer outFile.Close()

	_, err = io.Copy(outFile, resp.Body)
	if err != nil {
		_ = os.Remove(filePath)
		return fmt.Errorf("failed to write image data to %s: %w", filePath, err)
	}

	return nil
}
