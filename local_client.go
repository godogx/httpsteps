package httpdog

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"

	"github.com/bool64/httpmock"
	"github.com/bool64/shared"
	"github.com/cucumber/godog"
	"github.com/swaggest/assertjson/json5"
)

// NewLocalClient creates an instance of step-driven HTTP client.
func NewLocalClient(baseURL string) *LocalClient {
	if !strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://") {
		baseURL = "http://" + baseURL
	}

	baseURL = strings.TrimRight(baseURL, "/")

	l := LocalClient{
		Client: httpmock.NewClient(baseURL),
	}

	l.JSONComparer.Vars = &shared.Vars{}

	return &l
}

// LocalClient is step-driven HTTP client for application local HTTP service.
type LocalClient struct {
	*httpmock.Client
}

// RegisterSteps adds HTTP server steps to godog scenario context.
//
// Request Setup
//
// Request configuration needs at least HTTP method and URI.
//
//		When I request(.*) HTTP endpoint with method "GET" and URI "/get-something?foo=bar"
//
//
// An additional header can be supplied. For multiple headers, call step multiple times.
//
//		And I request(.*) HTTP endpoint with header "X-Foo: bar"
//
// An additional cookie can be supplied. For multiple cookie, call step multiple times.
//
//		And I request(.*) HTTP endpoint with cookie "name: value"
//
// Optionally request body can be configured. If body is a valid JSON5 payload, it will be converted to JSON before use.
// Otherwise, body is used as is.
//
//		And I request(.*) HTTP endpoint with body
//		"""
//		[
//		 // JSON5 comments are allowed.
//		 {"some":"json"}
//		]
//		"""
//
// Request body can be provided from file.
//
//		And I request(.*) HTTP endpoint with body from file
//		"""
//		path/to/file.json5
//		"""
//
// If endpoint is capable of handling duplicated requests, you can check it for idempotency. This would send multiple
// requests simultaneously and check
//   * if all responses are similar or (all successful like GET),
//   * if responses can be grouped into exactly ONE response of a kind
//     and OTHER responses of another kind (one successful, other failed like with POST).
//
// Number of requests can be configured with `LocalClient.ConcurrencyLevel`, default value is 10.
//
//		And I concurrently request idempotent HTTP endpoint
//
//
// Response Expectations
//
// Response expectation has to be configured with at least one step about status, response body or other responses body
// (idempotency mode).
//
// If response body is a valid JSON5 payload, it is converted to JSON before use.
//
// JSON bodies are compared with https://github.com/swaggest/assertjson which allows ignoring differences
// when expected value is set to `"<ignore-diff>"`.
//
//		And I should have response with body
//		"""
//		[
//		 {"some":"json","time":"<ignore-diff>"}
//		]
//		"""
//
// Response body can be provided from file.
//
//		And I should have response with body from file
//		"""
//		path/to/file.json
//		"""
//
// Status can be defined with either phrase or numeric code. Also you can set response header expectations.
//
//		Then I should have response with status "OK"
//		And I should have response with header "Content-Type: application/json"
//		And I should have response with header "X-Header: abc"
//
// In an idempotent mode you can set expectations for statuses of other responses.
//
//		Then I should have response with status "204"
//
//		And I should have other responses with status "Not Found"
//		And I should have other responses with header "Content-Type: application/json"
//
// And for bodies of other responses.
//
//		And I should have other responses with body
//		"""
//		{"status":"failed"}
//		"""
//
// Which can be defined as files.
//
//		And I should have other responses with body from file
//		"""
//		path/to/file.json
//		"""
func (l *LocalClient) RegisterSteps(s *godog.ScenarioContext) {
	s.Before(func(ctx context.Context, _ *godog.Scenario) (context.Context, error) {
		l.Reset()

		if l.JSONComparer.Vars != nil {
			l.JSONComparer.Vars.Reset()
		}

		return ctx, nil
	})

	s.After(func(ctx context.Context, sc *godog.Scenario, err error) (context.Context, error) {
		if err := l.CheckUnexpectedOtherResponses(); err != nil {
			err = fmt.Errorf("no other responses expected: %w", err)

			return ctx, err
		}

		return ctx, nil
	})

	s.Step(`^I request(.*) HTTP endpoint with method "([^"]*)" and URI (.*)$`, l.iRequestWithMethodAndURI)
	s.Step(`^I request(.*) HTTP endpoint with body$`, l.iRequestWithBody)
	s.Step(`^I request(.*) HTTP endpoint with body from file$`, l.iRequestWithBodyFromFile)
	s.Step(`^I request(.*) HTTP endpoint with header "([^"]*): ([^"]*)"$`, l.iRequestWithHeader)
	s.Step(`^I request(.*) HTTP endpoint with cookie "([^"]*): ([^"]*)"$`, l.iRequestWithCookie)

	s.Step(`^I concurrently request idempotent(.*) HTTP endpoint$`, l.iRequestWithConcurrency)

	s.Step(`^I should have(.*) response with status "([^"]*)"$`, l.iShouldHaveResponseWithStatus)
	s.Step(`^I should have(.*) response with header "([^"]*): ([^"]*)"$`, l.iShouldHaveResponseWithHeader)
	s.Step(`^I should have(.*) response with body from file$`, l.iShouldHaveResponseWithBodyFromFile)
	s.Step(`^I should have(.*) response with body$`, l.iShouldHaveResponseWithBody)

	s.Step(`^I should have(.*) other responses with status "([^"]*)"$`, l.iShouldHaveOtherResponsesWithStatus)
	s.Step(`^I should have(.*) other responses with header "([^"]*): ([^"]*)"$`, l.iShouldHaveOtherResponsesWithHeader)
	s.Step(`^I should have(.*) other responses with body$`, l.iShouldHaveOtherResponsesWithBody)
	s.Step(`^I should have(.*) other responses with body from file$`, l.iShouldHaveOtherResponsesWithBodyFromFile)
}

func (l *LocalClient) iRequestWithMethodAndURI(service, method, uri string) error {
	if err := l.CheckUnexpectedOtherResponses(); err != nil {
		return fmt.Errorf("unexpected other responses for previous request: %w", err)
	}

	uri = strings.Trim(uri, `"`)

	l.Reset()
	l.WithMethod(method)
	l.WithURI(uri)

	return nil
}

func loadBodyFromFile(filePath string, vars *shared.Vars) ([]byte, error) {
	body, err := ioutil.ReadFile(filePath) // nolint:gosec // File inclusion via variable during tests.
	if err != nil {
		return nil, err
	}

	return loadBody(body, vars)
}

func loadBody(body []byte, vars *shared.Vars) ([]byte, error) {
	var err error

	if json5.Valid(body) {
		if body, err = json5.Downgrade(body); err != nil {
			return nil, fmt.Errorf("failed to downgrade JSON5 to JSON: %w", err)
		}
	}

	if vars != nil {
		for k, v := range vars.GetAll() {
			jv, err := json.Marshal(v)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal var %s (%v): %w", k, v, err)
			}

			body = bytes.ReplaceAll(body, []byte(`"`+k+`"`), jv)
		}
	}

	return body, nil
}

func (l *LocalClient) iRequestWithBodyFromFile(service string, filePath string) error {
	body, err := loadBodyFromFile(filePath, l.JSONComparer.Vars)

	if err == nil {
		l.WithBody(body)
	}

	return err
}

func (l *LocalClient) iRequestWithBody(service string, bodyDoc string) error {
	body, err := loadBody([]byte(bodyDoc), l.JSONComparer.Vars)

	if err == nil {
		l.WithBody(body)
	}

	return err
}

func (l *LocalClient) iRequestWithHeader(service, key, value string) error {
	l.WithHeader(key, value)

	return nil
}

func (l *LocalClient) iRequestWithCookie(service, name, value string) error {
	l.WithCookie(name, value)

	return nil
}

var (
	errUnknownStatusCode = errors.New("unknown http status")
	errNoMockForService  = errors.New("no mock for service")
	errUndefinedRequest  = errors.New("undefined request (missing `receives <METHOD> request` step)")
	errUndefinedResponse = errors.New("undefined response (missing `responds with status <STATUS>` step)")
)

func statusCode(statusOrCode string) (int, error) {
	code, err := strconv.ParseInt(statusOrCode, 10, 64)

	if len(statusMap) == 0 {
		initStatusMap()
	}

	if err != nil {
		code = int64(statusMap[statusOrCode])
	}

	if code == 0 {
		return 0, fmt.Errorf("%w: %q", errUnknownStatusCode, statusOrCode)
	}

	return int(code), nil
}

func (l *LocalClient) iShouldHaveOtherResponsesWithStatus(service, statusOrCode string) error {
	code, err := statusCode(statusOrCode)
	if err != nil {
		return err
	}

	return l.ExpectOtherResponsesStatus(code)
}

func (l *LocalClient) iShouldHaveResponseWithStatus(service, statusOrCode string) error {
	code, err := statusCode(statusOrCode)
	if err != nil {
		return err
	}

	return l.ExpectResponseStatus(code)
}

func (l *LocalClient) iShouldHaveOtherResponsesWithHeader(service, key, value string) error {
	return l.ExpectOtherResponsesHeader(key, value)
}

func (l *LocalClient) iShouldHaveResponseWithHeader(service, key, value string) error {
	return l.ExpectResponseHeader(key, value)
}

func (l *LocalClient) iShouldHaveResponseWithBody(service, bodyDoc string) error {
	body, err := loadBody([]byte(bodyDoc), l.JSONComparer.Vars)
	if err != nil {
		return err
	}

	return l.ExpectResponseBody(body)
}

func (l *LocalClient) iShouldHaveResponseWithBodyFromFile(service, filePath string) error {
	body, err := loadBodyFromFile(filePath, l.JSONComparer.Vars)
	if err != nil {
		return err
	}

	return l.ExpectResponseBody(body)
}

func (l *LocalClient) iShouldHaveOtherResponsesWithBody(service, bodyDoc string) error {
	body, err := loadBody([]byte(bodyDoc), l.JSONComparer.Vars)
	if err != nil {
		return err
	}

	return l.ExpectOtherResponsesBody(body)
}

func (l *LocalClient) iShouldHaveOtherResponsesWithBodyFromFile(service, filePath string) error {
	body, err := loadBodyFromFile(filePath, l.JSONComparer.Vars)
	if err != nil {
		return err
	}

	return l.ExpectOtherResponsesBody(body)
}

func (l *LocalClient) iRequestWithConcurrency(service string) error {
	l.Concurrently()

	return nil
}

var statusMap = map[string]int{}

func initStatusMap() {
	for i := 100; i < 599; i++ {
		status := http.StatusText(i)
		if status != "" {
			statusMap[status] = i
		}
	}
}
