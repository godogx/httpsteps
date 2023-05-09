package httpsteps

import (
	"context"
	"fmt"
	"strings"

	"github.com/bool64/httpmock"
	"github.com/bool64/shared"
	"github.com/cucumber/godog"
	"github.com/godogx/resource"
	"github.com/godogx/vars"
)

type exp struct {
	httpmock.Expectation
	async bool
}

// NewExternalServer creates an ExternalServer.
func NewExternalServer() *ExternalServer {
	es := &ExternalServer{}
	es.mocks = make(map[string]*mock, 1)
	es.lock = resource.NewLock(func(service string) error {
		m := es.mocks[service]
		if m == nil {
			return fmt.Errorf("%w: %s", errNoMockForService, service)
		}

		if m.exp != nil {
			return fmt.Errorf("%w in %s for %s %s",
				errUndefinedResponse, service, m.exp.Method, m.exp.RequestURI)
		}

		if err := m.srv.ExpectationsWereMet(); err != nil {
			return fmt.Errorf("expectations were not met for %s: %w", service, err)
		}

		return nil
	})

	return es
}

// ExternalServer is a collection of step-driven HTTP servers to serve requests of application with mocked data.
//
// Please use NewExternalServer() to create an instance.
type ExternalServer struct {
	mocks map[string]*mock
	lock  *resource.Lock

	// Deprecated: use VS.JSONComparer.Vars to seed initial values if necessary.
	Vars *shared.Vars

	VS *vars.Steps
}

type mock struct {
	exp *exp
	srv *httpmock.Server
}

// RegisterSteps adds steps to godog scenario context to serve outgoing requests with mocked data.
//
// In simple case you can define expected URL and response.
//
//	Given "some-service" receives "GET" request "/get-something?foo=bar"
//
//	And "some-service" responds with status "OK" and body
//	"""
//	{"key":"value"}
//	"""
//
// Or request with body.
//
//	And "another-service" receives "POST" request "/post-something" with body
//	"""
//	// Could be a JSON5 too.
//	{"foo":"bar"}
//	"""
//
// Request with body from a file.
//
//	And "another-service" receives "POST" request "/post-something" with body from file
//	"""
//	_testdata/sample.json
//	"""
//
// Request can expect to have a header.
//
//	And "some-service" request includes header "X-Foo: bar"
//
// By default, each configured request is expected to be received 1 time. This can be changed to a different number.
//
//	And "some-service" request is received 1234 times
//
// Or to be unlimited.
//
//	And "some-service" request is received several times
//
// By default, requests are expected in same sequential order as they are defined.
// If there is no stable order you can have an async expectation.
// Async requests are expected in any order.
//
//	And "some-service" request is async
//
// Response may have a header.
//
//	And "some-service" response includes header "X-Bar: foo"
//
// Response must have a status.
//
//	And "some-service" responds with status "OK"
//
// Response may also have a body.
//
//	And "some-service" responds with status "OK" and body
//	"""
//	{"key":"value"}
//	"""
//
// Response body can also be defined in file.
//
//	And "another-service" responds with status "200" and body from file
//	"""
//	_testdata/sample.json5
//	"""
func (e *ExternalServer) RegisterSteps(s *godog.ScenarioContext) {
	e.lock.Register(s)
	e.steps(s)
}

func (e *ExternalServer) steps(s *godog.ScenarioContext) {
	// Init request expectation.
	s.Step(`^"([^"]*)" receives "([^"]*)" request "([^"]*)"$`,
		e.serviceReceivesRequest)
	s.Step(`^"([^"]*)" receives "([^"]*)" request "([^"]*)" with body$`,
		e.serviceReceivesRequestWithBody)
	s.Step(`^"([^"]*)" receives "([^"]*)" request "([^"]*)" with body from file$`,
		e.serviceReceivesRequestWithBodyFromFile)

	// Configure request expectation.
	s.Step(`^"([^"]*)" request includes header "([^"]*): ([^"]*)"$`,
		e.serviceRequestIncludesHeader)
	s.Step(`^"([^"]*)" request is async$`,
		e.serviceRequestIsAsync)
	s.Step(`^"([^"]*)" request is received several times$`,
		e.serviceReceivesRequestMultipleTimes)
	s.Step(`^"([^"]*)" request is received (\d+) times$`,
		e.serviceReceivesRequestNTimes)

	// Configure response.
	s.Step(`^"([^"]*)" response includes header "([^"]*): ([^"]*)"$`,
		e.serviceResponseIncludesHeader)

	// Finalize request expectation.
	s.Step(`^"([^"]*)" responds with status "([^"]*)"$`,
		func(ctx context.Context, service, statusOrCode string) (context.Context, error) {
			return e.serviceRespondsWithStatusAndPreparedBody(ctx, service, statusOrCode, nil)
		})
	s.Step(`^"([^"]*)" responds with status "([^"]*)" and body$`,
		e.serviceRespondsWithStatusAndBody)
	s.Step(`^"([^"]*)" responds with status "([^"]*)" and body from file$`,
		e.serviceRespondsWithStatusAndBodyFromFile)
}

// GetMock exposes mock of external service for configuration.
func (e *ExternalServer) GetMock(service string) *httpmock.Server {
	return e.mocks[service].srv
}

func (e *ExternalServer) pending(ctx context.Context, service string) (context.Context, *mock, error) {
	ctx, m, err := e.mock(ctx, service)
	if err != nil {
		return ctx, nil, err
	}

	if m.exp == nil {
		return ctx, nil, fmt.Errorf("%w: %q", errUndefinedRequest, service)
	}

	return ctx, m, nil
}

// mock returns mock for a service or fails if service is not defined.
func (e *ExternalServer) mock(ctx context.Context, service string) (context.Context, *mock, error) {
	service = strings.Trim(service, `" `)

	if service == "" {
		service = Default
	}

	c, found := e.mocks[service]
	if !found {
		return ctx, nil, fmt.Errorf("%w: %s", errUnknownService, service)
	}

	acquired, err := e.lock.Acquire(ctx, service)
	if err != nil {
		return ctx, nil, err
	}

	// Reset client after acquiring lock.
	if acquired {
		c.exp = nil
		c.srv.ResetExpectations()
	}

	return ctx, c, nil
}

// Add starts a mocked server for a named service and returns url.
func (e *ExternalServer) Add(service string, options ...func(mock *httpmock.Server)) string {
	m, url := httpmock.NewServer()

	for _, option := range options {
		option(m)
	}

	e.mocks[service] = &mock{srv: m}

	return url
}

func (e *ExternalServer) serviceReceivesRequestWithPreparedBody(ctx context.Context, service, method, requestURI string, body []byte) (context.Context, error) {
	ctx, err := e.serviceReceivesRequest(ctx, service, method, requestURI)
	if err != nil {
		return ctx, err
	}

	ctx, m, err := e.pending(ctx, service)
	if err != nil {
		return ctx, err
	}

	m.exp.RequestBody = body

	return ctx, nil
}

func (e *ExternalServer) serviceRequestIncludesHeader(ctx context.Context, service, header, value string) (context.Context, error) {
	ctx, m, err := e.pending(ctx, service)
	if err != nil {
		return ctx, err
	}

	if m.exp.RequestHeader == nil {
		m.exp.RequestHeader = make(map[string]string, 1)
	}

	m.exp.RequestHeader[header] = value

	return ctx, nil
}

func (e *ExternalServer) serviceReceivesRequestWithBody(ctx context.Context, service, method, requestURI string, bodyDoc string) (context.Context, error) {
	ctx, body, err := e.VS.Replace(ctx, []byte(bodyDoc))
	if err != nil {
		return ctx, err
	}

	return e.serviceReceivesRequestWithPreparedBody(ctx, service, method, requestURI, body)
}

func (e *ExternalServer) serviceReceivesRequestWithBodyFromFile(ctx context.Context, service, method, requestURI string, filePath string) (context.Context, error) {
	ctx, body, err := e.VS.ReplaceFile(ctx, filePath)
	if err != nil {
		return ctx, err
	}

	return e.serviceReceivesRequestWithPreparedBody(ctx, service, method, requestURI, body)
}

func (e *ExternalServer) serviceReceivesRequest(ctx context.Context, service, method, requestURI string) (context.Context, error) {
	ctx, m, err := e.mock(ctx, service)
	if err != nil {
		return ctx, err
	}

	if m.exp != nil {
		return ctx, fmt.Errorf("%w for %q: %+v", errUnexpectedExpectations, service, *m.exp)
	}

	m.exp = &exp{}
	m.exp.Method = method
	m.exp.RequestURI = requestURI

	return ctx, nil
}

func (e *ExternalServer) serviceReceivesRequestNTimes(ctx context.Context, service string, n int) (context.Context, error) {
	ctx, m, err := e.pending(ctx, service)
	if err != nil {
		return ctx, err
	}

	m.exp.Repeated = n

	return ctx, nil
}

func (e *ExternalServer) serviceRequestIsAsync(ctx context.Context, service string) (context.Context, error) {
	ctx, m, err := e.pending(ctx, service)
	if err != nil {
		return ctx, err
	}

	m.exp.async = true

	return ctx, nil
}

func (e *ExternalServer) serviceReceivesRequestMultipleTimes(ctx context.Context, service string) (context.Context, error) {
	ctx, m, err := e.pending(ctx, service)
	if err != nil {
		return ctx, err
	}

	m.exp.Unlimited = true

	return ctx, nil
}

func (e *ExternalServer) serviceRespondsWithStatusAndPreparedBody(ctx context.Context, service, statusOrCode string, body []byte) (context.Context, error) {
	code, err := statusCode(statusOrCode)
	if err != nil {
		return ctx, err
	}

	ctx, m, err := e.pending(ctx, service)
	if err != nil {
		return ctx, err
	}

	pending := *m.exp
	m.exp = nil

	pending.Status = code
	pending.ResponseBody = body

	if pending.ResponseHeader == nil {
		pending.ResponseHeader = map[string]string{}
	}

	if pending.async {
		m.srv.ExpectAsync(pending.Expectation)
	} else {
		m.srv.Expect(pending.Expectation)
	}

	return ctx, nil
}

func (e *ExternalServer) serviceResponseIncludesHeader(ctx context.Context, service, header, value string) (context.Context, error) {
	ctx, m, err := e.pending(ctx, service)
	if err != nil {
		return ctx, err
	}

	if m.exp.ResponseHeader == nil {
		m.exp.ResponseHeader = make(map[string]string, 1)
	}

	m.exp.ResponseHeader[header] = value

	return ctx, nil
}

func (e *ExternalServer) serviceRespondsWithStatusAndBody(ctx context.Context, service, statusOrCode string, bodyDoc string) (context.Context, error) {
	ctx, body, err := e.VS.Replace(ctx, []byte(bodyDoc))
	if err != nil {
		return ctx, err
	}

	return e.serviceRespondsWithStatusAndPreparedBody(ctx, service, statusOrCode, body)
}

func (e *ExternalServer) serviceRespondsWithStatusAndBodyFromFile(ctx context.Context, service, statusOrCode string, filePath string) (context.Context, error) {
	ctx, body, err := e.VS.ReplaceFile(ctx, filePath)
	if err != nil {
		return ctx, err
	}

	return e.serviceRespondsWithStatusAndPreparedBody(ctx, service, statusOrCode, body)
}
