package commands

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/dekarrin/morc"
	"github.com/dekarrin/morc/cmd/morc/cmdio"
	"github.com/spf13/pflag"
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

type testResource struct {
	Name   string `json:"name"`
	Number int    `json:"number"`
	Title  string `json:"title"`
}

// testJWTData contains all data needed to create a minimal JWT with verification info.
// It will always be signed using the HMAC SHA-256 algorithm.
type testJWTData struct {
	Expiration time.Time `json:"exp,omitempty"`
	Subject    string    `json:"sub,"`
}

func (jwt testJWTData) Token(secretKey string) string {
	// create header and claims
	header := map[string]string{
		"typ": "JWT",
		"alg": "HS256",
	}

	// encode header
	headerJSON, _ := json.Marshal(header)
	headerB64 := base64.StdEncoding.EncodeToString(headerJSON)

	// encode claims
	claims := map[string]any{
		"exp": jwt.Expiration.Unix(),
		"sub": jwt.Subject,
	}
	claimsJSON, _ := json.Marshal(claims)
	claimsB64 := base64.StdEncoding.EncodeToString(claimsJSON)

	// create signature input
	sigInput := headerB64 + "." + claimsB64

	// sign using HMAC-SHA256
	h := hmac.New(sha256.New, []byte(secretKey))
	h.Write([]byte(sigInput))
	sig := h.Sum(nil)
	sigB64 := base64.StdEncoding.EncodeToString(sig)

	// combine all parts
	return headerB64 + "." + claimsB64 + "." + sigB64
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

	cookieExpTime := mustParseTime(time.RFC3339, time.Now().Add(1*time.Hour).UTC().Format(time.RFC3339))

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
			name: "request requires cookie-based auth - history saved - no output for auth req",
			args: []string{"send", "resource", "--hide-auth"},
			respFn: serverHandler_withProtectedResource_session(
				Creds{User: "test", Pass: "TEsT123!"},
				&http.Cookie{Name: "session", Value: "ABCDEFG", Expires: cookieExpTime},
				testResource{Name: "VRISKA", Number: 8, Title: "Thief of Light"},
			),
			p: morc.Project{
				Templates: testRequests_withProtectedResource_session(Creds{"test", "TEsT123!"}, "testauth"),
				Auths:     testAuths(testAuth_session("testauth", seqTemplate, "login", "session", enableExpiration, nil)),
				Config: morc.Settings{
					HistFile:      "::PROJ_DIR::/history.json",
					RecordHistory: true,
				},
			},
			expectP: morc.Project{
				Templates: testRequests_withProtectedResource_session(Creds{"test", "TEsT123!"}, "testauth"),
				Auths:     testAuths(testAuth_session("testauth", seqTemplate, "login", "session", enableExpiration, testProof_session("session", "ABCDEFG", cookieExpTime))),
				History: []morc.HistoryEntry{
					{
						Template: "login",
						Request: &http.Request{
							Method:     "POST",
							URL:        mustParseURL("/login"),
							Proto:      "HTTP/1.1",
							ProtoMajor: 1,
							ProtoMinor: 1,
							Header: http.Header{
								"Content-Type": []string{"application/json"},
							},
							Body:          io.NopCloser(strings.NewReader(`{"user":"test","pass":"TEsT123!"}`)),
							ContentLength: 33,
						},
						Response: &http.Response{
							Status:     fmt.Sprintf("%d %s", http.StatusNoContent, http.StatusText(http.StatusNoContent)),
							StatusCode: http.StatusNoContent,
							Proto:      "HTTP/1.1",
							ProtoMajor: 1,
							ProtoMinor: 1,
							Header: http.Header{
								"Set-Cookie": []string{"session=ABCDEFG; Expires=" + cookieExpTime.Format(http.TimeFormat)},
							},
						},
						Initiator: morc.Initiator{
							Cause:  morc.CauseChain,
							Parent: "resource",
							Auth:   "testauth",
							Links: morc.HistoryLinks{
								Parent: 1,
							},
						},
					},
					{
						Template: "resource",
						Request: &http.Request{
							Method:     "GET",
							URL:        mustParseURL("/protected"),
							Proto:      "HTTP/1.1",
							ProtoMajor: 1,
							ProtoMinor: 1,
							Body:       http.NoBody,
							Header: http.Header{
								"Content-Type": []string{"application/json"},
								"Cookie":       []string{"session=ABCDEFG"},
							},
						},
						Response: &http.Response{
							Status:     fmt.Sprintf("%d %s", http.StatusOK, http.StatusText(http.StatusOK)),
							StatusCode: http.StatusOK,
							Proto:      "HTTP/1.1",
							ProtoMajor: 1,
							ProtoMinor: 1,
							Header: http.Header{
								"Content-Length": []string{"53"},
								"Content-Type":   []string{"application/json"},
							},
							Body:          io.NopCloser(strings.NewReader(`{"name":"VRISKA","number":8,"title":"Thief of Light"}`)),
							ContentLength: 53,
						},
						Initiator: morc.Initiator{
							Cause: morc.CauseTemplateSpecified,
						},
					},
				},
				Config: morc.Settings{
					HistFile:      "::PROJ_DIR::/history.json",
					RecordHistory: true,
				},
			},
			expectStdoutOutput: `HTTP/1.1 200 OK
{"name":"VRISKA","number":8,"title":"Thief of Light"}
`,
			expectProjectSaved: true,
			expectHistorySaved: true,
			expectSessionSaved: false,
		},
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
						Initiator: morc.Initiator{
							Cause: morc.CauseTemplateSpecified,
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
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
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

			assert := NewAssertionsForInMemoryProject(t, tc.p, &fileRWs)
			resetSendFlags()

			// set up the root command and run
			output, outputErr, err := runTestCommand(sendCmd, assert.ProjFilePath, tc.args)

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
				assert.ProjectPersistedToBuffer(tc.expectP)
			} else {
				assert.NoProjectFileMutations()
			}

			if tc.expectHistorySaved {
				assert.HistoryPersistedToBuffer(tc.expectP.History)
			} else {
				assert.NoHistoryFileMutations()
			}

			if tc.expectSessionSaved {
				assert.SessionPersistedToBuffer(tc.expectP.Session)
			} else {
				assert.NoSessionFileMutations()
			}
		})
	}
}

func Test_Send_WithCaptures(t *testing.T) {
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
		expectP            morc.Project
		expectErr          string // set if command.Execute expected to fail, with a string that would be in the error message
		expectProjectSaved bool
		expectHistorySaved bool
		expectSessionSaved bool
		expectStderrOutput string // set with expected output to stderr
		expectStdoutOutput string // set with expected output to stdout
	}{
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
						Captures: map[string]morc.Scraper{
							"TEST": {Name: "TEST", Type: morc.SpecBodyOffset, OffsetStart: 18, OffsetEnd: 24},
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
						Captures: map[string]morc.Scraper{
							"TEST": {Name: "TEST", Type: morc.SpecBodyOffset, OffsetStart: 18, OffsetEnd: 24},
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
						Captures: map[string]morc.Scraper{
							"TEST": {Name: "TEST", Type: morc.SpecBodyJSON, Steps: []morc.TraversalStep{
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
						Captures: map[string]morc.Scraper{
							"TEST": {Name: "TEST", Type: morc.SpecBodyJSON, Steps: []morc.TraversalStep{
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
			name:   "send saves body captures - entire request",
			args:   []string{"send", "testreq"},
			respFn: respFnJSONBodyOK,
			p: morc.Project{
				Templates: map[string]morc.RequestTemplate{
					"testreq": {
						Name:   "testreq",
						Method: "GET",
						URL:    "/",
						Captures: map[string]morc.Scraper{
							"TEST": {Name: "TEST", Type: morc.SpecBodyOffset},
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
						Captures: map[string]morc.Scraper{
							"TEST": {Name: "TEST", Type: morc.SpecBodyOffset},
						},
					},
				},
				Vars: testVarStore("", map[string]map[string]string{
					"": {"TEST": `{"name":{"first":"VRISKA","last":"SERKET"}}`},
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
			name:   "send saves body captures - entire request -2",
			args:   []string{"send", "testreq"},
			respFn: respFnJSONBodyOK,
			p: morc.Project{
				Templates: map[string]morc.RequestTemplate{
					"testreq": {
						Name:   "testreq",
						Method: "GET",
						URL:    "/",
						Captures: map[string]morc.Scraper{
							"TEST": {Name: "TEST", Type: morc.SpecBodyOffset, OffsetEnd: -2},
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
						Captures: map[string]morc.Scraper{
							"TEST": {Name: "TEST", Type: morc.SpecBodyOffset, OffsetEnd: -2},
						},
					},
				},
				Vars: testVarStore("", map[string]map[string]string{
					"": {"TEST": `{"name":{"first":"VRISKA","last":"SERKET"`},
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
			name:   "print body captures",
			args:   []string{"send", "testreq", "--captures"},
			respFn: respFnJSONBodyOK,
			p: morc.Project{
				Templates: map[string]morc.RequestTemplate{
					"testreq": {
						Name:   "testreq",
						Method: "GET",
						URL:    "/",
						Captures: map[string]morc.Scraper{
							"TEST": {Name: "TEST", Type: morc.SpecBodyOffset, OffsetStart: 18, OffsetEnd: 24},
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
						Captures: map[string]morc.Scraper{
							"TEST": {Name: "TEST", Type: morc.SpecBodyOffset, OffsetStart: 18, OffsetEnd: 24},
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
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
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

			assert := NewAssertionsForInMemoryProject(t, tc.p, &fileRWs)
			resetSendFlags()

			// set up the root command and run
			output, outputErr, err := runTestCommand(sendCmd, assert.ProjFilePath, tc.args)

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
				assert.ProjectPersistedToBuffer(tc.expectP)
			} else {
				assert.NoProjectFileMutations()
			}

			if tc.expectHistorySaved {
				assert.HistoryPersistedToBuffer(tc.expectP.History)
			} else {
				assert.NoHistoryFileMutations()
			}

			if tc.expectSessionSaved {
				assert.SessionPersistedToBuffer(tc.expectP.Session)
			} else {
				assert.NoSessionFileMutations()
			}
		})
	}
}

func Test_Send_WithVars(t *testing.T) {
	respFnNoBodyOK := func(w http.ResponseWriter, r *http.Request) {
		// suppress date header
		w.Header()["Date"] = nil

		w.WriteHeader(http.StatusOK)
	}

	testCases := []struct {
		name               string
		args               []string // DO NOT INCLUDE -F; it is automatically set to a project file
		respFn             func(w http.ResponseWriter, r *http.Request)
		p                  morc.Project // endpoints are relative to some server; do not include host
		expectP            morc.Project
		expectErr          string // set if command.Execute expected to fail, with a string that would be in the error message
		expectProjectSaved bool
		expectHistorySaved bool
		expectSessionSaved bool
		expectStderrOutput string // set with expected output to stderr
		expectStdoutOutput string // set with expected output to stdout
	}{
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
		},
		{
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

			assert := NewAssertionsForInMemoryProject(t, tc.p, &fileRWs)
			resetSendFlags()

			// set up the root command and run
			output, outputErr, err := runTestCommand(sendCmd, assert.ProjFilePath, tc.args)

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
				assert.ProjectPersistedToBuffer(tc.expectP)
			} else {
				assert.NoProjectFileMutations()
			}

			if tc.expectHistorySaved {
				assert.HistoryPersistedToBuffer(tc.expectP.History)
			} else {
				assert.NoHistoryFileMutations()
			}

			if tc.expectSessionSaved {
				assert.SessionPersistedToBuffer(tc.expectP.Session)
			} else {
				assert.NoSessionFileMutations()
			}
		})
	}
}

func Test_Send_WithAuth(t *testing.T) {
	authExpTime := mustParseTime(time.RFC3339, time.Now().Add(1*time.Hour).UTC().Format(time.RFC3339))
	jwtKey := "testkey"
	testJWTClaims := testJWTData{Expiration: authExpTime}
	testTokenValue := "abc123xyz"

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
			name: "request requires cookie-based auth - history saved - no output for auth req",
			args: []string{"send", "resource", "--hide-auth"},
			respFn: serverHandler_withProtectedResource_session(
				Creds{User: "test", Pass: "TEsT123!"},
				&http.Cookie{Name: "session", Value: "ABCDEFG", Expires: authExpTime},
				testResource{Name: "VRISKA", Number: 8, Title: "Thief of Light"},
			),
			p: morc.Project{
				Templates: testRequests_withProtectedResource_session(Creds{"test", "TEsT123!"}, "testauth"),
				Auths:     testAuths(testAuth_session("testauth", seqTemplate, "login", "session", enableExpiration, nil)),
				Config: morc.Settings{
					HistFile:      "::PROJ_DIR::/history.json",
					RecordHistory: true,
				},
			},
			expectP: morc.Project{
				Templates: testRequests_withProtectedResource_session(Creds{"test", "TEsT123!"}, "testauth"),
				Auths:     testAuths(testAuth_session("testauth", seqTemplate, "login", "session", enableExpiration, testProof_session("session", "ABCDEFG", authExpTime))),
				History: []morc.HistoryEntry{
					{
						Template: "login",
						Request: &http.Request{
							Method:     "POST",
							URL:        mustParseURL("/login"),
							Proto:      "HTTP/1.1",
							ProtoMajor: 1,
							ProtoMinor: 1,
							Header: http.Header{
								"Content-Type": []string{"application/json"},
							},
							Body:          io.NopCloser(strings.NewReader(`{"user":"test","pass":"TEsT123!"}`)),
							ContentLength: 33,
						},
						Response: &http.Response{
							Status:     fmt.Sprintf("%d %s", http.StatusNoContent, http.StatusText(http.StatusNoContent)),
							StatusCode: http.StatusNoContent,
							Proto:      "HTTP/1.1",
							ProtoMajor: 1,
							ProtoMinor: 1,
							Header: http.Header{
								"Set-Cookie": []string{"session=ABCDEFG; Expires=" + authExpTime.Format(http.TimeFormat)},
							},
						},
						Initiator: morc.Initiator{
							Cause:  morc.CauseChain,
							Parent: "resource",
							Auth:   "testauth",
							Links: morc.HistoryLinks{
								Parent: 1,
							},
						},
					},
					{
						Template: "resource",
						Request: &http.Request{
							Method:     "GET",
							URL:        mustParseURL("/protected"),
							Proto:      "HTTP/1.1",
							ProtoMajor: 1,
							ProtoMinor: 1,
							Body:       http.NoBody,
							Header: http.Header{
								"Content-Type": []string{"application/json"},
								"Cookie":       []string{"session=ABCDEFG"},
							},
						},
						Response: &http.Response{
							Status:     fmt.Sprintf("%d %s", http.StatusOK, http.StatusText(http.StatusOK)),
							StatusCode: http.StatusOK,
							Proto:      "HTTP/1.1",
							ProtoMajor: 1,
							ProtoMinor: 1,
							Header: http.Header{
								"Content-Length": []string{"53"},
								"Content-Type":   []string{"application/json"},
							},
							Body:          io.NopCloser(strings.NewReader(`{"name":"VRISKA","number":8,"title":"Thief of Light"}`)),
							ContentLength: 53,
						},
						Initiator: morc.Initiator{
							Cause: morc.CauseTemplateSpecified,
						},
					},
				},
				Config: morc.Settings{
					HistFile:      "::PROJ_DIR::/history.json",
					RecordHistory: true,
				},
			},
			expectStdoutOutput: `HTTP/1.1 200 OK
{"name":"VRISKA","number":8,"title":"Thief of Light"}
`,
			expectProjectSaved: true,
			expectHistorySaved: true,
			expectSessionSaved: false,
		},
		{
			name: "request requires JWT auth - token extracted and used",
			args: []string{"send", "resource", "--hide-auth"},
			respFn: serverHandler_withProtectedResource_jwt(
				Creds{User: "test", Pass: "TEsT123!"},
				testJWTData{
					Expiration: time.Now().Add(1 * time.Hour),
					Subject:    "",
				},
				testResource{Name: "VRISKA", Number: 8, Title: "Thief of Light"},
				jwtKey,
			),
			p: morc.Project{
				Templates: testRequests_withProtectedResource_jwt(Creds{"test", "TEsT123!"}, "testauth"),
				Auths: map[string]morc.Auth{
					"testauth": testAuth_jwt("testauth", "login", "token"),
				},
				Config: morc.Settings{
					HistFile:      "::PROJ_DIR::/history.json",
					RecordHistory: true,
				},
			},
			expectP: morc.Project{
				Templates: testRequests_withProtectedResource_jwt(Creds{"test", "TEsT123!"}, "testauth"),
				Auths: map[string]morc.Auth{
					"testauth": testAuth_jwt("testauth", "login", "token", testProof_jwt(testJWTClaims, jwtKey)),
				},
				History: []morc.HistoryEntry{
					{
						Template: "login",
						Request: &http.Request{
							Method:     "POST",
							URL:        mustParseURL("/login"),
							Proto:      "HTTP/1.1",
							ProtoMajor: 1,
							ProtoMinor: 1,
							Header: http.Header{
								"Content-Type": []string{"application/json"},
							},
							Body:          io.NopCloser(strings.NewReader(`{"user":"test","pass":"TEsT123!"}`)),
							ContentLength: 33,
						},
						Response: &http.Response{
							Status:     fmt.Sprintf("%d %s", http.StatusOK, http.StatusText(http.StatusOK)),
							StatusCode: http.StatusOK,
							Proto:      "HTTP/1.1",
							ProtoMajor: 1,
							ProtoMinor: 1,
							Header: http.Header{
								"Content-Type":   []string{"application/json"},
								"Content-Length": []string{"130"},
							},
							Body:          io.NopCloser(strings.NewReader(`{"token":"` + testJWTClaims.Token(jwtKey) + `"}`)),
							ContentLength: 130,
						},
						Initiator: morc.Initiator{
							Cause:  morc.CauseChain,
							Parent: "resource",
							Auth:   "testauth",
							Links: morc.HistoryLinks{
								Parent: 1,
							},
						},
					},
					{
						Template: "resource",
						Request: &http.Request{
							Method:     "GET",
							URL:        mustParseURL("/protected"),
							Proto:      "HTTP/1.1",
							ProtoMajor: 1,
							ProtoMinor: 1,
							Body:       http.NoBody,
							Header: http.Header{
								"Content-Type":  []string{"application/json"},
								"Authorization": []string{"Bearer " + testJWTClaims.Token(jwtKey)},
							},
						},
						Response: &http.Response{
							Status:     fmt.Sprintf("%d %s", http.StatusOK, http.StatusText(http.StatusOK)),
							StatusCode: http.StatusOK,
							Proto:      "HTTP/1.1",
							ProtoMajor: 1,
							ProtoMinor: 1,
							Header: http.Header{
								"Content-Length": []string{"53"},
								"Content-Type":   []string{"application/json"},
							},
							Body:          io.NopCloser(strings.NewReader(`{"name":"VRISKA","number":8,"title":"Thief of Light"}`)),
							ContentLength: 53,
						},
						Initiator: morc.Initiator{
							Cause: morc.CauseTemplateSpecified,
						},
					},
				},
				Config: morc.Settings{
					HistFile:      "::PROJ_DIR::/history.json",
					RecordHistory: true,
				},
			},
			expectStdoutOutput: `HTTP/1.1 200 OK
{"name":"VRISKA","number":8,"title":"Thief of Light"}
`,
			expectProjectSaved: true,
			expectHistorySaved: true,
			expectSessionSaved: false,
		},
		{
			name: "request requires token auth - token extracted and used",
			args: []string{"send", "resource", "--hide-auth"},
			respFn: serverHandler_withProtectedResource_token(
				Creds{User: "test", Pass: "TEsT123!"},
				"abc123xyz", authExpTime,
				testResource{Name: "VRISKA", Number: 8, Title: "Thief of Light"},
			),
			p: morc.Project{
				Templates: testRequests_withProtectedResource_token(Creds{"test", "TEsT123!"}, "testauth"),
				Auths: map[string]morc.Auth{
					"testauth": testAuth_token("testauth", "login", "access_token"),
				},
				Config: morc.Settings{
					HistFile:      "::PROJ_DIR::/history.json",
					RecordHistory: true,
				},
			},
			expectP: morc.Project{
				Templates: testRequests_withProtectedResource_token(Creds{"test", "TEsT123!"}, "testauth"),
				Auths: map[string]morc.Auth{
					"testauth": testAuth_token("testauth", "login", "access_token", testProof_token(testTokenValue, authExpTime)),
				},
				History: []morc.HistoryEntry{
					{
						Template: "login",
						Request: &http.Request{
							Method:     "POST",
							URL:        mustParseURL("/login"),
							Proto:      "HTTP/1.1",
							ProtoMajor: 1,
							ProtoMinor: 1,
							Header: http.Header{
								"Content-Type": []string{"application/json"},
							},
							Body:          io.NopCloser(strings.NewReader(`{"user":"test","pass":"TEsT123!"}`)),
							ContentLength: 33,
						},
						Response: &http.Response{
							Status:     fmt.Sprintf("%d %s", http.StatusOK, http.StatusText(http.StatusOK)),
							StatusCode: http.StatusOK,
							Proto:      "HTTP/1.1",
							ProtoMajor: 1,
							ProtoMinor: 1,
							Header: http.Header{
								"Content-Type":   []string{"application/json"},
								"Content-Length": []string{"73"},
							},
							Body:          io.NopCloser(strings.NewReader(`{"access_token":"` + testTokenValue + `","expiration":"` + authExpTime.Format(time.RFC1123) + `"}`)),
							ContentLength: 73,
						},
						Initiator: morc.Initiator{
							Cause:  morc.CauseChain,
							Parent: "resource",
							Auth:   "testauth",
							Links: morc.HistoryLinks{
								Parent: 1,
							},
						},
					},
					{
						Template: "resource",
						Request: &http.Request{
							Method:     "GET",
							URL:        mustParseURL("/protected"),
							Proto:      "HTTP/1.1",
							ProtoMajor: 1,
							ProtoMinor: 1,
							Body:       http.NoBody,
							Header: http.Header{
								"Content-Type":  []string{"application/json"},
								"Authorization": []string{"Bearer " + testTokenValue},
							},
						},
						Response: &http.Response{
							Status:     fmt.Sprintf("%d %s", http.StatusOK, http.StatusText(http.StatusOK)),
							StatusCode: http.StatusOK,
							Proto:      "HTTP/1.1",
							ProtoMajor: 1,
							ProtoMinor: 1,
							Header: http.Header{
								"Content-Length": []string{"53"},
								"Content-Type":   []string{"application/json"},
							},
							Body:          io.NopCloser(strings.NewReader(`{"name":"VRISKA","number":8,"title":"Thief of Light"}`)),
							ContentLength: 53,
						},
						Initiator: morc.Initiator{
							Cause: morc.CauseTemplateSpecified,
						},
					},
				},
				Config: morc.Settings{
					HistFile:      "::PROJ_DIR::/history.json",
					RecordHistory: true,
				},
			},
			expectStdoutOutput: `HTTP/1.1 200 OK
{"name":"VRISKA","number":8,"title":"Thief of Light"}
`,
			expectProjectSaved: true,
			expectHistorySaved: true,
			expectSessionSaved: false,
		},
		{
			name: "request requires token auth - incorrect token triggers refresh",
			args: []string{"send", "resource", "--hide-auth"},
			respFn: serverHandler_withProtectedResource_token(
				Creds{User: "test", Pass: "TEsT123!"},
				"correct_token", authExpTime,
				testResource{Name: "VRISKA", Number: 8, Title: "Thief of Light"},
			),
			p: morc.Project{
				Templates: testRequests_withProtectedResource_token(Creds{"test", "TEsT123!"}, "testauth"),
				Auths: map[string]morc.Auth{
					"testauth": testAuth_token("testauth", "login", "access_token", testProof_token("incorrect_token", authExpTime)),
				},
				Config: morc.Settings{
					HistFile:      "::PROJ_DIR::/history.json",
					RecordHistory: true,
				},
			},
			expectP: morc.Project{
				Templates: testRequests_withProtectedResource_token(Creds{"test", "TEsT123!"}, "testauth"),
				Auths: map[string]morc.Auth{
					"testauth": testAuth_token("testauth", "login", "access_token", testProof_token("correct_token", authExpTime)),
				},
				History: []morc.HistoryEntry{
					{
						Template: "resource",
						Request: &http.Request{
							Method:     "GET",
							URL:        mustParseURL("/protected"),
							Proto:      "HTTP/1.1",
							ProtoMajor: 1,
							ProtoMinor: 1,
							Body:       http.NoBody,
							Header: http.Header{
								"Content-Type":  []string{"application/json"},
								"Authorization": []string{"Bearer incorrect_token"},
							},
						},
						Response: &http.Response{
							Status:     fmt.Sprintf("%d %s", http.StatusUnauthorized, http.StatusText(http.StatusUnauthorized)),
							StatusCode: http.StatusUnauthorized,
							Proto:      "HTTP/1.1",
							ProtoMajor: 1,
							ProtoMinor: 1,
							Header: http.Header{
								"Content-Length": []string{"0"},
							},
							Body:          http.NoBody,
							ContentLength: 0,
						},
						Initiator: morc.Initiator{
							Cause: morc.CauseTemplateSpecified,
						},
					},
					{
						Template: "login",
						Request: &http.Request{
							Method:     "POST",
							URL:        mustParseURL("/login"),
							Proto:      "HTTP/1.1",
							ProtoMajor: 1,
							ProtoMinor: 1,
							Header: http.Header{
								"Content-Type": []string{"application/json"},
							},
							Body:          io.NopCloser(strings.NewReader(`{"user":"test","pass":"TEsT123!"}`)),
							ContentLength: 33,
						},
						Response: &http.Response{
							Status:     fmt.Sprintf("%d %s", http.StatusOK, http.StatusText(http.StatusOK)),
							StatusCode: http.StatusOK,
							Proto:      "HTTP/1.1",
							ProtoMajor: 1,
							ProtoMinor: 1,
							Header: http.Header{
								"Content-Type":   []string{"application/json"},
								"Content-Length": []string{"77"},
							},
							Body:          io.NopCloser(strings.NewReader(`{"access_token":"correct_token","expiration":"` + authExpTime.Format(time.RFC1123) + `"}`)),
							ContentLength: 77,
						},
						Initiator: morc.Initiator{
							Cause:  morc.CauseChain,
							Parent: "resource",
							Auth:   "testauth",
							Links: morc.HistoryLinks{
								Parent: 2,
							},
						},
					},
					{
						Template: "resource",
						Request: &http.Request{
							Method:     "GET",
							URL:        mustParseURL("/protected"),
							Proto:      "HTTP/1.1",
							ProtoMajor: 1,
							ProtoMinor: 1,
							Body:       http.NoBody,
							Header: http.Header{
								"Content-Type":  []string{"application/json"},
								"Authorization": []string{"Bearer correct_token"},
							},
						},
						Response: &http.Response{
							Status:     fmt.Sprintf("%d %s", http.StatusOK, http.StatusText(http.StatusOK)),
							StatusCode: http.StatusOK,
							Proto:      "HTTP/1.1",
							ProtoMajor: 1,
							ProtoMinor: 1,
							Header: http.Header{
								"Content-Length": []string{"53"},
								"Content-Type":   []string{"application/json"},
							},
							Body:          io.NopCloser(strings.NewReader(`{"name":"VRISKA","number":8,"title":"Thief of Light"}`)),
							ContentLength: 53,
						},
						Initiator: morc.Initiator{
							Cause: morc.CauseTemplateSpecified,
						},
					},
				},
				Config: morc.Settings{
					HistFile:      "::PROJ_DIR::/history.json",
					RecordHistory: true,
				},
			},
			expectStdoutOutput: `HTTP/1.1 401 Unauthorized
(no response body)
Auth testauth failed for request resource
Retrying with newly-retrieved auth...
HTTP/1.1 200 OK
{"name":"VRISKA","number":8,"title":"Thief of Light"}
`,
			expectProjectSaved: true,
			expectHistorySaved: true,
			expectSessionSaved: false,
		},
		{
			name: "request requires token auth - expired token triggers refresh",
			args: []string{"send", "resource", "--hide-auth"},
			respFn: serverHandler_withProtectedResource_token(
				Creds{User: "test", Pass: "TEsT123!"},
				"new_token", authExpTime,
				testResource{Name: "VRISKA", Number: 8, Title: "Thief of Light"},
			),
			p: morc.Project{
				Templates: testRequests_withProtectedResource_token(Creds{"test", "TEsT123!"}, "testauth"),
				Auths: map[string]morc.Auth{
					"testauth": testAuth_token("testauth", "login", "access_token", testProof_token("expired_token", time.Now().Add(-1*time.Hour))),
				},
				Config: morc.Settings{
					HistFile:      "::PROJ_DIR::/history.json",
					RecordHistory: true,
				},
			},
			expectP: morc.Project{
				Templates: testRequests_withProtectedResource_token(Creds{"test", "TEsT123!"}, "testauth"),
				Auths: map[string]morc.Auth{
					"testauth": testAuth_token("testauth", "login", "access_token", testProof_token("new_token", authExpTime)),
				},
				History: []morc.HistoryEntry{
					{
						Template: "login",
						Request: &http.Request{
							Method:     "POST",
							URL:        mustParseURL("/login"),
							Proto:      "HTTP/1.1",
							ProtoMajor: 1,
							ProtoMinor: 1,
							Header: http.Header{
								"Content-Type": []string{"application/json"},
							},
							Body:          io.NopCloser(strings.NewReader(`{"user":"test","pass":"TEsT123!"}`)),
							ContentLength: 33,
						},
						Response: &http.Response{
							Status:     fmt.Sprintf("%d %s", http.StatusOK, http.StatusText(http.StatusOK)),
							StatusCode: http.StatusOK,
							Proto:      "HTTP/1.1",
							ProtoMajor: 1,
							ProtoMinor: 1,
							Header: http.Header{
								"Content-Type":   []string{"application/json"},
								"Content-Length": []string{"73"},
							},
							Body:          io.NopCloser(strings.NewReader(fmt.Sprintf(`{"access_token":"new_token","expiration":%q}`, authExpTime.Format(time.RFC1123)))),
							ContentLength: 73,
						},
						Initiator: morc.Initiator{
							Cause:  morc.CauseChain,
							Parent: "resource",
							Auth:   "testauth",
							Links: morc.HistoryLinks{
								Parent: 1,
							},
						},
					},
					{
						Template: "resource",
						Request: &http.Request{
							Method:     "GET",
							URL:        mustParseURL("/protected"),
							Proto:      "HTTP/1.1",
							ProtoMajor: 1,
							ProtoMinor: 1,
							Body:       http.NoBody,
							Header: http.Header{
								"Content-Type":  []string{"application/json"},
								"Authorization": []string{"Bearer new_token"},
							},
						},
						Response: &http.Response{
							Status:     fmt.Sprintf("%d %s", http.StatusOK, http.StatusText(http.StatusOK)),
							StatusCode: http.StatusOK,
							Proto:      "HTTP/1.1",
							ProtoMajor: 1,
							ProtoMinor: 1,
							Header: http.Header{
								"Content-Length": []string{"53"},
								"Content-Type":   []string{"application/json"},
							},
							Body:          io.NopCloser(strings.NewReader(`{"name":"VRISKA","number":8,"title":"Thief of Light"}`)),
							ContentLength: 53,
						},
						Initiator: morc.Initiator{
							Cause: morc.CauseTemplateSpecified,
						},
					},
				},
				Config: morc.Settings{
					HistFile:      "::PROJ_DIR::/history.json",
					RecordHistory: true,
				},
			},
			expectStdoutOutput: `HTTP/1.1 200 OK
{"name":"VRISKA","number":8,"title":"Thief of Light"}
`,
			expectProjectSaved: true,
			expectHistorySaved: true,
			expectSessionSaved: false,
		},
		{
			name: "request with valid HTTP Basic auth succeeds",
			args: []string{"send", "resource"},
			respFn: serverHandler_withProtectedResource_basic(
				Creds{User: "ectoBiologist", Pass: "letmein"},
				testResource{Name: "VRISKA", Number: 8, Title: "Thief of Light"},
			),
			p: morc.Project{
				Templates: testRequests_withProtectedResource_basic("testauth"),
				Auths:     testAuths(testAuth_basic("testauth", "ectoBiologist", "letmein")),
				Config: morc.Settings{
					HistFile:      "::PROJ_DIR::/history.json",
					RecordHistory: true,
				},
			},
			expectP: morc.Project{
				Templates: testRequests_withProtectedResource_basic("testauth"),
				Auths:     testAuths(testAuth_basic("testauth", "ectoBiologist", "letmein")),
				History: []morc.HistoryEntry{
					{
						Template: "resource",
						Request: &http.Request{
							Method:     "GET",
							URL:        mustParseURL("/protected"),
							Proto:      "HTTP/1.1",
							ProtoMajor: 1,
							ProtoMinor: 1,
							Body:       http.NoBody,
							Header: http.Header{
								"Content-Type":  []string{"application/json"},
								"Authorization": []string{"Basic " + base64.StdEncoding.EncodeToString([]byte("ectoBiologist:letmein"))},
							},
						},
						Response: &http.Response{
							Status:     fmt.Sprintf("%d %s", http.StatusOK, http.StatusText(http.StatusOK)),
							StatusCode: http.StatusOK,
							Proto:      "HTTP/1.1",
							ProtoMajor: 1,
							ProtoMinor: 1,
							Header: http.Header{
								"Content-Length": []string{"53"},
								"Content-Type":   []string{"application/json"},
							},
							Body:          io.NopCloser(strings.NewReader(`{"name":"VRISKA","number":8,"title":"Thief of Light"}`)),
							ContentLength: 53,
						},
						Initiator: morc.Initiator{
							Cause: morc.CauseTemplateSpecified,
						},
					},
				},
				Config: morc.Settings{
					HistFile:      "::PROJ_DIR::/history.json",
					RecordHistory: true,
				},
			},
			expectStdoutOutput: `HTTP/1.1 200 OK
{"name":"VRISKA","number":8,"title":"Thief of Light"}
`,
			expectProjectSaved: false,
			expectHistorySaved: true,
			expectSessionSaved: false,
		},
		{
			name: "request with invalid HTTP Basic auth fails",
			args: []string{"send", "resource"},
			respFn: serverHandler_withProtectedResource_basic(
				Creds{User: "ectoBiologist", Pass: "letmein"},
				testResource{Name: "VRISKA", Number: 8, Title: "Thief of Light"},
			),
			p: morc.Project{
				Templates: testRequests_withProtectedResource_basic("testauth"),
				Auths:     testAuths(testAuth_basic("testauth", "ectoBiologist", "wrongpass")),
				Config: morc.Settings{
					HistFile:      "::PROJ_DIR::/history.json",
					RecordHistory: true,
				},
			},
			expectP: morc.Project{
				Templates: testRequests_withProtectedResource_basic("testauth"),
				Auths:     testAuths(testAuth_basic("testauth", "ectoBiologist", "wrongpass")),
				History: []morc.HistoryEntry{
					{
						Template: "resource",
						Request: &http.Request{
							Method:     "GET",
							URL:        mustParseURL("/protected"),
							Proto:      "HTTP/1.1",
							ProtoMajor: 1,
							ProtoMinor: 1,
							Body:       http.NoBody,
							Header: http.Header{
								"Content-Type":  []string{"application/json"},
								"Authorization": []string{"Basic " + base64.StdEncoding.EncodeToString([]byte("ectoBiologist:wrongpass"))},
							},
						},
						Response: &http.Response{
							Status:     fmt.Sprintf("%d %s", http.StatusUnauthorized, http.StatusText(http.StatusUnauthorized)),
							StatusCode: http.StatusUnauthorized,
							Proto:      "HTTP/1.1",
							ProtoMajor: 1,
							ProtoMinor: 1,
							Header: http.Header{
								"Content-Length":   []string{"0"},
								"Www-Authenticate": []string{`Basic realm="test"`},
							},
							Body:          http.NoBody,
							ContentLength: 0,
						},
						Initiator: morc.Initiator{
							Cause: morc.CauseTemplateSpecified,
						},
					},
				},
				Config: morc.Settings{
					HistFile:      "::PROJ_DIR::/history.json",
					RecordHistory: true,
				},
			},
			expectStdoutOutput: `HTTP/1.1 401 Unauthorized
(no response body)
Auth testauth failed for request resource
Auth is static and cannot be refreshed; not retrying
`,
			expectProjectSaved: false,
			expectHistorySaved: true,
			expectSessionSaved: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
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

			assert := NewAssertionsForInMemoryProject(t, tc.p, &fileRWs)
			resetSendFlags()

			// set up the root command and run
			output, outputErr, err := runTestCommand(sendCmd, assert.ProjFilePath, tc.args)

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
				assert.ProjectPersistedToBuffer(tc.expectP)
			} else {
				assert.NoProjectFileMutations()
			}

			if tc.expectHistorySaved {
				assert.HistoryPersistedToBuffer(tc.expectP.History)
			} else {
				assert.NoHistoryFileMutations()
			}

			if tc.expectSessionSaved {
				assert.SessionPersistedToBuffer(tc.expectP.Session)
			} else {
				assert.NoSessionFileMutations()
			}
		})
	}
}

func resetSendFlags() {
	flags.ProjectFile = ""
	flags.Vars = nil
	flags.BInsecure = false
	flags.VarPrefix = "$"
	flags.BQuiet = false
	flags.resetOutputControl()

	sendCmd.Flags().VisitAll(func(fl *pflag.Flag) {
		fl.Changed = false
	})
}
