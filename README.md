# UScan

A lightweight CLI tool for retrieving URLs from URLScan.io for a specific domain.

## Description

UScan is a command-line utility that queries the URLScan.io API to fetch URLs related to a specified domain. It can be used to:

- Discover domains and URLs related to a target
- Perform reconnaissance for security assessments
- Find subdomains and related web properties
- Identify potential security risks by analyzing URL patterns

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

## Usage

```bash
# With go install
uscan -d example.com [-all]

# After building locally
./uscan -d example.com [-all]
```

### Parameters

- `-d`: Target domain (required)
- `-all`: Search for all URLs related to the domain, not just direct subdomains

### Examples

#### Basic search (only direct domain matches)

```bash
uscan -d twitter.com
```

Output:
```
https://twitter.com/login
https://twitter.com/i/flow/signup
https://twitter.com/home
...
```

#### Extended search (all related URLs)

```bash
uscan -d twitter.com -all
```

Output:
```
https://twitter.com/login
https://abs.twimg.com/responsive-web/client-web/bundle.download.Index.0ea0e22a.js
https://help.twitter.com/using-twitter
...
```

## How It Works

UScan leverages the URLScan.io API to retrieve URLs related to a specified domain:

1. Constructs a query URL for the URLScan.io API
2. Makes an HTTP request to fetch URL data
3. Parses the JSON response to extract URLs
4. Filters and displays the results based on the selected options

## Requirements

- Go 1.16 or higher

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
- For extensive scanning, consider obtaining an API key from URLScan.io
