package httpsteps // nolint:testpackage // This test extends internal implementation for better control, so it has to be internal.

import (
	"context"
	"fmt"
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/bool64/httpmock"
	"github.com/cucumber/godog"
)

func TestLocalClient_RegisterSteps_concurrency(t *testing.T) {
	mock, srvURL := httpmock.NewServer()
	concurrency := 50

	local := NewLocalClient(srvURL, func(client *httpmock.Client) {
		client.ConcurrencyLevel = concurrency
	})
	local.AddService("service-one", srvURL)
	local.AddService("service-two", srvURL)

	mock.ExpectAsync(httpmock.Expectation{
		Method:       http.MethodGet,
		Repeated:     concurrency,
		RequestURI:   "/get-something?service=one",
		ResponseBody: []byte(`[{"service":"one"}]`),
		ResponseHeader: map[string]string{
			"Content-Type": "application/json",
		},
	})

	mock.ExpectAsync(httpmock.Expectation{
		Method:       http.MethodGet,
		Repeated:     concurrency,
		RequestURI:   "/get-something?service=two",
		ResponseBody: []byte(`[{"service":"two"}]`),
		ResponseHeader: map[string]string{
			"Content-Type": "application/json",
		},
	})

	suite := godog.TestSuite{
		ScenarioInitializer: func(s *godog.ScenarioContext) {
			local.RegisterSteps(s)
			s.Step(`^I should not be blocked for "([^"]*)"$`, func(ctx context.Context, service string) error {
				if local.lock.IsLocked(ctx, service) {
					return fmt.Errorf("%s is locked", service)
				}

				return nil
			})
		},
		Options: &godog.Options{
			Format:      "pretty",
			Strict:      true,
			Paths:       []string{"_testdata/LocalClientConcurrent.feature"},
			Concurrency: 10,
		},
	}

	if suite.Run() != 0 {
		t.Fatal("test failed")
	}
}

func TestLocalClient_RegisterSteps_concurrencyBlocked(t *testing.T) {
	mock, srvURL := httpmock.NewServer()
	concurrency := 50

	local := NewLocalClient(srvURL, func(client *httpmock.Client) {
		client.ConcurrencyLevel = concurrency
	})
	local.AddService("service-one", srvURL)
	local.AddService("service-two", srvURL)

	mock.ExpectAsync(httpmock.Expectation{
		Method:       http.MethodGet,
		Repeated:     2 * concurrency,
		RequestURI:   "/get-something?service=one",
		ResponseBody: []byte(`[{"service":"one"}]`),
		ResponseHeader: map[string]string{
			"Content-Type": "application/json",
		},
	})

	mock.ExpectAsync(httpmock.Expectation{
		Method:       http.MethodGet,
		Repeated:     2 * concurrency,
		RequestURI:   "/get-something?service=two",
		ResponseBody: []byte(`[{"service":"two"}]`),
		ResponseHeader: map[string]string{
			"Content-Type": "application/json",
		},
	})

	var running int64

	suite := godog.TestSuite{
		ScenarioInitializer: func(s *godog.ScenarioContext) {
			s.After(func(ctx context.Context, sc *godog.Scenario, err error) (context.Context, error) {
				atomic.AddInt64(&running, -1)

				return ctx, nil
			})

			local.RegisterSteps(s)
			s.Step(`^there is only one scenario running$`, func() error {
				if atomic.AddInt64(&running, 1) != 1 {
					return fmt.Errorf("%d scenarios running", atomic.LoadInt64(&running))
				}

				return nil
			})
		},
		Options: &godog.Options{
			Format:      "pretty",
			Strict:      true,
			Paths:       []string{"_testdata/LocalClientConcurrentBlocked.feature"},
			Concurrency: 10,
		},
	}

	if suite.Run() != 0 {
		t.Fatal("test failed")
	}
}
