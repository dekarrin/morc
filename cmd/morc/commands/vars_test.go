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
			args:               []string{"vars"},
			p:                  morc.Project{},
			expectStdoutOutput: "(none)\n",
		},
		{
			name: "vars in default env",
			args: []string{"vars"},
			p: morc.Project{
				Vars: testVarStore("", map[string]map[string]string{
					"": {
						"VAR1": "1",
						"VAR2": "something",
					},
				}),
			},
			expectStdoutOutput: "${VAR1} = \"1\"\n${VAR2} = \"something\"\n",
		},
		{
			name: "vars with wrong case printed as uppercase",
			args: []string{"vars"},
			p: morc.Project{
				Vars: testVarStore("", map[string]map[string]string{
					"": {
						"vAr1": "1",
						"VAR2": "something",
					},
				}),
			},
			expectStdoutOutput: "${VAR1} = \"1\"\n${VAR2} = \"something\"\n",
		},
		{
			name: "vars in environment print correctly",
			args: []string{"vars"},
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
			expectStdoutOutput: "${HOST} = \"example.com\"\n${SCHEME} = \"https\"\n",
		},
		{
			name: "vars in default environment print correctly",
			args: []string{"vars"},
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
			expectStdoutOutput: "${HOST} = \"internal-test.example.com\"\n${SCHEME} = \"http\"\n",
		},
		{
			name: "missing vars filled in with defaults",
			args: []string{"vars"},
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
			expectStdoutOutput: "${EXTRA} = \"data\"\n${HOST} = \"example.com\"\n${SCHEME} = \"https\"\n",
		},
		{
			name: "use --current to list only current environment vars (from non-default)",
			args: []string{"vars", "--current"},
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
			expectStdoutOutput: "${HOST} = \"example.com\"\n${SCHEME} = \"https\"\n",
		},
		{
			name: "use --current to list only current environment vars (from default)",
			args: []string{"vars", "--current"},
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
			expectStdoutOutput: "${EXTRA} = \"data\"\n${HOST} = \"internal-test.example.com\"\n${SCHEME} = \"http\"\n",
		},
		{
			name: "use --env to list only specific environment vars (from same)",
			args: []string{"vars", "--env", "PROD"},
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
			expectStdoutOutput: "${HOST} = \"example.com\"\n${SCHEME} = \"https\"\n",
		},
		{
			name: "use --env to list only specific environment vars (from default)",
			args: []string{"vars", "--env", "PROD"},
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
			expectStdoutOutput: "${HOST} = \"example.com\"\n${SCHEME} = \"https\"\n",
		},
		{
			name: "use --env to list only specific environment vars (from another)",
			args: []string{"vars", "--env", "PROD"},
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
			expectStdoutOutput: "${HOST} = \"example.com\"\n${SCHEME} = \"https\"\n",
		},
		{
			name: "use --default to list only default environment values (from another)",
			args: []string{"vars", "--default"},
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
					"DEBUG": {
						"SCHEME": "invalid",
						"HOST":   "invalid.example.com",
					},
				}),
			},
			expectStdoutOutput: "${EXTRA} = \"data\"\n${HOST} = \"internal-test.example.com\"\n${SCHEME} = \"http\"\n",
		},
		{
			name: "use --default to list only default environment values (from default)",
			args: []string{"vars", "--default"},
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
						"SCHEME": "invalid",
						"HOST":   "invalid.example.com",
					},
				}),
			},
			expectStdoutOutput: "${EXTRA} = \"data\"\n${HOST} = \"internal-test.example.com\"\n${SCHEME} = \"http\"\n",
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

func Test_Vars_Delete(t *testing.T) {
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
			name:      "empty project",
			args:      []string{"vars", "-D", "VAR1"},
			p:         morc.Project{},
			expectErr: "${VAR1} does not exist",
		},
		{
			name: "var not present in current (default)",
			args: []string{"vars", "-D", "VAR1"},
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
						"SCHEME": "invalid",
						"HOST":   "invalid.example.com",
					},
				}),
			},
			expectErr: "${VAR1} does not exist",
		},
		{
			name: "var is present in current (default) and no others",
			args: []string{"vars", "-D", "EXTRA"},
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
						"SCHEME": "invalid",
						"HOST":   "invalid.example.com",
					},
				}),
			},
			expectP: morc.Project{
				Vars: testVarStore("", map[string]map[string]string{
					"": {
						"SCHEME": "http",
						"HOST":   "internal-test.example.com",
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
			expectStdoutOutput: "Deleted ${EXTRA}\n",
		},
		{
			name: "var is present in current (default) and one other",
			args: []string{"vars", "-D", "EXTRA"},
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
						"EXTRA":  "something",
					},
					"DEBUG": {
						"SCHEME": "invalid",
						"HOST":   "invalid.example.com",
					},
				}),
			},
			expectErr: "${EXTRA} is also defined in non-default envs: PROD\nSet --all to delete from all",
		},
		{
			name: "var not present in current (non-default), is present in default, is present in others",
			args: []string{"vars", "-D", "EXTRA"},
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
					"DEBUG": {
						"SCHEME": "invalid",
						"HOST":   "invalid.example.com",
						"EXTRA":  "something",
					},
				}),
			},
			expectErr: "cannot remove ${EXTRA}\nValue is via default env and var is defined in envs: DEBUG\nSet --all to delete from all",
		},
		{
			name: "var not present in current (non-default), is present in default, not present in others",
			args: []string{"vars", "-D", "EXTRA"},
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
					"DEBUG": {
						"SCHEME": "invalid",
						"HOST":   "invalid.example.com",
					},
				}),
			},
			expectP: morc.Project{
				Vars: testVarStore("PROD", map[string]map[string]string{
					"": {
						"SCHEME": "http",
						"HOST":   "internal-test.example.com",
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
			expectStdoutOutput: "Deleted ${EXTRA} from the default environment\n",
		},
		{
			name: "var not present in current (non-default), not present in default",
			args: []string{"vars", "-D", "VAR"},
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
					"DEBUG": {
						"SCHEME": "invalid",
						"HOST":   "invalid.example.com",
					},
				}),
			},
			expectErr: "${VAR} does not exist",
		},
		{
			name: "var is present in current (non-default), is present in default",
			args: []string{"vars", "-D", "HOST"},
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
					"DEBUG": {
						"SCHEME": "invalid",
						"HOST":   "invalid.example.com",
					},
				}),
			},
			expectP: morc.Project{
				Vars: testVarStore("PROD", map[string]map[string]string{
					"": {
						"SCHEME": "http",
						"HOST":   "internal-test.example.com",
						"EXTRA":  "data",
					},
					"PROD": {
						"SCHEME": "https",
					},
					"DEBUG": {
						"SCHEME": "invalid",
						"HOST":   "invalid.example.com",
					},
				}),
			},
			expectStdoutOutput: "Deleted ${HOST}\n",
		},
		{
			name: "--current, var not present in current (non-default), is present in default",
			args: []string{"vars", "-D", "EXTRA", "--current"},
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
					"DEBUG": {
						"SCHEME": "invalid",
						"HOST":   "invalid.example.com",
					},
				}),
			},
			expectErr: "${EXTRA} is not defined in current env; value is via default env",
		},
		{
			name: "--current, var not present in current (non-default), is not present in default",
			args: []string{"vars", "-D", "PASSWORD", "--current"},
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
					"DEBUG": {
						"SCHEME": "invalid",
						"HOST":   "invalid.example.com",
					},
				}),
			},
			expectErr: "${PASSWORD} does not exist in current env",
		},
		{
			name: "--current, var is present in current (non-default)",
			args: []string{"vars", "-D", "HOST", "--current"},
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
					"DEBUG": {
						"SCHEME": "invalid",
						"HOST":   "invalid.example.com",
					},
				}),
			},
			expectP: morc.Project{
				Vars: testVarStore("PROD", map[string]map[string]string{
					"": {
						"SCHEME": "http",
						"HOST":   "internal-test.example.com",
						"EXTRA":  "data",
					},
					"PROD": {
						"SCHEME": "https",
					},
					"DEBUG": {
						"SCHEME": "invalid",
						"HOST":   "invalid.example.com",
					},
				}),
			},
			expectStdoutOutput: "Deleted ${HOST} from the current environment\n",
		},
		{
			name: "--current, var is present in current (default) and no others",
			args: []string{"vars", "-D", "EXTRA", "--current"},
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
						"SCHEME": "invalid",
						"HOST":   "invalid.example.com",
					},
				}),
			},
			expectP: morc.Project{
				Vars: testVarStore("", map[string]map[string]string{
					"": {
						"SCHEME": "http",
						"HOST":   "internal-test.example.com",
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
			expectStdoutOutput: "Deleted ${EXTRA} from the current environment\n",
		},
		{
			name: "--current, var is present in current (default) and others",
			args: []string{"vars", "-D", "HOST", "--current"},
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
						"SCHEME": "invalid",
						"HOST":   "invalid.example.com",
					},
				}),
			},
			expectErr: "cannot remove ${HOST} from current env (default env)\nValue is also defined in envs: DEBUG, PROD\nSet --all to delete from all",
		},
		{
			name: "--env=current, var is present in current",
			args: []string{"vars", "-D", "HOST", "--env", "PROD"},
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
					"DEBUG": {
						"SCHEME": "invalid",
						"HOST":   "invalid.example.com",
					},
				}),
			},
			expectP: morc.Project{
				Vars: testVarStore("PROD", map[string]map[string]string{
					"": {
						"SCHEME": "http",
						"HOST":   "internal-test.example.com",
						"EXTRA":  "data",
					},
					"PROD": {
						"SCHEME": "https",
					},
					"DEBUG": {
						"SCHEME": "invalid",
						"HOST":   "invalid.example.com",
					},
				}),
			},
			expectStdoutOutput: "Deleted ${HOST} from environment PROD\n",
		},
		{
			name: "--env=current, var not present in current, is present in default",
			args: []string{"vars", "-D", "EXTRA", "--env", "PROD"},
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
					"DEBUG": {
						"SCHEME": "invalid",
						"HOST":   "invalid.example.com",
					},
				}),
			},
			expectErr: "${EXTRA} is not defined in env PROD; value is via default env",
		},
		{
			name: "--env=current, var not present in current, not present in default",
			args: []string{"vars", "-D", "PASSWORD", "--env", "PROD"},
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
					"DEBUG": {
						"SCHEME": "invalid",
						"HOST":   "invalid.example.com",
					},
				}),
			},
			expectErr: "${PASSWORD} does not exist in env PROD",
		},
		{
			name: "--env=other, cur is default, var is present in other",
			args: []string{"vars", "-D", "HOST", "--env", "PROD"},
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
						"SCHEME": "invalid",
						"HOST":   "invalid.example.com",
					},
				}),
			},
			expectP: morc.Project{
				Vars: testVarStore("", map[string]map[string]string{
					"": {
						"SCHEME": "http",
						"HOST":   "internal-test.example.com",
						"EXTRA":  "data",
					},
					"PROD": {
						"SCHEME": "https",
					},
					"DEBUG": {
						"SCHEME": "invalid",
						"HOST":   "invalid.example.com",
					},
				}),
			},
			expectStdoutOutput: "Deleted ${HOST} from environment PROD\n",
		},
		{
			name: "--env=other, cur is default, var not present in other, is present in default",
			args: []string{"vars", "-D", "EXTRA", "--env", "PROD"},
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
						"SCHEME": "invalid",
						"HOST":   "invalid.example.com",
					},
				}),
			},
			expectErr: "${EXTRA} is not defined in env PROD; value is via default env",
		},
		{
			name: "--env=other, cur is default, var not present in other, not present in default",
			args: []string{"vars", "-D", "PASSWORD", "--env", "PROD"},
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
						"SCHEME": "invalid",
						"HOST":   "invalid.example.com",
					},
				}),
			},
			expectErr: "${PASSWORD} does not exist in env PROD",
		},
		{
			name: "--env=other, cur is non-default, var is present in other",
			args: []string{"vars", "-D", "HOST", "--env", "PROD"},
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
			expectP: morc.Project{
				Vars: testVarStore("DEBUG", map[string]map[string]string{
					"": {
						"SCHEME": "http",
						"HOST":   "internal-test.example.com",
						"EXTRA":  "data",
					},
					"PROD": {
						"SCHEME": "https",
					},
					"DEBUG": {
						"SCHEME": "invalid",
						"HOST":   "invalid.example.com",
					},
				}),
			},
			expectStdoutOutput: "Deleted ${HOST} from environment PROD\n",
		},
		{
			name: "--env=other, cur is non-default, var not present in other, is present in default",
			args: []string{"vars", "-D", "EXTRA", "--env", "PROD"},
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
			expectErr: "${EXTRA} is not defined in env PROD; value is via default env",
		},
		{
			name: "--env=other, cur is non-default, var not present in other, not present in default",
			args: []string{"vars", "-D", "PASSWORD", "--env", "PROD"},
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
			expectErr: "${PASSWORD} does not exist in env PROD",
		},
		{
			name: "--env=default ERRORS",
			args: []string{"vars", "-D", "EXTRA", "--env", reservedDefaultEnvName},
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
			expectErr: "cannot specify reserved env name \"<DEFAULT>\"; use --default or --all to specify the default env",
		},
		{
			name: "--env='' ERRORS",
			args: []string{"vars", "-D", "PASSWORD", "--env", ""},
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
			expectErr: "cannot specify env \"\"; use --default or --all to specify the default env",
		},

		// after this, we need test cases for explicit env selection:
		// * --default IS ALLOWED AS LONG AS A DELETE FROM DEFAULT WOULD BE VALID.
		//   * var is not present
		//   * var is present
		//     * var is present in others FAIL; --all required.
		//     * var is not present in others.
		// * --all cases
		//  * current=default
		//    * var is present in current
		//      * var is present in no others
		//      * var is present in at least one other
		//    * var is not present in current
		//  * current=non-default
		//    * var is present in current (and therefore default)
		//      * var is present in others
		//      * var is not present in others
		//    * var is not present in current
		//      * var is present in default
		//        * var is present in others
		//        * var is not present in others
		//  * var is present in current (default) and one other
		//  * var not present in current (non-default), is present in default, is present in others
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
			} else if tc.expectErr != "" {
				t.Fatalf("expected error %q, got no error", tc.expectErr)
			}

			// assertions

			assert.Equal(tc.expectStdoutOutput, output, "stdout output mismatch")
			assert.Equal(tc.expectStderrOutput, outputErr, "stderr output mismatch")

			assert_projectInFileMatches(assert, tc.expectP, projFilePath)
		})
	}
}

// GET cases
// * unspecified env
//   * current is default
//     * current has var - VALUE
//     * current does not have var - NO VALUE
//   * current is non-default
//     * current has own var def - VALUE
//     * current does not define var
//       * default does - VALUE
//       * default does not -NO VALUE
// * specify --current
//   * current is default
//     * current has var - VALUE
//     * current does not have var - NO VALUE
//   * current is non-default
//     * current has own var def - VALUE
//     * current does not define var
//       * default does - NO VALUE, IT IS VIA DEFAULT
//       * default does not - NO VALUE
// * specify --env
//   * current is default
//     * specified env has var - VALUE
//     * specified env has no var
//       * default does - NO VALUE, IT IS VIA DEFAULT
//       * default does not - NO VALUE
//   * current is non-default
//     * specified env has var - VALUE
//     * specified env has no var
//       * default does - NO VALUE, IT IS VIA DEFAULT
//       * default does not - NO VALUE
// * specifiy --default
//   * current is default
//     * default has value - VALUE
//     * default has no value - NO VALUE
//   * current is non-default
//     * default has value - VALUE
//     * default has no value - NO VALUE
// * specify --all - list all values by env.

// SET cases
// * unspecified env
//   * current is default
//     * var already existed in default and no others - it is updated
//     * var already existed in default and others - it is updated ONLY in default
//     * var did not exist in default - it is set in default only
//   * current is non-default
//     * var exists in default
//       * var exists in other non-default - var is set in current only.
//       * var exists in no other non-default - var is set in current only.
//     * var does not exist in default - var is set in current and to BLANK in default.
// * specify --current
//   * current is default
//     * var already existed in default - PASS
//     * var did not exist in default - PASS
//   * current is non-default
//     * var exists in default - var is set in current only
//     * var does not exist in default - var is set in current AND in default as blank.
// * specify --env
//   * current is default
//     * var already existed in default - var is set in specified only
//     * var did not exist in default - var is set in specified AND in default as blank
//   * current is non-default
//     * var exists in default - var is set in specified only
//     * var does not exist in default - var is set in specified AND in default as blank
// * specify --default
//   * current is default
//     * var already existed in default - PASS
//     * var did not exist in default - PASS
//   * current is non-default
//     * var exists in default - var is set in default only
//     * var does not exist in default - var is set in default only

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
