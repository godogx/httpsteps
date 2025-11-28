package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
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

	req, err := parseCurl(os.Args[offset:])
	if err != nil {
		log.Fatal(err)
	}

	baseURL := req.URL.Scheme + "://" + req.URL.Hostname()
	method := req.Method
	path := req.URL.Path
	if serviceName == "" {
		serviceName = baseURL
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

	body, err := io.ReadAll(req.Body)
	if err != nil {
		log.Fatal(err)
	}

	if len(body) > 0 {
		if req.Header.Get("Content-Type") == "application/x-www-form-urlencoded" {
			form, err := url.ParseQuery(string(body))
			if err != nil {
				log.Fatal(err)
			}

			fmt.Printf("    And I request %q HTTP endpoint with urlencoded form data\n", serviceName)
			printTable(form)
		}

		if req.Header.Get("Content-Type") == "application/json" {
			fmt.Printf("    And I request %q HTTP endpoint with body\n", serviceName)
			fmt.Printf(fmt.Sprintf("    ```json\n    %s\n    ```\n", string(body)))
		}

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

// parseCurl takes a raw curl command and converts it into an *http.Request.
func parseCurl(tokens []string) (*http.Request, error) {
	var (
		method          = http.MethodGet
		rawURL          string
		body            []byte
		headers         = http.Header{}
		contentTypeHint = ""
	)

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
			if method == http.MethodGet {
				method = http.MethodPost
			}
			if contentTypeHint == "" {
				contentTypeHint = "application/x-www-form-urlencoded"
			}

		default:
			// URL is usually the last non-flag argument
			if !strings.HasPrefix(t, "-") && rawURL == "" && strings.HasPrefix(t, "http") {
				rawURL = t
			}
		}
	}

	if headers.Get("Content-Type") == "" && contentTypeHint != "" {
		headers.Set("Content-Type", contentTypeHint)
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
