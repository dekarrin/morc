package commands

import (
	"strings"
	"testing"

	"github.com/dekarrin/morc"
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
			resetReqsFlags()

			// set up the root command and run
			output, outputErr, err := runTestCommand(reqsCmd, assert.ProjFilePath, tc.args)

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
			resetReqsFlags()

			// set up the root command and run
			output, outputErr, err := runTestCommand(reqsCmd, assert.ProjFilePath, tc.args)

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
