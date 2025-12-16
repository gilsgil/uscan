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
	"strings"
	"time"
)

type Task struct {
	URL string `json:"url"`
}

type Result struct {
	Task Task        `json:"task"`
	Sort interface{} `json:"sort"` // Sort field used for search_after pagination (can be array of numbers or string)
}

type APIResponse struct {
	Results []Result `json:"results"`
	Total   int      `json:"total"`
	HasMore bool     `json:"has_more"`
	Cursor  string   `json:"cursor"` // Deprecated, but kept for compatibility
}

func buildQuery(filters map[string]string, customQuery string) string {
	if customQuery != "" {
		return customQuery
	}

	var queryParts []string

	// Domain filter
	// By default, use page.domain for exact page domain matches
	// Use domain: only when -all flag is set (to include all contacted domains)
	if domain, ok := filters["domain"]; ok && domain != "" {
		allFlag := filters["all"] == "true"
		if allFlag {
			// -all flag: search in all contacted domains (broader search)
			queryParts = append(queryParts, "domain:"+domain)
		} else {
			// Default: search only in page domain (exact match)
			// According to urlscan.io docs, page.domain searches the domain and subdomains
			// To get exact match only (no subdomains), we need to use a more specific query
			// Try using page.domain.keyword for exact string match, or use NOT to exclude subdomains
			// For now, use page.domain but we'll filter results if needed
			queryParts = append(queryParts, "page.domain:"+domain)
		}
	}

	// Page domain filter
	if pageDomain, ok := filters["page-domain"]; ok && pageDomain != "" {
		queryParts = append(queryParts, "page.domain:"+pageDomain)
	}

	// IP filter
	if ip, ok := filters["ip"]; ok && ip != "" {
		queryParts = append(queryParts, "page.ip:"+ip)
	}

	// URL filter
	if urlFilter, ok := filters["url"]; ok && urlFilter != "" {
		// Escape special characters for URL search (ElasticSearch Query String)
		// Escape backslash first to avoid double escaping
		escapedURL := strings.ReplaceAll(urlFilter, "\\", "\\\\")
		// Then escape other special characters
		escapedURL = strings.ReplaceAll(escapedURL, ":", "\\:")
		escapedURL = strings.ReplaceAll(escapedURL, "/", "\\/")
		escapedURL = strings.ReplaceAll(escapedURL, "(", "\\(")
		escapedURL = strings.ReplaceAll(escapedURL, ")", "\\)")
		escapedURL = strings.ReplaceAll(escapedURL, "[", "\\[")
		escapedURL = strings.ReplaceAll(escapedURL, "]", "\\]")
		escapedURL = strings.ReplaceAll(escapedURL, "*", "\\*")
		escapedURL = strings.ReplaceAll(escapedURL, "?", "\\?")
		queryParts = append(queryParts, "page.url.keyword:"+escapedURL)
	}

	// ASN filter
	if asn, ok := filters["asn"]; ok && asn != "" {
		// Ensure AS prefix if not present
		if !strings.HasPrefix(asn, "AS") {
			asn = "AS" + asn
		}
		queryParts = append(queryParts, "page.asn:"+asn)
	}

	// ASN Name filter
	if asnName, ok := filters["asnname"]; ok && asnName != "" {
		queryParts = append(queryParts, "page.asnname:"+asnName)
	}

	// Hash filter
	if hash, ok := filters["hash"]; ok && hash != "" {
		queryParts = append(queryParts, "hash:"+hash)
	}

	// Date filter
	if dateFilter, ok := filters["date"]; ok && dateFilter != "" {
		queryParts = append(queryParts, "date:"+dateFilter)
	}

	// Filename filter
	if filename, ok := filters["filename"]; ok && filename != "" {
		queryParts = append(queryParts, "filename:"+filename)
	}

	if len(queryParts) == 0 {
		return ""
	}

	return strings.Join(queryParts, " AND ")
}

func applyPreset(preset string, filters map[string]string) {
	switch preset {
	case "phishing":
		// Domain was contacted but isn't the page/primary domain
		if domain, ok := filters["domain"]; ok && domain != "" {
			filters["preset-query"] = fmt.Sprintf("domain:%s AND NOT page.domain:%s", domain, domain)
		}
	case "recent":
		// Scans in the past 7 days
		filters["date"] = ">now-7d"
	case "recent-month":
		// Scans in the past month
		filters["date"] = ">now-1M"
	case "recent-year":
		// Scans in the past year
		filters["date"] = ">now-1y"
	case "with-ip":
		// Non-empty IP scans
		filters["preset-query"] = "page.ip:*"
	case "fuzzy-domain":
		// Fuzzy search for domain (excluding exact match)
		if domain, ok := filters["domain"]; ok && domain != "" {
			filters["preset-query"] = fmt.Sprintf("page.domain:(%s~ AND NOT %s)", domain, domain)
		}
	}
}

func main() {
	// Basic filters
	domain := flag.String("d", "", "Domain to search (ex: target.com) - searches page.domain by default")
	allFlag := flag.Bool("all", false, "Use domain: instead of page.domain: (includes third parties, CDNs, etc.)")
	domainOnly := flag.Bool("domain-only", false, "Same as -all, uses domain: instead of page.domain:")

	// Advanced filters
	pageDomain := flag.String("page-domain", "", "Page domain filter (ex: target.com)")
	ip := flag.String("ip", "", "IP address filter (ex: 1.2.3.4 or 1.2.3.0/24)")
	urlFilter := flag.String("url", "", "URL filter (ex: https://target.com/path)")
	asn := flag.String("asn", "", "ASN filter (ex: AS24940 or 24940)")
	asnName := flag.String("asnname", "", "ASN name filter (ex: hetzner)")
	hash := flag.String("hash", "", "SHA256 hash filter")
	dateFilter := flag.String("date", "", "Date filter (ex: >now-7d or [2020-01-01 TO 2020-02-01])")
	filename := flag.String("filename", "", "Filename filter (ex: wp-content/uploads/)")

	// Custom query
	customQuery := flag.String("q", "", "Custom query (overrides all other filters)")

	// Presets
	preset := flag.String("preset", "", "Preset filter: phishing, recent, recent-month, recent-year, with-ip, fuzzy-domain")

	// Pagination options
	maxResults := flag.Int("max", 0, "Maximum number of results to return (0 = unlimited)")
	verbose := flag.Bool("v", false, "Verbose output (show progress)")

	// API token - can be provided via flag or environment variable URLSCAN
	apiTokenFlag := flag.String("token", "", "API token for urlscan.io (optional, defaults to URLSCAN env var)")

	flag.Parse()

	// Get API token from flag or environment variable
	apiToken := *apiTokenFlag
	if apiToken == "" {
		apiToken = os.Getenv("URLSCAN")
	}

	// Build filters map
	filters := make(map[string]string)
	if *domain != "" {
		filters["domain"] = *domain
	}
	// -all or -domain-only both use domain: instead of page.domain:
	if *allFlag || *domainOnly {
		filters["all"] = "true"
	}
	if *pageDomain != "" {
		filters["page-domain"] = *pageDomain
	}
	if *ip != "" {
		filters["ip"] = *ip
	}
	if *urlFilter != "" {
		filters["url"] = *urlFilter
	}
	if *asn != "" {
		filters["asn"] = *asn
	}
	if *asnName != "" {
		filters["asnname"] = *asnName
	}
	if *hash != "" {
		filters["hash"] = *hash
	}
	if *dateFilter != "" {
		filters["date"] = *dateFilter
	}
	if *filename != "" {
		filters["filename"] = *filename
	}

	// Apply preset if specified
	if *preset != "" {
		applyPreset(*preset, filters)
	}

	// Build query
	var baseQuery string
	if *customQuery != "" {
		baseQuery = *customQuery
	} else if presetQuery, ok := filters["preset-query"]; ok && presetQuery != "" {
		baseQuery = presetQuery
		// Combine with other filters if needed (excluding domain if preset already uses it)
		otherFilters := make(map[string]string)
		for k, v := range filters {
			if k != "preset-query" && k != "domain" && k != "all" {
				otherFilters[k] = v
			}
		}
		otherQuery := buildQuery(otherFilters, "")
		if otherQuery != "" {
			baseQuery = baseQuery + " AND " + otherQuery
		}
	} else {
		baseQuery = buildQuery(filters, "")
	}

	if baseQuery == "" && *customQuery == "" {
		fmt.Println("Usage: ./uscan -d <domain> [options]")
		fmt.Println("\nBasic filters:")
		fmt.Println("  -d string          Domain to search (ex: target.com)")
		fmt.Println("                     Default: uses page.domain: (exact page domain matches)")
		fmt.Println("  -all                Use domain: instead of page.domain: (includes third parties, CDNs, etc.)")
		fmt.Println("  -domain-only        Same as -all, uses domain: instead of page.domain:")
		fmt.Println("\nAdvanced filters:")
		fmt.Println("  -page-domain string Page domain filter")
		fmt.Println("  -ip string          IP address filter (ex: 1.2.3.4 or 1.2.3.0/24)")
		fmt.Println("  -url string         URL filter (ex: https://target.com/path)")
		fmt.Println("  -asn string         ASN filter (ex: AS24940 or 24940)")
		fmt.Println("  -asnname string     ASN name filter (ex: hetzner)")
		fmt.Println("  -hash string        SHA256 hash filter")
		fmt.Println("  -date string        Date filter (ex: >now-7d or [2020-01-01 TO 2020-02-01])")
		fmt.Println("  -filename string    Filename filter (ex: wp-content/uploads/)")
		fmt.Println("\nCustom query:")
		fmt.Println("  -q string           Custom query (overrides all other filters)")
		fmt.Println("\nPresets:")
		fmt.Println("  -preset string      Preset: phishing, recent, recent-month, recent-year, with-ip, fuzzy-domain")
		fmt.Println("\nPagination options:")
		fmt.Println("  -max int            Maximum number of results to return (0 = unlimited, default: 0)")
		fmt.Println("  -v                  Verbose output (show progress)")
		fmt.Println("  -token string       API token for urlscan.io (optional, defaults to URLSCAN env var)")
		fmt.Println("\nExamples:")
		fmt.Println("  ./uscan -d paypal.com")
		fmt.Println("  ./uscan -d paypal.com -preset phishing")
		fmt.Println("  ./uscan -ip 148.251.0.0/16 -date [2018 TO 2019]")
		fmt.Println("  ./uscan -q 'page.domain:(paypal.com~ AND NOT paypal.com)'")
		fmt.Println("  ./uscan -d target.com -max 500 -v")
		os.Exit(1)
	}

	// Use size 10000 as requested, but note that API may limit first page to 100 results
	// The API uses search_after (based on sort field) for pagination, not cursor
	size := "10000"
	baseURL := "https://urlscan.io/api/v1/search/?q=" + url.QueryEscape(baseQuery) + "&size=" + size
	searchAfter := "" // Will contain the sort value from last result
	totalResults := 0
	pageCount := 0
	seenURLs := make(map[string]bool) // Track seen URLs to avoid duplicates

	if *verbose {
		fmt.Fprintf(os.Stderr, "Query: %s\n", baseQuery)
		fmt.Fprintf(os.Stderr, "Full URL: %s\n", baseURL)
	}

	for {
		pageCount++
		reqURL := baseURL
		if searchAfter != "" {
			reqURL += "&search_after=" + url.QueryEscape(searchAfter)
		}

		if *verbose {
			fmt.Fprintf(os.Stderr, "Fetching page %d...\n", pageCount)
		}

		// Make request with retry logic for rate limiting
		var resp *http.Response
		var err error
		maxRetries := 3
		retryDelay := 2 * time.Second

		for attempt := 0; attempt < maxRetries; attempt++ {
			req, err := http.NewRequest("GET", reqURL, nil)
			if err != nil {
				if attempt < maxRetries-1 {
					if *verbose {
						fmt.Fprintf(os.Stderr, "Request creation failed, retrying in %v...\n", retryDelay)
					}
					time.Sleep(retryDelay)
					retryDelay *= 2
					continue
				}
				log.Fatalf("Error creating request after %d attempts: %v", maxRetries, err)
			}

			// Add API token if provided (from flag or environment variable)
			// urlscan.io uses API-Key header for authentication
			if apiToken != "" {
				req.Header.Set("API-Key", apiToken)
			}

			resp, err = http.DefaultClient.Do(req)
			if err != nil {
				if attempt < maxRetries-1 {
					if *verbose {
						fmt.Fprintf(os.Stderr, "Request failed, retrying in %v...\n", retryDelay)
					}
					time.Sleep(retryDelay)
					retryDelay *= 2 // Exponential backoff
					continue
				}
				log.Fatalf("Error making request after %d attempts: %v", maxRetries, err)
			}

			// Handle rate limiting (429 Too Many Requests)
			if resp.StatusCode == http.StatusTooManyRequests {
				resp.Body.Close()
				if attempt < maxRetries-1 {
					if *verbose {
						fmt.Fprintf(os.Stderr, "Rate limited, waiting %v before retry...\n", retryDelay)
					}
					time.Sleep(retryDelay)
					retryDelay *= 2
					continue
				}
				log.Fatalf("Rate limited. Please wait before trying again.")
			}

			if resp.StatusCode != http.StatusOK {
				body, _ := ioutil.ReadAll(resp.Body)
				resp.Body.Close()
				log.Fatalf("Error: status code %d. Response: %s", resp.StatusCode, string(body))
			}
			break
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

		resultsInPage := 0
		for _, result := range apiResp.Results {
			if result.Task.URL != "" {
				// Skip duplicates if we've seen this URL before
				if seenURLs[result.Task.URL] {
					continue
				}

				// If filtering by domain (-d) without -all or -domain-only flags, verify the URL actually matches the domain
				// This helps filter out false positives from the API when using page.domain:
				// When -all or -domain-only is used, we want all related domains (third parties, CDNs, etc.), so don't filter
				if *domain != "" && !*allFlag && !*domainOnly {
					parsedURL, err := url.Parse(result.Task.URL)
					if err == nil && parsedURL.Host != "" {
						// Check if the host matches the domain (exact or subdomain)
						host := strings.ToLower(parsedURL.Host)
						domainLower := strings.ToLower(*domain)

						// Remove port if present
						if idx := strings.Index(host, ":"); idx != -1 {
							host = host[:idx]
						}

						// Exact match or subdomain match (e.g., www.google.com matches google.com)
						if host != domainLower && !strings.HasSuffix(host, "."+domainLower) {
							// Doesn't match, skip this result
							if *verbose {
								fmt.Fprintf(os.Stderr, "Filtered out: %s (host: %s doesn't match domain: %s)\n", result.Task.URL, host, domainLower)
							}
							continue
						}
					}
				}

				seenURLs[result.Task.URL] = true

				// Check if we've reached the max results limit
				if *maxResults > 0 && totalResults >= *maxResults {
					if *verbose {
						fmt.Fprintf(os.Stderr, "Reached maximum results limit (%d)\n", *maxResults)
					}
					return
				}
				fmt.Println(result.Task.URL)
				totalResults++
				resultsInPage++
			}
		}

		if *verbose {
			fmt.Fprintf(os.Stderr, "Page %d: %d results (total: %d, HasMore: %v, Total in API: %d)\n",
				pageCount, resultsInPage, totalResults, apiResp.HasMore, apiResp.Total)
		}

		// Check if there are more results
		if !apiResp.HasMore {
			if *verbose {
				fmt.Fprintf(os.Stderr, "No more results available. Total: %d results (API reports total: %d)\n", totalResults, apiResp.Total)
			}
			break
		}

		// Get the sort value from the last result for search_after pagination
		if len(apiResp.Results) == 0 {
			if *verbose {
				fmt.Fprintf(os.Stderr, "No results in page, stopping pagination.\n")
			}
			break
		}

		// Use the sort field from the last result as search_after for next page
		// The sort field is an array [timestamp, uuid] that needs to be formatted as "timestamp,uuid"
		// API expects format: /^\d{13},[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i
		lastResult := apiResp.Results[len(apiResp.Results)-1]
		if lastResult.Sort != nil {
			// Parse the sort field - it can be an array or other format
			var timestamp, uuid string

			// Try to parse as array
			if sortArray, ok := lastResult.Sort.([]interface{}); ok && len(sortArray) >= 2 {
				// Extract timestamp (first element)
				switch v := sortArray[0].(type) {
				case float64:
					timestamp = fmt.Sprintf("%.0f", v)
				case int64:
					timestamp = fmt.Sprintf("%d", v)
				case int:
					timestamp = fmt.Sprintf("%d", v)
				case string:
					timestamp = v
				default:
					timestamp = fmt.Sprintf("%v", v)
				}

				// Extract UUID (second element) - remove quotes if present
				uuid = fmt.Sprintf("%v", sortArray[1])
				uuid = strings.Trim(uuid, "\"'")
			} else if sortStr, ok := lastResult.Sort.(string); ok {
				// If it's already a string, try to parse it
				// Format might be "timestamp,uuid" or JSON array string
				if strings.HasPrefix(sortStr, "[") {
					// It's a JSON array string, parse it
					var arr []interface{}
					if err := json.Unmarshal([]byte(sortStr), &arr); err == nil && len(arr) >= 2 {
						timestamp = fmt.Sprintf("%v", arr[0])
						uuid = fmt.Sprintf("%v", arr[1])
						uuid = strings.Trim(uuid, "\"'")
					} else {
						searchAfter = sortStr
					}
				} else if strings.Contains(sortStr, ",") {
					// Already in the correct format
					searchAfter = sortStr
				} else {
					searchAfter = sortStr
				}
			} else {
				// Fallback: try to marshal and parse
				sortBytes, err := json.Marshal(lastResult.Sort)
				if err == nil {
					var arr []interface{}
					if err := json.Unmarshal(sortBytes, &arr); err == nil && len(arr) >= 2 {
						timestamp = fmt.Sprintf("%v", arr[0])
						uuid = fmt.Sprintf("%v", arr[1])
						uuid = strings.Trim(uuid, "\"'")
					}
				}
			}

			// Format as "timestamp,uuid" if we have both values
			if timestamp != "" && uuid != "" && searchAfter == "" {
				// Remove any quotes from UUID
				uuid = strings.Trim(uuid, "\"'")
				searchAfter = fmt.Sprintf("%s,%s", timestamp, uuid)
			}

			if searchAfter == "" {
				if *verbose {
					fmt.Fprintf(os.Stderr, "ERROR: Could not parse sort field for search_after. Sort value: %v\n", lastResult.Sort)
				}
				break
			}

			if *verbose {
				fmt.Fprintf(os.Stderr, "Next page will use search_after: %s\n", searchAfter)
			}
		} else {
			// If no sort field, we can't continue pagination
			if *verbose {
				fmt.Fprintf(os.Stderr, "WARNING: Last result has no sort field. Cannot continue pagination.\n")
				fmt.Fprintf(os.Stderr, "Total collected: %d results (API reports total: %d)\n", totalResults, apiResp.Total)
			}
			break
		}

		// Small delay to avoid hitting rate limits
		time.Sleep(100 * time.Millisecond)
	}
}
