package httpsteps_test

import (
	"net/http"
	"testing"

	"github.com/bool64/httpmock"
	"github.com/cucumber/godog"
	"github.com/godogx/httpsteps"
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
		ResponseBody: []byte(`{"credential":"CREDENTIAL_VALUE"}`),
		Status:       http.StatusCreated,
	})
	sm.Expect(httpmock.Expectation{
		Method:      http.MethodPost,
		RequestURI:  "/v1/clients/foo/auth",
		RequestBody: []byte(`{"credential":"CREDENTIAL_VALUE"}`),
		Status:      http.StatusOK,
	})

	local := httpsteps.NewLocalClient(u)

	suite := godog.TestSuite{
		ScenarioInitializer: func(s *godog.ScenarioContext) {
			local.RegisterSteps(s)
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
      "credential":"$credential"
    }
    """

  Scenario: Client is authorized with a new valid credentials
    When I request HTTP endpoint with method "POST" and URI "/v1/clients/foo/auth"
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
