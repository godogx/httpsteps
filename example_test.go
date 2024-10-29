package httpsteps_test

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"

	"github.com/cucumber/godog"
	"github.com/godogx/httpsteps"
)

func ExampleNewLocalClient() {
	external := httpsteps.NewExternalServer()
	templateService := external.Add("template-service")

	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req, _ := http.NewRequest(http.MethodGet, templateService+"/template/hello", nil)
		resp, _ := http.DefaultTransport.RoundTrip(req)
		tpl, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()

		_, _ = w.Write([]byte(fmt.Sprintf(string(tpl), r.URL.Query().Get("name"))))
	})

	srv := httptest.NewServer(h)
	defer srv.Close()

	local := httpsteps.NewLocalClient(srv.URL)

	suite := godog.TestSuite{
		ScenarioInitializer: func(s *godog.ScenarioContext) {
			local.RegisterSteps(s)
			external.RegisterSteps(s)
		},
		Options: &godog.Options{
			Format: "pretty",
			Strict: true,
			Paths:  []string{"_testdata/Example.feature"},
			Output: io.Discard,
		},
	}

	if suite.Run() != 0 {
		fmt.Println("test failed")
	} else {
		fmt.Println("test passed")
	}

	// Output:
	// test passed
}
