package httpsteps_test

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"

	"github.com/cucumber/godog"
	httpsteps "github.com/godogx/httpsteps"
)

func ExampleNewLocalClient() {
	external := httpsteps.ExternalServer{}
	templateService := external.Add("template-service")

	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req, _ := http.NewRequest(http.MethodGet, templateService+"/template/hello", nil)
		resp, _ := http.DefaultTransport.RoundTrip(req)
		tpl, _ := ioutil.ReadAll(resp.Body)

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
			Output: ioutil.Discard,
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
