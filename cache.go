package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
)

type IDSet map[string]struct{}

var cardCacheMutex sync.Mutex
var fieldCacheMutex sync.Mutex

func loadCache() (IDSet, IDSet) {
	cardSet := make(IDSet)
	fieldSet := make(IDSet)
	var wg sync.WaitGroup
	wg.Add(2)

	fmt.Println("Loading cached IDs from .tmp files (if any)...")

	go func() {
		defer wg.Done()
		loadSetFromFile(CardCachePath, cardSet)
	}()
	go func() {
		defer wg.Done()
		loadSetFromFile(FieldCachePath, fieldSet)
	}()

	wg.Wait()
	fmt.Printf("Finished loading cache: %d cards, %d fields found.\n", len(cardSet), len(fieldSet))
	return cardSet, fieldSet
}

func loadSetFromFile(filePath string, targetSet IDSet) {
	file, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return
		}
		log.Printf("Error opening cache file %s: %v. Assuming empty.\n", filePath, err)
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		id := strings.TrimSpace(scanner.Text())
		if id != "" {
			targetSet[id] = struct{}{}
		}
	}

	if err := scanner.Err(); err != nil {
		log.Printf("Error reading cache file %s: %v\n", filePath, err)
	}
}

func appendIDToCacheFile(filePath string, idStr string, mutex *sync.Mutex) error {
	mutex.Lock()
	defer mutex.Unlock()

	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open cache file %s for appending: %w", filePath, err)
	}
	defer file.Close()

	if _, err := fmt.Fprintln(file, idStr); err != nil {
		return fmt.Errorf("failed to write id %s to %s: %w", idStr, filePath, err)
	}
	return nil
}
