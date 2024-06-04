package commands

import (
	"strings"
	"testing"

	"github.com/dekarrin/morc"
	"github.com/dekarrin/morc/cmd/morc/commonflags"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
)

const (
	testFlowName        = "test"
	testRequestBaseName = "req"
)

func Test_Flows_Delete(t *testing.T) {
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
			name:      "no flows present - empty project",
			args:      []string{"flows", "-D", "test"},
			p:         morc.Project{},
			expectErr: "no flow named test exists",
		},
		{
			name:      "no flows present - empty Flows",
			args:      []string{"flows", "-D", "test"},
			p:         morc.Project{Flows: map[string]morc.Flow{}},
			expectErr: "no flow named test exists",
		},
		{
			name:      "delete needs value",
			args:      []string{"flows", "-D"},
			p:         testProject_singleFlowWithNSteps(3),
			expectErr: "flag needs an argument: 'D'",
		},
		{
			name: "normal delete",
			args: []string{"flows", "-D", "test"},
			p:    testProject_singleFlowWithNSteps(3),
			expectP: morc.Project{
				Flows:     map[string]morc.Flow{},
				Templates: testRequestsN(3),
			},
			expectStdoutOutput: "Deleted flow test\n",
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

			// assertions

			assert.Equal(tc.expectStdoutOutput, output, "stdout output mismatch")
			assert.Equal(tc.expectStderrOutput, outputErr, "stderr output mismatch")

			assert_projectInFileMatches(assert, tc.expectP, projFilePath)
		})
	}
}

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
			args:               []string{"flows", "test", "-n", "nepeta"},
			p:                  testProject_singleFlowWithNSteps(2),
			expectP:            testProject_singleFlowWithNameAndNSteps("nepeta", 2),
			expectStdoutOutput: "Set flow name to nepeta\n",
		},
		{
			name:               "update 2nd step",
			args:               []string{"flows", "test", "-u", "1:req1"},
			p:                  testProject_singleFlowWithSequence(1, 2, 3),
			expectP:            testProject_singleFlowWithSequence(1, 1, 3),
			expectStdoutOutput: "Set step[1] to req1\n",
		},
		{
			name:               "update no-op",
			args:               []string{"flows", "test", "-u", "1:req2"},
			p:                  testProject_singleFlowWithNSteps(3),
			expectP:            testProject_singleFlowWithNSteps(3),
			expectStderrOutput: "No change to step[1]; already set to req2\n",
		},
		{
			name:               "add at start",
			args:               []string{"flows", "test", "-a", "0:req2"},
			p:                  testProject_singleFlowWithNSteps(3),
			expectP:            testProject_3Requests_singleFlowWithSequence(2, 1, 2, 3),
			expectStdoutOutput: "Set step[0] to req2 (added)\n",
		},
		{
			name:               "add at end (implied position)",
			args:               []string{"flows", "test", "-a", "req2"},
			p:                  testProject_singleFlowWithNSteps(3),
			expectP:            testProject_3Requests_singleFlowWithSequence(1, 2, 3, 2),
			expectStdoutOutput: "Set step[3] to req2 (added)\n",
		},
		{
			name:               "add at end (explicit position)",
			args:               []string{"flows", "test", "-a", "3:req2"},
			p:                  testProject_singleFlowWithNSteps(3),
			expectP:            testProject_3Requests_singleFlowWithSequence(1, 2, 3, 2),
			expectStdoutOutput: "Set step[3] to req2 (added)\n",
		},
		{
			name:               "remove from start",
			args:               []string{"flows", "test", "-r", "0"},
			p:                  testProject_singleFlowWithNSteps(3),
			expectP:            testProject_3Requests_singleFlowWithSequence(2, 3),
			expectStdoutOutput: "Set step[0] to no longer exist; was req1 (removed)\n",
		},
		{
			name:               "remove from end",
			args:               []string{"flows", "test", "-r", "2"},
			p:                  testProject_singleFlowWithNSteps(3),
			expectP:            testProject_3Requests_singleFlowWithSequence(1, 2),
			expectStdoutOutput: "Set step[2] to no longer exist; was req3 (removed)\n",
		},
		{
			name:               "move first to third",
			args:               []string{"flows", "test", "-m", "0:2"},
			p:                  testProject_singleFlowWithNSteps(3),
			expectP:            testProject_singleFlowWithSequence(2, 3, 1),
			expectStdoutOutput: "Set step[0] to index 2\n",
		},
		{
			name:               "move second to end",
			args:               []string{"flows", "test", "-m", "1:"},
			p:                  testProject_singleFlowWithNSteps(3),
			expectP:            testProject_singleFlowWithSequence(1, 3, 2),
			expectStdoutOutput: "Set step[1] to index 2\n",
		},
		{
			name:               "move third to first",
			args:               []string{"flows", "test", "-m", "2:0"},
			p:                  testProject_singleFlowWithNSteps(3),
			expectP:            testProject_singleFlowWithSequence(3, 1, 2),
			expectStdoutOutput: "Set step[2] to index 0\n",
		},
		{
			name:               "no-op move",
			args:               []string{"flows", "test", "-m", "2:"},
			p:                  testProject_singleFlowWithNSteps(3),
			expectP:            testProject_singleFlowWithNSteps(3),
			expectStderrOutput: "No change to step[2]; already set to index 2\n",
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

			// assertions

			assert.Equal(tc.expectStdoutOutput, output, "stdout output mismatch")
			assert.Equal(tc.expectStderrOutput, outputErr, "stderr output mismatch")

			assert_projectInFileMatches(assert, tc.expectP, projFilePath)
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
			args:      []string{"flows", "test"},
			p:         morc.Project{},
			expectErr: "no flow named test exists in project",
		},
		{
			name:               "get first request",
			args:               []string{"flows", "test", "--get", "0"},
			p:                  testProject_singleFlowWithNSteps(2),
			expectStdoutOutput: testReq(1) + "\n",
		},
		{
			name:               "get second request",
			args:               []string{"flows", "test", "--get", "1"},
			p:                  testProject_singleFlowWithNSteps(2),
			expectStdoutOutput: testReq(2) + "\n",
		},
		{
			name:               "get name",
			args:               []string{"flows", "test", "-G", "NAME"},
			p:                  testProject_singleFlowWithNSteps(2),
			expectStdoutOutput: testFlowName + "\n",
		},
		{
			name:      "error: get -1st step",
			args:      []string{"flows", "test", "--get", "-1"},
			p:         testProject_singleFlowWithNSteps(2),
			expectErr: "must be a step index or one of",
		},
		{
			name:      "get too big errors",
			args:      []string{"flows", "test", "--get", "2"},
			p:         testProject_singleFlowWithNSteps(2),
			expectErr: "does not exist",
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

			// assertions

			assert.Equal(tc.expectStdoutOutput, output)
			assert.Equal(tc.expectStderrOutput, outputErr)

			assert_projectInFileMatches(assert, tc.p, projFilePath)
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
			args:               []string{"flows", "req1", "req2", "req3", "--new", "test"},
			p:                  testProject_nRequests(3),
			expectP:            testProject_singleFlowWithNSteps(3),
			expectStdoutOutput: "Created new flow test with 3 steps\n",
		},
		{
			name:      "need more than 1 request",
			args:      []string{"flows", "--new", "test", "req1"},
			p:         testProject_nRequests(2),
			expectErr: "--new requires at least two requests",
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

			// assertions

			assert.Equal(tc.expectStdoutOutput, output)
			assert.Equal(tc.expectStderrOutput, outputErr)

			assert_projectInFileMatches(assert, tc.expectP, projFilePath)
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
			expectStdoutOutput: "0: req1 (GET https://example.com)\n1:! req2 (!non-existent req)\n",
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
			expectStdoutOutput: "0:! req1 (??? https://example.com)\n1: req2 (POST https://example.com)\n",
		},
		{
			name:               "flow is present - all steps are valid",
			args:               []string{"flows", "test"},
			p:                  testProject_singleFlowWithNSteps(2),
			expectStdoutOutput: "0: req1 (GET https://example.com)\n1: req2 (POST https://example.com)\n",
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
					return
				}
			}

			// assertions

			assert.Equal(tc.expectStdoutOutput, output)
			assert.Equal(tc.expectStderrOutput, outputErr)

			assert_projectInFileMatches(assert, tc.p, projFilePath)
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
					return
				}
			}

			// assertions

			assert.Equal(tc.expectStdoutOutput, output)
			assert.Equal(tc.expectStderrOutput, outputErr)

			assert_projectInFileMatches(assert, tc.p, projFilePath)
		})
	}
}

func resetFlowsFlags() {
	commonflags.ProjectFile = ""
	commonflags.New = ""
	flagFlowDelete = ""
	flagFlowGet = ""
	flagFlowName = ""
	flagFlowStepRemovals = nil
	flagFlowStepAdds = nil
	flagFlowStepMoves = nil
	flagFlowStepReplaces = nil

	flowsCmd.Flags().VisitAll(func(fl *pflag.Flag) {
		fl.Changed = false
	})
}
