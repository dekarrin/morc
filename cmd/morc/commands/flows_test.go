package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dekarrin/morc"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func Test_Flows_Show(t *testing.T) {
	testCases := []struct {
		name               string
		p                  morc.Project
		args               []string // DO NOT INCLUDE -F; it is automatically set to a project file
		expectErr          string   // set if command.Execute expected to fail, with a string that would be in the error message
		expectStderrOutput string   // set with expected output to stderr
		expectOutput       string   // set with expected output to stdout
	}{
		{
			name:      "flow not present",
			p:         morc.Project{},
			args:      []string{"flows", "test"},
			expectErr: "no flow named test exists",
		},
		{
			name:      "flow is explicitly blank",
			p:         morc.Project{},
			args:      []string{"flows", ""},
			expectErr: "no flow named \"\" exists",
		},
		// {
		// 	name: "flow is present",
		// 	p: morc.Project{
		// 		Flows: map[string]morc.Flow{
		// 			"test": {
		// 				Name: "Test",
		// 				Steps: []morc.FlowStep{
		// 					{Template: "req1"},
		// 				},
		// 			},
		// 		},
		// 	},
		// 	args:            []string{"flows", "test"},
		// 	expectErrOutput: "no flow named test",
		// },
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert := assert.New(t)

			// create project and dump config to a temp dir
			projTestDir := createTestProjectFiles(t, tc.p)
			// set up the root command and run
			output, outputErr, err := runTestCommand(flowsCmd, projTestDir, tc.args)

			// assert and check stdout and stderr
			if err != nil {
				if tc.expectErr == "" {
					t.Fatalf("unexpected returned error: %v", err)
					return
				}
				if !strings.Contains(err.Error(), tc.expectErr) {
					t.Fatalf("expected returned error to contain %q, got %q", tc.expectErr, err)
					return
				}
			}

			// okay, check stdout and stderr

			assert.Equal(tc.expectOutput, output)
			assert.Equal(tc.expectStderrOutput, outputErr)
		})
	}

}

func Test_Flows_List(t *testing.T) {
	testCases := []struct {
		name               string
		p                  morc.Project
		args               []string // DO NOT INCLUDE -F; it is automatically set to a project file
		expectErr          string   // set if command.Execute expected to fail, with a string that would be in the error message
		expectStderrOutput string   // set with expected output to stderr
		expectOutput       string   // set with expected output to stdout
	}{
		{
			name:         "no flows present - empty project",
			p:            morc.Project{},
			args:         []string{"flows"},
			expectOutput: "(none)\n",
		},
		{
			name: "no flows present - empty Flows",
			p: morc.Project{
				Flows: map[string]morc.Flow{},
			},
			args:         []string{"flows"},
			expectOutput: "(none)\n",
		},
		{
			name: "one flow present - no steps",
			p: morc.Project{
				Flows: map[string]morc.Flow{
					"test": {
						Name: "test",
					},
				},
			},
			args:         []string{"flows"},
			expectOutput: "test:! 0 requests\n",
		},
		{
			name: "one flow present - valid",
			p: morc.Project{
				Flows: map[string]morc.Flow{
					"test": {
						Name: "test",
						Steps: []morc.FlowStep{
							{
								Template: "req1",
							},
						},
					},
				},
				Templates: map[string]morc.RequestTemplate{
					"req1": {
						Name:   "req1",
						Method: "GET",
						URL:    "https://example.com",
					},
				},
			},
			args:         []string{"flows"},
			expectOutput: "test: 1 request\n",
		},
		{
			name: "3 flows present - valid",
			p: morc.Project{
				Flows: map[string]morc.Flow{
					"test": {
						Name: "test",
						Steps: []morc.FlowStep{
							{Template: "req1"},
							{Template: "req2"},
							{Template: "req3"},
						},
					},
				},
				Templates: map[string]morc.RequestTemplate{
					"req1": {
						Name:   "req1",
						Method: "GET",
						URL:    "https://example.com",
					},
					"req2": {
						Name:   "req2",
						Method: "GET",
						URL:    "https://example.com",
					},
					"req3": {
						Name:   "req3",
						Method: "GET",
						URL:    "https://example.com",
					},
				},
			},
			args:         []string{"flows"},
			expectOutput: "test: 3 requests\n",
		},
		{
			name: "one flow present - req not sendable",
			p: morc.Project{
				Flows: map[string]morc.Flow{
					"test": {
						Name: "test",
						Steps: []morc.FlowStep{
							{
								Template: "req1",
							},
						},
					},
				},
				Templates: map[string]morc.RequestTemplate{
					"req1": {
						Name: "req1",
					},
				},
			},
			args:         []string{"flows"},
			expectOutput: "test:! 1 request\n",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert := assert.New(t)

			// create project and dump config to a temp dir
			projTestDir := createTestProjectFiles(t, tc.p)
			// set up the root command and run
			output, outputErr, err := runTestCommand(flowsCmd, projTestDir, tc.args)

			// assert and check stdout and stderr
			if err != nil {
				if tc.expectErr == "" {
					t.Fatalf("unexpected returned error: %v", err)
					return
				}
				if !strings.Contains(err.Error(), tc.expectErr) {
					t.Fatalf("expected returned error to contain %q, got %q", tc.expectErr, err)
					return
				}
			}

			// okay, check stdout and stderr

			assert.Equal(tc.expectOutput, output)
			assert.Equal(tc.expectStderrOutput, outputErr)
		})
	}
}

// DO NOT INCLUDE -F IN args!!! It is added automatically from projFilePath
func runTestCommand(cmd *cobra.Command, projFilePath string, args []string) (stdout string, stderr string, err error) {
	stdoutCapture := &bytes.Buffer{}
	stderrCapture := &bytes.Buffer{}

	args = append(args, "-F", projFilePath)

	cmd.Root().SetOut(stdoutCapture)
	cmd.Root().SetErr(stderrCapture)
	cmd.Root().SetArgs(args)

	err = cmd.Execute()
	return stdoutCapture.String(), stderrCapture.String(), err
}

func createTestProjectFiles(t *testing.T, p morc.Project) string {
	dir := t.TempDir()
	projFilePath := filepath.Join(dir, "project.json")
	f, err := os.Create(projFilePath)
	if err != nil {
		t.Fatal(err)
		return ""
	}
	if err := p.Dump(f); err != nil {
		t.Fatal(err)
		return ""
	}
	defer f.Close()

	return projFilePath
}
