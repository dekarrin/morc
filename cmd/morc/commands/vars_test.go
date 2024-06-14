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
			expectErr: "${EXTRA} is defined via default env and other non-default envs: DEBUG\nSet --all to delete from all",
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
			expectStderrOutput: "Deleted ${EXTRA} from default env\n",
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
			// TODO: normally, this must allow deletion if it can; if not in current, but in default, AND in no others,
			// it should delete it from the default. THIS behavior should be selected only by --current or --env=current.
			expectErr: "${EXTRA} is not defined in current env; value is via default env",
		},

		// after this, we need test cases for explicit env selection:
		// * --env=non-default
		//   * current env is the same
		//     * var is present in current env
		//     * var is not present in current env
		//       * var is present in default
		//       * var is not present in default
		//   * current env is another non-default
		//     * var is present in other env
		//     * var is not present in other env
		//       * var is present in default
		//       * var is not present in default
		//   * current env is default
		//     * var is present in other env
		//     * var is not present in other env
		//       * var is present in default
		//       * var is not present in default
		// * --env=default - ALWAYS FAIL
		// * --current
		//   * current env is default
		//     * var is present in no other envs - SUCCEED
		//     * var is present in another env - FAIL
		//   * current env is non-default
		//     * var is present - S
		//     * var is not present
		//       * var is in default
		//         * var is in no OTHER envs - SUCCEED and update prior non env selection tests AND --env=current that have TODO to fix their cases
		//         * var IS in other envs - FAIL
		//       * var not in default - F
		// * --all cases
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

			assert_projectInFileMatches(assert, tc.expectP, projFilePath)
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
