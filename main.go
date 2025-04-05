package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
)

type Page struct {
	URL string `json:"url"`
}

type Result struct {
	Page Page `json:"page"`
}

type APIResponse struct {
	Results []Result `json:"results"`
}

func main() {
	domain := flag.String("d", "", "Domain to search (ex: target.com)")
	allFlag := flag.Bool("all", false, "Search for all related URLs")
	flag.Parse()
	if *domain == "" {
		fmt.Println("Usage: ./fetch_urlscan -d <domain> [-all]")
		os.Exit(1)
	}
	var apiURL string
	if *allFlag {
		apiURL = "https://urlscan.io/api/v1/search/?q=" + *domain + "&size=10000"
	} else {
		apiURL = "https://urlscan.io/api/v1/search/?q=domain:" + *domain + "&size=10000"
	}
	resp, err := http.Get(apiURL)
	if err != nil {
		log.Fatalf("Error making request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		log.Fatalf("Error: status code %d", resp.StatusCode)
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("Error reading response: %v", err)
	}
	var apiResp APIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		log.Fatalf("Error decoding JSON: %v", err)
	}
	for _, result := range apiResp.Results {
		if result.Page.URL != "" {
			fmt.Println(result.Page.URL)
		}
	}
}
