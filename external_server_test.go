package httpdog_test

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"testing"
	"time"

	"github.com/cucumber/godog"
	httpdog "github.com/godogx/httpsteps"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/swaggest/assertjson"
)

func TestRegisterExternal(t *testing.T) {
	es := httpdog.ExternalServer{}

	someServiceURL := es.Add("some-service")
	anotherServiceURL := es.Add("another-service")

	assert.NotNil(t, es.GetMock("some-service"))

	out := bytes.NewBuffer(nil)

	suite := godog.TestSuite{
		ScenarioInitializer: func(s *godog.ScenarioContext) {
			es.RegisterSteps(s)

			s.Step(`^I call external services I receive mocked responses$`,
				callServices(t, someServiceURL, anotherServiceURL))
		},
		Options: &godog.Options{
			Format:    "pretty",
			Output:    out,
			NoColors:  true,
			Strict:    true,
			Paths:     []string{"_testdata/ExternalServer.feature"},
			Randomize: time.Now().UTC().UnixNano(),
		},
	}

	assert.Equal(t, 1, suite.Run())

	assert.Contains(t, out.String(), "Error: after scenario hook failed: check failed for external services:\n"+
		"undefined response (missing `responds with status <STATUS>` step) in some-service for GET /never-called,\n"+
		"expectations were not met for another-service: there are remaining expectations that were not met: POST /post-something")
}

func callServices(t *testing.T, someServiceURL, anotherServiceURL string) func() error {
	t.Helper()

	return func() error {
		// Hitting `"some-service" receives "GET" request "/get-something?foo=bar"`.
		req, err := http.NewRequest(http.MethodGet, someServiceURL+"/get-something?foo=bar", nil)
		require.NoError(t, err)

		req.Header.Set("X-Foo", "bar")

		resp, err := http.DefaultTransport.RoundTrip(req)
		require.NoError(t, err)

		assert.Equal(t, "foo", resp.Header.Get("X-Bar"))

		respBody, err := ioutil.ReadAll(resp.Body)
		require.NoError(t, resp.Body.Close())
		require.NoError(t, err)

		assertjson.Equal(t, []byte(`{"key":"value"}`), respBody, string(respBody))

		for i := 0; i < 10; i++ {
			go func() {
				// Hitting `"another-service" receives "POST" request "/post-something" with body`.
				req, err := http.NewRequest(http.MethodPost, anotherServiceURL+"/post-something", bytes.NewReader([]byte(`{"foo":"bar"}`)))
				require.NoError(t, err)

				resp, err := http.DefaultTransport.RoundTrip(req)
				require.NoError(t, err)

				respBody, err := ioutil.ReadAll(resp.Body)
				require.NoError(t, resp.Body.Close())
				require.NoError(t, err)

				assertjson.Equal(t, []byte(`{"theFooWas":"bar"}`), respBody)
			}()
		}

		// Hitting `"some-service" responds with status "OK"`.
		req, err = http.NewRequest(http.MethodGet, someServiceURL+"/no-response-body", nil)
		require.NoError(t, err)

		resp, err = http.DefaultTransport.RoundTrip(req)
		require.NoError(t, err)

		respBody, err = ioutil.ReadAll(resp.Body)
		require.NoError(t, resp.Body.Close())
		require.NoError(t, err)
		require.Empty(t, respBody)

		// Hitting `"some-service" responds with status "OK" and body`.
		req, err = http.NewRequest(http.MethodGet, someServiceURL+"/ask-for-foo", nil)
		require.NoError(t, err)

		resp, err = http.DefaultTransport.RoundTrip(req)
		require.NoError(t, err)

		respBody, err = ioutil.ReadAll(resp.Body)
		require.NoError(t, resp.Body.Close())
		require.NoError(t, err)

		assertjson.Equal(t, []byte(`"foo"`), respBody)

		return nil
	}
}
