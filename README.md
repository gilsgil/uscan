# UScan

A lightweight CLI tool for retrieving URLs from URLScan.io for a specific domain or using advanced search filters.

## Description

UScan is a command-line utility that queries the URLScan.io Search API to fetch URLs based on various filters and criteria. It can be used to:

- Discover domains and URLs related to a target
- Perform reconnaissance for security assessments
- Find subdomains and related web properties
- Identify potential security risks by analyzing URL patterns
- Search using advanced filters (IP, ASN, date ranges, hashes, etc.)
- Use custom ElasticSearch queries for complex searches

## Installation

### Option 1: Go Install

If you have Go installed, you can install directly using:

```bash
go install github.com/gilsgil/uscan@latest
```

### Option 2: Clone and Build

```bash
git clone https://github.com/gilsgil/uscan.git
cd uscan
go build
```

## API Setup

UScan uses the URLScan.io Search API. For better rate limits and access to more results, it's recommended to use an API token.

### Getting an API Token

1. Sign up for a free account at [urlscan.io](https://urlscan.io)
2. Go to your account settings and generate an API key
3. Set it as an environment variable:

```bash
export URLSCAN=your-api-token-here
```

Or on Windows:

```powershell
$env:URLSCAN="your-api-token-here"
```

**Note:** The API token is optional but recommended. Without it, you may be limited to 100 results per query. With a free account, you can get up to 1,000 results, and with paid plans, up to 10,000 results per query.

## Usage

```bash
# Basic usage with domain filter
uscan -d example.com

# With API token from environment variable (automatic)
uscan -d example.com

# With explicit API token
uscan -d example.com -token your-api-token

# With verbose output to see pagination progress
uscan -d example.com -v
```

### Parameters

#### Basic Filters

- `-d string`: Target domain to search (ex: `target.com`)
  - **Default behavior (without `-all`)**: Searches `page.domain:` for exact page domain matches
    - Returns only URLs from the specified domain and its subdomains
    - Includes local filtering to ensure only matching domains are shown
    - Example: `-d google.com` returns only `google.com`, `www.google.com`, `mail.google.com`, etc.
  
- `-all`: Include all contacted domains (third parties, CDNs, external resources)
  - When used with `-d`, searches `domain:` instead of `page.domain:`
  - Returns URLs from all domains that were contacted during scans of the target domain
  - Useful for finding third-party services, CDNs, analytics, and external resources
  - Example: `-d google.com -all` returns URLs from Google domains plus all third-party domains contacted

#### Advanced Filters

- `-page-domain string`: Filter by page domain (ex: `target.com`)
- `-ip string`: Filter by IP address (ex: `1.2.3.4` or `1.2.3.0/24` for CIDR)
- `-url string`: Filter by URL pattern (ex: `https://target.com/path`)
- `-asn string`: Filter by ASN (ex: `AS24940` or `24940`)
- `-asnname string`: Filter by ASN name (ex: `hetzner`)
- `-hash string`: Filter by SHA256 hash
- `-date string`: Filter by date range (ex: `>now-7d` or `[2020-01-01 TO 2020-02-01]`)
- `-filename string`: Filter by filename pattern (ex: `wp-content/uploads/`)

#### Custom Query

- `-q string`: Custom ElasticSearch query (overrides all other filters)
  - Allows full control over the search query using URLScan.io's ElasticSearch Query String syntax
  - Example: `-q 'page.domain:(paypal.com~ AND NOT paypal.com)'`

#### Presets

- `-preset string`: Use a predefined filter preset:
  - `phishing`: Find domains that were contacted but aren't the page/primary domain (useful for detecting phishing)
  - `recent`: Scans from the past 7 days
  - `recent-month`: Scans from the past month
  - `recent-year`: Scans from the past year
  - `with-ip`: Scans with non-empty IP addresses
  - `fuzzy-domain`: Fuzzy search for domain (excluding exact match)

#### Pagination Options

- `-max int`: Maximum number of results to return (0 = unlimited, default: 0)
- `-v`: Verbose output (shows pagination progress, page counts, and API responses)

#### Authentication

- `-token string`: API token for urlscan.io (optional, defaults to `URLSCAN` environment variable)

## Examples

### Basic Domain Search

```bash
# Search for exact page domain matches only (default)
# Returns only URLs from twitter.com and its subdomains
uscan -d twitter.com

# Include all contacted domains (third parties, CDNs, etc.)
# Returns URLs from twitter.com plus all domains contacted during scans
uscan -d twitter.com -all
```

**Difference between `-d` and `-d -all`:**
- `-d domain.com`: Exact match - only URLs where the page domain is `domain.com` or its subdomains
- `-d domain.com -all`: Broad search - all domains that were contacted during scans of `domain.com` (useful for finding third-party services, CDNs, analytics, etc.)

### Advanced Filters

```bash
# Search by IP address with date range
uscan -ip 148.251.0.0/16 -date "[2018 TO 2019]"

# Search by ASN
uscan -asn AS24940

# Search by ASN name
uscan -asnname hetzner

# Search by URL pattern
uscan -url "https://target.com/path"

# Search by date (last 7 days)
uscan -d target.com -date ">now-7d"
```

### Presets

```bash
# Find potential phishing domains
uscan -d paypal.com -preset phishing

# Find recent scans
uscan -d target.com -preset recent

# Fuzzy domain search (typosquatting detection)
uscan -d google.com -preset fuzzy-domain
```

### Custom Queries

```bash
# Custom ElasticSearch query
uscan -q 'page.domain:(paypal.com~ AND NOT paypal.com)'

# Complex query with multiple conditions
uscan -q 'domain:paypal.com AND NOT page.domain:paypal.com AND date:>now-7d'

# Regex search
uscan -q 'page.domain:(/payp.*/ AND NOT paypal.com)'
```

### Combining Filters

```bash
# Combine multiple filters
uscan -d target.com -date ">now-1M" -ip "1.2.3.4"

# Limit results
uscan -d target.com -max 500

# Verbose output to see pagination
uscan -d target.com -v
```

## How It Works

UScan leverages the URLScan.io Search API to retrieve URLs based on various criteria:

1. **Query Construction**: Builds an ElasticSearch Query String based on provided filters
   - `-d domain.com` uses `page.domain:domain.com` for exact matches
   - `-d domain.com -all` uses `domain:domain.com` for all contacted domains
2. **API Requests**: Makes HTTP requests to the URLScan.io Search API endpoint with proper authentication
3. **Pagination**: Automatically handles pagination using `search_after` parameter based on the `sort` field from results
   - Formats `search_after` correctly as `timestamp,uuid` (not JSON array format)
   - Continues fetching pages until all results are retrieved
4. **Result Processing**: Parses JSON responses and extracts URLs
5. **Local Filtering**: When using `-d` without `-all`, applies additional filtering to ensure only matching domains are shown
   - Validates that URLs actually belong to the specified domain
   - Filters out false positives from the API
6. **Deduplication**: Filters duplicate URLs when pagination restarts or when switching page sizes

### Pagination Details

- The API uses `search_after` for pagination (not cursor-based)
- Each result contains a `sort` field (array `[timestamp, uuid]`) used for pagination
- The tool formats `search_after` correctly as `timestamp,uuid` (not JSON array format)
- The tool automatically continues fetching pages until all results are retrieved
- With `size=10000`, the API may limit the first page to 100 results; the tool automatically switches to `size=100` for proper pagination
- Handles rate limiting (HTTP 429) with exponential backoff retry
- Includes delays between requests to avoid hitting API limits

## Search API Reference

UScan uses the URLScan.io Search API which supports ElasticSearch Query String syntax. Key features:

- **Field Names**: Always use field names (e.g., `page.domain`, `page.ip`, `domain`)
- **Operators**: Support for `AND`, `OR`, `NOT`, and grouping with `()`
- **Wildcards**: Leading wildcards and regex on supported fields (requires authentication)
- **Date Queries**: Relative queries like `date:>now-7d` or ranges like `date:[2020-01-01 TO 2020-02-01]`
- **Escaping**: Special characters must be escaped with backslash: `+ - = && || > < ! ( ) { } [ ] ^ " ~ * ? : \ /`

For more details, see the [URLScan.io Search API Documentation](https://docs.urlscan.io/pages/search-api-reference).

## Requirements

- Go 1.16 or higher
- Internet connection for API access
- (Optional) URLScan.io API token for better rate limits and more results

## Rate Limits

The URLScan.io API has rate limits that vary by account type:

- **Anonymous users**: 100 results per query
- **Free users**: 1,000 results per query
- **Paid users**: 10,000 results per query

UScan automatically handles:
- Rate limiting (HTTP 429) with exponential backoff retry
- Delays between requests to avoid hitting limits
- Proper pagination to retrieve all available results

## License

This project is open source and available under the [MIT License](LICENSE).

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## Notes

- This tool uses the public URLScan.io API, which may have rate limits
- For extensive scanning, consider obtaining an API key from URLScan.io and setting it as `URLSCAN` environment variable
- The tool automatically handles pagination and deduplication
- Use `-v` flag to see detailed progress information, including:
  - Query being sent to the API
  - Pagination progress (page count, results per page)
  - Filtered URLs (when using `-d` without `-all`)
  - API responses and errors
- Custom queries (`-q`) give you full control over the search but require knowledge of ElasticSearch Query String syntax
- **Domain Filtering Behavior:**
  - `-d domain.com`: Returns only URLs from the exact domain and subdomains (with local filtering)
  - `-d domain.com -all`: Returns all domains contacted during scans (no local filtering, includes third parties)
  - Local filtering ensures accuracy when using exact domain search
