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

	respFnNoBodyOKCookie := func(w http.ResponseWriter, r *http.Request) {
		// suppress date header
		w.Header()["Date"] = nil

		w.Header().Set("Set-Cookie", "testcookie=1234")

		w.WriteHeader(http.StatusOK)
	}

	respFnJSONBodyOK := func(w http.ResponseWriter, r *http.Request) {
		// suppress date header
		w.Header()["Date"] = nil

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"name":{"first":"VRISKA","last":"SERKET"}}`))
	}

	testCases := []struct {
		name               string
		args               []string // DO NOT INCLUDE -F; it is automatically set to a project file
		respFn             func(w http.ResponseWriter, r *http.Request)
		p                  morc.Project // endpoints are relative to some server; do not include host
		reqs               []morc.RequestTemplate
		expectP            morc.Project
		expectErr          string // set if command.Execute expected to fail, with a string that would be in the error message
		expectProjectSaved bool
		expectHistorySaved bool
		expectSessionSaved bool
		expectStderrOutput string // set with expected output to stderr
		expectStdoutOutput string // set with expected output to stdout
	}{
		{
			name:   "send saves history",
			args:   []string{"send", "testreq"},
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
			expectStdoutOutput: `HTTP/1.1 200 OK
(no response body)
`,
			expectProjectSaved: false,
			expectHistorySaved: true,
			expectSessionSaved: false,
		},
		{
			name:   "send saves session data",
			args:   []string{"send", "testreq"},
			respFn: respFnNoBodyOKCookie,
			p: morc.Project{
				Templates: map[string]morc.RequestTemplate{
					"testreq": {Name: "testreq", Method: "GET", URL: "/"},
				},
				Config: morc.Settings{
					SeshFile:      "::PROJ_DIR::/session.json",
					RecordSession: true,
				},
			},
			expectP: morc.Project{
				Templates: map[string]morc.RequestTemplate{
					"testreq": {Name: "testreq", Method: "GET", URL: "/"},
				},
				Session: morc.Session{
					Cookies: []morc.SetCookiesCall{
						{URL: mustParseURL("/"), Cookies: []*http.Cookie{{Name: "testcookie", Value: "1234", Raw: "testcookie=1234"}}},
					},
				},
				Config: morc.Settings{
					SeshFile:      "::PROJ_DIR::/session.json",
					RecordSession: true,
				},
			},
			expectStdoutOutput: `HTTP/1.1 200 OK
(no response body)
`,
			expectProjectSaved: false,
			expectHistorySaved: false,
			expectSessionSaved: true,
		},
		{
			name:   "send saves body captures - offset",
			args:   []string{"send", "testreq"},
			respFn: respFnJSONBodyOK,
			p: morc.Project{
				Templates: map[string]morc.RequestTemplate{
					"testreq": {
						Name:   "testreq",
						Method: "GET",
						URL:    "/",
						Captures: map[string]morc.VarScraper{
							"TEST": {Name: "TEST", OffsetStart: 18, OffsetEnd: 24},
						},
					},
				},
			},
			expectP: morc.Project{
				Templates: map[string]morc.RequestTemplate{
					"testreq": {
						Name:   "testreq",
						Method: "GET",
						URL:    "/",
						Captures: map[string]morc.VarScraper{
							"TEST": {Name: "TEST", OffsetStart: 18, OffsetEnd: 24},
						},
					},
				},
				Vars: testVarStore("", map[string]map[string]string{
					"": {"TEST": "VRISKA"},
				}),
			},
			expectStdoutOutput: `HTTP/1.1 200 OK
{"name":{"first":"VRISKA","last":"SERKET"}}
`,
			expectProjectSaved: true,
			expectHistorySaved: false,
			expectSessionSaved: false,
		},
		{
			name:   "send saves body captures - path",
			args:   []string{"send", "testreq"},
			respFn: respFnJSONBodyOK,
			p: morc.Project{
				Templates: map[string]morc.RequestTemplate{
					"testreq": {
						Name:   "testreq",
						Method: "GET",
						URL:    "/",
						Captures: map[string]morc.VarScraper{
							"TEST": {Name: "TEST", Steps: []morc.TraversalStep{
								{Key: "name"},
								{Key: "last"},
							}},
						},
					},
				},
			},
			expectP: morc.Project{
				Templates: map[string]morc.RequestTemplate{
					"testreq": {
						Name:   "testreq",
						Method: "GET",
						URL:    "/",
						Captures: map[string]morc.VarScraper{
							"TEST": {Name: "TEST", Steps: []morc.TraversalStep{
								{Key: "name"},
								{Key: "last"},
							}},
						},
					},
				},
				Vars: testVarStore("", map[string]map[string]string{
					"": {"TEST": "SERKET"},
				}),
			},
			expectStdoutOutput: `HTTP/1.1 200 OK
{"name":{"first":"VRISKA","last":"SERKET"}}
`,
			expectProjectSaved: true,
			expectHistorySaved: false,
			expectSessionSaved: false,
		},
		{
			name:   "request has a body",
			args:   []string{"send", "testreq"},
			respFn: respFnNoBodyOK,
			p: morc.Project{
				Templates: map[string]morc.RequestTemplate{
					"testreq": {Name: "testreq", Method: "GET", URL: "/", Body: []byte("testbody")},
				},
			},
			expectP: morc.Project{
				Templates: map[string]morc.RequestTemplate{
					"testreq": {Name: "testreq", Method: "GET", URL: "/", Body: []byte("testbody")},
				},
			},
			expectStdoutOutput: `HTTP/1.1 200 OK
(no response body)
`,
			expectProjectSaved: false,
			expectHistorySaved: false,
			expectSessionSaved: false,
		},
		{
			name:   "request has headers",
			args:   []string{"send", "testreq"},
			respFn: respFnNoBodyOK,
			p: morc.Project{
				Templates: map[string]morc.RequestTemplate{
					"testreq": {Name: "testreq", Method: "GET", URL: "/", Headers: http.Header{"X-Test": []string{"test"}}},
				},
			},
			expectP: morc.Project{
				Templates: map[string]morc.RequestTemplate{
					"testreq": {Name: "testreq", Method: "GET", URL: "/", Headers: http.Header{"X-Test": []string{"test"}}},
				},
			},
			expectStdoutOutput: `HTTP/1.1 200 OK
(no response body)
`,
			expectProjectSaved: false,
			expectHistorySaved: false,
			expectSessionSaved: false,
		},
		{
			name:   "print request",
			args:   []string{"send", "testreq", "--request"},
			respFn: respFnNoBodyOK,
			p: morc.Project{
				Templates: map[string]morc.RequestTemplate{
					"testreq": {Name: "testreq", Method: "GET", URL: "/", Body: []byte("testvalue"), Headers: http.Header{"X-Test": []string{"test"}}},
				},
			},
			expectP: morc.Project{
				Templates: map[string]morc.RequestTemplate{
					"testreq": {Name: "testreq", Method: "GET", URL: "/", Body: []byte("testvalue"), Headers: http.Header{"X-Test": []string{"test"}}},
				},
			},
			expectStdoutOutput: `------------------- REQUEST -------------------
Request URI: $TESTSERVER_URL$/

GET / HTTP/1.1` + "\r" + `
Host: $TESTSERVER_HOST$` + "\r" + `
User-Agent: Go-http-client/1.1` + "\r" + `
Content-Length: 9` + "\r" + `
X-Test: test` + "\r" + `
Accept-Encoding: gzip` + "\r" + `
` + "\r" + `
testvalue
----------------- END REQUEST -----------------
HTTP/1.1 200 OK
(no response body)
`,
			expectProjectSaved: false,
			expectHistorySaved: false,
			expectSessionSaved: false,
		},
		{
			name:   "print response headers",
			args:   []string{"send", "testreq", "--headers"},
			respFn: respFnNoBodyOK,
			p: morc.Project{
				Templates: map[string]morc.RequestTemplate{
					"testreq": {Name: "testreq", Method: "GET", URL: "/"},
				},
			},
			expectP: morc.Project{
				Templates: map[string]morc.RequestTemplate{
					"testreq": {Name: "testreq", Method: "GET", URL: "/"},
				},
			},
			expectStdoutOutput: `HTTP/1.1 200 OK
------------------- HEADERS -------------------
Content-Length: 0
-----------------------------------------------
(no response body)
`,

			expectProjectSaved: false,
			expectHistorySaved: false,
			expectSessionSaved: false,
		},
		{
			name:   "suppress response",
			args:   []string{"send", "testreq", "--no-body"},
			respFn: respFnJSONBodyOK,
			p: morc.Project{
				Templates: map[string]morc.RequestTemplate{
					"testreq": {Name: "testreq", Method: "GET", URL: "/"},
				},
			},
			expectP: morc.Project{
				Templates: map[string]morc.RequestTemplate{
					"testreq": {Name: "testreq", Method: "GET", URL: "/"},
				},
			},
			expectStdoutOutput: `HTTP/1.1 200 OK
`,
		},
		{
			name:   "print body captures",
			args:   []string{"send", "testreq", "--captures"},
			respFn: respFnJSONBodyOK,
			p: morc.Project{
				Templates: map[string]morc.RequestTemplate{
					"testreq": {
						Name:   "testreq",
						Method: "GET",
						URL:    "/",
						Captures: map[string]morc.VarScraper{
							"TEST": {Name: "TEST", OffsetStart: 18, OffsetEnd: 24},
						},
					},
				},
			},
			expectP: morc.Project{
				Templates: map[string]morc.RequestTemplate{
					"testreq": {
						Name:   "testreq",
						Method: "GET",
						URL:    "/",
						Captures: map[string]morc.VarScraper{
							"TEST": {Name: "TEST", OffsetStart: 18, OffsetEnd: 24},
						},
					},
				},
				Vars: testVarStore("", map[string]map[string]string{
					"": {"TEST": "VRISKA"},
				}),
			},
			expectStdoutOutput: `----------------- VAR CAPTURES ----------------
TEST: VRISKA
-----------------------------------------------
HTTP/1.1 200 OK
{"name":{"first":"VRISKA","last":"SERKET"}}
`,
			expectProjectSaved: true,
			expectHistorySaved: false,
			expectSessionSaved: false,
		},
		{
			name:   "send template with var in url",
			args:   []string{"send", "testreq", "--request"},
			respFn: respFnNoBodyOK,
			p: morc.Project{
				Templates: map[string]morc.RequestTemplate{
					"testreq": {
						Name:   "testreq",
						Method: "GET",
						URL:    "${PATH}",
					},
				},
				Vars: testVarStore("", map[string]map[string]string{
					"": {"PATH": "/path"},
				}),
			},
			expectP: morc.Project{
				Templates: map[string]morc.RequestTemplate{
					"testreq": {
						Name:   "testreq",
						Method: "GET",
						URL:    "${PATH}",
					},
				},
				Vars: testVarStore("", map[string]map[string]string{
					"": {"PATH": "/path"},
				}),
			},
			expectStdoutOutput: `------------------- REQUEST -------------------
Request URI: $TESTSERVER_URL$/path

GET /path HTTP/1.1` + "\r" + `
Host: $TESTSERVER_HOST$` + "\r" + `
User-Agent: Go-http-client/1.1` + "\r" + `
Accept-Encoding: gzip` + "\r" + `
` + "\r" + `

(no request body)
----------------- END REQUEST -----------------
HTTP/1.1 200 OK
(no response body)
`,
			expectProjectSaved: false,
			expectHistorySaved: false,
			expectSessionSaved: false,
		},
		{
			name:   "send template with var in headers",
			args:   []string{"send", "testreq", "--request"},
			respFn: respFnNoBodyOK,
			p: morc.Project{
				Templates: map[string]morc.RequestTemplate{
					"testreq": {
						Name:    "testreq",
						Method:  "GET",
						URL:     "/",
						Headers: http.Header{"${API_KEY_HEADER}": []string{"${API_KEY}"}},
					},
				},
				Vars: testVarStore("", map[string]map[string]string{
					"": {"API_KEY": "fake", "API_KEY_HEADER": "X-Api-Key"},
				}),
			},
			expectP: morc.Project{
				Templates: map[string]morc.RequestTemplate{
					"testreq": {
						Name:    "testreq",
						Method:  "GET",
						URL:     "/",
						Headers: http.Header{"${API_KEY_HEADER}": []string{"${API_KEY}"}},
					},
				},
				Vars: testVarStore("", map[string]map[string]string{
					"": {"API_KEY": "fake", "API_KEY_HEADER": "X-Api-Key"},
				}),
			},
			expectStdoutOutput: `------------------- REQUEST -------------------
Request URI: $TESTSERVER_URL$/

GET / HTTP/1.1` + "\r" + `
Host: $TESTSERVER_HOST$` + "\r" + `
User-Agent: Go-http-client/1.1` + "\r" + `
X-Api-Key: fake` + "\r" + `
Accept-Encoding: gzip` + "\r" + `
` + "\r" + `

(no request body)
----------------- END REQUEST -----------------
HTTP/1.1 200 OK
(no response body)
`,
			expectProjectSaved: false,
			expectHistorySaved: false,
			expectSessionSaved: false,
		},
		{
			name:   "send template with var in body",
			args:   []string{"send", "testreq", "--request"},
			respFn: respFnNoBodyOK,
			p: morc.Project{
				Templates: map[string]morc.RequestTemplate{
					"testreq": {
						Name:   "testreq",
						Method: "GET",
						URL:    "/",
						Body:   []byte("special.key=${API_KEY}"),
					},
				},
				Vars: testVarStore("", map[string]map[string]string{
					"": {"API_KEY": "fake"},
				}),
			},
			expectP: morc.Project{
				Templates: map[string]morc.RequestTemplate{
					"testreq": {
						Name:   "testreq",
						Method: "GET",
						URL:    "/",
						Body:   []byte("special.key=${API_KEY}"),
					},
				},
				Vars: testVarStore("", map[string]map[string]string{
					"": {"API_KEY": "fake"},
				}),
			},
			expectStdoutOutput: `------------------- REQUEST -------------------
Request URI: $TESTSERVER_URL$/

GET / HTTP/1.1` + "\r" + `
Host: $TESTSERVER_HOST$` + "\r" + `
User-Agent: Go-http-client/1.1` + "\r" + `
Content-Length: 16` + "\r" + `
Accept-Encoding: gzip` + "\r" + `
` + "\r" + `
special.key=fake
----------------- END REQUEST -----------------
HTTP/1.1 200 OK
(no response body)
`,
			expectProjectSaved: false,
			expectHistorySaved: false,
			expectSessionSaved: false,
		},

		{
			name:   "send template with CLI-overriden var in body",
			args:   []string{"send", "testreq", "--request", "-V", "API_KEY=test-value"},
			respFn: respFnNoBodyOK,
			p: morc.Project{
				Templates: map[string]morc.RequestTemplate{
					"testreq": {
						Name:   "testreq",
						Method: "GET",
						URL:    "/",
						Body:   []byte("special.key=${API_KEY}"),
					},
				},
				Vars: testVarStore("", map[string]map[string]string{
					"": {"API_KEY": "fake"},
				}),
			},
			expectP: morc.Project{
				Templates: map[string]morc.RequestTemplate{
					"testreq": {
						Name:   "testreq",
						Method: "GET",
						URL:    "/",
						Body:   []byte("special.key=${API_KEY}"),
					},
				},
				Vars: testVarStore("", map[string]map[string]string{
					"": {"API_KEY": "fake"},
				}),
			},
			expectStdoutOutput: `------------------- REQUEST -------------------
Request URI: $TESTSERVER_URL$/

GET / HTTP/1.1` + "\r" + `
Host: $TESTSERVER_HOST$` + "\r" + `
User-Agent: Go-http-client/1.1` + "\r" + `
Content-Length: 22` + "\r" + `
Accept-Encoding: gzip` + "\r" + `
` + "\r" + `
special.key=test-value
----------------- END REQUEST -----------------
HTTP/1.1 200 OK
(no response body)
`,
			expectProjectSaved: false,
			expectHistorySaved: false,
			expectSessionSaved: false,
		},
		{
			name:   "send template with var in url, non-default prefix",
			args:   []string{"send", "testreq", "--request"},
			respFn: respFnNoBodyOK,
			p: morc.Project{
				Templates: map[string]morc.RequestTemplate{
					"testreq": {
						Name:   "testreq",
						Method: "GET",
						URL:    "^{PATH}",
					},
				},
				Vars: testVarStore("", map[string]map[string]string{
					"": {"PATH": "/path"},
				}),
				Config: morc.Settings{
					VarPrefix: "^",
				},
			},
			expectP: morc.Project{
				Templates: map[string]morc.RequestTemplate{
					"testreq": {
						Name:   "testreq",
						Method: "GET",
						URL:    "^{PATH}",
					},
				},
				Vars: testVarStore("", map[string]map[string]string{
					"": {"PATH": "/path"},
				}),
				Config: morc.Settings{
					VarPrefix: "^",
				},
			},
			expectStdoutOutput: `------------------- REQUEST -------------------
Request URI: $TESTSERVER_URL$/path

GET /path HTTP/1.1` + "\r" + `
Host: $TESTSERVER_HOST$` + "\r" + `
User-Agent: Go-http-client/1.1` + "\r" + `
Accept-Encoding: gzip` + "\r" + `
` + "\r" + `

(no request body)
----------------- END REQUEST -----------------
HTTP/1.1 200 OK
(no response body)
`,
			expectProjectSaved: false,
			expectHistorySaved: false,
			expectSessionSaved: false,
		}, {
			name:   "send template with var in url, prefix override",
			args:   []string{"send", "testreq", "--request", "-p", "^"},
			respFn: respFnNoBodyOK,
			p: morc.Project{
				Templates: map[string]morc.RequestTemplate{
					"testreq": {
						Name:   "testreq",
						Method: "GET",
						URL:    "^{PATH}",
					},
				},
				Vars: testVarStore("", map[string]map[string]string{
					"": {"PATH": "/path"},
				}),
			},
			expectP: morc.Project{
				Templates: map[string]morc.RequestTemplate{
					"testreq": {
						Name:   "testreq",
						Method: "GET",
						URL:    "^{PATH}",
					},
				},
				Vars: testVarStore("", map[string]map[string]string{
					"": {"PATH": "/path"},
				}),
			},
			expectStdoutOutput: `------------------- REQUEST -------------------
Request URI: $TESTSERVER_URL$/path

GET /path HTTP/1.1` + "\r" + `
Host: $TESTSERVER_HOST$` + "\r" + `
User-Agent: Go-http-client/1.1` + "\r" + `
Accept-Encoding: gzip` + "\r" + `
` + "\r" + `

(no request body)
----------------- END REQUEST -----------------
HTTP/1.1 200 OK
(no response body)
`,
			expectProjectSaved: false,
			expectHistorySaved: false,
			expectSessionSaved: false,
		},
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

			// make shore that expected session set-cookie-calls have proper prefix
			if tc.expectP.Session.Cookies != nil {
				for i := range tc.expectP.Session.Cookies {
					tc.expectP.Session.Cookies[i].URL = mustParseURL(srv.URL + tc.expectP.Session.Cookies[i].URL.Path)
				}
			}

			// make shore stdout output replaces server things
			tc.expectStdoutOutput = strings.ReplaceAll(tc.expectStdoutOutput, "$TESTSERVER_URL$", srv.URL)
			srvHost := mustParseURL(srv.URL).Host
			tc.expectStdoutOutput = strings.ReplaceAll(tc.expectStdoutOutput, "$TESTSERVER_HOST$", srvHost)

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
	flags.VarPrefix = "$"
	flags.BQuiet = false

	sendCmd.Flags().VisitAll(func(fl *pflag.Flag) {
		fl.Changed = false
	})
}
