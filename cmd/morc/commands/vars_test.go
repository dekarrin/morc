package commands

import (
	"strings"
	"testing"

	"github.com/dekarrin/morc"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
)

var (
	test_3EnvVarsMap = map[string]map[string]string{
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
	}

	test_3EnvVarsMap_noHost = map[string]map[string]string{
		"": {
			"SCHEME": "http",
			"EXTRA":  "data",
		},
		"PROD": {
			"SCHEME": "https",
		},
		"DEBUG": {
			"SCHEME": "invalid",
		},
	}

	test_3EnvVarsMap_debugHasExtra = map[string]map[string]string{
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
	}

	test_3EnvVarsMap_prodNoHost = map[string]map[string]string{
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
	}

	test_3EnvVarsMap_noExtra = map[string]map[string]string{
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
	}
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
			name:               "empty project, quiet mode",
			args:               []string{"vars", "-q"},
			p:                  morc.Project{},
			expectStdoutOutput: "",
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
				Vars: testVarStore("DEBUG", test_3EnvVarsMap),
			},
			expectStdoutOutput: "${HOST} = \"example.com\"\n${SCHEME} = \"https\"\n",
		},
		{
			name: "use --default to list only default environment values (from another)",
			args: []string{"vars", "--default"},
			p: morc.Project{
				Vars: testVarStore("PROD", test_3EnvVarsMap),
			},
			expectStdoutOutput: "${EXTRA} = \"data\"\n${HOST} = \"internal-test.example.com\"\n${SCHEME} = \"http\"\n",
		},
		{
			name: "use --default to list only default environment values (from default)",
			args: []string{"vars", "--default"},
			p: morc.Project{
				Vars: testVarStore("", test_3EnvVarsMap),
			},
			expectStdoutOutput: "${EXTRA} = \"data\"\n${HOST} = \"internal-test.example.com\"\n${SCHEME} = \"http\"\n",
		},
		{
			name: "alt var prefix in project",
			args: []string{"vars", "--default"},
			p: morc.Project{
				Vars: testVarStore("", test_3EnvVarsMap),
				Config: morc.Settings{
					VarPrefix: "@",
				},
			},
			expectStdoutOutput: "@{EXTRA} = \"data\"\n@{HOST} = \"internal-test.example.com\"\n@{SCHEME} = \"http\"\n",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert := assert.New(t)
			resetVarsFlags()

			// create project and dump config to a temp dir
			projFilePath := createTestProjectIO(t, tc.p)
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

			assert_noProjectMutations(assert)
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
			name:      "var not present in current (default)",
			args:      []string{"vars", "-D", "VAR1"},
			p:         testProject_vars("", test_3EnvVarsMap),
			expectErr: "${VAR1} does not exist",
		},
		{
			name:               "var is present in current (default) and no others",
			args:               []string{"vars", "-D", "EXTRA"},
			p:                  testProject_vars("", test_3EnvVarsMap),
			expectP:            testProject_vars("", test_3EnvVarsMap_noExtra),
			expectStdoutOutput: "Deleted ${EXTRA}\n",
		},
		{
			name:               "var is present in current (default) and no others, quiet mode",
			args:               []string{"vars", "-D", "EXTRA", "-q"},
			p:                  testProject_vars("", test_3EnvVarsMap),
			expectP:            testProject_vars("", test_3EnvVarsMap_noExtra),
			expectStdoutOutput: "",
		},
		{
			name:      "var is present in current (default) and one other",
			args:      []string{"vars", "-D", "EXTRA"},
			p:         testProject_vars("", test_3EnvVarsMap_debugHasExtra),
			expectErr: "${EXTRA} is also defined in non-default envs: DEBUG\nSet --all to delete from all",
		},
		{
			name:      "var not present in current (non-default), is present in default, is present in others",
			args:      []string{"vars", "-D", "EXTRA"},
			p:         testProject_vars("PROD", test_3EnvVarsMap_debugHasExtra),
			expectErr: "cannot remove ${EXTRA}\nValue is via default env and var is defined in envs: DEBUG\nSet --all to delete from all",
		},
		{
			name:               "var not present in current (non-default), is present in default, not present in others",
			args:               []string{"vars", "-D", "EXTRA"},
			p:                  testProject_vars("PROD", test_3EnvVarsMap),
			expectP:            testProject_vars("PROD", test_3EnvVarsMap_noExtra),
			expectStdoutOutput: "Deleted ${EXTRA} from the default environment\n",
		},
		{
			name:               "var not present in current (non-default), is present in default, not present in others, quiet mode",
			args:               []string{"vars", "-D", "EXTRA", "-q"},
			p:                  testProject_vars("PROD", test_3EnvVarsMap),
			expectP:            testProject_vars("PROD", test_3EnvVarsMap_noExtra),
			expectStdoutOutput: "",
		},
		{
			name:      "var not present in current (non-default), not present in default",
			args:      []string{"vars", "-D", "VAR"},
			p:         testProject_vars("PROD", test_3EnvVarsMap),
			expectErr: "${VAR} does not exist",
		},
		{
			name:               "var is present in current (non-default), is present in default",
			args:               []string{"vars", "-D", "HOST"},
			p:                  testProject_vars("PROD", test_3EnvVarsMap),
			expectP:            testProject_vars("PROD", test_3EnvVarsMap_prodNoHost),
			expectStdoutOutput: "Deleted ${HOST}\n",
		},
		{
			name:      "--current, var not present in current (non-default), is present in default",
			args:      []string{"vars", "-D", "EXTRA", "--current"},
			p:         testProject_vars("PROD", test_3EnvVarsMap),
			expectErr: "${EXTRA} is not defined in current env; value is via default env",
		},
		{
			name:      "--current, var not present in current (non-default), is not present in default",
			args:      []string{"vars", "-D", "PASSWORD", "--current"},
			p:         testProject_vars("PROD", test_3EnvVarsMap),
			expectErr: "${PASSWORD} does not exist in current env",
		},
		{
			name:               "--current, var is present in current (non-default)",
			args:               []string{"vars", "-D", "HOST", "--current"},
			p:                  testProject_vars("PROD", test_3EnvVarsMap),
			expectP:            testProject_vars("PROD", test_3EnvVarsMap_prodNoHost),
			expectStdoutOutput: "Deleted ${HOST} from the current environment\n",
		},
		{
			name:               "--current, var is present in current (non-default), quiet mode",
			args:               []string{"vars", "-D", "HOST", "--current", "-q"},
			p:                  testProject_vars("PROD", test_3EnvVarsMap),
			expectP:            testProject_vars("PROD", test_3EnvVarsMap_prodNoHost),
			expectStdoutOutput: "",
		},
		{
			name:               "--current, var is present in current (default) and no others",
			args:               []string{"vars", "-D", "EXTRA", "--current"},
			p:                  testProject_vars("", test_3EnvVarsMap),
			expectP:            testProject_vars("", test_3EnvVarsMap_noExtra),
			expectStdoutOutput: "Deleted ${EXTRA} from the current environment\n",
		},
		{
			name:      "--current, var is present in current (default) and others",
			args:      []string{"vars", "-D", "HOST", "--current"},
			p:         testProject_vars("", test_3EnvVarsMap),
			expectErr: "cannot remove ${HOST} from current env (default env)\nValue is also defined in envs: DEBUG, PROD\nSet --all to delete from all",
		},
		{
			name:               "--env=current, var is present in current",
			args:               []string{"vars", "-D", "HOST", "--env", "PROD"},
			p:                  testProject_vars("PROD", test_3EnvVarsMap),
			expectP:            testProject_vars("PROD", test_3EnvVarsMap_prodNoHost),
			expectStdoutOutput: "Deleted ${HOST} from environment PROD\n",
		},
		{
			name:      "--env=current, var not present in current, is present in default",
			args:      []string{"vars", "-D", "EXTRA", "--env", "PROD"},
			p:         testProject_vars("PROD", test_3EnvVarsMap),
			expectErr: "${EXTRA} is not defined in env PROD; value is via default env",
		},
		{
			name:      "--env=current, var not present in current, not present in default",
			args:      []string{"vars", "-D", "PASSWORD", "--env", "PROD"},
			p:         testProject_vars("PROD", test_3EnvVarsMap),
			expectErr: "${PASSWORD} does not exist in env PROD",
		},
		{
			name:               "--env=other, cur is default, var is present in other",
			args:               []string{"vars", "-D", "HOST", "--env", "PROD"},
			p:                  testProject_vars("", test_3EnvVarsMap),
			expectP:            testProject_vars("", test_3EnvVarsMap_prodNoHost),
			expectStdoutOutput: "Deleted ${HOST} from environment PROD\n",
		},
		{
			name:               "--env=other, cur is default, var is present in other, quiet mode",
			args:               []string{"vars", "-D", "HOST", "--env", "PROD", "-q"},
			p:                  testProject_vars("", test_3EnvVarsMap),
			expectP:            testProject_vars("", test_3EnvVarsMap_prodNoHost),
			expectStdoutOutput: "",
		},
		{
			name:      "--env=other, cur is default, var not present in other, is present in default",
			args:      []string{"vars", "-D", "EXTRA", "--env", "PROD"},
			p:         testProject_vars("", test_3EnvVarsMap),
			expectErr: "${EXTRA} is not defined in env PROD; value is via default env",
		},
		{
			name:      "--env=other, cur is default, var not present in other, not present in default",
			args:      []string{"vars", "-D", "PASSWORD", "--env", "PROD"},
			p:         testProject_vars("", test_3EnvVarsMap),
			expectErr: "${PASSWORD} does not exist in env PROD",
		},
		{
			name:               "--env=other, cur is non-default, var is present in other",
			args:               []string{"vars", "-D", "HOST", "--env", "PROD"},
			p:                  testProject_vars("DEBUG", test_3EnvVarsMap),
			expectP:            testProject_vars("DEBUG", test_3EnvVarsMap_prodNoHost),
			expectStdoutOutput: "Deleted ${HOST} from environment PROD\n",
		},
		{
			name:      "--env=other, cur is non-default, var not present in other, is present in default",
			args:      []string{"vars", "-D", "EXTRA", "--env", "PROD"},
			p:         testProject_vars("DEBUG", test_3EnvVarsMap),
			expectErr: "${EXTRA} is not defined in env PROD; value is via default env",
		},
		{
			name:      "--env=other, cur is non-default, var not present in other, not present in default",
			args:      []string{"vars", "-D", "PASSWORD", "--env", "PROD"},
			p:         testProject_vars("DEBUG", test_3EnvVarsMap),
			expectErr: "${PASSWORD} does not exist in env PROD",
		},
		{
			name:      "--env=default ERRORS",
			args:      []string{"vars", "-D", "EXTRA", "--env", reservedDefaultEnvName},
			p:         testProject_vars("DEBUG", test_3EnvVarsMap),
			expectErr: "cannot specify reserved env name \"<DEFAULT>\"; use --default or --all to specify the default env",
		},
		{
			name:      "--env='' ERRORS",
			args:      []string{"vars", "-D", "PASSWORD", "--env", ""},
			p:         testProject_vars("DEBUG", test_3EnvVarsMap),
			expectErr: "cannot specify env \"\"; use --default or --all to specify the default env",
		},
		{
			name:               "--default, current is default, var is present in default and no others",
			args:               []string{"vars", "-D", "EXTRA", "--default"},
			p:                  testProject_vars("", test_3EnvVarsMap),
			expectP:            testProject_vars("", test_3EnvVarsMap_noExtra),
			expectStdoutOutput: "Deleted ${EXTRA} from the default environment\n",
		},
		{
			name:               "--default, current is non-default, var is present in default and no others",
			args:               []string{"vars", "-D", "EXTRA", "--default"},
			p:                  testProject_vars("PROD", test_3EnvVarsMap),
			expectP:            testProject_vars("PROD", test_3EnvVarsMap_noExtra),
			expectStdoutOutput: "Deleted ${EXTRA} from the default environment\n",
		},
		{
			name:               "--default, current is non-default, var is present in default and no others, quiet mode",
			args:               []string{"vars", "-D", "EXTRA", "--default", "-q"},
			p:                  testProject_vars("PROD", test_3EnvVarsMap),
			expectP:            testProject_vars("PROD", test_3EnvVarsMap_noExtra),
			expectStdoutOutput: "",
		},
		{
			name:      "--default, current is default, var is present in default and others",
			args:      []string{"vars", "-D", "HOST", "--default"},
			p:         testProject_vars("", test_3EnvVarsMap),
			expectErr: "cannot remove ${HOST} from default env\nValue is also defined in envs: DEBUG, PROD\nSet --all to delete from all",
		},
		{
			name:      "--default, current is non-default, var is present in default and others",
			args:      []string{"vars", "-D", "HOST", "--default"},
			p:         testProject_vars("PROD", test_3EnvVarsMap),
			expectErr: "cannot remove ${HOST} from default env\nValue is also defined in envs: DEBUG, PROD\nSet --all to delete from all",
		},
		{
			name:      "--default, current is default, var is not present in default",
			args:      []string{"vars", "-D", "PASSWORD", "--default"},
			p:         testProject_vars("", test_3EnvVarsMap),
			expectErr: "${PASSWORD} does not exist in default env",
		},
		{
			name:      "--default, current is non-default, var is not present in default",
			args:      []string{"vars", "-D", "PASSWORD", "--default"},
			p:         testProject_vars("PROD", test_3EnvVarsMap),
			expectErr: "${PASSWORD} does not exist in default env",
		},
		{
			name:      "--all, current is default, var is not present in default",
			args:      []string{"vars", "-D", "PASSWORD", "--all"},
			p:         testProject_vars("", test_3EnvVarsMap),
			expectErr: "${PASSWORD} does not exist in any environment",
		},
		{
			name:               "--all, current is default, var is present in default, no others",
			args:               []string{"vars", "-D", "EXTRA", "--all"},
			p:                  testProject_vars("", test_3EnvVarsMap),
			expectP:            testProject_vars("", test_3EnvVarsMap_noExtra),
			expectStdoutOutput: "Deleted ${EXTRA} from all environments\n",
		},
		{
			name:               "--all, current is default, var is present in default and others",
			args:               []string{"vars", "-D", "HOST", "--all"},
			p:                  testProject_vars("", test_3EnvVarsMap),
			expectP:            testProject_vars("", test_3EnvVarsMap_noHost),
			expectStdoutOutput: "Deleted ${HOST} from all environments\n",
		},
		{
			name:               "--all, current is default, var is present in default and others, quiet mode",
			args:               []string{"vars", "-D", "HOST", "--all", "-q"},
			p:                  testProject_vars("", test_3EnvVarsMap),
			expectP:            testProject_vars("", test_3EnvVarsMap_noHost),
			expectStdoutOutput: "",
		},
		{
			name:      "--all, current is non-default, var is not present in current, not present in others",
			args:      []string{"vars", "-D", "PASSWORD", "--all"},
			p:         testProject_vars("PROD", test_3EnvVarsMap),
			expectErr: "${PASSWORD} does not exist in any environment",
		},
		{
			name:               "--all, current is non-default, var is not present in current, present in others",
			args:               []string{"vars", "-D", "EXTRA", "--all"},
			p:                  testProject_vars("PROD", test_3EnvVarsMap_debugHasExtra),
			expectP:            testProject_vars("PROD", test_3EnvVarsMap_noExtra),
			expectStdoutOutput: "Deleted ${EXTRA} from all environments\n",
		},
		{
			name:               "--all, current is non-default, var is present in current, no others",
			args:               []string{"vars", "-D", "EXTRA", "--all"},
			p:                  testProject_vars("DEBUG", test_3EnvVarsMap_debugHasExtra),
			expectP:            testProject_vars("DEBUG", test_3EnvVarsMap_noExtra),
			expectStdoutOutput: "Deleted ${EXTRA} from all environments\n",
		},
		{
			name:               "--all, current is non-default, var is present in current, and others",
			args:               []string{"vars", "-D", "HOST", "--all"},
			p:                  testProject_vars("PROD", test_3EnvVarsMap),
			expectP:            testProject_vars("PROD", test_3EnvVarsMap_noHost),
			expectStdoutOutput: "Deleted ${HOST} from all environments\n",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert := assert.New(t)
			resetVarsFlags()

			// create project and dump config to a temp dir
			projFilePath := createTestProjectIO(t, tc.p)
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

			assert_projectFilesInBuffersMatch(assert, tc.expectP)
		})
	}
}

// TODO: undefined var should be an error.
func Test_Vars_Get(t *testing.T) {
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
			args:               []string{"vars", "VAR1"},
			p:                  morc.Project{},
			expectStderrOutput: "${VAR1} is not defined\n",
		},
		{
			name:               "unspecified env, current=default, var is present",
			args:               []string{"vars", "HOST"},
			p:                  testProject_vars("", test_3EnvVarsMap),
			expectStdoutOutput: test_3EnvVarsMap[""]["HOST"] + "\n",
		},
		{
			name:               "unspecified env, current=default, var is present, quiet mode still prints",
			args:               []string{"vars", "HOST", "-q"},
			p:                  testProject_vars("", test_3EnvVarsMap),
			expectStdoutOutput: test_3EnvVarsMap[""]["HOST"] + "\n",
		},
		{
			name:               "unspecified env, current=default, var is not present",
			args:               []string{"vars", "PASSWORD"},
			p:                  testProject_vars("", test_3EnvVarsMap),
			expectStderrOutput: "${PASSWORD} is not defined\n",
		},
		{
			name:               "unspecified env, current=non-default, var present in env",
			args:               []string{"vars", "HOST"},
			p:                  testProject_vars("PROD", test_3EnvVarsMap),
			expectStdoutOutput: test_3EnvVarsMap["PROD"]["HOST"] + "\n",
		},
		{
			name:               "unspecified env, current=non-default, var not present in env, present in default",
			args:               []string{"vars", "EXTRA"},
			p:                  testProject_vars("PROD", test_3EnvVarsMap),
			expectStdoutOutput: test_3EnvVarsMap[""]["EXTRA"] + "\n",
		},
		{
			name:               "unspecified env, current=non-default, var not present in env or default",
			args:               []string{"vars", "PASSWORD"},
			p:                  testProject_vars("PROD", test_3EnvVarsMap),
			expectStderrOutput: "${PASSWORD} is not defined\n",
		},
		{
			name:               "--current, current=default, var is present",
			args:               []string{"vars", "HOST", "--current"},
			p:                  testProject_vars("", test_3EnvVarsMap),
			expectStdoutOutput: test_3EnvVarsMap[""]["HOST"] + "\n",
		},
		{
			name:               "--current, current=default, var is not present",
			args:               []string{"vars", "PASSWORD", "--current"},
			p:                  testProject_vars("", test_3EnvVarsMap),
			expectStderrOutput: "${PASSWORD} is not defined in current env (default env)\n",
		},
		{
			name:               "--current, current=non-default, var present in env",
			args:               []string{"vars", "HOST", "--current"},
			p:                  testProject_vars("PROD", test_3EnvVarsMap),
			expectStdoutOutput: test_3EnvVarsMap["PROD"]["HOST"] + "\n",
		},
		{
			name:               "--current, current=non-default, var not present in env, present in default",
			args:               []string{"vars", "EXTRA", "--current"},
			p:                  testProject_vars("PROD", test_3EnvVarsMap),
			expectStderrOutput: "${EXTRA} is not defined in current env (PROD); value is via default env\n",
		},
		{
			name:               "--current, current=non-default, var not present in env or default",
			args:               []string{"vars", "PASSWORD", "--current"},
			p:                  testProject_vars("PROD", test_3EnvVarsMap),
			expectStderrOutput: "${PASSWORD} is not defined in current env (PROD)\n",
		},
		{
			name:               "--env=current, current=non-default, var present in env",
			args:               []string{"vars", "HOST", "--env", "PROD"},
			p:                  testProject_vars("PROD", test_3EnvVarsMap),
			expectStdoutOutput: test_3EnvVarsMap["PROD"]["HOST"] + "\n",
		},
		{
			name:               "--env=current, current=non-default, var not present in env, present in default",
			args:               []string{"vars", "EXTRA", "--env", "PROD"},
			p:                  testProject_vars("PROD", test_3EnvVarsMap),
			expectStderrOutput: "${EXTRA} is not defined in env PROD; value is via default env\n",
		},
		{
			name:               "--env=current, current=non-default, var not present in env or default",
			args:               []string{"vars", "PASSWORD", "--env", "PROD"},
			p:                  testProject_vars("PROD", test_3EnvVarsMap),
			expectStderrOutput: "${PASSWORD} is not defined in env PROD\n",
		},
		{
			name:               "--env=other, current=non-default, var present in env",
			args:               []string{"vars", "HOST", "--env", "PROD"},
			p:                  testProject_vars("DEBUG", test_3EnvVarsMap),
			expectStdoutOutput: test_3EnvVarsMap["PROD"]["HOST"] + "\n",
		},
		{
			name:               "--env=other, current=non-default, var not present in env, present in default",
			args:               []string{"vars", "EXTRA", "--env", "PROD"},
			p:                  testProject_vars("DEBUG", test_3EnvVarsMap),
			expectStderrOutput: "${EXTRA} is not defined in env PROD; value is via default env\n",
		},
		{
			name:               "--env=other, current=non-default, var not present in env or default",
			args:               []string{"vars", "PASSWORD", "--env", "PROD"},
			p:                  testProject_vars("DEBUG", test_3EnvVarsMap),
			expectStderrOutput: "${PASSWORD} is not defined in env PROD\n",
		},
		{
			name:               "--env=other, current=default, var present in env",
			args:               []string{"vars", "HOST", "--env", "PROD"},
			p:                  testProject_vars("", test_3EnvVarsMap),
			expectStdoutOutput: test_3EnvVarsMap["PROD"]["HOST"] + "\n",
		},
		{
			name:               "--env=other, current=default, var not present in env, present in default",
			args:               []string{"vars", "EXTRA", "--env", "PROD"},
			p:                  testProject_vars("", test_3EnvVarsMap),
			expectStderrOutput: "${EXTRA} is not defined in env PROD; value is via default env\n",
		},
		{
			name:               "--env=other, current=default, var not present in env or default",
			args:               []string{"vars", "PASSWORD", "--env", "PROD"},
			p:                  testProject_vars("", test_3EnvVarsMap),
			expectStderrOutput: "${PASSWORD} is not defined in env PROD\n",
		},
		{
			name:      "--env='' ERRORS",
			args:      []string{"vars", "PASSWORD", "--env", ""},
			p:         testProject_vars("DEBUG", test_3EnvVarsMap),
			expectErr: "cannot specify env \"\"; use --default to get from default env",
		},
		{
			name:      "--env=default ERRORS",
			args:      []string{"vars", "EXTRA", "--env", reservedDefaultEnvName},
			p:         testProject_vars("DEBUG", test_3EnvVarsMap),
			expectErr: "cannot specify reserved env name \"<DEFAULT>\"; use --default to get from default env",
		},
		{
			name:               "--default, current=default, var is present",
			args:               []string{"vars", "HOST", "--default"},
			p:                  testProject_vars("", test_3EnvVarsMap),
			expectStdoutOutput: test_3EnvVarsMap[""]["HOST"] + "\n",
		},
		{
			name:               "--default, current=default, var is not present",
			args:               []string{"vars", "PASSWORD", "--default"},
			p:                  testProject_vars("", test_3EnvVarsMap),
			expectStderrOutput: "${PASSWORD} is not defined in default env\n",
		},
		{
			name:               "--default, current=non-default, var is present in default",
			args:               []string{"vars", "HOST", "--default"},
			p:                  testProject_vars("PROD", test_3EnvVarsMap),
			expectStdoutOutput: test_3EnvVarsMap[""]["HOST"] + "\n",
		},
		{
			name:               "--default, current=non-default, var is not present in default",
			args:               []string{"vars", "PASSWORD", "--default"},
			p:                  testProject_vars("PROD", test_3EnvVarsMap),
			expectStderrOutput: "${PASSWORD} is not defined in default env\n",
		},
		{
			name: "--all, var is present in some",
			args: []string{"vars", "EXTRA", "--all"},
			p:    testProject_vars("PROD", test_3EnvVarsMap_debugHasExtra),
			expectStdoutOutput: "" +
				`ENV         VALUE      ` + "\n" +
				`-----------------------` + "\n" +
				`(default):  "data"     ` + "\n" +
				`DEBUG:      "something"` + "\n",
		},
		{
			name: "--all, var is present in all",
			args: []string{"vars", "HOST", "--all"},
			p:    testProject_vars("PROD", test_3EnvVarsMap),
			expectStdoutOutput: "" +
				`ENV         VALUE                      ` + "\n" +
				`---------------------------------------` + "\n" +
				`(default):  "internal-test.example.com"` + "\n" +
				`DEBUG:      "invalid.example.com"      ` + "\n" +
				`PROD:       "example.com"              ` + "\n",
		},
		{
			name: "--all, var is present in all, quiet mode",
			args: []string{"vars", "HOST", "--all", "-q"},
			p:    testProject_vars("PROD", test_3EnvVarsMap),
			expectStdoutOutput: "" +
				`(default)  internal-test.example.com` + "\n" +
				`DEBUG      invalid.example.com      ` + "\n" +
				`PROD       example.com              ` + "\n",
		},
		{
			name:               "--all, var is present in none",
			args:               []string{"vars", "PASSWORD", "--all"},
			p:                  testProject_vars("PROD", test_3EnvVarsMap_debugHasExtra),
			expectStderrOutput: "(no values defined)\n",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert := assert.New(t)
			resetVarsFlags()

			// create project and dump config to a temp dir
			projFilePath := createTestProjectIO(t, tc.p)
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

			assert_noProjectMutations(assert)
		})
	}
}

func Test_Vars_Set(t *testing.T) {
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
			name:               "make a new var",
			args:               []string{"vars", "var1", "VRISKA"},
			p:                  morc.Project{},
			expectP:            testProject_vars("", map[string]map[string]string{"": {"VAR1": "VRISKA"}}),
			expectStdoutOutput: "Set ${VAR1} to \"VRISKA\"\n",
		},
		{
			name:               "make a new var, quiet mode",
			args:               []string{"vars", "var1", "VRISKA", "-q"},
			p:                  morc.Project{},
			expectP:            testProject_vars("", map[string]map[string]string{"": {"VAR1": "VRISKA"}}),
			expectStdoutOutput: "",
		},
		{
			name:               "unspecified env, current=default, var not present",
			args:               []string{"vars", "var1", "VRISKA"},
			p:                  testProject_vars("", test_3EnvVarsMap),
			expectP:            testProject_vars("", test_3EnvVarsMap, map[string]map[string]string{"": {"VAR1": "VRISKA"}}),
			expectStdoutOutput: "Set ${VAR1} to \"VRISKA\"\n",
		},
		{
			name:               "unspecified env, current=default, var present in default and no others",
			args:               []string{"vars", "var1", "NEPETA"},
			p:                  testProject_vars("", test_3EnvVarsMap, map[string]map[string]string{"": {"VAR1": "VRISKA"}}),
			expectP:            testProject_vars("", test_3EnvVarsMap, map[string]map[string]string{"": {"VAR1": "NEPETA"}}),
			expectStdoutOutput: "Set ${VAR1} to \"NEPETA\"\n",
		},
		{
			name:               "unspecified env, current=default, var present in default and others",
			args:               []string{"vars", "var1", "NEPETA"},
			p:                  testProject_vars("", test_3EnvVarsMap, map[string]map[string]string{"": {"VAR1": "VRISKA"}, "PROD": {"VAR1": "TEREZI"}, "DEBUG": {"VAR1": "KANAYA"}}),
			expectP:            testProject_vars("", test_3EnvVarsMap, map[string]map[string]string{"": {"VAR1": "NEPETA"}, "PROD": {"VAR1": "TEREZI"}, "DEBUG": {"VAR1": "KANAYA"}}),
			expectStdoutOutput: "Set ${VAR1} to \"NEPETA\"\n",
		},
		{
			name:               "unspecified env, current=non-default, var not present in default",
			args:               []string{"vars", "var1", "VRISKA"},
			p:                  testProject_vars("PROD", test_3EnvVarsMap),
			expectP:            testProject_vars("PROD", test_3EnvVarsMap, map[string]map[string]string{"": {"VAR1": ""}, "PROD": {"VAR1": "VRISKA"}}),
			expectStdoutOutput: "Set ${VAR1} to \"VRISKA\"\n",
		},
		{
			name:               "unspecified env, current=non-default, var present in default and no others",
			args:               []string{"vars", "var1", "NEPETA"},
			p:                  testProject_vars("PROD", test_3EnvVarsMap, map[string]map[string]string{"": {"VAR1": "VRISKA"}}),
			expectP:            testProject_vars("PROD", test_3EnvVarsMap, map[string]map[string]string{"": {"VAR1": "VRISKA"}, "PROD": {"VAR1": "NEPETA"}}),
			expectStdoutOutput: "Set ${VAR1} to \"NEPETA\"\n",
		},
		{
			name:               "unspecified env, current=non-default, var present in default and others",
			args:               []string{"vars", "var1", "NEPETA"},
			p:                  testProject_vars("PROD", test_3EnvVarsMap, map[string]map[string]string{"": {"VAR1": "VRISKA"}, "PROD": {"VAR1": "TEREZI"}, "DEBUG": {"VAR1": "KANAYA"}}),
			expectP:            testProject_vars("PROD", test_3EnvVarsMap, map[string]map[string]string{"": {"VAR1": "VRISKA"}, "PROD": {"VAR1": "NEPETA"}, "DEBUG": {"VAR1": "KANAYA"}}),
			expectStdoutOutput: "Set ${VAR1} to \"NEPETA\"\n",
		},
		{
			name:               "--current, current=default, var not present",
			args:               []string{"vars", "var1", "VRISKA", "--current"},
			p:                  testProject_vars("", test_3EnvVarsMap),
			expectP:            testProject_vars("", test_3EnvVarsMap, map[string]map[string]string{"": {"VAR1": "VRISKA"}}),
			expectStdoutOutput: "Set ${VAR1} to \"VRISKA\" in current env\n",
		},
		{
			name:               "--current, current=default, var present",
			args:               []string{"vars", "var1", "NEPETA", "--current"},
			p:                  testProject_vars("", test_3EnvVarsMap, map[string]map[string]string{"": {"VAR1": "VRISKA"}}),
			expectP:            testProject_vars("", test_3EnvVarsMap, map[string]map[string]string{"": {"VAR1": "NEPETA"}}),
			expectStdoutOutput: "Set ${VAR1} to \"NEPETA\" in current env\n",
		},
		{
			name:               "--current, current=non-default, var not present",
			args:               []string{"vars", "var1", "VRISKA", "--current"},
			p:                  testProject_vars("PROD", test_3EnvVarsMap),
			expectP:            testProject_vars("PROD", test_3EnvVarsMap, map[string]map[string]string{"": {"VAR1": ""}, "PROD": {"VAR1": "VRISKA"}}),
			expectStdoutOutput: "Set ${VAR1} to \"VRISKA\" in current env\n",
		},
		{
			name:               "--current, current=non-default, var present in default",
			args:               []string{"vars", "var1", "NEPETA", "--current"},
			p:                  testProject_vars("PROD", test_3EnvVarsMap, map[string]map[string]string{"": {"VAR1": "VRISKA"}}),
			expectP:            testProject_vars("PROD", test_3EnvVarsMap, map[string]map[string]string{"": {"VAR1": "VRISKA"}, "PROD": {"VAR1": "NEPETA"}}),
			expectStdoutOutput: "Set ${VAR1} to \"NEPETA\" in current env\n",
		},
		{
			name:               "--env=other, current=default, var not present",
			args:               []string{"vars", "var1", "VRISKA", "--env", "PROD"},
			p:                  testProject_vars("", test_3EnvVarsMap),
			expectP:            testProject_vars("", test_3EnvVarsMap, map[string]map[string]string{"": {"VAR1": ""}, "PROD": {"VAR1": "VRISKA"}}),
			expectStdoutOutput: "Set ${VAR1} to \"VRISKA\" in env PROD\n",
		},
		{
			name:               "--env=other, current=default, var present",
			args:               []string{"vars", "var1", "NEPETA", "--env", "PROD"},
			p:                  testProject_vars("", test_3EnvVarsMap, map[string]map[string]string{"": {"VAR1": "TEREZI"}, "PROD": {"VAR1": "VRISKA"}}),
			expectP:            testProject_vars("", test_3EnvVarsMap, map[string]map[string]string{"": {"VAR1": "TEREZI"}, "PROD": {"VAR1": "NEPETA"}}),
			expectStdoutOutput: "Set ${VAR1} to \"NEPETA\" in env PROD\n",
		},
		{
			name:               "--env=current, current=non-default, var not present",
			args:               []string{"vars", "var1", "VRISKA", "--env", "PROD"},
			p:                  testProject_vars("PROD", test_3EnvVarsMap),
			expectP:            testProject_vars("PROD", test_3EnvVarsMap, map[string]map[string]string{"": {"VAR1": ""}, "PROD": {"VAR1": "VRISKA"}}),
			expectStdoutOutput: "Set ${VAR1} to \"VRISKA\" in env PROD\n",
		},
		{
			name:               "--env=current, current=non-default, var present",
			args:               []string{"vars", "var1", "NEPETA", "--env", "PROD"},
			p:                  testProject_vars("PROD", test_3EnvVarsMap, map[string]map[string]string{"": {"VAR1": "TEREZI"}, "PROD": {"VAR1": "VRISKA"}}),
			expectP:            testProject_vars("PROD", test_3EnvVarsMap, map[string]map[string]string{"": {"VAR1": "TEREZI"}, "PROD": {"VAR1": "NEPETA"}}),
			expectStdoutOutput: "Set ${VAR1} to \"NEPETA\" in env PROD\n",
		},
		{
			name:               "--env=other, current=non-default, var not present",
			args:               []string{"vars", "var1", "VRISKA", "--env", "DEBUG"},
			p:                  testProject_vars("PROD", test_3EnvVarsMap),
			expectP:            testProject_vars("PROD", test_3EnvVarsMap, map[string]map[string]string{"": {"VAR1": ""}, "DEBUG": {"VAR1": "VRISKA"}}),
			expectStdoutOutput: "Set ${VAR1} to \"VRISKA\" in env DEBUG\n",
		},
		{
			name:               "--env=other, current=non-default, var present in default",
			args:               []string{"vars", "var1", "NEPETA", "--env", "DEBUG"},
			p:                  testProject_vars("PROD", test_3EnvVarsMap, map[string]map[string]string{"": {"VAR1": "TEREZI"}, "DEBUG": {"VAR1": "VRISKA"}}),
			expectP:            testProject_vars("PROD", test_3EnvVarsMap, map[string]map[string]string{"": {"VAR1": "TEREZI"}, "DEBUG": {"VAR1": "NEPETA"}}),
			expectStdoutOutput: "Set ${VAR1} to \"NEPETA\" in env DEBUG\n",
		},
		{
			name:      "--env=default ERRORS",
			args:      []string{"vars", "var1", "VRISKA", "--env", reservedDefaultEnvName},
			p:         testProject_vars("PROD", test_3EnvVarsMap),
			expectErr: "cannot specify reserved env name \"<DEFAULT>\"; use --default to set in default env",
		},
		{
			name:      "--env='' ERRORS",
			args:      []string{"vars", "var1", "VRISKA", "--env", ""},
			p:         testProject_vars("DEBUG", test_3EnvVarsMap),
			expectErr: "cannot specify env \"\"; use --default to set in default env",
		},
		{
			name:               "--default, current=default, var not present",
			args:               []string{"vars", "var1", "VRISKA", "--default"},
			p:                  testProject_vars("", test_3EnvVarsMap),
			expectP:            testProject_vars("", test_3EnvVarsMap, map[string]map[string]string{"": {"VAR1": "VRISKA"}}),
			expectStdoutOutput: "Set ${VAR1} to \"VRISKA\" in default env\n",
		},
		{
			name:               "--default, current=default, var present",
			args:               []string{"vars", "var1", "NEPETA", "--default"},
			p:                  testProject_vars("", test_3EnvVarsMap, map[string]map[string]string{"": {"VAR1": "VRISKA"}}),
			expectP:            testProject_vars("", test_3EnvVarsMap, map[string]map[string]string{"": {"VAR1": "NEPETA"}}),
			expectStdoutOutput: "Set ${VAR1} to \"NEPETA\" in default env\n",
		},
		{
			name:               "--default, current=non-default, var not present",
			args:               []string{"vars", "var1", "VRISKA", "--default"},
			p:                  testProject_vars("PROD", test_3EnvVarsMap),
			expectP:            testProject_vars("PROD", test_3EnvVarsMap, map[string]map[string]string{"": {"VAR1": "VRISKA"}}),
			expectStdoutOutput: "Set ${VAR1} to \"VRISKA\" in default env\n",
		},
		{
			name:               "--default, current=non-default, var present in default",
			args:               []string{"vars", "var1", "NEPETA", "--default"},
			p:                  testProject_vars("PROD", test_3EnvVarsMap, map[string]map[string]string{"": {"VAR1": "VRISKA"}}),
			expectP:            testProject_vars("PROD", test_3EnvVarsMap, map[string]map[string]string{"": {"VAR1": "NEPETA"}}),
			expectStdoutOutput: "Set ${VAR1} to \"NEPETA\" in default env\n",
		},
		{
			name:               "--all, from nil varset",
			args:               []string{"vars", "var1", "VRISKA", "--all"},
			p:                  testProject_vars("PROD", nil),
			expectP:            testProject_vars("PROD", map[string]map[string]string{"": {"VAR1": "VRISKA"}, "PROD": {"VAR1": "VRISKA"}}),
			expectStdoutOutput: "Set ${VAR1} to \"VRISKA\" in all envs\n",
		},
		{
			name:               "--all, from empty varset",
			args:               []string{"vars", "var1", "VRISKA", "--all"},
			p:                  testProject_vars("PROD", map[string]map[string]string{}),
			expectP:            testProject_vars("PROD", map[string]map[string]string{"": {"VAR1": "VRISKA"}, "PROD": {"VAR1": "VRISKA"}}),
			expectStdoutOutput: "Set ${VAR1} to \"VRISKA\" in all envs\n",
		},
		{
			name:               "--all, var did not exist",
			args:               []string{"vars", "var1", "VRISKA", "--all"},
			p:                  testProject_vars("PROD", test_3EnvVarsMap),
			expectP:            testProject_vars("PROD", test_3EnvVarsMap, map[string]map[string]string{"": {"VAR1": "VRISKA"}, "PROD": {"VAR1": "VRISKA"}, "DEBUG": {"VAR1": "VRISKA"}}),
			expectStdoutOutput: "Set ${VAR1} to \"VRISKA\" in all envs\n",
		},
		{
			name:               "--all, var exists in some envs",
			args:               []string{"vars", "var1", "VRISKA", "--all"},
			p:                  testProject_vars("", test_3EnvVarsMap, map[string]map[string]string{"": {"VAR1": ""}, "PROD": {"VAR1": "NEPETA"}}),
			expectP:            testProject_vars("", test_3EnvVarsMap, map[string]map[string]string{"": {"VAR1": "VRISKA"}, "PROD": {"VAR1": "VRISKA"}, "DEBUG": {"VAR1": "VRISKA"}}),
			expectStdoutOutput: "Set ${VAR1} to \"VRISKA\" in all envs\n",
		},
		{
			name:               "--all, var exists in all envs",
			args:               []string{"vars", "var1", "VRISKA", "--all"},
			p:                  testProject_vars("", test_3EnvVarsMap, map[string]map[string]string{"": {"VAR1": "VRISKA"}, "PROD": {"VAR1": "KANAYA"}, "DEBUG": {"VAR1": "TEREZI"}}),
			expectP:            testProject_vars("", test_3EnvVarsMap, map[string]map[string]string{"": {"VAR1": "VRISKA"}, "PROD": {"VAR1": "VRISKA"}, "DEBUG": {"VAR1": "VRISKA"}}),
			expectStdoutOutput: "Set ${VAR1} to \"VRISKA\" in all envs\n",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert := assert.New(t)
			resetVarsFlags()

			// create project and dump config to a temp dir
			projFilePath := createTestProjectIO(t, tc.p)
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

			assert_projectFilesInBuffersMatch(assert, tc.expectP)
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
	flags.BQuiet = false

	varsCmd.Flags().VisitAll(func(fl *pflag.Flag) {
		fl.Changed = false
	})
}
