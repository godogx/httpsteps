package httpsteps_test

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bool64/httpmock"
	"github.com/cucumber/godog"
	httpsteps "github.com/godogx/httpsteps"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLocal_RegisterSteps(t *testing.T) {
	mock, srvURL := httpmock.NewServer()
	mock.OnError = func(err error) {
		require.NoError(t, err)
	}

	defer mock.Close()

	concurrencyLevel := 5
	setExpectations(mock, concurrencyLevel)

	local := httpsteps.NewLocalClient(srvURL, func(client *httpmock.Client) {
		client.Headers = map[string]string{
			"X-Foo": "bar",
		}
		client.ConcurrencyLevel = concurrencyLevel
	})
	local.AddService("some-service", srvURL)

	suite := godog.TestSuite{
		ScenarioInitializer: func(s *godog.ScenarioContext) {
			local.RegisterSteps(s)
		},
		Options: &godog.Options{
			Format: "pretty",
			Strict: true,
			Paths:  []string{"_testdata/LocalClient.feature"},
		},
	}

	if suite.Run() != 0 {
		t.Fatal("test failed")
	}

	require.NoError(t, mock.ExpectationsWereMet())
}

func setExpectations(mock *httpmock.Server, concurrencyLevel int) {
	mock.Expect(httpmock.Expectation{
		Method:       http.MethodGet,
		RequestURI:   "/get-something?foo=bar",
		ResponseBody: []byte(`[{"some":"json", "dyn": "abc"}]`),
		ResponseHeader: map[string]string{
			"Content-Type": "application/json",
		},
	})

	mock.Expect(httpmock.Expectation{
		Method:     http.MethodDelete,
		RequestURI: "/bad-request",
		RequestHeader: map[string]string{
			"X-Foo": "bar",
		},
		RequestCookie: map[string]string{
			"c1": "v1",
			"c2": "v2",
		},
		ResponseBody: []byte(`{"error":"oops"}`),
		Status:       http.StatusBadRequest,
	})

	mock.Expect(httpmock.Expectation{
		Method:     http.MethodPost,
		RequestURI: "/with-body",
		RequestHeader: map[string]string{
			"X-Foo": "bar",
		},
		RequestBody:  []byte(`[{"some":"json","dyn":"abc"}]`),
		ResponseBody: []byte(`{"status":"ok"}`),
		ResponseHeader: map[string]string{
			"Content-Type": "application/json",
		},
	})

	del := httpmock.Expectation{
		Method:     http.MethodDelete,
		RequestURI: "/delete-something",
		Status:     http.StatusNoContent,
		ResponseHeader: map[string]string{
			"Content-Type": "application/json",
		},
	}

	// Expecting 2 similar requests.
	mock.Expect(del)
	mock.Expect(del)

	// Due to idempotence testing several more requests should be expected.
	delNotFound := del
	delNotFound.Status = http.StatusNotFound
	delNotFound.ResponseBody = []byte(`{"status":"failed","error":"foo"}`)

	for i := 0; i < concurrencyLevel-1; i++ {
		mock.Expect(delNotFound)
	}

	// Expecting request containing json5 comments.
	mock.Expect(httpmock.Expectation{
		Method:     http.MethodPost,
		RequestURI: "/with-json5-body",
		RequestHeader: map[string]string{
			"X-Foo": "bar",
		},
		RequestBody:  []byte(`[{"some":"json5"}]`),
		ResponseBody: []byte(`{"status":"ok"}`),
		ResponseHeader: map[string]string{
			"Content-Type": "application/json",
		},
	})

	// Expecting request does not contain a valid json.
	mock.Expect(httpmock.Expectation{
		Method:       http.MethodGet,
		RequestURI:   "/with-csv-body",
		RequestBody:  []byte(`a,b,c`),
		ResponseBody: []byte(`a,b,c`),
	})

	// Expecting request for "Successful call against named service".
	mock.Expect(httpmock.Expectation{
		Method:       http.MethodGet,
		RequestURI:   "/get-something?foo=bar",
		ResponseBody: []byte(`[{"some":"json","dyn":"abc"}]`),
		ResponseHeader: map[string]string{
			"Content-Type": "application/json",
		},
	})
}

func TestLocal_RegisterSteps_unexpectedOtherResp(t *testing.T) {
	mock, srvURL := httpmock.NewServer()
	mock.OnError = func(err error) {
		require.NoError(t, err)
	}

	defer mock.Close()

	concurrencyLevel := 5
	del := httpmock.Expectation{
		Method:     http.MethodDelete,
		RequestURI: "/delete-something",
		Status:     http.StatusNoContent,
		ResponseHeader: map[string]string{
			"Content-Type": "application/json",
		},
	}

	mock.Expect(del)

	// Due to idempotence testing several more requests should be expected.
	delNotFound := del
	delNotFound.Status = http.StatusNotFound
	delNotFound.ResponseBody = []byte(`{"status":"failed","error":"foo"}`)

	for i := 0; i < concurrencyLevel-1; i++ {
		mock.Expect(delNotFound)
	}

	local := httpsteps.NewLocalClient(srvURL, func(client *httpmock.Client) {
		client.ConcurrencyLevel = concurrencyLevel
	})
	out := bytes.NewBuffer(nil)

	suite := godog.TestSuite{
		ScenarioInitializer: func(s *godog.ScenarioContext) {
			local.RegisterSteps(s)
		},
		Options: &godog.Options{
			Output:   out,
			Format:   "pretty",
			NoColors: true,
			Strict:   true,
			Paths:    []string{"_testdata/LocalClientFail1.feature"},
		},
	}

	assert.Equal(t, 1, suite.Run())
	require.NoError(t, mock.ExpectationsWereMet())
	assert.Contains(t, out.String(), "Error: after scenario hook failed: no other responses expected for default: unexpected response status, expected: 204 (No Content), received: 404 (Not Found)")
}

func TestLocal_RegisterSteps_dynamic(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/user" {
			_, err := w.Write([]byte(`{"id":12345, "user":"bob", "name": "John Doe","created_at":"any","updated_at": "any"}`))
			require.NoError(t, err)

			return
		}

		if r.URL.Path == "/order/12345/" && r.URL.Query().Get("user_id") == "12345" {
			assert.Equal(t, "12345", r.Header.Get("X-Userid"))

			cookie, err := r.Cookie("user_id")
			require.NoError(t, err)
			assert.Equal(t, "12345", cookie.Value)

			b, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			require.NoError(t, r.Body.Close())

			assert.Equal(t, `{"user_id":12345,"item_name":"Watermelon"}`, string(b))

			_, err = w.Write([]byte(`{"id":54321,"created_at":"any","updated_at": "any","prefixed_user":"static_prefix::bob","prefixed_user_id":"static_prefix::12345","user_id":12345}`))
			require.NoError(t, err)

			return
		}
	}))
	defer srv.Close()

	local := httpsteps.NewLocalClient(srv.URL)

	suite := godog.TestSuite{
		ScenarioInitializer: func(s *godog.ScenarioContext) {
			local.RegisterSteps(s)
		},
		Options: &godog.Options{
			Format: "pretty",
			Strict: true,
			Paths:  []string{"_testdata/Dynamic.feature"},
		},
	}

	if suite.Run() != 0 {
		t.Fatal("test failed")
	}
}

func TestLocal_RegisterSteps_AttachmentFile(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/file-attached" {
			// Maximum upload of 10 MB files
			require.NoError(t, r.ParseMultipartForm(10<<20))

			// Get handler for filename, size and headers
			file, _, err := r.FormFile("file")
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)

				return
			}

			defer func() {
				require.NoError(t, file.Close())
			}()

			const maxBufferSize = 1024 * 512
			reader := io.LimitReader(file, maxBufferSize)

			content, err := io.ReadAll(reader)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)

				return
			}

			_, err = w.Write([]byte(`{"content":"` + string(content) + `"}`))
			require.NoError(t, err)

			return
		}
	}))
	defer srv.Close()

	local := httpsteps.NewLocalClient(srv.URL)

	suite := godog.TestSuite{
		ScenarioInitializer: func(s *godog.ScenarioContext) {
			local.RegisterSteps(s)
		},
		Options: &godog.Options{
			Format: "pretty",
			Strict: true,
			Paths:  []string{"_testdata/AttachmentFile.feature"},
		},
	}

	if suite.Run() != 0 {
		t.Fatal("test failed")
	}
}

func TestLocal_RegisterSteps_followRedirects(t *testing.T) {
	mock, srvURL := httpmock.NewServer()
	mock.OnError = func(err error) {
		require.NoError(t, err)
	}

	defer mock.Close()

	mock.Expect(httpmock.Expectation{
		RequestURI: "/one",
		Status:     http.StatusFound,
		ResponseHeader: map[string]string{
			"Location": "/two",
		},
	})
	mock.Expect(httpmock.Expectation{
		RequestURI: "/two",
		Status:     http.StatusFound,
		ResponseHeader: map[string]string{
			"Location": "/three",
		},
	})
	mock.Expect(httpmock.Expectation{
		RequestURI:   "/three",
		Status:       http.StatusOK,
		ResponseBody: []byte("OK"),
	})

	local := httpsteps.NewLocalClient(srvURL)
	out := bytes.NewBuffer(nil)

	suite := godog.TestSuite{
		ScenarioInitializer: func(s *godog.ScenarioContext) {
			local.RegisterSteps(s)
		},
		Options: &godog.Options{
			Output:   out,
			Format:   "pretty",
			NoColors: true,
			Strict:   true,
			Paths:    []string{"_testdata/FollowRedirects.feature"},
		},
	}

	assert.Equal(t, 0, suite.Run())
}

func TestLocal_RegisterSteps_tableSetup(t *testing.T) {
	mock, srvURL := httpmock.NewServer()
	mock.OnError = func(err error) {
		require.NoError(t, err)
	}

	defer mock.Close()

	mock.Expect(httpmock.Expectation{
		RequestURI: "/hello?qbar=123&qbar=456&qfoo=foo",
		RequestHeader: map[string]string{
			"X-Foo": "foo",
			"X-Bar": "123",
		},
		RequestCookie: map[string]string{
			"cfoo": "foo",
			"cbar": "123",
		},
		RequestBody:  []byte(`fbar=123&fbar=456&ffoo=abc`),
		Status:       http.StatusOK,
		ResponseBody: []byte(`[{"some":"json","dyn":"abc"}]`),
		ResponseHeader: map[string]string{
			"X-Baz":        "abc",
			"Content-Type": "application/json",
		},
	})

	local := httpsteps.NewLocalClient(srvURL)
	local.AddService("some-service", srvURL)

	out := bytes.NewBuffer(nil)

	suite := godog.TestSuite{
		ScenarioInitializer: func(s *godog.ScenarioContext) {
			local.RegisterSteps(s)
		},
		Options: &godog.Options{
			Output:   out,
			Format:   "pretty",
			NoColors: true,
			Strict:   true,
			Paths:    []string{"_testdata/TableSetup.feature"},
		},
	}

	if !assert.Equal(t, 0, suite.Run()) {
		fmt.Println(out.String())
	}
}
