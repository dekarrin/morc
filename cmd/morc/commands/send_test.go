package commands

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/dekarrin/morc"
	"github.com/dekarrin/morc/cmd/morc/cmdio"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
)

type urlBaseRoundTripper struct {
	base string
	old  http.RoundTripper
}

func (rt urlBaseRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	fullURL := rt.base + req.URL.Path
	if req.URL.RawQuery != "" {
		fullURL += "?" + req.URL.RawQuery
	}

	var err error
	req.URL, err = url.Parse(fullURL)
	if err != nil {
		return nil, err
	}

	return rt.old.RoundTrip(req)
}

func Test_Send(t *testing.T) {
	respFnNoBodyOK := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}

	testCases := []struct {
		name               string
		respFn             func(w http.ResponseWriter, r *http.Request)
		p                  morc.Project // endpoints are relative to some server; do not include host
		reqs               []morc.RequestTemplate
		args               []string // DO NOT INCLUDE -F; it is automatically set to a project file
		expectP            morc.Project
		expectErr          string // set if command.Execute expected to fail, with a string that would be in the error message
		expectStderrOutput string // set with expected output to stderr
		expectStdoutOutput string // set with expected output to stdout
	}{
		{
			name:   "minimal request/response",
			respFn: respFnNoBodyOK,
			p: testProject_withRequests(
				morc.RequestTemplate{Name: "testreq", Method: "GET", URL: "/"},
			),
			expectP: testProject_withRequests(
				morc.RequestTemplate{Name: "testreq", Method: "GET", URL: "/"},
			),
			args: []string{"send", "testreq"},
			expectStdoutOutput: `HTTP/1.1 200 OK
(no response body)
`,
		},
		// 		{
		// 			name:   "minimal request/response with headers",
		// 			respFn: respFnNoBodyOK,
		// 			reqs: []morc.RequestTemplate{
		// 				{
		// 					Name:   "testreq",
		// 					Method: "GET",
		// 					URL:    "/",
		// 				},
		// 			},
		// 			args: []string{"send", "testreq", "--headers"},
		// 			expectOutput: `HTTP/1.1 200 OK
		// ------------------- HEADERS -------------------
		// Content-Length: 0
		// Date: Tue, 14 May 2024 15:27:36 GMT
		// -----------------------------------------------
		// (no response body)
		// `,
		// 		},

		// TODO: make above pass even though date is not exact
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert := assert.New(t)

			// setup test server
			srv := httptest.NewServer(http.HandlerFunc(tc.respFn))
			defer srv.Close()
			srvClient := srv.Client()

			// inject a custom transport so we always append the server root URL
			srvClient.Transport = urlBaseRoundTripper{
				base: srv.URL,
				old:  srvClient.Transport,
			}

			cmdio.HTTPClient = srvClient

			resetSendFlags()

			// create project and dump config to a temp dir
			projFilePath := createTestProjectIO(t, tc.p)
			// set up the root command and run
			output, outputErr, err := runTestCommand(sendCmd, projFilePath, tc.args)

			// assert and check stdout and stderr
			if err != nil {
				if tc.expectErr == "" {
					t.Fatalf("unexpected returned error: %v", err)
					return
				}
				if !strings.Contains(err.Error(), tc.expectErr) {
					t.Fatalf("expected returned error to contain %q, got %q", tc.expectErr, err)
				}
				return
			} else if tc.expectErr != "" {
				t.Fatalf("expected error %q, got no error", tc.expectErr)
			}

			// assertions

			assert.Equal(tc.expectStdoutOutput, output, "stdout output mismatch")
			assert.Equal(tc.expectStderrOutput, outputErr, "stderr output mismatch")

			assert_projectFilesInBuffersMatch(assert, tc.expectP)
		})
	}
}

func resetSendFlags() {
	flags.ProjectFile = ""
	flags.Vars = nil
	flags.BInsecure = false
	flags.BHeaders = false
	flags.BCaptures = false
	flags.BNoBody = false
	flags.BRequest = false
	flags.Format = "pretty" // TODO: make this default not be magic but rather have the cmd flag init and the reset use it

	sendCmd.Flags().VisitAll(func(fl *pflag.Flag) {
		fl.Changed = false
	})
}
