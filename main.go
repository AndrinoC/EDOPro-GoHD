package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"sync"
	"time"
)

func setupEnvironment() {
	fmt.Println("Initializing...")
	for _, dir := range RequiredDirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			log.Printf("Warning: Could not create directory %s: %v\n", dir, err)
		}
	}
	for _, fPath := range TempCacheFiles {
		if _, err := os.Stat(fPath); os.IsNotExist(err) {
			file, err := os.Create(fPath)
			if err != nil {
				log.Printf("Warning: Could not create tracker file %s: %v\n", fPath, err)
			} else {
				_ = file.Close()
			}
		}
	}

	fmt.Printf(IntroString, ProgramVersion)
}

func cleanupTrackerFiles() {
	fmt.Println("\nCleaning up temporary tracking files...")
	for _, fPath := range TempCacheFiles {
		err := os.Remove(fPath)
		if err != nil {
			if !os.IsNotExist(err) {
				log.Printf("Warning: Could not remove tracker file %s: %v\n", fPath, err)
			}
		} else {
			fmt.Printf("Removed: %s\n", fPath)
		}
	}
}

func main() {
	startTime := time.Now()
	setupEnvironment()

	processCompletedSuccessfully := false
	defer func() {
		if processCompletedSuccessfully {
			cleanupTrackerFiles()
		} else {
			fmt.Println("\nTracking files kept due to an error or interruption during the process.")
		}
		duration := time.Since(startTime)
		fmt.Printf("\nProcess finished in %s.\n", duration.Round(time.Millisecond))
	}()

	cachedCardIDs, cachedFieldIDs := loadCache()

	var allCardIDs, allFieldIDs []int
	var cardErr, fieldErr error
	var wgApi sync.WaitGroup
	wgApi.Add(2)
	fmt.Println("Fetching card lists from API...")
	go func() {
		defer wgApi.Done()
		allCardIDs, cardErr = getAllCardIDs()
	}()
	go func() {
		defer wgApi.Done()
		allFieldIDs, fieldErr = getAllFieldIDs()
	}()
	wgApi.Wait()

	if cardErr != nil {
		log.Printf("ERROR: Failed to fetch card list: %v", cardErr)
		return
	}
	if fieldErr != nil {
		log.Printf("ERROR: Failed to fetch field list: %v", fieldErr)
		return
	}
	fmt.Printf("Found %d cards and %d fields from API.\n", len(allCardIDs), len(allFieldIDs))

	potentialDownloads := make([]DownloadItem, 0, len(allCardIDs)+len(allFieldIDs))
	for _, id := range allCardIDs {
		if id != 0 {
			potentialDownloads = append(potentialDownloads, DownloadItem{ID: id, IsField: false})
		}
	}
	for _, id := range allFieldIDs {
		if id != 0 {
			potentialDownloads = append(potentialDownloads, DownloadItem{ID: id, IsField: true})
		}
	}

	itemsToDownload := []DownloadItem{}
	fmt.Println("Filtering already downloaded images...")
	for _, item := range potentialDownloads {
		idStr := strconv.Itoa(item.ID)
		var found bool
		if item.IsField {
			_, found = cachedFieldIDs[idStr]
		} else {
			_, found = cachedCardIDs[idStr]
		}
		if !found {
			itemsToDownload = append(itemsToDownload, item)
		}
	}

	totalToAttempt := len(itemsToDownload)
	fmt.Printf("Found %d new images to download.\n", totalToAttempt)

	if totalToAttempt > 0 {
		httpErrors := downloadImagesConcurrently(itemsToDownload)

		fmt.Printf("\nDownload Summary: Attempted: %d. HTTP errors (e.g., 404s): %d.\n", totalToAttempt, httpErrors)
		processCompletedSuccessfully = true

	} else {
		fmt.Println("\nNo new images to download. Everything is up to date!")
		processCompletedSuccessfully = true
	}

}
