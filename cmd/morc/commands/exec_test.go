package commands

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dekarrin/morc"
	"github.com/dekarrin/morc/cmd/morc/cmdio"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
)

func Test_Exec_Auth(t *testing.T) {
	respFnNoContentWithBasicAuth := func(user, pass string) func(w http.ResponseWriter, r *http.Request) {
		return func(w http.ResponseWriter, r *http.Request) {
			// suppress date header
			w.Header()["Date"] = nil

			u, p, ok := r.BasicAuth()
			if !ok || u != user || p != pass {
				w.Header().Set("WWW-Authenticate", `Basic realm="test"`)
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			w.WriteHeader(http.StatusNoContent)
		}
	}

	cookieExpTime := time.Now().Add(24 * time.Hour)

	respFnCookieLogin := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/login" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
		}

		type creds struct {
			User string `json:"user"`
			Pass string `json:"pass"`
		}

		var c creds
		if err := json.Unmarshal(bodyBytes, &c); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if c.User != "ectoBiologist" || c.Pass != "ghostbusters3" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:    "session",
			Value:   "123456",
			Expires: cookieExpTime,
		})
		w.WriteHeader(http.StatusNoContent)
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
			name:   "basic auth",
			args:   []string{"exec", "auth1", "-a"},
			respFn: respFnNoContentWithBasicAuth("doesntmatter", "nothittingendpoint"),
			p: morc.Project{
				Auths: map[string]morc.Auth{
					"auth1": {
						Name: "auth1",
						Type: morc.AuthTypeHTTPBasic,
						Proof: morc.HTTPBasicCredentials{
							Username: "ectoBiologist",
							Password: "letmein",
						},
					},
				},
			},
			expectP: morc.Project{
				Auths: map[string]morc.Auth{
					"auth1": {
						Name: "auth1",
						Type: morc.AuthTypeHTTPBasic,
						Proof: morc.HTTPBasicCredentials{
							Username: "ectoBiologist",
							Password: "letmein",
						},
					},
				},
			},
			expectProjectSaved: false,
			expectHistorySaved: false,
			expectSessionSaved: false,
			expectStdoutOutput: "Got auth after 0 requests\nAuth proof: ectoBiologist:letmein\n(no expiration)\n",
		},
		{
			name:   "session cookie login - no initial, request sequence, detect expiration",
			args:   []string{"exec", "auth1", "-a"},
			respFn: respFnCookieLogin,
			p: morc.Project{
				Auths: map[string]morc.Auth{
					"auth1": {
						Name: "auth1",
						Type: morc.AuthTypeSession,
						Fetcher: morc.NewSessionCookieFetcher(
							morc.RequestSequence{Name: "log-me-in"},
							"session",
							true,
						),
					},
				},
				Templates: map[string]morc.RequestTemplate{
					"log-me-in": {
						Name:    "testreq",
						Method:  "POST",
						URL:     "/login",
						Body:    []byte(`{"user":"ectoBiologist","pass":"ghostbusters3"}`),
						Headers: http.Header{"Content-Type": []string{"application/json"}},
					},
				},
			},
			expectP: morc.Project{
				Auths: map[string]morc.Auth{
					"auth1": {
						Name: "auth1",
						Type: morc.AuthTypeSession,
						Fetcher: morc.NewSessionCookieFetcher(
							morc.RequestSequence{Name: "log-me-in"},
							"session",
							true,
						),
						Proof: morc.DynamicProof{
							Value:     "123456",
							ExpiresAt: cookieExpTime,
							Dest: morc.ProofDestination{
								Location: morc.ProofLocationCookie,
								Key:      "session",
							},
						},
					},
				},
				Templates: map[string]morc.RequestTemplate{
					"log-me-in": {
						Name:    "testreq",
						Method:  "POST",
						URL:     "/login",
						Body:    []byte(`{"user":"ectoBiologist","pass":"ghostbusters3"}`),
						Headers: http.Header{"Content-Type": []string{"application/json"}},
					},
				},
			},
			expectProjectSaved: true,
			expectHistorySaved: false,
			expectSessionSaved: false,
			expectStdoutOutput: "Got auth after 0 requests\nAuth proof: ectoBiologist:letmein\n(no expiration)\n",
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

			resetExecFlags()

			// create project and dump config to a temp dir
			projFilePath := createTestProjectIO(t, tc.p)
			// set up the root command and run
			output, outputErr, err := runTestCommand(execCmd, projFilePath, tc.args)

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

func resetExecFlags() {
	flags.ProjectFile = ""
	flags.Vars = nil
	flags.BInsecure = false
	flags.VarPrefix = "$"
	flags.BQuiet = false
	flags.BAuth = false
	flags.BHeaders = false
	flags.BCaptures = false
	flags.BNoBody = false
	flags.BRequest = false
	flags.Format = "pretty" // TODO: make this default not be magic but rather have the cmd flag init and the reset use it

	execCmd.Flags().VisitAll(func(fl *pflag.Flag) {
		fl.Changed = false
	})
}
