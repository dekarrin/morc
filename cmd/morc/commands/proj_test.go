package commands

import (
	"strings"
	"testing"

	"github.com/dekarrin/morc"
	"github.com/dekarrin/morc/cmd/morc/cliflags"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
)

func Test_Proj_Show(t *testing.T) {
	testCases := []struct {
		name               string
		args               []string // DO NOT INCLUDE -F; it is automatically set to a project file
		p                  morc.Project
		expectErr          string // set if command.Execute expected to fail, with a string that would be in the error message
		expectStderrOutput string // set with expected output to stderr
		expectStdoutOutput string // set with expected output to stdout
	}{
		{
			name: "show project",
			args: []string{"proj"},
			p:    morc.Project{},
			expectStdoutOutput: `Project: 
0 requests, 0 flows
0 history items
0 variables across 1 environment
0 cookies in active session

Cookie record lifetime: 0s`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert := assert.New(t)
			resetProjFlags()

			// create project and dump config to a temp dir
			projFilePath := createTestProjectFiles(t, tc.p)
			// set up the root command and run
			output, outputErr, err := runTestCommand(projCmd, projFilePath, tc.args)

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

			// reload the project and make sure it matches the expected project
			updatedProj, err := morc.LoadProjectFromDisk(projFilePath, true)
			if err != nil {
				t.Fatalf("error loading project post execution: %v", err)
				return
			}

			// okay, check stdout and stderr, running contains check to be sure

			if tc.expectStdoutOutput != "" {
				assert.Contains(output, tc.expectStdoutOutput, "stdout output mismatch")
			}
			if tc.expectStderrOutput != "" {
				assert.Contains(outputErr, tc.expectStderrOutput, "stderr output mismatch")
			}

			// ignore the project file path
			updatedProj.Config.ProjFile = tc.p.Config.ProjFile
			assert.Equal(tc.p, updatedProj)
		})
	}
}

func Test_Proj_Get(t *testing.T) {
	testCases := []struct {
		name               string
		args               []string // DO NOT INCLUDE -F; it is automatically set to a project file
		p                  morc.Project
		expectErr          string // set if command.Execute expected to fail, with a string that would be in the error message
		expectStderrOutput string // set with expected output to stderr
		expectStdoutOutput string // set with expected output to stdout
	}{
		{
			name: "get name",
			args: []string{"proj", "-G", "name"},
			p: morc.Project{
				Name: "TEST",
			},
			expectStdoutOutput: "TEST\n",
		},
		{
			name: "get cookie lifetime",
			args: []string{"proj", "-G", "cookie-lifetime"},
			p: morc.Project{
				Name: "TEST",
			},
			expectStdoutOutput: "0s\n",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert := assert.New(t)
			resetProjFlags()

			// create project and dump config to a temp dir
			projFilePath := createTestProjectFiles(t, tc.p)
			// set up the root command and run
			output, outputErr, err := runTestCommand(projCmd, projFilePath, tc.args)

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

			// reload the project and make sure it matches the expected project
			updatedProj, err := morc.LoadProjectFromDisk(projFilePath, true)
			if err != nil {
				t.Fatalf("error loading project post execution: %v", err)
				return
			}

			// okay, check stdout and stderr, running contains check to be sure

			assert.Equal(tc.expectStdoutOutput, output, "stdout output mismatch")
			assert.Equal(tc.expectStderrOutput, outputErr, "stderr output mismatch")

			// ignore the project file path
			updatedProj.Config.ProjFile = tc.p.Config.ProjFile
			assert.Equal(tc.p, updatedProj)
		})
	}
}

func resetProjFlags() {
	cliflags.BNew = false
	cliflags.Get = ""
	cliflags.Name = ""
	cliflags.CookieLifetime = ""
	cliflags.SessionFile = ""
	cliflags.HistoryFile = ""
	cliflags.RecordCookies = ""
	cliflags.RecordHistory = ""

	projCmd.Flags().VisitAll(func(fl *pflag.Flag) {
		fl.Changed = false
	})
}
