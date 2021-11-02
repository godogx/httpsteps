package httpsteps

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/bool64/httpmock"
	"github.com/cucumber/godog"
)

func (l *LocalClient) isLocked(ctx context.Context, service string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	lock := l.locks[service]

	return lock != nil && lock != ctx.Value(ctxScenarioLockKey{}).(chan struct{})
}

func TestLocalClient_RegisterSteps_concurrency(t *testing.T) {
	mock, srvURL := httpmock.NewServer()

	local := NewLocalClient(srvURL)
	local.AddService("service-one", srvURL)
	local.AddService("service-two", srvURL)

	mock.ExpectAsync(httpmock.Expectation{
		Method:       http.MethodGet,
		RequestURI:   "/get-something?service=one",
		ResponseBody: []byte(`[{"service":"one"}]`),
		ResponseHeader: map[string]string{
			"Content-Type": "application/json",
		},
	})

	mock.ExpectAsync(httpmock.Expectation{
		Method:       http.MethodGet,
		RequestURI:   "/get-something?service=two",
		ResponseBody: []byte(`[{"service":"two"}]`),
		ResponseHeader: map[string]string{
			"Content-Type": "application/json",
		},
	})

	suite := godog.TestSuite{
		ScenarioInitializer: func(s *godog.ScenarioContext) {
			local.RegisterSteps(s)
			s.Step(`^I wait for another scenario to finish$`, func(ctx context.Context) {
				// local.
				time.Sleep(time.Second)
			})
			s.Step(`^I should not be blocked for "([^"]*)"$`, func(ctx context.Context, service string) error {
				if local.isLocked(ctx, service) {
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
