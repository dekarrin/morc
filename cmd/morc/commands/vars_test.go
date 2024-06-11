package commands

import (
	"strings"
	"testing"

	"github.com/dekarrin/morc"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
)

func Test_Vars_List(t *testing.T) {
	testCases := []struct {
		name               string
		args               []string // DO NOT INCLUDE -F; it is automatically set to a project file
		p                  morc.Project
		expectErr          string // set if command.Execute expected to fail, with a string that would be in the error message
		expectStderrOutput string // set with expected output to stderr
		expectStdoutOutput string // set with expected output to stdout
	}{
		{
			name:               "empty project",
			p:                  morc.Project{},
			args:               []string{"vars"},
			expectStdoutOutput: "(none)\n",
		},
		{
			name: "vars in default env",
			p: morc.Project{
				Vars: testVarStore("", map[string]map[string]string{
					"": {
						"VAR1": "1",
						"VAR2": "something",
					},
				}),
			},
			args:               []string{"vars"},
			expectStdoutOutput: "${VAR1} = \"1\"\n${VAR2} = \"something\"\n",
		},
		{
			name: "vars with wrong case printed as uppercase",
			p: morc.Project{
				Vars: testVarStore("", map[string]map[string]string{
					"": {
						"vAr1": "1",
						"VAR2": "something",
					},
				}),
			},
			args:               []string{"vars"},
			expectStdoutOutput: "${VAR1} = \"1\"\n${VAR2} = \"something\"\n",
		},
		{
			name: "vars in environment print correctly",
			p: morc.Project{
				Vars: testVarStore("PROD", map[string]map[string]string{
					"": {
						"SCHEME": "http",
						"HOST":   "internal-test.example.com",
					},
					"PROD": {
						"SCHEME": "https",
						"HOST":   "example.com",
					},
				}),
			},
			args:               []string{"vars"},
			expectStdoutOutput: "${HOST} = \"example.com\"\n${SCHEME} = \"https\"\n",
		},
		{
			name: "vars in default environment print correctly",
			p: morc.Project{
				Vars: testVarStore("", map[string]map[string]string{
					"": {
						"SCHEME": "http",
						"HOST":   "internal-test.example.com",
					},
					"PROD": {
						"SCHEME": "https",
						"HOST":   "example.com",
					},
				}),
			},
			args:               []string{"vars"},
			expectStdoutOutput: "${HOST} = \"internal-test.example.com\"\n${SCHEME} = \"http\"\n",
		},
		{
			name: "missing vars filled in with defaults",
			p: morc.Project{
				Vars: testVarStore("PROD", map[string]map[string]string{
					"": {
						"SCHEME": "http",
						"HOST":   "internal-test.example.com",
						"EXTRA":  "data",
					},
					"PROD": {
						"SCHEME": "https",
						"HOST":   "example.com",
					},
				}),
			},
			args:               []string{"vars"},
			expectStdoutOutput: "${EXTRA} = \"data\"\n${HOST} = \"example.com\"\n${SCHEME} = \"https\"\n",
		},
		{
			name: "use --current to list only current environment vars (from non-default)",
			p: morc.Project{
				Vars: testVarStore("PROD", map[string]map[string]string{
					"": {
						"SCHEME": "http",
						"HOST":   "internal-test.example.com",
						"EXTRA":  "data",
					},
					"PROD": {
						"SCHEME": "https",
						"HOST":   "example.com",
					},
				}),
			},
			args:               []string{"vars", "--current"},
			expectStdoutOutput: "${HOST} = \"example.com\"\n${SCHEME} = \"https\"\n",
		},
		{
			name: "use --current to list only current environment vars (from default)",
			p: morc.Project{
				Vars: testVarStore("", map[string]map[string]string{
					"": {
						"SCHEME": "http",
						"HOST":   "internal-test.example.com",
						"EXTRA":  "data",
					},
					"PROD": {
						"SCHEME": "https",
						"HOST":   "example.com",
					},
				}),
			},
			args:               []string{"vars", "--current"},
			expectStdoutOutput: "${EXTRA} = \"data\"\n${HOST} = \"internal-test.example.com\"\n${SCHEME} = \"http\"\n",
		},
		{
			name: "use --env to list only specific environment vars (from same)",
			p: morc.Project{
				Vars: testVarStore("PROD", map[string]map[string]string{
					"": {
						"SCHEME": "http",
						"HOST":   "internal-test.example.com",
						"EXTRA":  "data",
					},
					"PROD": {
						"SCHEME": "https",
						"HOST":   "example.com",
					},
				}),
			},
			args:               []string{"vars", "--env", "PROD"},
			expectStdoutOutput: "${HOST} = \"example.com\"\n${SCHEME} = \"https\"\n",
		},
		{
			name: "use --env to list only specific environment vars (from default)",
			p: morc.Project{
				Vars: testVarStore("", map[string]map[string]string{
					"": {
						"SCHEME": "http",
						"HOST":   "internal-test.example.com",
						"EXTRA":  "data",
					},
					"PROD": {
						"SCHEME": "https",
						"HOST":   "example.com",
					},
				}),
			},
			args:               []string{"vars", "--env", "PROD"},
			expectStdoutOutput: "${HOST} = \"example.com\"\n${SCHEME} = \"https\"\n",
		},
		{
			name: "use --env to list only specific environment vars (from another)",
			p: morc.Project{
				Vars: testVarStore("DEBUG", map[string]map[string]string{
					"": {
						"SCHEME": "http",
						"HOST":   "internal-test.example.com",
						"EXTRA":  "data",
					},
					"PROD": {
						"SCHEME": "https",
						"HOST":   "example.com",
					},
					"DEBUG": {
						"SCHEME": "invalid",
						"HOST":   "invalid.example.com",
					},
				}),
			},
			args:               []string{"vars", "--env", "PROD"},
			expectStdoutOutput: "${HOST} = \"example.com\"\n${SCHEME} = \"https\"\n",
		},
		{
			name: "use --default to list only default environment values (from another)",
			p: morc.Project{
				Vars: testVarStore("", map[string]map[string]string{
					"": {
						"SCHEME": "http",
						"HOST":   "internal-test.example.com",
						"EXTRA":  "data",
					},
					"PROD": {
						"SCHEME": "https",
						"HOST":   "example.com",
					},
					"DEBUG": {
						"SCHEME": "https",
						"HOST":   "example.com",
					},
				}),
			},
			args:               []string{"vars", "--env", "PROD"},
			expectStdoutOutput: "${HOST} = \"example.com\"\n${SCHEME} = \"https\"\n",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert := assert.New(t)
			resetVarsFlags()

			// create project and dump config to a temp dir
			projFilePath := createTestProjectFiles(t, tc.p)
			// set up the root command and run
			output, outputErr, err := runTestCommand(varsCmd, projFilePath, tc.args)

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

			assert_projectInFileMatches(assert, tc.p, projFilePath)
		})
	}
}

func resetVarsFlags() {
	flags.ProjectFile = ""
	flags.Delete = ""
	flags.Env = ""
	flags.BDefault = false
	flags.BCurrent = false
	flags.BAll = false

	varsCmd.Flags().VisitAll(func(fl *pflag.Flag) {
		fl.Changed = false
	})
}
