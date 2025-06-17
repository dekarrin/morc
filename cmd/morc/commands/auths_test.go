package commands

import (
	"strings"
	"testing"
	"time"

	"github.com/dekarrin/morc"
	"github.com/spf13/pflag"
)

func Test_Auths_Delete(t *testing.T) {
	testCases := []struct {
		name               string
		args               []string // DO NOT INCLUDE -F; it is automatically set to a project file
		p                  morc.Project
		expectP            morc.Project
		expectErr          string // set if command.Execute expected to fail, with a string that would be in the error message
		expectStderrOutput string // set with expected output to stderr
		expectStdoutOutput string // set with expected output to stdout
	}{
		{
			name:      "no auths present - empty project",
			args:      []string{"auths", "-D", "auth1"},
			p:         morc.Project{},
			expectErr: "no auth method named auth1 exists in project",
		},
		{
			name:      "no auths present - empty Auths",
			args:      []string{"auths", "-D", "auth1"},
			p:         morc.Project{Auths: map[string]morc.Auth{}},
			expectErr: "no auth method named auth1 exists in project",
		},
		{
			name:      "delete needs value",
			args:      []string{"auths", "-D"},
			p:         testProject_singleAuth(),
			expectErr: "flag needs an argument: 'D'",
		},
		{
			name: "normal delete",
			args: []string{"auths", "-D", "auth1"},
			p:    testProject_singleAuth(),
			expectP: morc.Project{
				Auths: map[string]morc.Auth{},
			},
			expectStdoutOutput: "Deleted auth method auth1\n",
		},
		{
			name: "normal delete, quiet mode",
			args: []string{"auths", "-D", "auth1", "-q"},
			p:    testProject_singleAuth(),
			expectP: morc.Project{
				Auths: map[string]morc.Auth{},
			},
			expectStdoutOutput: "",
		},
		{
			name: "attached to a request - can't delete",
			args: []string{"auths", "-D", "auth1"},
			p: morc.Project{
				Templates: map[string]morc.RequestTemplate{"req1": testRequest_withAllPropertiesSet()},
				Auths: map[string]morc.Auth{
					"auth1": testAuth_basic("auth1", "user", "pass"),
				},
			},
			expectErr: "auth1 is used in request template req1\nUse -f to force-delete",
		},
		{
			name: "attached to a request - delete with force",
			args: []string{"auths", "-D", "auth1", "-f"},
			p: morc.Project{
				Templates: map[string]morc.RequestTemplate{"req1": testRequest_withAllPropertiesSet()},
				Auths: map[string]morc.Auth{
					"auth1": testAuth_basic("auth1", "user", "pass"),
				},
			},
			expectP: morc.Project{
				Templates: map[string]morc.RequestTemplate{"req1": testRequest_withAllPropertiesSet()},
				Auths:     map[string]morc.Auth{},
			},
			expectStdoutOutput: "Deleted auth method auth1\n",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert := NewAssertionsForInMemoryProject(t, tc.p, &fileRWs)
			resetAuthsFlags()

			// set up the root command and run
			output, outputErr, err := runTestCommand(authsCmd, assert.ProjFilePath, tc.args)

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
			}

			// assertions

			assert.Equal(tc.expectStdoutOutput, output, "stdout output mismatch")
			assert.Equal(tc.expectStderrOutput, outputErr, "stderr output mismatch")

			assert.ProjectFilesInBuffersMatch(tc.expectP)
		})
	}
}

// TODO: these are totally incomplete, we need to add cases to match the get tests.
func Test_Auths_Edit(t *testing.T) {
	testCases := []struct {
		name               string
		args               []string // DO NOT INCLUDE -F; it is automatically set to a project file
		p                  morc.Project
		expectP            morc.Project
		expectErr          string // set if command.Execute expected to fail, with a string that would be in the error message
		expectStderrOutput string // set with expected output to stderr
		expectStdoutOutput string // set with expected output to stdout
	}{
		{
			name:      "edit non-existent auth",
			args:      []string{"auths", "non-existent", "-u", "new-user"},
			p:         testProject_singleAuth(),
			expectErr: "no auth method named non-existent exists in project",
		},
		{
			name:               "set basic auth name",
			args:               []string{"auths", "auth1", "-n", "auth2"},
			p:                  testProject_withAuths(testAuth_basic("auth1", "user", "pass")),
			expectP:            testProject_withAuths(testAuth_basic("auth2", "user", "pass")),
			expectStdoutOutput: "Set auth method name to auth2\n",
		},
		{
			name:               "set session auth name",
			args:               []string{"auths", "auth1", "-n", "auth2"},
			p:                  testProject_withAuths(testAuth_session("auth1", seqFlow, "get-sess", "SESSID", enableExpiration, nil)),
			expectP:            testProject_withAuths(testAuth_session("auth2", seqFlow, "get-sess", "SESSID", enableExpiration, nil)),
			expectStdoutOutput: "Set auth method name to auth2\n",
		},
		{
			name:               "set jwt auth name",
			args:               []string{"auths", "auth1", "-n", "auth2"},
			p:                  testProject_withAuths(testAuth_jwt("auth1", "get-sess", "SESSID")),
			expectP:            testProject_withAuths(testAuth_jwt("auth2", "get-sess", "SESSID")),
			expectStdoutOutput: "Set auth method name to auth2\n",
		},
		{
			name:               "set token auth name",
			args:               []string{"auths", "auth1", "-n", "auth2"},
			p:                  testProject_withAuths(testAuth_token("auth1", "get-sess", "SESSID", enableExpiration)),
			expectP:            testProject_withAuths(testAuth_token("auth2", "get-sess", "SESSID", enableExpiration)),
			expectStdoutOutput: "Set auth method name to auth2\n",
		},
		{
			name:               "set basic auth type to session",
			args:               []string{"auths", "auth1", "-T", "session"},
			p:                  testProject_withAuths(testAuth_basic("auth1", "user", "pass")),
			expectP:            testProject_withAuths(morc.Auth{Name: "auth1", Type: morc.AuthTypeSession}),
			expectStdoutOutput: "Set auth method type to session\n",
		},
		{
			name:               "set basic auth type to session, and set cookie",
			args:               []string{"auths", "auth1", "-T", "session", "-c", "LOGIN_SESSION"},
			p:                  testProject_withAuths(testAuth_basic("auth1", "user", "pass")),
			expectP:            testProject_withAuths(morc.Auth{Name: "auth1", Type: morc.AuthTypeSession, Fetcher: morc.NewSessionCookieFetcher(morc.RequestSequence{}, "LOGIN_SESSION", false)}),
			expectStdoutOutput: "Set auth method type to session and session cookie to LOGIN_SESSION\n",
		},
		{
			name:      "set basic auth type to session, and set username fails",
			args:      []string{"auths", "auth1", "-T", "session", "-u", "ectoBiologist"},
			p:         testProject_withAuths(testAuth_basic("auth1", "user", "pass")),
			expectErr: "--username/-u is not a valid option for auth type \"session\"",
		},
		{
			name:               "set basic auth type to jwt",
			args:               []string{"auths", "auth1", "-T", "jwt"},
			p:                  testProject_withAuths(testAuth_basic("auth1", "user", "pass")),
			expectP:            testProject_withAuths(morc.Auth{Name: "auth1", Type: morc.AuthTypeJWT}),
			expectStdoutOutput: "Set auth method type to jwt\n",
		},
		{
			name:               "set basic auth type to token",
			args:               []string{"auths", "auth1", "-T", "token"},
			p:                  testProject_withAuths(testAuth_basic("auth1", "user", "pass")),
			expectP:            testProject_withAuths(morc.Auth{Name: "auth1", Type: morc.AuthTypeToken}),
			expectStdoutOutput: "Set auth method type to token\n",
		},
		{
			name:               "set token auth type to basic",
			args:               []string{"auths", "auth1", "-T", "basic"},
			p:                  testProject_withAuths(testAuth_token("auth1", "get-sess", "SESSID", enableExpiration)),
			expectP:            testProject_withAuths(morc.Auth{Name: "auth1", Type: morc.AuthTypeHTTPBasic}),
			expectStdoutOutput: "Set auth method type to basic\n",
		},
		// TODO: add tests for setting auth types.
		{
			name:               "set basic auth username",
			args:               []string{"auths", "auth1", "-u", "ectoBiologist"},
			p:                  testProject_withAuths(testAuth_basic("auth1", "user", "pass")),
			expectP:            testProject_withAuths(testAuth_basic("auth1", "ectoBiologist", "pass")),
			expectStdoutOutput: "Set basic auth username to ectoBiologist\n",
		},
		{
			name:      "set session auth username fails",
			args:      []string{"auths", "auth1", "-u", "ectoBiologist"},
			p:         testProject_withAuths(testAuth_session("auth1", seqFlow, "get-sess", "SESSID", enableExpiration, nil)),
			expectErr: "--username/-u is not a valid option for auth type \"session\"",
		},
		{
			name:      "set jwt auth username fails",
			args:      []string{"auths", "auth1", "-u", "ectoBiologist"},
			p:         testProject_withAuths(testAuth_jwt("auth1", "get-sess", "SESSID")),
			expectErr: "--username/-u is not a valid option for auth type \"jwt\"",
		},
		{
			name:      "set token auth username fails",
			args:      []string{"auths", "auth1", "-u", "ectoBiologist"},
			p:         testProject_withAuths(testAuth_token("auth1", "get-sess", "SESSID", enableExpiration)),
			expectErr: "--username/-u is not a valid option for auth type \"token\"",
		},
		{
			name:               "set basic auth password",
			args:               []string{"auths", "auth1", "-p", "vriskaa"},
			p:                  testProject_withAuths(testAuth_basic("auth1", "user", "pass")),
			expectP:            testProject_withAuths(testAuth_basic("auth1", "user", "vriskaa")),
			expectStdoutOutput: "Set basic auth password to *******\n",
		},
		{
			name:               "set basic auth password unmasked",
			args:               []string{"auths", "auth1", "-p", "vriskaa", "--unmask"},
			p:                  testProject_withAuths(testAuth_basic("auth1", "user", "pass")),
			expectP:            testProject_withAuths(testAuth_basic("auth1", "user", "vriskaa")),
			expectStdoutOutput: "Set basic auth password to vriskaa\n",
		},
		{
			name:      "set session auth password fails",
			args:      []string{"auths", "auth1", "-p", "vriskaa"},
			p:         testProject_withAuths(testAuth_session("auth1", seqFlow, "get-sess", "SESSID", enableExpiration, nil)),
			expectErr: "--password/-p is not a valid option for auth type \"session\"",
		},
		{
			name:      "set jwt auth password fails",
			args:      []string{"auths", "auth1", "-p", "vriskaa"},
			p:         testProject_withAuths(testAuth_jwt("auth1", "get-sess", "SESSID")),
			expectErr: "--password/-p is not a valid option for auth type \"jwt\"",
		},
		{
			name:      "set token auth password fails",
			args:      []string{"auths", "auth1", "-p", "vriskaa"},
			p:         testProject_withAuths(testAuth_token("auth1", "get-sess", "SESSID", enableExpiration)),
			expectErr: "--password/-p is not a valid option for auth type \"token\"",
		},
		{
			name:      "set basic auth cookie fails",
			args:      []string{"auths", "auth1", "-c", "LOGIN_SESSION"},
			p:         testProject_withAuths(testAuth_basic("auth1", "user", "pass")),
			expectErr: `--cookie/-c is not a valid option for auth type "basic"`,
		},
		{
			name:               "set session auth cookie",
			args:               []string{"auths", "auth1", "-c", "LOGIN_SESSION"},
			p:                  testProject_withAuths(testAuth_session("auth1", seqFlow, "get-sess", "SESSID", enableExpiration, nil)),
			expectP:            testProject_withAuths(testAuth_session("auth1", seqFlow, "get-sess", "LOGIN_SESSION", enableExpiration, nil)),
			expectStdoutOutput: "Set session cookie to LOGIN_SESSION\n",
		},
		{
			name:      "set jwt auth cookie fails",
			args:      []string{"auths", "auth1", "-c", "LOGIN_SESSION"},
			p:         testProject_withAuths(testAuth_jwt("auth1", "get-sess", "SESSID")),
			expectErr: `--cookie/-c is not a valid option for auth type "jwt"`,
		},
		{
			name:      "set token auth cookie fails",
			args:      []string{"auths", "auth1", "-c", "LOGIN_SESSION"},
			p:         testProject_withAuths(testAuth_token("auth1", "get-sess", "SESSID", enableExpiration)),
			expectErr: `--cookie/-c is not a valid option for auth type "token"`,
		},
		{
			name:      "disable basic auth expiration detection fails",
			args:      []string{"auths", "auth1", "--no-exp"},
			p:         testProject_withAuths(testAuth_basic("auth1", "user", "pass")),
			expectErr: `--no-exp is not a valid option for auth type "basic"`,
		},
		{
			name:               "disable session auth expiration detection",
			args:               []string{"auths", "auth1", "--no-exp"},
			p:                  testProject_withAuths(testAuth_session("auth1", seqFlow, "get-sess", "SESSID", enableExpiration, nil)),
			expectP:            testProject_withAuths(testAuth_session("auth1", seqFlow, "get-sess", "SESSID", disableExpiration, nil)),
			expectStdoutOutput: "Set auth proof expiration detection to OFF\n",
		},
		{
			name:      "disable jwt auth expiration detection fails",
			args:      []string{"auths", "auth1", "--no-exp"},
			p:         testProject_withAuths(testAuth_jwt("auth1", "get-sess", "SESSID")),
			expectErr: `--no-exp is not a valid option for auth type "jwt"`,
		},
		{
			name:               "disable token auth expiration detection",
			args:               []string{"auths", "auth1", "--no-exp"},
			p:                  testProject_withAuths(testAuth_token("auth1", "get-sess", "SESSID", enableExpiration)),
			expectP:            testProject_withAuths(testAuth_token("auth1", "get-sess", "SESSID", disableExpiration)),
			expectStdoutOutput: "Set auth proof expiration detection to OFF\n",
		},
		{
			name:      "enable basic auth expiration detection fails",
			args:      []string{"auths", "auth1", "--exp"},
			p:         testProject_withAuths(testAuth_basic("auth1", "user", "pass")),
			expectErr: `--exp is not a valid option for auth type "basic"`,
		},
		{
			name:               "enable session auth expiration detection",
			args:               []string{"auths", "auth1", "--exp"},
			p:                  testProject_withAuths(testAuth_session("auth1", seqFlow, "get-sess", "SESSID", disableExpiration, nil)),
			expectP:            testProject_withAuths(testAuth_session("auth1", seqFlow, "get-sess", "SESSID", enableExpiration, nil)),
			expectStdoutOutput: "Set auth proof expiration detection to ON\n",
		},
		{
			name:      "enable jwt auth expiration detection fails",
			args:      []string{"auths", "auth1", "--exp"},
			p:         testProject_withAuths(testAuth_jwt("auth1", "get-sess", "SESSID")),
			expectErr: `--exp is not a valid option for auth type "jwt"`,
		},
		{
			name:      "enable token auth expiration detection fails with auto-conf --exp flag",
			args:      []string{"auths", "auth1", "--exp"},
			p:         testProject_withAuths(testAuth_token("auth1", "get-sess", "SESSID", disableExpiration)),
			expectErr: `--exp is not a valid option for auth type "token"`,
		},
		{
			name:      "set basic auth retrieval fails",
			args:      []string{"auths", "auth1", "-r", "F:flow1"},
			p:         testProject_withAuths(testAuth_basic("auth1", "user", "pass")),
			expectErr: `--retrieval/-r is not a valid option for auth type "basic"`,
		},
		{
			name:               "set session auth retrieval",
			args:               []string{"auths", "auth1", "-r", "F:flow1"},
			p:                  testProject_withAuths(testAuth_session("auth1", seqTemplate, "get-sess", "SESSID", enableExpiration, nil)),
			expectP:            testProject_withAuths(testAuth_session("auth1", seqFlow, "flow1", "SESSID", enableExpiration, nil)),
			expectStdoutOutput: "Set auth proof retrieval sequence to F:flow1\n",
		},
		{
			name:               "set jwt auth retrieval",
			args:               []string{"auths", "auth1", "-r", "R:req1"},
			p:                  testProject_withAuths(testAuth_jwt("auth1", "get-sess", "SESSID")),
			expectP:            testProject_withAuths(testAuth_jwt("auth1", "req1", "SESSID")),
			expectStdoutOutput: "Set auth proof retrieval sequence to R:req1\n",
		},
		{
			name:               "set token auth retrieval",
			args:               []string{"auths", "auth1", "-r", "R:req1"},
			p:                  testProject_withAuths(testAuth_token("auth1", "get-sess", "SESSID", enableExpiration)),
			expectP:            testProject_withAuths(testAuth_token("auth1", "req1", "SESSID", enableExpiration)),
			expectStdoutOutput: "Set auth proof retrieval sequence to R:req1\n",
		},
		{
			name:      "set basic auth token scraper fails",
			args:      []string{"auths", "auth1", "-t", ".token"},
			p:         testProject_withAuths(testAuth_basic("auth1", "user", "pass")),
			expectErr: `--token-scraper/-t is not a valid option for auth type "basic"`,
		},
		{
			name:      "set session auth token scraper fails",
			args:      []string{"auths", "auth1", "-t", ".token"},
			p:         testProject_withAuths(testAuth_session("auth1", seqFlow, "get-sess", "SESSID", enableExpiration, nil)),
			expectErr: `--token-scraper/-t is not a valid option for auth type "session"`,
		},
		{
			name: "set jwt auth token scraper to JSON path",
			args: []string{"auths", "auth1", "-t", ".other.token"},
			p:    testProject_withAuths(testAuth_jwt("auth1", "get-sess", "TOKEN")),
			expectP: testProject_withAuths(testAuth_jwt_withTokenScraper("auth1", "get-sess", morc.Scraper{
				Name:  "TOKEN",
				Type:  morc.SpecBodyJSON,
				Steps: []morc.TraversalStep{{Key: "other"}, {Key: "token"}},
			})),
			expectStdoutOutput: "Set token scraper spec to .other.token\n",
		},
		{
			name: "set jwt auth token scraper to offset",
			args: []string{"auths", "auth1", "-t", "bytes:25,35"},
			p:    testProject_withAuths(testAuth_jwt("auth1", "get-sess", "TOKEN")),
			expectP: testProject_withAuths(testAuth_jwt_withTokenScraper("auth1", "get-sess", morc.Scraper{
				Name:        "TOKEN",
				Type:        morc.SpecBodyOffset,
				OffsetStart: 25,
				OffsetEnd:   35,
			})),
			expectStdoutOutput: "Set token scraper spec to offset 25,35\n",
		},
		{
			name: "set jwt auth token scraper to cookie",
			args: []string{"auths", "auth1", "-t", "cookie:TOKEN_NAME"},
			p:    testProject_withAuths(testAuth_jwt("auth1", "get-sess", "TOKEN")),
			expectP: testProject_withAuths(testAuth_jwt_withTokenScraper("auth1", "get-sess", morc.Scraper{
				Name:             "TOKEN",
				Type:             morc.SpecCookie,
				CookieName:       "TOKEN_NAME",
				CookieExpiration: true,
			})),
			expectStdoutOutput: "Set token scraper spec to cookie TOKEN_NAME (with expiration)\n",
		},
		{
			name: "set jwt auth token scraper to header",
			args: []string{"auths", "auth1", "-t", "header:X-API-KEY"},
			p:    testProject_withAuths(testAuth_jwt("auth1", "get-sess", "TOKEN")),
			expectP: testProject_withAuths(testAuth_jwt_withTokenScraper("auth1", "get-sess", morc.Scraper{
				Name: "TOKEN",
				Type: morc.SpecHeader,
				Key:  "X-API-KEY",
			})),
			expectStdoutOutput: "Set token scraper spec to header X-API-KEY[0]\n",
		},
		{
			name: "set token auth token scraper to header",
			args: []string{"auths", "auth1", "-t", "header:X-API-KEY"},
			p:    testProject_withAuths(testAuth_token("auth1", "get-sess", "TOKEN", enableExpiration)),
			expectP: testProject_withAuths(testAuth_token_withTokenScraper("auth1", "get-sess", morc.Scraper{
				Name: "TOKEN",
				Type: morc.SpecHeader,
				Key:  "X-API-KEY",
			}, enableExpiration)),
			expectStdoutOutput: "Set token scraper spec to header X-API-KEY[0]\n",
		},
		{
			name:      "set basic auth dest fails",
			args:      []string{"auths", "auth1", "-d", "header:X-API-KEY"},
			p:         testProject_withAuths(testAuth_basic("auth1", "user", "pass")),
			expectErr: `--dest/-d is not a valid option for auth type "basic"`,
		},
		{
			name:      "set session auth dest fails",
			args:      []string{"auths", "auth1", "-d", "header:X-API-KEY"},
			p:         testProject_withAuths(testAuth_session("auth1", seqFlow, "get-sess", "SESSID", enableExpiration, nil)),
			expectErr: `--dest/-d is not a valid option for auth type "session"`,
		},
		{
			name:      "set jwt auth dest fails",
			args:      []string{"auths", "auth1", "-d", "header:X-API-KEY"},
			p:         testProject_withAuths(testAuth_jwt("auth1", "get-sess", "SESSID")),
			expectErr: `--dest/-d is not a valid option for auth type "jwt"`,
		},
		{
			name:               "set token auth dest",
			args:               []string{"auths", "auth1", "-d", "header:X-API-KEY"},
			p:                  testProject_withAuths(testAuth_token("auth1", "get-sess", "SESSID", enableExpiration)),
			expectP:            testProject_withAuths(testAuth_token_withDest("auth1", "get-sess", "SESSID", enableExpiration, morc.ProofDestination{Location: morc.ProofLocationHeader, Key: "X-API-KEY", Format: morc.ProofFormatTypeBearer})),
			expectStdoutOutput: "Set auth proof destination to header:X-API-KEY\n",
		},
		{
			name:      "set basic auth format fails",
			args:      []string{"auths", "auth1", "--format", "bearer"},
			p:         testProject_withAuths(testAuth_basic("auth1", "user", "pass")),
			expectErr: `--format is not a valid option for auth type "basic"`,
		},
		{
			name:      "set session auth format fails",
			args:      []string{"auths", "auth1", "--format", "bearer"},
			p:         testProject_withAuths(testAuth_session("auth1", seqFlow, "get-sess", "SESSID", enableExpiration, nil)),
			expectErr: `--format is not a valid option for auth type "session"`,
		},
		{
			name:      "set jwt auth format fails",
			args:      []string{"auths", "auth1", "--format", "bearer"},
			p:         testProject_withAuths(testAuth_jwt("auth1", "get-sess", "SESSID")),
			expectErr: `--format is not a valid option for auth type "jwt"`,
		},
		{
			name:               "set token auth format",
			args:               []string{"auths", "auth1", "--format", "none"},
			p:                  testProject_withAuths(testAuth_token("auth1", "get-sess", "SESSID", enableExpiration)),
			expectP:            testProject_withAuths(testAuth_token_withDest("auth1", "get-sess", "SESSID", enableExpiration, morc.ProofDestination{Location: morc.ProofLocationHeader, Key: "Authorization"})),
			expectStdoutOutput: "Set auth proof value format to none\n",
		},
		{
			name:      "set basic auth expiration scraper fails",
			args:      []string{"auths", "auth1", "--exp-scraper", ".expires"},
			p:         testProject_withAuths(testAuth_basic("auth1", "user", "pass")),
			expectErr: `--exp-scraper/-e is not a valid option for auth type "basic"`,
		},
		{
			name:      "set session auth expiration scraper fails",
			args:      []string{"auths", "auth1", "--exp-scraper", ".expires"},
			p:         testProject_withAuths(testAuth_session("auth1", seqFlow, "get-sess", "SESSID", enableExpiration, nil)),
			expectErr: `--exp-scraper/-e is not a valid option for auth type "session"`,
		},
		{
			name:      "set jwt auth expiration scraper fails",
			args:      []string{"auths", "auth1", "--exp-scraper", ".expires"},
			p:         testProject_withAuths(testAuth_jwt("auth1", "get-sess", "SESSID")),
			expectErr: `--exp-scraper/-e is not a valid option for auth type "jwt"`,
		},
		{
			name:               "set token auth expiration scraper",
			args:               []string{"auths", "auth1", "--exp-scraper", ".expires.time"},
			p:                  testProject_withAuths(testAuth_token("auth1", "get-sess", "SESSID", enableExpiration)),
			expectP:            testProject_withAuths(testAuth_token_withExpScraper("auth1", "get-sess", "SESSID", morc.Scraper{Type: morc.SpecBodyJSON, Steps: []morc.TraversalStep{{Key: "expires"}, {Key: "time"}}})),
			expectStdoutOutput: "Set auth proof expiration scraper to .expires.time\n",
		},
		{
			name:      "set basic auth expiration layout fails",
			args:      []string{"auths", "auth1", "-L", "RFC3339"},
			p:         testProject_withAuths(testAuth_basic("auth1", "user", "pass")),
			expectErr: `--exp-layout/-L is not a valid option for auth type "basic"`,
		},
		{
			name:      "set session auth expiration layout fails",
			args:      []string{"auths", "auth1", "-L", "RFC3339"},
			p:         testProject_withAuths(testAuth_session("auth1", seqFlow, "get-sess", "SESSID", enableExpiration, nil)),
			expectErr: `--exp-layout/-L is not a valid option for auth type "session"`,
		},
		{
			name:      "set jwt auth expiration layout fails",
			args:      []string{"auths", "auth1", "-L", "RFC3339"},
			p:         testProject_withAuths(testAuth_jwt("auth1", "get-sess", "SESSID")),
			expectErr: `--exp-layout/-L is not a valid option for auth type "jwt"`,
		},
		{
			name:               "set token auth expiration layout",
			args:               []string{"auths", "auth1", "-L", "RFC3339"},
			p:                  testProject_withAuths(testAuth_token("auth1", "get-sess", "SESSID", enableExpiration)),
			expectP:            testProject_withAuths(testAuth_token_withExpLayout("auth1", "get-sess", "SESSID", enableExpiration, time.RFC3339)),
			expectStdoutOutput: "Set auth proof expiration layout to " + time.RFC3339 + "\n",
		},
		{
			name:               "set token auth expiration layout - no change",
			args:               []string{"auths", "auth1", "-L", "RFC1123"},
			p:                  testProject_withAuths(testAuth_token("auth1", "get-sess", "SESSID", enableExpiration)),
			expectP:            testProject_withAuths(testAuth_token_withExpLayout("auth1", "get-sess", "SESSID", enableExpiration, time.RFC1123)),
			expectStderrOutput: "No change to auth proof expiration layout; already set to " + time.RFC1123 + "\n",
		},
		{
			name:               "set token auth expiration layout - no expiration detection",
			args:               []string{"auths", "auth1", "-L", "RFC3339"},
			p:                  testProject_withAuths(testAuth_token("auth1", "get-sess", "SESSID", disableExpiration)),
			expectP:            testProject_withAuths(testAuth_token_withExpLayout("auth1", "get-sess", "SESSID", disableExpiration, time.RFC3339)),
			expectStdoutOutput: "Set auth proof expiration layout to " + time.RFC3339 + "\n",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert := NewAssertionsForInMemoryProject(t, tc.p, &fileRWs)
			resetAuthsFlags()

			// set up the root command and run
			output, outputErr, err := runTestCommand(authsCmd, assert.ProjFilePath, tc.args)

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
				t.Fatalf("expected error containing %q, but got none", tc.expectErr)
				return
			}

			// assertions

			assert.Equal(tc.expectStdoutOutput, output, "stdout output mismatch")
			assert.Equal(tc.expectStderrOutput, outputErr, "stderr output mismatch")

			assert.ProjectFilesInBuffersMatch(tc.expectP)
		})
	}
}

func Test_Auths_Get(t *testing.T) {
	testCases := []struct {
		name               string
		args               []string // DO NOT INCLUDE -F; it is automatically set to a project file
		p                  morc.Project
		expectErr          string // set if command.Execute expected to fail, with a string that would be in the error message
		expectStderrOutput string // set with expected output to stderr
		expectStdoutOutput string // set with expected output to stdout
	}{
		{
			name:               "get name from basic auth",
			args:               []string{"auths", "auth1", "-G", "name"},
			p:                  testProject_singleAuth(),
			expectStdoutOutput: "auth1\n",
		},
		{
			name:               "get type from basic auth",
			args:               []string{"auths", "auth1", "-G", "type"},
			p:                  testProject_singleAuth(),
			expectStdoutOutput: "basic\n",
		},
		{
			name:               "get username from basic auth",
			args:               []string{"auths", "auth1", "-G", "username"},
			p:                  testProject_singleAuth(),
			expectStdoutOutput: "user\n",
		},
		{
			name:               "get password from basic auth, masked",
			args:               []string{"auths", "auth1", "-G", "password"},
			p:                  testProject_singleAuth(),
			expectStdoutOutput: "****\n",
		},
		{
			name:               "get password from basic auth, unmasked",
			args:               []string{"auths", "auth1", "-G", "password", "--unmask"},
			p:                  testProject_singleAuth(),
			expectStdoutOutput: "pass\n",
		},
		{
			name:               "get cookie from session auth",
			args:               []string{"auths", "auth1", "-G", "cookie"},
			p:                  testProject_withAuths(testAuth_session("auth1", seqFlow, "get-sess", "SESSID", enableExpiration, nil)),
			expectStdoutOutput: "SESSID\n",
		},
		{
			name:               "get dest from basic auth",
			args:               []string{"auths", "auth1", "-G", "dest"},
			p:                  testProject_withAuths(testAuth_basic("auth1", "user", "pass")),
			expectStdoutOutput: "header:Authorization\n",
		},
		{
			name:               "get dest from session auth",
			args:               []string{"auths", "auth1", "-G", "dest"},
			p:                  testProject_withAuths(testAuth_session("auth1", seqFlow, "get-sess", "SESSID", disableExpiration, nil)),
			expectStdoutOutput: "cookie:SESSID\n",
		},
		{
			name:               "get dest from token auth",
			args:               []string{"auths", "auth1", "-G", "dest"},
			p:                  testProject_withAuths(testAuth_token("auth1", "get-sess", "SESSID", disableExpiration, nil)),
			expectStdoutOutput: "header:Authorization\n",
		},
		{
			name:               "get dest from jwt auth",
			args:               []string{"auths", "auth1", "-G", "dest"},
			p:                  testProject_withAuths(testAuth_jwt("auth1", "get-sess", "SESSID")),
			expectStdoutOutput: "header:Authorization\n",
		},
		{
			name:               "get expDetection from session auth, on",
			args:               []string{"auths", "auth1", "-G", "exp-detection"},
			p:                  testProject_withAuths(testAuth_session("auth1", seqFlow, "get-sess", "SESSID", enableExpiration, nil)),
			expectStdoutOutput: "ON\n",
		},
		{
			name:               "get expDetection from session auth, off",
			args:               []string{"auths", "auth1", "-G", "exp-detection"},
			p:                  testProject_withAuths(testAuth_session("auth1", seqFlow, "get-sess", "SESSID", disableExpiration, nil)),
			expectStdoutOutput: "OFF\n",
		},
		{
			name:               "get expDetection from token auth, on",
			args:               []string{"auths", "auth1", "-G", "exp-detection"},
			p:                  testProject_withAuths(testAuth_token("auth1", "get-sess", "SESSID", enableExpiration)),
			expectStdoutOutput: "ON\n",
		},
		{
			name:               "get expDetection from token auth, off",
			args:               []string{"auths", "auth1", "-G", "exp-detection"},
			p:                  testProject_withAuths(testAuth_token("auth1", "get-sess", "SESSID", disableExpiration)),
			expectStdoutOutput: "OFF\n",
		},
		{
			name:               "get expDetection from jwt auth, on",
			args:               []string{"auths", "auth1", "-G", "exp-detection"},
			p:                  testProject_withAuths(testAuth_jwt("auth1", "get-sess", "SESSID")),
			expectStdoutOutput: "ON\n",
		},
		// JWT can't have expiration detection disabled, so skipping that test case

		{
			name:               "get expScraper from session auth, expiration on",
			args:               []string{"auths", "auth1", "-G", "exp-scraper"},
			p:                  testProject_withAuths(testAuth_session("auth1", seqFlow, "get-sess", "SESSID", enableExpiration, nil)),
			expectStdoutOutput: "cookie SESSID (with expiration)\n",
		},
		{
			name:               "get expScraper from session auth, expiration off",
			args:               []string{"auths", "auth1", "-G", "exp-scraper"},
			p:                  testProject_withAuths(testAuth_session("auth1", seqFlow, "get-sess", "SESSID", disableExpiration, nil)),
			expectStdoutOutput: "(none)\n",
		},
		{
			name:               "get expScraper from token auth, expiration on",
			args:               []string{"auths", "auth1", "-G", "exp-scraper"},
			p:                  testProject_withAuths(testAuth_token("auth1", "get-sess", "SESSID", enableExpiration)),
			expectStdoutOutput: ".expiration\n",
		},
		{
			name:               "get expScraper from token auth, expiration off",
			args:               []string{"auths", "auth1", "-G", "exp-scraper"},
			p:                  testProject_withAuths(testAuth_token("auth1", "get-sess", "SESSID", disableExpiration)),
			expectStdoutOutput: "(none)\n",
		},
		{
			name:               "get expScraper from jwt auth, expiration on",
			args:               []string{"auths", "auth1", "-G", "exp-scraper"},
			p:                  testProject_withAuths(testAuth_jwt("auth1", "get-sess", "SESSID")),
			expectStdoutOutput: ".token\n",
		},
		// JWT can't have expiration detection disabled, so skipping that test case
		{
			name:               "get expLayout from basic auth",
			args:               []string{"auths", "auth1", "-G", "exp-layout"},
			p:                  testProject_withAuths(testAuth_basic("auth1", "user", "pass")),
			expectStdoutOutput: "(n/a)\n",
		},
		{
			name:               "get expLayout from session auth",
			args:               []string{"auths", "auth1", "-G", "exp-layout"},
			p:                  testProject_withAuths(testAuth_session("auth1", seqFlow, "get-sess", "SESSID", enableExpiration, nil)),
			expectStdoutOutput: "(none)\n",
		},
		{
			name:               "get expLayout from token auth",
			args:               []string{"auths", "auth1", "-G", "exp-layout"},
			p:                  testProject_withAuths(testAuth_token("auth1", "get-sess", "SESSID", enableExpiration)),
			expectStdoutOutput: time.RFC1123 + "\n",
		},
		{
			name:               "get expLayout from jwt auth",
			args:               []string{"auths", "auth1", "-G", "exp-layout"},
			p:                  testProject_withAuths(testAuth_jwt("auth1", "get-sess", "SESSID")),
			expectStdoutOutput: "(none)\n",
		},
		{
			name:               "get expLayout from jwt auth, quiet mode",
			args:               []string{"auths", "auth1", "-qG", "exp-layout"},
			p:                  testProject_withAuths(testAuth_jwt("auth1", "get-sess", "SESSID")),
			expectStdoutOutput: "",
		},
		{
			name:               "get format from basic auth",
			args:               []string{"auths", "auth1", "-G", "format"},
			p:                  testProject_withAuths(testAuth_basic("auth1", "user", "pass")),
			expectStdoutOutput: "basic\n",
		},
		{
			name:               "get format from session auth",
			args:               []string{"auths", "auth1", "-G", "format"},
			p:                  testProject_withAuths(testAuth_session("auth1", seqFlow, "get-sess", "SESSID", enableExpiration, nil)),
			expectStdoutOutput: "(none)\n",
		},
		{
			name:               "get format from token auth",
			args:               []string{"auths", "auth1", "-G", "format"},
			p:                  testProject_withAuths(testAuth_token("auth1", "get-sess", "SESSID", enableExpiration)),
			expectStdoutOutput: "bearer\n",
		},
		{
			name:               "get format from jwt auth",
			args:               []string{"auths", "auth1", "-G", "format"},
			p:                  testProject_withAuths(testAuth_jwt("auth1", "get-sess", "SESSID")),
			expectStdoutOutput: "bearer\n",
		},
		{
			name:               "get format from auth with no format, quiet mode",
			args:               []string{"auths", "auth1", "-qG", "format"},
			p:                  testProject_withAuths(testAuth_session("auth1", seqFlow, "get-sess", "SESSID", enableExpiration, nil)),
			expectStdoutOutput: "",
		},
		{
			name:               "get retrieval from basic auth",
			args:               []string{"auths", "auth1", "-G", "retrieval"},
			p:                  testProject_withAuths(testAuth_basic("auth1", "user", "pass")),
			expectStdoutOutput: "(n/a)\n",
		},
		{
			name:               "get retrieval from session auth",
			args:               []string{"auths", "auth1", "-G", "retrieval"},
			p:                  testProject_withAuths(testAuth_session("auth1", seqFlow, "get-sess", "SESSID", enableExpiration, nil)),
			expectStdoutOutput: "F:get-sess\n",
		},
		{
			name:               "get retrieval from session auth, quiet mode",
			args:               []string{"auths", "auth1", "-qG", "retrieval"},
			p:                  testProject_withAuths(testAuth_session("auth1", seqTemplate, "get-sess", "SESSID", enableExpiration, nil)),
			expectStdoutOutput: "R:get-sess\n",
		},
		{
			name:               "get retrieval from token auth",
			args:               []string{"auths", "auth1", "-G", "retrieval"},
			p:                  testProject_withAuths(testAuth_token("auth1", "get-sess", "SESSID", enableExpiration)),
			expectStdoutOutput: "R:get-sess\n",
		},
		{
			name:               "get retrieval from jwt auth",
			args:               []string{"auths", "auth1", "-G", "retrieval"},
			p:                  testProject_withAuths(testAuth_jwt("auth1", "get-sess", "SESSID")),
			expectStdoutOutput: "R:get-sess\n",
		},
		{
			name:               "get token scraper from basic auth",
			args:               []string{"auths", "auth1", "-G", "token-scraper"},
			p:                  testProject_withAuths(testAuth_basic("auth1", "user", "pass")),
			expectStdoutOutput: "(n/a)\n",
		},
		{
			name:               "get token scraper from session auth",
			args:               []string{"auths", "auth1", "-G", "token-scraper"},
			p:                  testProject_withAuths(testAuth_session("auth1", seqFlow, "get-sess", "SESSID", enableExpiration, nil)),
			expectStdoutOutput: "(n/a)\n",
		},
		{
			name:               "get token scraper from token auth",
			args:               []string{"auths", "auth1", "-G", "token-scraper"},
			p:                  testProject_withAuths(testAuth_token("auth1", "get-sess", "SESSID", enableExpiration)),
			expectStdoutOutput: ".access_token\n",
		},
		{
			name:               "get token scraper from jwt auth",
			args:               []string{"auths", "auth1", "-G", "token-scraper"},
			p:                  testProject_withAuths(testAuth_jwt("auth1", "get-sess", "SESSID")),
			expectStdoutOutput: ".token\n",
		},
		{
			name:               "get property not valid for auth type",
			args:               []string{"auths", "auth1", "-G", "cookie"},
			p:                  testProject_singleAuth(),
			expectStdoutOutput: "(n/a)\n",
		},
		{
			name:               "get property not valid for auth type, quiet mode",
			args:               []string{"auths", "auth1", "-G", "cookie", "-q"},
			p:                  testProject_singleAuth(),
			expectStdoutOutput: "",
		},
		{
			name:      "get non-existent property",
			args:      []string{"auths", "auth1", "-G", "foobar"},
			p:         testProject_singleAuth(),
			expectErr: `invalid attribute "foobar"`,
		},
		{
			name:      "get property from non-existent auth",
			args:      []string{"auths", "non-existent", "-G", "name"},
			p:         testProject_singleAuth(),
			expectErr: "no auth method named non-existent exists in project",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert := NewAssertionsForInMemoryProject(t, tc.p, &fileRWs)
			resetAuthsFlags()

			// set up the root command and run
			output, outputErr, err := runTestCommand(authsCmd, assert.ProjFilePath, tc.args)

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
			}

			// assertions

			assert.Equal(tc.expectStdoutOutput, output, "stdout output mismatch")
			assert.Equal(tc.expectStderrOutput, outputErr, "stderr output mismatch")

			assert.NoProjectMutations()
		})
	}
}

func Test_Auths_List(t *testing.T) {
	testCases := []struct {
		name               string
		args               []string // DO NOT INCLUDE -F; it is automatically set to a project file
		p                  morc.Project
		expectErr          string // set if command.Execute expected to fail, with a string that would be in the error message
		expectStderrOutput string // set with expected output to stderr
		expectStdoutOutput string // set with expected output to stdout
	}{
		{
			name:               "no auths present - empty project",
			args:               []string{"auths"},
			p:                  morc.Project{},
			expectStdoutOutput: "(none)\n",
		},
		{
			name:               "no auths present - empty project, quiet mode",
			args:               []string{"auths", "-q"},
			p:                  morc.Project{},
			expectStdoutOutput: "",
		},
		{
			name:               "no auths present - empty Auths",
			args:               []string{"auths"},
			p:                  morc.Project{Auths: map[string]morc.Auth{}},
			expectStdoutOutput: "(none)\n",
		},
		{
			name:               "one auth present",
			args:               []string{"auths"},
			p:                  testProject_singleAuth(),
			expectStdoutOutput: "auth1: BASIC\n",
		},
		{
			name: "two auths present",
			args: []string{"auths"},
			p: morc.Project{Auths: map[string]morc.Auth{
				"auth1": testAuth_basic("auth1", "user", "pass"),
				"auth2": testAuth_basic("auth2", "user2", "pass2"),
			}},
			expectStdoutOutput: "auth1: BASIC\nauth2: BASIC\n",
		},
		{
			name: "two auths present - check sorting",
			args: []string{"auths"},
			p: morc.Project{Auths: map[string]morc.Auth{
				"zebra": testAuth_basic("zebra", "user", "pass"),
				"alpha": testAuth_basic("alpha", "user2", "pass2"),
			}},
			expectStdoutOutput: "alpha: BASIC\nzebra: BASIC\n",
		},
		{
			name:               "one auth present, quiet mode",
			args:               []string{"auths", "-q"},
			p:                  testProject_singleAuth(),
			expectStdoutOutput: "auth1: BASIC\n",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert := NewAssertionsForInMemoryProject(t, tc.p, &fileRWs)
			resetAuthsFlags()

			// set up the root command and run
			output, outputErr, err := runTestCommand(authsCmd, assert.ProjFilePath, tc.args)

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
			}

			// assertions

			assert.Equal(tc.expectStdoutOutput, output, "stdout output mismatch")
			assert.Equal(tc.expectStderrOutput, outputErr, "stderr output mismatch")

			assert.NoProjectMutations()
		})
	}
}

func resetAuthsFlags() {
	flags.ProjectFile = ""
	flags.New = ""
	flags.Delete = ""
	flags.BForce = false
	flags.Get = ""
	flags.Clear = ""
	flags.Name = ""
	flags.Type = ""
	flags.Username = ""
	flags.Password = ""
	flags.Retrieval = ""
	flags.Cookie = ""
	flags.BNoExpiration = false
	flags.BYesExpiration = false
	flags.TokenScraper = ""
	flags.Dest = ""
	flags.Format = ""
	flags.ExpirationScraper = ""
	flags.ExpirationLayout = ""
	flags.BUnmask = false
	flags.BQuiet = false

	authsCmd.Flags().VisitAll(func(fl *pflag.Flag) {
		fl.Changed = false
	})
}
