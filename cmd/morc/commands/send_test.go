package commands

import (
	"fmt"
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

func mustParseURL(s string) *url.URL {
	u, err := url.Parse(s)
	if err != nil {
		panic(err)
	}
	return u
}

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
		// suppress date header
		w.Header()["Date"] = nil

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
		expectProjectSaved bool
		expectHistorySaved bool
		expectSessionSaved bool
		expectStderrOutput string // set with expected output to stderr
		expectStdoutOutput string // set with expected output to stdout
	}{
		{
			name:   "minimal request/response - save history",
			respFn: respFnNoBodyOK,
			p: morc.Project{
				Templates: map[string]morc.RequestTemplate{
					"testreq": {Name: "testreq", Method: "GET", URL: "/"},
				},
				Config: morc.Settings{
					HistFile:      "::PROJ_DIR::/history.json",
					RecordHistory: true,
				},
			},
			expectP: morc.Project{
				Templates: map[string]morc.RequestTemplate{
					"testreq": {Name: "testreq", Method: "GET", URL: "/"},
				},
				History: []morc.HistoryEntry{
					{
						Template: "testreq",
						Request: &http.Request{
							Method:     "GET",
							URL:        mustParseURL("/"),
							Proto:      "HTTP/1.1",
							ProtoMajor: 1,
							ProtoMinor: 1,
							Body:       http.NoBody,
						},
						Response: &http.Response{
							Status:     fmt.Sprintf("%d %s", http.StatusOK, http.StatusText(http.StatusOK)),
							StatusCode: http.StatusOK,
							Proto:      "HTTP/1.1",
							ProtoMajor: 1,
							ProtoMinor: 1,
							Header: http.Header{
								"Content-Length": []string{"0"},
							},
							Body: http.NoBody,
						},
					},
				},
				Config: morc.Settings{
					HistFile:      "::PROJ_DIR::/history.json",
					RecordHistory: true,
				},
			},
			args: []string{"send", "testreq"},
			expectStdoutOutput: `HTTP/1.1 200 OK
(no response body)
`,
			expectProjectSaved: false,
			expectHistorySaved: true,
			expectSessionSaved: false,
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

			// make shore that expected historic entries have proper prefix
			if tc.expectP.History != nil {
				for i := range tc.expectP.History {
					tc.expectP.History[i].Request.URL = mustParseURL(srv.URL + tc.expectP.History[i].Request.URL.Path)
				}
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

			if tc.expectProjectSaved {
				assert_projectPersistedToBuffer(assert, tc.expectP)
			} else {
				assert_noProjectFileMutations(assert)
			}

			if tc.expectHistorySaved {
				assert_historyPersistedToBuffer(assert, tc.expectP.History)
			} else {
				assert_noHistoryFileMutations(assert)
			}

			if tc.expectSessionSaved {
				assert_sessionPersistedToBuffer(assert, tc.expectP.Session)
			} else {
				assert_noSessionFileMutations(assert)
			}
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
