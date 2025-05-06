package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

type apiCardImage struct {
	ID int `json:"id"`
}
type apiCardData struct {
	CardImages []apiCardImage `json:"card_images"`
}
type apiResponse struct {
	Data []apiCardData `json:"data"`
}

func fetchIDs(apiURL string, params map[string]string) ([]int, error) {
	client := &http.Client{Timeout: DefaultTimeout}
	reqURL, err := url.Parse(apiURL)
	if err != nil {
		return nil, fmt.Errorf("invalid API URL %s: %w", apiURL, err)
	}

	if params != nil {
		query := reqURL.Query()
		for k, v := range params {
			query.Add(k, v)
		}
		reqURL.RawQuery = query.Encode()
	}

	req, err := http.NewRequest("GET", reqURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request for %s: %w", reqURL.String(), err)
	}

	for key, value := range RequestHeaders {
		req.Header.Set(key, value)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request to %s: %w", reqURL.String(), err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("bad status code %d from %s: %s", resp.StatusCode, reqURL.String(), string(bodyBytes))
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body from %s: %w", reqURL.String(), err)
	}

	var apiResp apiResponse
	if err := json.Unmarshal(bodyBytes, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response from %s: %w", reqURL.String(), err)
	}

	ids := []int{}
	for _, card := range apiResp.Data {
		for _, img := range card.CardImages {
			if img.ID != 0 {
				ids = append(ids, img.ID)
			}
		}
	}

	return ids, nil
}

func getAllCardIDs() ([]int, error) {
	fmt.Println("Fetching all card IDs from API...")
	ids, err := fetchIDs(YGOProDeckCardsURL, nil)
	if err != nil {
		return nil, err
	}
	return ids, nil
}

func getAllFieldIDs() ([]int, error) {
	fmt.Println("Fetching all field spell IDs from API...")
	params := map[string]string{"type": "spell card", "race": "field"}
	ids, err := fetchIDs(YGOProDeckCardsURL, params)
	if err != nil {
		return nil, err
	}
	return ids, nil
}
