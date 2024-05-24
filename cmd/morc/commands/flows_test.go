package commands

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dekarrin/morc"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

const (
	testFlowName        = "test"
	testRequestBaseName = "req"
)

func Test_Flows_Edit(t *testing.T) {
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
			name:               "set name",
			args:               []string{"flows", "test", "name", "nepeta"},
			p:                  testProject_singleFlowWithNSteps(2),
			expectP:            testProject_singleFlowWithNameAndNSteps("nepeta", 2),
			expectStdoutOutput: "Set flow name to nepeta\n",
		},
		{
			name:               "replace 2nd step",
			args:               []string{"flows", "test", "2", "req1"},
			p:                  testProject_singleFlowWithSequence(1, 2, 3),
			expectP:            testProject_singleFlowWithSequence(1, 1, 3),
			expectStdoutOutput: "Set step #2 to req1\n",
		},
		{
			name:               "replace no-op",
			args:               []string{"flows", "test", "2", "req2"},
			p:                  testProject_singleFlowWithNSteps(3),
			expectP:            testProject_singleFlowWithNSteps(3),
			expectStderrOutput: "No change to step #2; already set to req2\n",
		},
		{
			name:               "add at start",
			args:               []string{"flows", "test", "-a", "1:req2"},
			p:                  testProject_singleFlowWithNSteps(3),
			expectP:            testProject_3Requests_singleFlowWithSequence(2, 1, 2, 3),
			expectStdoutOutput: "Set step #1 to req2 (added)\n",
		},
		{
			name:               "add at end (implied position)",
			args:               []string{"flows", "test", "-a", "req2"},
			p:                  testProject_singleFlowWithNSteps(3),
			expectP:            testProject_3Requests_singleFlowWithSequence(1, 2, 3, 2),
			expectStdoutOutput: "Set step #4 to req2 (added)\n",
		},
		{
			name:               "move first to third",
			args:               []string{"flows", "test", "-m", "1:3"},
			p:                  testProject_singleFlowWithNSteps(3),
			expectP:            testProject_singleFlowWithSequence(2, 3, 1),
			expectStdoutOutput: "Set step #1 to position #3\n",
		},
		{
			name:               "move second to end",
			args:               []string{"flows", "test", "-m", "2:"},
			p:                  testProject_singleFlowWithNSteps(3),
			expectP:            testProject_singleFlowWithSequence(1, 3, 2),
			expectStdoutOutput: "Set step #2 to position #3\n",
		},
		{
			name:               "move third to first",
			args:               []string{"flows", "test", "-m", "3:1"},
			p:                  testProject_singleFlowWithNSteps(3),
			expectP:            testProject_singleFlowWithSequence(3, 1, 2),
			expectStdoutOutput: "Set step #3 to position #1\n",
		},
		{
			name:               "no-op move",
			args:               []string{"flows", "test", "-m", "3:"},
			p:                  testProject_singleFlowWithNSteps(3),
			expectP:            testProject_singleFlowWithNSteps(3),
			expectStderrOutput: "No change to step #3; already set to position #3\n",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert := assert.New(t)
			resetFlowsFlags()

			// create project and dump config to a temp dir
			projFilePath := createTestProjectFiles(t, tc.p)
			// set up the root command and run
			output, outputErr, err := runTestCommand(flowsCmd, projFilePath, tc.args)

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

			// okay, check stdout and stderr

			assert.Equal(tc.expectStdoutOutput, output, "stdout output mismatch")
			assert.Equal(tc.expectStderrOutput, outputErr, "stderr output mismatch")

			// ignore the project file path
			tc.expectP.Config.ProjFile = ""
			updatedProj.Config.ProjFile = ""
			assert.Equal(tc.expectP, updatedProj, "resulting project does not match expected")
		})
	}

}

func Test_Flows_Get(t *testing.T) {
	testCases := []struct {
		name               string
		args               []string // DO NOT INCLUDE -F; it is automatically set to a project file
		p                  morc.Project
		expectErr          string // set if command.Execute expected to fail, with a string that would be in the error message
		expectStderrOutput string // set with expected output to stderr
		expectStdoutOutput string // set with expected output to stdout
	}{
		{
			name:      "flow not present",
			args:      []string{"flows", testFlowName},
			p:         morc.Project{},
			expectErr: "no flow named " + testFlowName + " exists in project",
		},
		{
			name:               "get first request",
			args:               []string{"flows", testFlowName, "1"},
			p:                  testProject_singleFlowWithNSteps(2),
			expectStdoutOutput: testReq(1) + "\n",
		},
		{
			name:               "get second request",
			args:               []string{"flows", testFlowName, "2"},
			p:                  testProject_singleFlowWithNSteps(2),
			expectStdoutOutput: testReq(2) + "\n",
		},
		{
			name:               "get name",
			args:               []string{"flows", testFlowName, "NAME"},
			p:                  testProject_singleFlowWithNSteps(2),
			expectStdoutOutput: testFlowName + "\n",
		},
		{
			name:      "error: get 0th step",
			args:      []string{"flows", testFlowName, "0"},
			p:         testProject_singleFlowWithNSteps(2),
			expectErr: "does not exist",
		},
		{
			name:      "get too big errors",
			args:      []string{"flows", testFlowName, "3"},
			p:         testProject_singleFlowWithNSteps(2),
			expectErr: "does not exist",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert := assert.New(t)

			// create project and dump config to a temp dir
			projFilePath := createTestProjectFiles(t, tc.p)
			// set up the root command and run
			output, outputErr, err := runTestCommand(flowsCmd, projFilePath, tc.args)

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

			// reload the project and make sure it matches project at start (no mutations)
			updatedProj, err := morc.LoadProjectFromDisk(projFilePath, true)
			if err != nil {
				t.Fatalf("error loading project post execution: %v", err)
				return
			}

			// okay, check stdout and stderr

			assert.Equal(tc.expectStdoutOutput, output)
			assert.Equal(tc.expectStderrOutput, outputErr)

			// ignore the project file path
			updatedProj.Config.ProjFile = tc.p.Config.ProjFile
			assert.Equal(tc.p, updatedProj)
		})
	}

}

func Test_Flows_New(t *testing.T) {
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
			name:               "happy path - 2 requests",
			args:               []string{"flows", "--new", "test", "req1", "req2"},
			p:                  testProject_nRequests(2),
			expectP:            testProject_singleFlowWithSequence(1, 2),
			expectStdoutOutput: "Created new flow test with 2 steps\n",
		},
		{
			name:               "happy path - 3 requests",
			args:               []string{"flows", "test", "req1", "req2", "req3", "--new"},
			p:                  testProject_nRequests(3),
			expectP:            testProject_singleFlowWithNSteps(3),
			expectStdoutOutput: "Created new flow test with 3 steps\n",
		},
		{
			name:      "need more than 1 request",
			args:      []string{"flows", "--new", "test", "req1"},
			p:         testProject_nRequests(2),
			expectErr: "--new requires a name and at least two requests",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert := assert.New(t)

			resetFlowsFlags()

			// create project and dump config to a temp dir
			projFilePath := createTestProjectFiles(t, tc.p)
			// set up the root command and run
			output, outputErr, err := runTestCommand(flowsCmd, projFilePath, tc.args)

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

			// okay, check stdout and stderr

			assert.Equal(tc.expectStdoutOutput, output)
			assert.Equal(tc.expectStderrOutput, outputErr)

			// ignore the project file path
			tc.expectP.Config.ProjFile = ""
			updatedProj.Config.ProjFile = ""
			assert.Equal(tc.expectP, updatedProj)
		})
	}

}

func Test_Flows_Show(t *testing.T) {
	testCases := []struct {
		name               string
		args               []string // DO NOT INCLUDE -F; it is automatically set to a project file
		p                  morc.Project
		expectErr          string // set if command.Execute expected to fail, with a string that would be in the error message
		expectStderrOutput string // set with expected output to stderr
		expectStdoutOutput string // set with expected output to stdout
	}{
		{
			name:      "flow not present",
			args:      []string{"flows", "test"},
			p:         morc.Project{},
			expectErr: "no flow named test exists in project",
		},
		{
			name:      "flow is explicitly blank",
			args:      []string{"flows", ""},
			p:         morc.Project{},
			expectErr: "no flow named \"\" exists in project",
		},
		{
			name:               "flow is present - no steps",
			args:               []string{"flows", "test"},
			p:                  testProject_singleFlowWithNSteps(0),
			expectStdoutOutput: "(no steps in flow)\n",
		},
		{
			name: "flow is present - one step is missing",
			args: []string{"flows", "test"},
			p: morc.Project{
				Flows:     testFlows_singleFlowWithNSteps(2),
				Templates: testRequestsN(1),
			},
			expectStdoutOutput: "1: req1 (GET https://example.com)\n2:! req2 (!non-existent req)\n",
		},
		{
			name: "flow is present - one step is unsendable",
			args: []string{"flows", "test"},
			p: morc.Project{
				Flows: testFlows_singleFlowWithNSteps(2),
				Templates: map[string]morc.RequestTemplate{
					testReq(1): {
						Name:   testReq(1),
						Method: "",
						URL:    "https://example.com",
					},
					testReq(2): {
						Name:   testReq(2),
						Method: "POST",
						URL:    "https://example.com",
					},
				},
			},
			expectStdoutOutput: "1:! req1 (??? https://example.com)\n2: req2 (POST https://example.com)\n",
		},
		{
			name:               "flow is present - all steps are valid",
			args:               []string{"flows", "test"},
			p:                  testProject_singleFlowWithNSteps(2),
			expectStdoutOutput: "1: req1 (GET https://example.com)\n2: req2 (POST https://example.com)\n",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert := assert.New(t)

			resetFlowsFlags()

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

			assert.Equal(tc.expectStdoutOutput, output)
			assert.Equal(tc.expectStderrOutput, outputErr)
		})
	}

}

func Test_Flows_List(t *testing.T) {
	testCases := []struct {
		name               string
		args               []string // DO NOT INCLUDE -F; it is automatically set to a project file
		p                  morc.Project
		expectErr          string // set if command.Execute expected to fail, with a string that would be in the error message
		expectStderrOutput string // set with expected output to stderr
		expectStdoutOutput string // set with expected output to stdout
	}{
		{
			name:               "no flows present - empty project",
			args:               []string{"flows"},
			p:                  morc.Project{},
			expectStdoutOutput: "(none)\n",
		},
		{
			name:               "no flows present - empty Flows",
			args:               []string{"flows"},
			p:                  morc.Project{Flows: map[string]morc.Flow{}},
			expectStdoutOutput: "(none)\n",
		},
		{
			name:               "one flow present - no steps",
			args:               []string{"flows"},
			p:                  testProject_singleFlowWithNSteps(0),
			expectStdoutOutput: "test:! 0 requests\n",
		},
		{
			name:               "one flow present - 1 step, valid",
			args:               []string{"flows"},
			p:                  testProject_singleFlowWithNSteps(1),
			expectStdoutOutput: "test: 1 request\n",
		},
		{
			name:               "1 flow present - 3 steps, valid",
			args:               []string{"flows"},
			p:                  testProject_singleFlowWithNSteps(3),
			expectStdoutOutput: "test: 3 requests\n",
		},
		{
			name: "one flow present - req not sendable",
			args: []string{"flows"},
			p: morc.Project{
				Flows: testFlows_singleFlowWithNSteps(1),
				Templates: map[string]morc.RequestTemplate{
					testReq(1): {
						Name: testReq(1),
					},
				},
			},
			expectStdoutOutput: "test:! 1 request\n",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert := assert.New(t)

			resetFlowsFlags()

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

			assert.Equal(tc.expectStdoutOutput, output)
			assert.Equal(tc.expectStderrOutput, outputErr)
		})
	}
}

func resetFlowsFlags() {
	flagFlowNew = false
	flagFlowDelete = false
	flagFlowStepRemovals = nil
	flagFlowStepAdds = nil
	flagFlowStepMoves = nil
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

	// set the proj file path in project at this point or there will be issues
	// on persistence
	p.Config.ProjFile, err = filepath.Abs(projFilePath)
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

func testReq(n int) string {
	return fmt.Sprintf(testRequestBaseName+"%d", n)
}

func testProject_nRequests(n int) morc.Project {
	return morc.Project{
		Templates: testRequestsN(n),
	}
}

func testProject_singleFlowWithNSteps(n int) morc.Project {
	return morc.Project{
		Flows:     testFlows_singleFlowWithNSteps(n),
		Templates: testRequestsN(n),
	}
}

func testProject_singleFlowWithNameAndNSteps(name string, n int) morc.Project {
	return morc.Project{
		Flows:     testFlows_singleFlowWithNameAndNSteps(name, n),
		Templates: testRequestsN(n),
	}
}

func testProject_3Requests_singleFlowWithSequence(reqNums ...int) morc.Project {
	return morc.Project{
		Flows:     testFlows_singleFlowWithSequence(reqNums...),
		Templates: testRequestsN(3),
	}
}

func testProject_singleFlowWithSequence(reqNums ...int) morc.Project {
	return morc.Project{
		Flows:     testFlows_singleFlowWithSequence(reqNums...),
		Templates: testRequestsN(len(reqNums)),
	}
}

func testRequestsN(n int) map[string]morc.RequestTemplate {
	methods := []string{"GET", "POST", "PATCH", "DELETE", "PUT"}
	urlAppend := 0

	reqs := make(map[string]morc.RequestTemplate)

	for i := 0; i < n; i++ {
		if i > 0 && i%len(methods) == 0 {
			urlAppend++
		}

		tmpl := morc.RequestTemplate{
			Name:   testReq(i + 1),
			Method: methods[i%len(methods)],
			URL:    "https://example.com",
		}

		if urlAppend > 0 {
			tmpl.URL += fmt.Sprintf("/%d", urlAppend)
		}

		reqs[tmpl.Name] = tmpl
	}

	return reqs
}

func testFlows_singleFlowWithNameAndSequence(name string, reqNums ...int) map[string]morc.Flow {
	fl := morc.Flow{
		Name:  name,
		Steps: make([]morc.FlowStep, len(reqNums)),
	}

	for i, req := range reqNums {
		fl.Steps[i] = morc.FlowStep{
			Template: testReq(req),
		}
	}

	return map[string]morc.Flow{
		strings.ToLower(name): fl,
	}
}

func testFlows_singleFlowWithNameAndNSteps(name string, n int) map[string]morc.Flow {
	sequence := make([]int, n)
	for i := 0; i < n; i++ {
		sequence[i] = i + 1
	}

	return testFlows_singleFlowWithNameAndSequence(name, sequence...)
}

func testFlows_singleFlowWithNSteps(n int) map[string]morc.Flow {
	return testFlows_singleFlowWithNameAndNSteps(testFlowName, n)
}

func testFlows_singleFlowWithSequence(reqNums ...int) map[string]morc.Flow {
	return testFlows_singleFlowWithNameAndSequence(testFlowName, reqNums...)
}
