package main

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
)

func main() {
	if len(os.Args) < 2 || os.Args[1] == "-h" {
		fmt.Println("Usage: curl2steps [service-name] <curl command>")
		return
	}

	serviceName := os.Args[1]
	offset := 2
	if serviceName == "curl" {
		serviceName = ""
		offset = 1
	}

	req, err := ParseCurl(os.Args[offset:])
	if err != nil {
		log.Fatal(err)
	}

	host := /*req.URL.Scheme + "://" + */ req.URL.Hostname()
	method := req.Method
	path := req.URL.Path
	if serviceName == "" {
		serviceName = host
	}

	fmt.Printf("    When I request %q HTTP endpoint with method %q and URI %q\n", serviceName, method, path)

	if len(req.Header) > 0 {
		fmt.Printf("    And I request %q HTTP endpoint with headers\n", serviceName)
		printTable(req.Header)
	}

	query := req.URL.Query()
	if len(query) > 0 {
		fmt.Printf("    And I request %q HTTP endpoint with query parameters\n", serviceName)
		printTable(query)
	}
}

func printTable(t map[string][]string) {
	keys := make([]string, 0, len(t))
	for k := range t {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		v := t[k]

		for _, vv := range v {
			fmt.Printf("        | %s | %s |\n", k, vv)
		}
	}
}

// ParseCurl takes a raw curl command and converts it into an *http.Request.
func ParseCurl(tokens []string) (*http.Request, error) {
	method := "GET"
	var rawURL string
	var body []byte
	headers := http.Header{}

	// Iterate over tokens and extract important parts.
	for i := 0; i < len(tokens); i++ {
		t := tokens[i]

		switch t {
		case "curl":
			continue

		case "-X", "--request":
			i++
			method = tokens[i]

		case "-H", "--header":
			i++
			h := tokens[i]
			parts := strings.SplitN(h, ":", 2)
			if len(parts) != 2 {
				return nil, errors.New("invalid header: " + h)
			}
			headers.Set(strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))

		case "-d", "--data", "--data-raw", "--data-binary":
			i++
			body = []byte(tokens[i])
			if method == "GET" {
				method = "POST"
			}

		default:
			// URL is usually the last non-flag argument
			if !strings.HasPrefix(t, "-") && rawURL == "" && strings.HasPrefix(t, "http") {
				rawURL = t
			}
		}
	}

	if rawURL == "" {
		return nil, errors.New("URL not found in curl command")
	}

	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}

	bodyReader := bytes.NewReader(body)

	req, err := http.NewRequest(method, parsedURL.String(), bodyReader)
	if err != nil {
		return nil, err
	}

	req.Header = headers
	return req, nil
}

//
// --- Tokenizer for curl-like shell commands ---
//

func shellTokens(s string) ([]string, error) {
	var out []string
	var buf strings.Builder
	inQuote := false
	quoteChar := byte(0)

	for i := 0; i < len(s); i++ {
		c := s[i]

		switch c {
		case ' ', '\t', '\n':
			if inQuote {
				buf.WriteByte(c)
			} else if buf.Len() > 0 {
				out = append(out, buf.String())
				buf.Reset()
			}
		case '\'', '"':
			if inQuote {
				if c == quoteChar {
					inQuote = false
				} else {
					buf.WriteByte(c)
				}
			} else {
				inQuote = true
				quoteChar = c
			}
		default:
			buf.WriteByte(c)
		}
	}

	if buf.Len() > 0 {
		out = append(out, buf.String())
	}

	return out, nil
}
