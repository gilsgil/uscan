package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
)

type Task struct {
	URL string `json:"url"`
}

type Result struct {
	Task Task `json:"task"`
}

type APIResponse struct {
	Results []Result `json:"results"`
	Total   int      `json:"total"`
	HasMore bool     `json:"has_more"`
	Cursor  string   `json:"cursor"`
}

func main() {
	domain := flag.String("d", "", "Domain to search (ex: target.com)")
	allFlag := flag.Bool("all", false, "Include related URLs")
	flag.Parse()
	if *domain == "" {
		fmt.Println("Usage: ./fetch_urlscan -d <domain> [-all]")
		os.Exit(1)
	}
	var baseQuery string
	if *allFlag {
		baseQuery = *domain
	} else {
		baseQuery = "domain:" + *domain
	}
	size := "100"
	baseURL := "https://urlscan.io/api/v1/search/?q=" + url.QueryEscape(baseQuery) + "&size=" + size
	cursor := ""
	for {
		reqURL := baseURL
		if cursor != "" {
			reqURL += "&cursor=" + url.QueryEscape(cursor)
		}
		resp, err := http.Get(reqURL)
		if err != nil {
			log.Fatalf("Error making request: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			log.Fatalf("Error: status code %d", resp.StatusCode)
		}
		body, err := ioutil.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			log.Fatalf("Error reading response: %v", err)
		}
		var apiResp APIResponse
		if err := json.Unmarshal(body, &apiResp); err != nil {
			log.Fatalf("Error decoding JSON: %v", err)
		}
		for _, result := range apiResp.Results {
			if result.Task.URL != "" {
				fmt.Println(result.Task.URL)
			}
		}
		if !apiResp.HasMore || apiResp.Cursor == "" {
			break
		}
		cursor = apiResp.Cursor
	}
}
