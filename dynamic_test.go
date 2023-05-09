package httpsteps_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/bool64/httpmock"
	"github.com/cucumber/godog"
	"github.com/godogx/httpsteps"
	"github.com/godogx/vars"
	"github.com/stretchr/testify/assert"
)

func TestDynamic(t *testing.T) {
	sm, u := httpmock.NewServer()
	defer sm.Close()

	sm.OnError = func(err error) {
		assert.NoError(t, err, "server mock")
	}

	sm.Expect(httpmock.Expectation{
		Method:       http.MethodPost,
		RequestURI:   "/v1/clients/foo/credentials",
		ResponseBody: []byte(`{"credential":"CREDENTIAL_VALUE","sequence":123}`),
		Status:       http.StatusCreated,
	})
	sm.Expect(httpmock.Expectation{
		Method:      http.MethodPost,
		RequestURI:  "/v1/clients/foo/auth?sequence=133",
		RequestBody: []byte(`{"credential":"CREDENTIAL_VALUE"}`),
		Status:      http.StatusOK,
	})

	local := httpsteps.NewLocalClient(u)

	varIsMore := func(ctx context.Context, newVar string, val int64, oldVar string) (context.Context, error) {
		ctx, v := vars.Fork(ctx)

		oldVal, ok := v.Get("$" + oldVar)
		if !ok {
			return ctx, errors.New("could not find $" + oldVar)
		}

		v.Set("$"+newVar, oldVal.(int64)+val)

		return ctx, nil
	}

	suite := godog.TestSuite{
		ScenarioInitializer: func(s *godog.ScenarioContext) {
			local.RegisterSteps(s)
			s.Step(`^\$(\w+) is (\d+) more than \$(\w+)$`, varIsMore)
		},
		Options: &godog.Options{
			Format: "pretty",
			Strict: true,
			FeatureContents: []godog.Feature{
				{
					Name: "Dynamic.feature",
					Contents: []byte(`
Feature: Client credentials management
  Background:
    When I request HTTP endpoint with method "POST" and URI "/v1/clients/foo/credentials"
    Then I should have response with status "Created"
    And I should have response with body
    """json5
    {
      "credential":"$credential",
      "sequence":"$sequence"
    }
    """

  Scenario: Client is authorized with a new valid credentials
	Given $newSequence is 10 more than $sequence

    When I request HTTP endpoint with method "POST" and URI "/v1/clients/foo/auth?sequence=$newSequence"
    And I request HTTP endpoint with body
    """json5
    {
      "credential":"$credential"
    }
    """
    Then I should have response with status "OK"
`),
				},
			},
		},
	}

	if suite.Run() != 0 {
		t.Fatal("test failed")
	}

	assert.NoError(t, sm.ExpectationsWereMet())
}
