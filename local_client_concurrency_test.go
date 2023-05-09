package httpsteps //nolint:testpackage // This test extends internal implementation for better control, so it has to be internal.

import (
	"net/http"
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

func TestLocalClient_RegisterSteps_concurrencyNonBlocked(t *testing.T) {
	mock, srvURL := httpmock.NewServer()
	defer mock.Close()

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

	suite := godog.TestSuite{
		ScenarioInitializer: func(s *godog.ScenarioContext) {
			local.RegisterSteps(s)
		},
		Options: &godog.Options{
			Format:      "pretty",
			Strict:      true,
			Paths:       []string{"_testdata/LocalClientConcurrentNonBlocked.feature"},
			Concurrency: 10,
		},
	}

	if suite.Run() != 0 {
		t.Fatal("test failed")
	}
}
