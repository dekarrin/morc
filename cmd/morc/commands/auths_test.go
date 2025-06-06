package commands

import (
	"strings"
	"testing"

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
			name: "edit basic auth username",
			args: []string{"auths", "auth1", "-u", "new-user"},
			p:    testProject_singleAuth(),
			expectP: morc.Project{
				Auths: map[string]morc.Auth{
					"auth1": testAuth_basic("auth1", "new-user", "pass"),
				},
			},
			expectStdoutOutput: "Set basic auth username to new-user\n",
		},
		{
			name: "edit basic auth password",
			args: []string{"auths", "auth1", "-p", "new-pass"},
			p:    testProject_singleAuth(),
			expectP: morc.Project{
				Auths: map[string]morc.Auth{
					"auth1": testAuth_basic("auth1", "user", "new-pass"),
				},
			},
			expectStdoutOutput: "Set basic auth password to ********\n",
		},
		{
			name: "edit basic auth password and show with --unmask",
			args: []string{"auths", "auth1", "-p", "new-pass", "--unmask"},
			p:    testProject_singleAuth(),
			expectP: morc.Project{
				Auths: map[string]morc.Auth{
					"auth1": testAuth_basic("auth1", "user", "new-pass"),
				},
			},
			expectStdoutOutput: "Set basic auth password to new-pass\n",
		},
		{
			name: "edit basic auth username and password",
			args: []string{"auths", "auth1", "-u", "new-user", "-p", "new-pass"},
			p:    testProject_singleAuth(),
			expectP: morc.Project{
				Auths: map[string]morc.Auth{
					"auth1": testAuth_basic("auth1", "new-user", "new-pass"),
				},
			},
			expectStdoutOutput: "Set basic auth username to new-user and basic auth password to ********\n",
		},
		{
			name: "rename auth",
			args: []string{"auths", "auth1", "-n", "renamed_auth"},
			p:    testProject_singleAuth(),
			expectP: morc.Project{
				Auths: map[string]morc.Auth{
					"renamed_auth": testAuth_basic("renamed_auth", "user", "pass"),
				},
			},
			expectStdoutOutput: "Set auth method name to renamed_auth\n",
		},
		{
			name: "rename auth that is used in a request",
			args: []string{"auths", "auth1", "-n", "renamed_auth"},
			p: morc.Project{
				Templates: map[string]morc.RequestTemplate{
					"req1": {Name: "req1", Method: "GET", URL: "http://example.com", Auth: "auth1"},
				},
				Auths: map[string]morc.Auth{
					"auth1": testAuth_basic("auth1", "user", "pass"),
				},
			},
			expectP: morc.Project{
				Templates: map[string]morc.RequestTemplate{
					"req1": {Name: "req1", Method: "GET", URL: "http://example.com", Auth: "renamed_auth"},
				},
				Auths: map[string]morc.Auth{
					"renamed_auth": testAuth_basic("renamed_auth", "user", "pass"),
				},
			},
			expectStdoutOutput: "Set auth method name to renamed_auth\n",
		},
		{
			name: "rename to existing name",
			args: []string{"auths", "auth1", "-n", "auth2"},
			p: morc.Project{
				Auths: map[string]morc.Auth{
					"auth1": testAuth_basic("auth1", "user", "pass"),
					"auth2": testAuth_basic("auth2", "user2", "pass2"),
				},
			},
			expectErr: "auth method named auth2 already exists",
		},
		{
			name: "change type of auth - basic to session",
			args: []string{"auths", "auth1", "-T", "session", "-c", "SESSID", "-r", "flow:get-sess"},
			p: morc.Project{
				Auths: map[string]morc.Auth{
					"auth1": testAuth_basic("auth1", "user", "pass"),
				},
				Flows: map[string]morc.Flow{"get-sess": {Name: "get-sess"}},
			},
			expectP: morc.Project{
				Flows: map[string]morc.Flow{"get-sess": {Name: "get-sess"}},
				Auths: map[string]morc.Auth{
					"auth1": {
						Name: "auth1",
						Type: morc.AuthTypeSession,
						Fetcher: morc.NewSessionCookieFetcher(
							morc.RequestSequence{Name: "flow:get-sess", IsFlow: true},
							"SESSID",
							false,
						),
					},
				},
			},
			expectStdoutOutput: "Set auth method type to session, auth proof retrieval sequence to F:flow:get-sess, and session cookie to SESSID\n",
		},
		{
			name:      "invalid flag for type",
			args:      []string{"auths", "auth1", "-c", "SESSID"},
			p:         testProject_singleAuth(),
			expectErr: "--cookie/-c is not a valid option for auth type \"basic\"",
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
