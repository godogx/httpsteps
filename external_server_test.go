package httpsteps_test

import (
	"bytes"
	"io"
	"io/ioutil"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/cucumber/godog"
	"github.com/godogx/httpsteps"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/swaggest/assertjson"
)

func TestRegisterExternal(t *testing.T) {
	es := httpsteps.NewExternalServer()

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

	assert.Contains(t, out.String(), "Error: after scenario hook failed:")
	assert.Contains(t, out.String(), "undefined response (missing `responds with status <STATUS>` step) in some-service for GET /never-called")
	assert.Contains(t, out.String(), "expectations were not met for another-service: there are remaining expectations that were not met: POST /post-something")
}

func callServices(t *testing.T, someServiceURL, anotherServiceURL string) func() {
	t.Helper()

	return func() {
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

		wg := sync.WaitGroup{}
		for i := 0; i < 10; i++ {
			wg.Add(1)

			go func() {
				defer wg.Done()

				// Hitting `"another-service" receives "POST" request "/post-something" with body`.
				req, err := http.NewRequest(http.MethodPost, anotherServiceURL+"/post-something", bytes.NewReader([]byte(`{"foo":"bar"}`)))
				assert.NoError(t, err)

				resp, err := http.DefaultTransport.RoundTrip(req)
				assert.NoError(t, err)

				respBody, err := io.ReadAll(resp.Body)
				assert.NoError(t, resp.Body.Close())
				assert.NoError(t, err)

				assertjson.Equal(t, []byte(`{"theFooWas":"bar"}`), respBody)
			}()
		}
		wg.Wait()

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
	}
}
