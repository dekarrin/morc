package commands

import (
	"strings"
	"testing"

	"github.com/dekarrin/morc"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
)

func Test_Env_List(t *testing.T) {
	testCases := []struct {
		name               string
		args               []string // DO NOT INCLUDE -F; it is automatically set to a project file
		p                  morc.Project
		expectErr          string // set if command.Execute expected to fail, with a string that would be in the error message
		expectStderrOutput string // set with expected output to stderr
		expectStdoutOutput string // set with expected output to stdout
	}{
		{
			name:               "no envs, empty project - default still exists",
			args:               []string{"env", "--all"},
			p:                  morc.Project{},
			expectStdoutOutput: reservedDefaultEnvName + "\n",
		},
		{
			name:               "no envs, empty project - default still exists, quiet mode still prints",
			args:               []string{"env", "--all", "-q"},
			p:                  morc.Project{},
			expectStdoutOutput: reservedDefaultEnvName + "\n",
		},
		{
			name: "envs are present",
			args: []string{"env", "--all"},
			p: morc.Project{
				Vars: testVarStore("", map[string]map[string]string{
					"env1": {
						"var1": "1",
					},
					"": {
						"var1": "2",
					},
				}),
			},
			expectStdoutOutput: `` +
				reservedDefaultEnvName + "\n" +
				"ENV1\n",
		},
		{
			name: "envs are present, quiet mode still prints",
			args: []string{"env", "--all", "-q"},
			p: morc.Project{
				Vars: testVarStore("", map[string]map[string]string{
					"env1": {
						"var1": "1",
					},
					"": {
						"var1": "2",
					},
				}),
			},
			expectStdoutOutput: `` +
				reservedDefaultEnvName + "\n" +
				"ENV1\n",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert := assert.New(t)
			resetEnvFlags()

			// create project and dump config to a temp dir
			projFilePath := createTestProjectIO(t, tc.p)
			// set up the root command and run
			output, outputErr, err := runTestCommand(envCmd, projFilePath, tc.args)

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

			assert.Equal(tc.expectStdoutOutput, output)
			assert.Equal(tc.expectStderrOutput, outputErr)

			assert_noProjectMutations(assert)
		})
	}
}

func Test_Env_Delete(t *testing.T) {
	testCases := []struct {
		name               string
		args               []string // DO NOT INCLUDE -F; it is automatically set to a project file
		p                  morc.Project
		expectNoModify     bool
		expectP            morc.Project
		expectErr          string // set if command.Execute expected to fail, with a string that would be in the error message
		expectStderrOutput string // set with expected output to stderr
		expectStdoutOutput string // set with expected output to stdout
	}{
		{
			name: "--delete and mutually-exclusive flag errors",
			args: []string{"env", "--delete", "env1", "--delete-all"},
			p: morc.Project{
				Vars: testVarStore("env1", map[string]map[string]string{
					"env1": {"var": "1"},
				}),
			},
			expectErr: "if any flags in the group [all default delete delete-all] are set none of the others can be",
		},
		{
			name: "using reserved constant to delete default errors",
			args: []string{"env", "-D", reservedDefaultEnvName},
			p: morc.Project{
				Vars: testVarStore("env1", map[string]map[string]string{
					"env1": {"var": "1"},
				}),
			},
			expectErr: "cannot use reserved environment name \"" + reservedDefaultEnvName + "\"",
		},
		{
			name: "deleted env exists",
			args: []string{"env", "-D", "env1"},
			p: morc.Project{
				Vars: testVarStore("", map[string]map[string]string{
					"":     {"var": "1"},
					"env1": {"var": "2"},
				}),
			},
			expectP: morc.Project{
				Vars: testVarStore("", map[string]map[string]string{
					"": {"var": "1"},
				}),
			},
			expectStdoutOutput: "Deleted environment \"env1\"\n",
		},
		{
			name: "deleted env exists, quiet mode",
			args: []string{"env", "-D", "env1", "-q"},
			p: morc.Project{
				Vars: testVarStore("", map[string]map[string]string{
					"":     {"var": "1"},
					"env1": {"var": "2"},
				}),
			},
			expectP: morc.Project{
				Vars: testVarStore("", map[string]map[string]string{
					"": {"var": "1"},
				}),
			},
			expectStdoutOutput: "",
		},
		{
			name: "deleted env does not exist",
			args: []string{"env", "-D", "env2"},
			p: morc.Project{
				Vars: testVarStore("env1", map[string]map[string]string{
					"":     {"var": "1"},
					"env1": {"var": "2"},
				}),
			},
			expectNoModify:     true,
			expectStderrOutput: "Environment \"env2\" does not contain any variables\n",
		},
		{
			name: "deleted env does not exist, quiet mode",
			args: []string{"env", "-D", "env2", "-q"},
			p: morc.Project{
				Vars: testVarStore("env1", map[string]map[string]string{
					"":     {"var": "1"},
					"env1": {"var": "2"},
				}),
			},
			expectNoModify:     true,
			expectStderrOutput: "",
		},
		{
			name: "delete all, only default exists",
			args: []string{"env", "--delete-all"},
			p: morc.Project{
				Vars: testVarStore("env1", map[string]map[string]string{
					"": {"var": "1"},
				}),
			},
			expectP: morc.Project{
				Vars: testVarStore("env1", map[string]map[string]string{
					"": {},
				}),
			},
			expectStdoutOutput: "Deleted all environments and variables\n",
		},
		{
			name: "delete all, only default exists, quiet mode",
			args: []string{"env", "--delete-all", "-q"},
			p: morc.Project{
				Vars: testVarStore("env1", map[string]map[string]string{
					"": {"var": "1"},
				}),
			},
			expectP: morc.Project{
				Vars: testVarStore("env1", map[string]map[string]string{
					"": {},
				}),
			},
			expectStdoutOutput: "",
		},
		{
			name: "delete all, default and other exists",
			args: []string{"env", "--delete-all"},
			p: morc.Project{
				Vars: testVarStore("env1", map[string]map[string]string{
					"env1": {"var": "2"},
					"":     {"var": "1", "var2": "VRISKA"},
				}),
			},
			expectP: morc.Project{
				Vars: testVarStore("env1", map[string]map[string]string{
					"": {},
				}),
			},
			expectStdoutOutput: "Deleted all environments and variables\n",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert := assert.New(t)
			resetEnvFlags()

			// create project and dump config to a temp dir
			projFilePath := createTestProjectIO(t, tc.p)
			// set up the root command and run
			output, outputErr, err := runTestCommand(envCmd, projFilePath, tc.args)

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

			if !tc.expectNoModify {
				assert_projectFilesInBuffersMatch(assert, tc.expectP)
			} else {
				assert_noProjectMutations(assert)
			}
		})
	}
}

func Test_Env_Switch(t *testing.T) {
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
			name: "swapping to default via value errors",
			args: []string{"env", reservedDefaultEnvName},
			p: morc.Project{
				Vars: testVarStore("", map[string]map[string]string{
					"": {"var": "1"},
				}),
			},
			expectErr: "cannot specify reserved name \"" + reservedDefaultEnvName + "\"",
		},
		{
			name: "swap to new environment",
			args: []string{"env", "env1"},
			p: morc.Project{
				Vars: testVarStore("", map[string]map[string]string{
					"": {"var": "1"},
				}),
			},
			expectP: morc.Project{
				Vars: testVarStore("env1", map[string]map[string]string{
					"": {"var": "1"},
				}),
			},
			expectStdoutOutput: "Switched to environment \"env1\"\n",
		},
		{
			name: "swap to new environment, quiet mode",
			args: []string{"env", "env1", "-q"},
			p: morc.Project{
				Vars: testVarStore("", map[string]map[string]string{
					"": {"var": "1"},
				}),
			},
			expectP: morc.Project{
				Vars: testVarStore("env1", map[string]map[string]string{
					"": {"var": "1"},
				}),
			},
			expectStdoutOutput: "",
		},
		{
			name: "swap to default env",
			args: []string{"env", "--default"},
			p: morc.Project{
				Vars: testVarStore("env1", map[string]map[string]string{
					"": {"var": "1"},
				}),
			},
			expectP: morc.Project{
				Vars: testVarStore("", map[string]map[string]string{
					"": {"var": "1"},
				}),
			},
			expectStdoutOutput: "Switched to the default environment\n",
		},
		{
			name: "swap to default env, quiet mode",
			args: []string{"env", "--default", "-q"},
			p: morc.Project{
				Vars: testVarStore("env1", map[string]map[string]string{
					"": {"var": "1"},
				}),
			},
			expectP: morc.Project{
				Vars: testVarStore("", map[string]map[string]string{
					"": {"var": "1"},
				}),
			},
			expectStdoutOutput: "",
		},
		{
			name: "swap to current environment",
			args: []string{"env", "env1"},
			p: morc.Project{
				Vars: testVarStore("env1", map[string]map[string]string{
					"": {"var": "1"},
				}),
			},
			expectP: morc.Project{
				Vars: testVarStore("env1", map[string]map[string]string{
					"": {"var": "1"},
				}),
			},
			expectStdoutOutput: "Switched to environment \"env1\"\n",
		},
		{
			name: "swap to current environment, quiet mode",
			args: []string{"env", "env1", "-q"},
			p: morc.Project{
				Vars: testVarStore("env1", map[string]map[string]string{
					"": {"var": "1"},
				}),
			},
			expectP: morc.Project{
				Vars: testVarStore("env1", map[string]map[string]string{
					"": {"var": "1"},
				}),
			},
			expectStdoutOutput: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert := assert.New(t)
			resetEnvFlags()

			// create project and dump config to a temp dir
			projFilePath := createTestProjectIO(t, tc.p)
			// set up the root command and run
			output, outputErr, err := runTestCommand(envCmd, projFilePath, tc.args)

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

			if tc.expectErr != "" {
				t.Fatalf("expected error %q, got none", tc.expectErr)
			}

			// assertions

			assert.Equal(tc.expectStdoutOutput, output, "stdout output mismatch")
			assert.Equal(tc.expectStderrOutput, outputErr, "stderr output mismatch")

			assert_projectFilesInBuffersMatch(assert, tc.expectP)
		})
	}
}

func Test_Env_ShowCurrent(t *testing.T) {
	testCases := []struct {
		name               string
		args               []string // DO NOT INCLUDE -F; it is automatically set to a project file
		p                  morc.Project
		expectErr          string // set if command.Execute expected to fail, with a string that would be in the error message
		expectStderrOutput string // set with expected output to stderr
		expectStdoutOutput string // set with expected output to stdout
	}{
		// * current is default
		// * current is not default
		// * quiet mode variants
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert := assert.New(t)
			resetEnvFlags()

			// create project and dump config to a temp dir
			projFilePath := createTestProjectIO(t, tc.p)
			// set up the root command and run
			output, outputErr, err := runTestCommand(envCmd, projFilePath, tc.args)

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

			assert.Equal(tc.expectStdoutOutput, output)
			assert.Equal(tc.expectStderrOutput, outputErr)

			assert_noProjectMutations(assert)
		})
	}
}

func resetEnvFlags() {
	flags.Delete = ""
	flags.BDeleteAll = false
	flags.BAll = false
	flags.BDefault = false
	flags.BQuiet = false

	envCmd.Flags().VisitAll(func(fl *pflag.Flag) {
		fl.Changed = false
	})
}
