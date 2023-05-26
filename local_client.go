package httpsteps

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/bool64/httpmock"
	"github.com/bool64/shared"
	"github.com/cenkalti/backoff/v4"
	"github.com/cucumber/godog"
	"github.com/godogx/vars"
	"github.com/swaggest/assertjson/json5"
)

type sentinelError string

// Error returns the error message.
func (e sentinelError) Error() string {
	return string(e)
}

// NewLocalClient creates an instance of step-driven HTTP service.
func NewLocalClient(defaultBaseURL string, options ...func(*httpmock.Client)) *LocalClient {
	if defaultBaseURL != "" &&
		!strings.HasPrefix(defaultBaseURL, "http://") &&
		!strings.HasPrefix(defaultBaseURL, "https://") {
		defaultBaseURL = "http://" + defaultBaseURL
	}

	defaultBaseURL = strings.TrimRight(defaultBaseURL, "/")

	l := LocalClient{
		options: options,
	}

	l.AddService(Default, defaultBaseURL)

	return &l
}

// LocalClient is step-driven HTTP service for application local HTTP service.
type LocalClient struct {
	services map[string]*httpmock.Client
	options  []func(*httpmock.Client)

	// Deprecated: use VS.JSONComparer.Vars.
	Vars *shared.Vars

	VS *vars.Steps
}

// AddService registers a URL for named service.
func (l *LocalClient) AddService(name, baseURL string) {
	if l.services == nil {
		l.services = make(map[string]*httpmock.Client)
	}

	l.services[name] = l.makeClient(baseURL)
}

// RegisterSteps adds HTTP server steps to godog scenario context.
//
// # Request Setup
//
// Request configuration needs at least HTTP method and URI.
//
//	When I request HTTP endpoint with method "GET" and URI "/get-something?foo=bar"
//
// Configuration can be bound to a specific named service. This service must be registered before.
// service name should be added before `HTTP endpoint`.
//
//	And I request "some-service" HTTP endpoint with header "X-Foo: bar"
//
// An additional header can be supplied. For multiple headers, call step multiple times.
//
//	And I request HTTP endpoint with header "X-Foo: bar"
//
// An additional cookie can be supplied. For multiple cookie, call step multiple times.
//
//	And I request HTTP endpoint with cookie "name: value"
//
// Optionally request body can be configured. If body is a valid JSON5 payload, it will be converted to JSON before use.
// Otherwise, body is used as is.
//
//	And I request HTTP endpoint with body
//	"""
//	[
//	 // JSON5 comments are allowed.
//	 {"some":"json"}
//	]
//	"""
//
// Request body can be provided from file.
//
//	And I request HTTP endpoint with body from file
//	"""
//	path/to/file.json5
//	"""
//
// If endpoint is capable of handling duplicated requests, you can check it for idempotency. This would send multiple
// requests simultaneously and check
//   - if all responses are similar or (all successful like GET),
//   - if responses can be grouped into exactly ONE response of a kind
//     and OTHER responses of another kind (one successful, other failed like with POST).
//
// Number of requests can be configured with `LocalClient.ConcurrencyLevel`, default value is 10.
//
//	And I concurrently request idempotent HTTP endpoint
//
// # Response Expectations
//
// Response expectation has to be configured with at least one step about status, response body or other responses body
// (idempotency mode).
//
// If response body is a valid JSON5 payload, it is converted to JSON before use.
//
// JSON bodies are compared with https://github.com/swaggest/assertjson which allows ignoring differences
// when expected value is set to `"<ignore-diff>"`.
//
//	And I should have response with body
//	"""
//	[
//	 {"some":"json","time":"<ignore-diff>"}
//	]
//	"""
//
// Response body can be provided from file.
//
//	And I should have response with body from file
//	"""
//	path/to/file.json
//	"""
//
// Status can be defined with either phrase or numeric code. Also, you can set response header expectations.
//
//	Then I should have response with status "OK"
//	And I should have response with header "Content-Type: application/json"
//	And I should have response with header "X-Header: abc"
//
// In an idempotent mode you can set expectations for statuses of other responses.
//
//	Then I should have response with status "204"
//
//	And I should have other responses with status "Not Found"
//	And I should have other responses with header "Content-Type: application/json"
//
// And for bodies of other responses.
//
//	And I should have other responses with body
//	"""
//	{"status":"failed"}
//	"""
//
// Which can be defined as files.
//
//	And I should have other responses with body from file
//	"""
//	path/to/file.json
//	"""
//
// More information at https://github.com/godogx/httpsteps/#local-client.
func (l *LocalClient) RegisterSteps(s *godog.ScenarioContext) {
	s.Step(`^I request(.*) HTTP endpoint with method "([^"]*)" and URI (.*)$`, l.iRequestWithMethodAndURI)
	s.Step(`^I request(.*) HTTP endpoint with body$`, l.iRequestWithBody)
	s.Step(`^I request(.*) HTTP endpoint with body from file$`, l.iRequestWithBodyFromFile)
	s.Step(`^I request(.*) HTTP endpoint with header "([^"]*): ([^"]*)"$`, l.iRequestWithHeader)
	s.Step(`^I request(.*) HTTP endpoint with cookie "([^"]*): ([^"]*)"$`, l.iRequestWithCookie)

	s.Step(`^I request(.*) HTTP endpoint with cookies$`, l.iRequestWithCookies)
	s.Step(`^I request(.*) HTTP endpoint with headers$`, l.iRequestWithHeaders)
	s.Step(`^I request(.*) HTTP endpoint with query parameters$`, l.iRequestWithQueryParameters)
	s.Step(`^I request(.*) HTTP endpoint with urlencoded form data$`, l.iRequestWithFormDataParameters)

	s.Step(`^I follow redirects from(.*) HTTP endpoint$`, l.iFollowRedirects)
	s.Step(`^I retry(.*) HTTP request up to (\d+ time[s]?|.*)$`, l.iRetry)
	s.Step(`^I concurrently request idempotent(.*) HTTP endpoint$`, l.iRequestWithConcurrency)

	s.Step(`^I request(.*) HTTP endpoint with attachment as field "([^"]*)" and file name "([^"]*)"$`, l.iRequestWithAttachment)
	s.Step(`^I request(.*) HTTP endpoint with attachment as field "([^"]*)" from file$`, l.iRequestWithAttachmentFromFile)

	s.Step(`^I should have(.*) response with status "([^"]*)"$`, l.iShouldHaveResponseWithStatus)
	s.Step(`^I should have(.*) response with header "([^"]*): ([^"]*)"$`, l.iShouldHaveResponseWithHeader)
	s.Step(`^I should have(.*) response with headers$`, l.iShouldHaveResponseWithHeaders)

	s.Step(`^I should have(.*) response with body from file$`, l.iShouldHaveResponseWithBodyFromFile)
	s.Step(`^I should have(.*) response with body$`, l.iShouldHaveResponseWithBody)
	s.Step(`^I should have(.*) response with body, that matches JSON from file$`, l.iShouldHaveResponseWithBodyThatMatchesJSONFromFile)
	s.Step(`^I should have(.*) response with body, that matches JSON$`, l.iShouldHaveResponseWithBodyThatMatchesJSON)
	s.Step(`^I should have(.*) response with body, that matches JSON paths$`, l.iShouldHaveResponseWithBodyThatMatchesJSONPaths)

	s.Step(`^I should have(.*) other responses with status "([^"]*)"$`, l.iShouldHaveOtherResponsesWithStatus)
	s.Step(`^I should have(.*) other responses with header "([^"]*): ([^"]*)"$`, l.iShouldHaveOtherResponsesWithHeader)
	s.Step(`^I should have(.*) other responses with headers$`, l.iShouldHaveOtherResponsesWithHeaders)
	s.Step(`^I should have(.*) other responses with body$`, l.iShouldHaveOtherResponsesWithBody)
	s.Step(`^I should have(.*) other responses with body from file$`, l.iShouldHaveOtherResponsesWithBodyFromFile)
	s.Step(`^I should have(.*) other responses with body, that matches JSON$`, l.iShouldHaveOtherResponsesWithBodyThatMatchesJSON)
	s.Step(`^I should have(.*) other responses with body, that matches JSON from file$`, l.iShouldHaveOtherResponsesWithBodyThatMatchesJSONFromFile)
	s.Step(`^I should have(.*) other responses with body, that matches JSON paths$`, l.iShouldHaveOtherResponsesWithBodyThatMatchesJSONPaths)

	s.After(l.afterScenario)
}

func (l *LocalClient) afterScenario(ctx context.Context, _ *godog.Scenario, err error) (context.Context, error) {
	var errs []string

	if err != nil {
		errs = append(errs, err.Error())
	}

	for service := range l.services {
		client, _, err := l.Service(ctx, service)
		if err != nil {
			errs = append(errs, service+": "+err.Error())

			continue
		}

		if err := client.CheckUnexpectedOtherResponses(); err != nil {
			errs = append(errs, fmt.Sprintf("no other responses expected for %s: %s", service, err.Error()))
		}
	}

	if len(errs) > 0 {
		return ctx, errors.New(strings.Join(errs, "\n")) //nolint:goerr113
	}

	return ctx, nil
}

func (l *LocalClient) iRequestWithMethodAndURI(ctx context.Context, service, method, uri string) (context.Context, error) {
	c, ctx, err := l.Service(ctx, service)
	if err != nil {
		return ctx, err
	}

	if err := c.CheckUnexpectedOtherResponses(); err != nil {
		return ctx, fmt.Errorf("unexpected other responses for previous request: %w", err)
	}

	uri = strings.Trim(uri, `"`)

	ctx, rv, err := l.VS.Replace(ctx, []byte(uri))
	if err != nil {
		return ctx, fmt.Errorf("failed to replace vars in URI: %w", err)
	}

	c.Reset()
	c.WithMethod(method)
	c.WithURI(string(rv))

	return ctx, nil
}

// LoadBodyFromFile loads body from file and replaces vars in it.
//
// Deprecated: use github.com/godogx/vars.(*Steps).ReplaceFile.
func LoadBodyFromFile(filePath string, vars *shared.Vars) ([]byte, error) {
	body, err := ioutil.ReadFile(filePath) //nolint // File inclusion via variable during tests.
	if err != nil {
		return nil, err
	}

	return LoadBody(body, vars)
}

// LoadBody loads body from bytes and replaces vars in it.
//
// Deprecated: use github.com/godogx/vars.(*Steps).Replace.
func LoadBody(body []byte, vars *shared.Vars) ([]byte, error) {
	var err error

	if json5.Valid(body) {
		if body, err = json5.Downgrade(body); err != nil {
			return nil, fmt.Errorf("failed to downgrade JSON5 to JSON: %w", err)
		}
	}

	if vars != nil {
		varMap := vars.GetAll()
		varNames := make([]string, 0, len(varMap))
		varJV := make(map[string][]byte)

		for k, v := range vars.GetAll() {
			varNames = append(varNames, k)

			jv, err := json.Marshal(v)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal var %s (%v): %w", k, v, err)
			}

			varJV[k] = jv

			body = bytes.ReplaceAll(body, []byte(`"`+k+`"`), jv)
		}

		sort.Slice(varNames, func(i, j int) bool {
			return len(varNames[i]) > len(varNames[j])
		})

		for _, k := range varNames {
			jv := varJV[k]

			if jv[0] == '"' && jv[len(jv)-1] == '"' {
				jv = jv[1 : len(jv)-1]
			}

			body = bytes.ReplaceAll(body, []byte(k), jv)
		}
	}

	return body, nil
}

func (l *LocalClient) iRequestWithBodyFromFile(ctx context.Context, service string, filePath string) (context.Context, error) {
	c, ctx, err := l.Service(ctx, service)
	if err != nil {
		return ctx, err
	}

	ctx, body, err := l.VS.ReplaceFile(ctx, filePath)
	if err == nil {
		c.WithBody(body)
	}

	return ctx, err
}

func (l *LocalClient) iRequestWithBody(ctx context.Context, service string, bodyDoc string) (context.Context, error) {
	c, ctx, err := l.Service(ctx, service)
	if err != nil {
		return ctx, err
	}

	ctx, body, err := l.VS.Replace(ctx, []byte(bodyDoc))

	if err == nil {
		c.WithBody(body)
	}

	return ctx, err
}

func (l *LocalClient) iRequestWithHeader(ctx context.Context, service, key, value string) (context.Context, error) {
	c, ctx, err := l.Service(ctx, service)
	if err != nil {
		return ctx, err
	}

	ctx, rv, err := l.VS.Replace(ctx, []byte(value))
	if err != nil {
		return ctx, fmt.Errorf("failed to replace vars in header %s: %w", key, err)
	}

	c.WithHeader(key, string(rv))

	return ctx, nil
}

func mapOfData(data *godog.Table) (url.Values, error) {
	if len(data.Rows[0].Cells) != 2 {
		return nil, fmt.Errorf("%w, 2 expected, %d received",
			errInvalidNumberOfColumns, len(data.Rows[0].Cells))
	}

	res := make(url.Values, len(data.Rows))

	for _, r := range data.Rows {
		k := r.Cells[0].Value
		v := r.Cells[1].Value
		res[k] = append(res[k], v)
	}

	return res, nil
}

func (l *LocalClient) tableSetup(
	ctx context.Context,
	data *godog.Table,
	receiverName string,
	receiver func(name, value string) *httpmock.Client,
) (context.Context, error) {
	m, err := mapOfData(data)
	if err != nil {
		return ctx, err
	}

	var rv []byte

	for key, values := range m {
		for _, value := range values {
			ctx, rv, err = l.VS.Replace(ctx, []byte(value))
			if err != nil {
				return ctx, fmt.Errorf("failed to replace vars in %s %s: %w", receiverName, key, err)
			}

			receiver(key, string(rv))
		}
	}

	return ctx, nil
}

func (l *LocalClient) iRequestWithFormDataParameters(ctx context.Context, service string, data *godog.Table) (context.Context, error) {
	c, ctx, err := l.Service(ctx, service)
	if err != nil {
		return ctx, err
	}

	return l.tableSetup(ctx, data, "form data parameter", c.WithURLEncodedFormDataParam)
}

func (l *LocalClient) iRequestWithQueryParameters(ctx context.Context, service string, data *godog.Table) (context.Context, error) {
	c, ctx, err := l.Service(ctx, service)
	if err != nil {
		return ctx, err
	}

	return l.tableSetup(ctx, data, "query parameter", c.WithQueryParam)
}

func (l *LocalClient) iRequestWithHeaders(ctx context.Context, service string, data *godog.Table) (context.Context, error) {
	c, ctx, err := l.Service(ctx, service)
	if err != nil {
		return ctx, err
	}

	return l.tableSetup(ctx, data, "header", c.WithHeader)
}

func (l *LocalClient) iRequestWithCookie(ctx context.Context, service, name, value string) (context.Context, error) {
	c, ctx, err := l.Service(ctx, service)
	if err != nil {
		return ctx, err
	}

	ctx, rv, err := l.VS.Replace(ctx, []byte(value))
	if err != nil {
		return ctx, fmt.Errorf("failed to replace vars in cookie %s: %w", name, err)
	}

	c.WithCookie(name, string(rv))

	return ctx, nil
}

func (l *LocalClient) iRequestWithCookies(ctx context.Context, service string, data *godog.Table) (context.Context, error) {
	c, ctx, err := l.Service(ctx, service)
	if err != nil {
		return ctx, err
	}

	return l.tableSetup(ctx, data, "cookie", c.WithCookie)
}

func (l *LocalClient) iRequestWithAttachment(ctx context.Context, service, fieldName, fileName string, fileContent string) (context.Context, error) {
	c, ctx, err := l.Service(ctx, service)
	if err != nil {
		return ctx, err
	}

	ctx, body, contentType, err := l.appendAttachmentFileIntoBody(ctx, strings.NewReader(fileContent), fieldName, fileName)
	if err == nil {
		c.WithBody(body)
		c.WithContentType(contentType)
	}

	return ctx, err
}

func (l *LocalClient) iRequestWithAttachmentFromFile(ctx context.Context, service, fieldName string, filePath string) (context.Context, error) {
	c, ctx, err := l.Service(ctx, service)
	if err != nil {
		return ctx, err
	}

	file, err := os.Open(filePath) //nolint: gosec
	if err != nil {
		return ctx, err
	}
	defer file.Close() //nolint: gosec, errcheck

	ctx, body, contentType, err := l.appendAttachmentFileIntoBody(ctx, file, fieldName, filepath.Base(filePath))
	if err == nil {
		c.WithBody(body)
		c.WithContentType(contentType)
	}

	return ctx, err
}

func (l *LocalClient) appendAttachmentFileIntoBody(ctx context.Context, file io.Reader, fieldName, fileName string) (context.Context, []byte, string, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile(fieldName, fileName)
	if err != nil {
		return ctx, nil, "", err
	}

	_, err = io.Copy(part, file)
	if err != nil {
		return ctx, nil, "", err
	}

	err = writer.Close()
	if err != nil {
		return ctx, nil, "", err
	}

	ctx, resBody, err := l.VS.Replace(ctx, body.Bytes())
	if err != nil {
		return ctx, nil, "", err
	}

	return ctx, resBody, writer.FormDataContentType(), nil
}

const (
	// Default is the name of default service.
	Default = "default"

	errUnknownStatusCode      = sentinelError("unknown http status")
	errNoMockForService       = sentinelError("no mock for service")
	errUndefinedRequest       = sentinelError("undefined request (missing `receives <METHOD> request` step)")
	errUndefinedResponse      = sentinelError("undefined response (missing `responds with status <STATUS>` step)")
	errUnknownService         = sentinelError("unknown service")
	errUnexpectedExpectations = sentinelError("unexpected existing expectations")
	errInvalidNumberOfColumns = sentinelError("invalid number of columns")
	errUnexpectedBody         = sentinelError("unexpected body")
)

func statusCode(statusOrCode string) (int, error) {
	code, err := strconv.ParseInt(statusOrCode, 10, 64)
	if err != nil {
		code = int64(statusMap[statusOrCode])
	}

	if code == 0 {
		return 0, fmt.Errorf("%w: %q", errUnknownStatusCode, statusOrCode)
	}

	return int(code), nil
}

func (l *LocalClient) iShouldHaveOtherResponsesWithStatus(ctx context.Context, service, statusOrCode string) (context.Context, error) {
	c, ctx, err := l.Service(ctx, service)
	if err != nil {
		return ctx, err
	}

	code, err := statusCode(statusOrCode)
	if err != nil {
		return ctx, err
	}

	return ctx, c.ExpectOtherResponsesStatus(code)
}

func (l *LocalClient) iShouldHaveResponseWithStatus(ctx context.Context, service, statusOrCode string) (context.Context, error) {
	c, ctx, err := l.Service(ctx, service)
	if err != nil {
		return ctx, err
	}

	code, err := statusCode(statusOrCode)
	if err != nil {
		return ctx, err
	}

	return ctx, c.ExpectResponseStatus(code)
}

func (l *LocalClient) iShouldHaveOtherResponsesWithHeader(ctx context.Context, service, key, value string) (context.Context, error) {
	c, ctx, err := l.Service(ctx, service)
	if err != nil {
		return ctx, err
	}

	return ctx, c.ExpectOtherResponsesHeader(key, value)
}

func (l *LocalClient) iShouldHaveOtherResponsesWithHeaders(ctx context.Context, service string, data *godog.Table) (context.Context, error) {
	c, ctx, err := l.Service(ctx, service)
	if err != nil {
		return ctx, err
	}

	m, err := mapOfData(data)
	if err != nil {
		return ctx, err
	}

	for key, values := range m {
		for _, value := range values {
			if err := c.ExpectOtherResponsesHeader(key, value); err != nil {
				return ctx, fmt.Errorf("failed to assert response header %s: %w", key, err)
			}
		}
	}

	return ctx, nil
}

func (l *LocalClient) iShouldHaveResponseWithHeader(ctx context.Context, service, key, value string) (context.Context, error) {
	c, ctx, err := l.Service(ctx, service)
	if err != nil {
		return ctx, err
	}

	return ctx, c.ExpectResponseHeader(key, value)
}

func (l *LocalClient) iShouldHaveResponseWithHeaders(ctx context.Context, service string, data *godog.Table) (context.Context, error) {
	c, ctx, err := l.Service(ctx, service)
	if err != nil {
		return ctx, err
	}

	m, err := mapOfData(data)
	if err != nil {
		return ctx, err
	}

	for key, values := range m {
		for _, value := range values {
			if err := c.ExpectResponseHeader(key, value); err != nil {
				return ctx, fmt.Errorf("failed to assert response header %s: %w", key, err)
			}
		}
	}

	return ctx, nil
}

func (l *LocalClient) iShouldHaveResponseWithBody(ctx context.Context, service, bodyDoc string) (context.Context, error) {
	c, ctx, err := l.Service(ctx, service)
	if err != nil {
		return ctx, err
	}

	err = c.ExpectResponseBodyCallback(func(received []byte) error {
		ctx, err = augmentBodyErr(l.VS.Assert(ctx, []byte(bodyDoc), received, false))

		return err
	})

	return ctx, err
}

func (l *LocalClient) iShouldHaveResponseWithBodyFromFile(ctx context.Context, service, filePath string) (context.Context, error) {
	c, ctx, err := l.Service(ctx, service)
	if err != nil {
		return ctx, err
	}

	err = c.ExpectResponseBodyCallback(func(received []byte) error {
		ctx, err = augmentBodyErr(l.VS.AssertFile(ctx, filePath, received, false))

		return err
	})

	return ctx, err
}

func (l *LocalClient) iShouldHaveResponseWithBodyThatMatchesJSON(ctx context.Context, service, bodyDoc string) (context.Context, error) {
	c, ctx, err := l.Service(ctx, service)
	if err != nil {
		return ctx, err
	}

	err = c.ExpectResponseBodyCallback(func(received []byte) error {
		ctx, err = augmentBodyErr(l.VS.Assert(ctx, []byte(bodyDoc), received, true))

		return err
	})

	return ctx, err
}

func augmentBodyErr(ctx context.Context, err error) (context.Context, error) {
	if err != nil {
		return ctx, fmt.Errorf("%w %s", errUnexpectedBody, err.Error())
	}

	return ctx, nil
}

func (l *LocalClient) iShouldHaveResponseWithBodyThatMatchesJSONPaths(ctx context.Context, service string, jsonPaths *godog.Table) (context.Context, error) {
	c, ctx, err := l.Service(ctx, service)
	if err != nil {
		return ctx, err
	}

	err = c.ExpectResponseBodyCallback(func(received []byte) error {
		ctx, err = augmentBodyErr(l.VS.AssertJSONPaths(ctx, jsonPaths, received, true))

		return err
	})

	return ctx, err
}

func (l *LocalClient) iShouldHaveResponseWithBodyThatMatchesJSONFromFile(ctx context.Context, service, filePath string) (context.Context, error) {
	c, ctx, err := l.Service(ctx, service)
	if err != nil {
		return ctx, err
	}

	err = c.ExpectResponseBodyCallback(func(received []byte) error {
		ctx, err = augmentBodyErr(l.VS.AssertFile(ctx, filePath, received, true))

		return err
	})

	return ctx, err
}

func (l *LocalClient) iShouldHaveOtherResponsesWithBody(ctx context.Context, service, bodyDoc string) (context.Context, error) {
	c, ctx, err := l.Service(ctx, service)
	if err != nil {
		return ctx, err
	}

	err = c.ExpectOtherResponsesBodyCallback(func(received []byte) error {
		ctx, err = augmentBodyErr(l.VS.Assert(ctx, []byte(bodyDoc), received, false))

		return err
	})

	return ctx, err
}

func (l *LocalClient) iShouldHaveOtherResponsesWithBodyFromFile(ctx context.Context, service, filePath string) (context.Context, error) {
	c, ctx, err := l.Service(ctx, service)
	if err != nil {
		return ctx, err
	}

	err = c.ExpectOtherResponsesBodyCallback(func(received []byte) error {
		ctx, err = augmentBodyErr(l.VS.AssertFile(ctx, filePath, received, false))

		return err
	})

	return ctx, err
}

func (l *LocalClient) iShouldHaveOtherResponsesWithBodyThatMatchesJSON(ctx context.Context, service, bodyDoc string) (context.Context, error) {
	c, ctx, err := l.Service(ctx, service)
	if err != nil {
		return ctx, err
	}

	err = c.ExpectOtherResponsesBodyCallback(func(received []byte) error {
		ctx, err = augmentBodyErr(l.VS.Assert(ctx, []byte(bodyDoc), received, true))

		return err
	})

	return ctx, err
}

func (l *LocalClient) iShouldHaveOtherResponsesWithBodyThatMatchesJSONPaths(ctx context.Context, service string, jsonPaths *godog.Table) (context.Context, error) {
	c, ctx, err := l.Service(ctx, service)
	if err != nil {
		return ctx, err
	}

	err = c.ExpectOtherResponsesBodyCallback(func(received []byte) error {
		ctx, err = augmentBodyErr(l.VS.AssertJSONPaths(ctx, jsonPaths, received, true))

		return err
	})

	return ctx, err
}

func (l *LocalClient) iShouldHaveOtherResponsesWithBodyThatMatchesJSONFromFile(ctx context.Context, service, filePath string) (context.Context, error) {
	c, ctx, err := l.Service(ctx, service)
	if err != nil {
		return ctx, err
	}

	err = c.ExpectOtherResponsesBodyCallback(func(received []byte) error {
		ctx, err = augmentBodyErr(l.VS.AssertFile(ctx, filePath, received, false))

		return err
	})

	return ctx, err
}

func (l *LocalClient) iFollowRedirects(ctx context.Context, service string) (context.Context, error) {
	c, ctx, err := l.Service(ctx, service)
	if err != nil {
		return ctx, err
	}

	c.FollowRedirects()

	return ctx, nil
}

func (l *LocalClient) iRetry(ctx context.Context, service string, tries string) (context.Context, error) {
	c, ctx, err := l.Service(ctx, service)
	if err != nil {
		return ctx, err
	}

	tries = strings.TrimSuffix(strings.TrimSuffix(tries, " times"), " time")
	if maxTries, err := strconv.Atoi(tries); err == nil && maxTries > 0 {
		eb := backoff.NewExponentialBackOff()

		b := httpmock.RetryBackOffFunc(func() time.Duration {
			maxTries--

			if maxTries <= 0 {
				return backoff.Stop
			}

			return eb.NextBackOff()
		})

		c.AllowRetries(b)

		return ctx, nil
	}

	dur, err := time.ParseDuration(tries)
	if err != nil {
		return ctx, fmt.Errorf("parsing retry limit: %w", err)
	}

	eb := backoff.NewExponentialBackOff()
	eb.MaxElapsedTime = dur

	c.AllowRetries(eb)

	return ctx, nil
}

func (l *LocalClient) iRequestWithConcurrency(ctx context.Context, service string) (context.Context, error) {
	c, ctx, err := l.Service(ctx, service)
	if err != nil {
		return ctx, err
	}

	c.Concurrently()

	return ctx, nil
}

func (l *LocalClient) makeClient(baseURL string) *httpmock.Client {
	c := httpmock.NewClient(baseURL)

	for _, o := range l.options {
		o(c)
	}

	return c
}

// SetBaseURL sets the base URL for the client.
func (l *LocalClient) SetBaseURL(baseURL string, service string) error {
	if service == "" {
		service = Default
	}

	s, ok := l.services[service]
	if !ok {
		return fmt.Errorf("%w: %s", errUnknownService, service)
	}

	s.SetBaseURL(baseURL)

	return nil
}

// Service returns named service client or fails for undefined service.
func (l *LocalClient) Service(ctx context.Context, service string) (*httpmock.Client, context.Context, error) {
	service = strings.Trim(service, `" `)

	if service == "" {
		service = Default
	}

	c, found := l.services[service]
	if !found {
		return nil, ctx, fmt.Errorf("%w: %s", errUnknownService, service)
	}

	ctx, c = c.Fork(ctx)

	return c, ctx, nil
}

var statusMap = map[string]int{}

//nolint:gochecknoinits // Init is better than extra runtime complexity to lock the statuses.
func init() {
	for i := 100; i < 599; i++ {
		status := http.StatusText(i)
		if status != "" {
			statusMap[status] = i
		}
	}
}
