package commands

import (
	"strings"
	"testing"

	"github.com/dekarrin/morc"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
)

func Test_Reqs_Show(t *testing.T) {
	testCases := []struct {
		name               string
		args               []string // DO NOT INCLUDE -F; it is automatically set to a project file
		p                  morc.Project
		expectErr          string // set if command.Execute expected to fail, with a string that would be in the error message
		expectStderrOutput string // set with expected output to stderr
		expectStdoutOutput string // set with expected output to stdout
	}{
		{
			name:      "req not present",
			args:      []string{"reqs", "test"},
			p:         morc.Project{},
			expectErr: "no req named test exists in project",
		},
		{
			name:      "req is explicitly blank",
			args:      []string{"reqs", ""},
			p:         morc.Project{},
			expectErr: "no req named \"\" exists in project",
		},
		/*{
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
		},*/
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert := assert.New(t)
			resetReqsFlags()

			// create project and dump config to a temp dir
			projTestDir := createTestProjectFiles(t, tc.p)
			// set up the root command and run
			output, outputErr, err := runTestCommand(reqsCmd, projTestDir, tc.args)

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

			assert_projectInFileMatches(assert, tc.p, projTestDir)
		})
	}
}

func Test_Reqs_List(t *testing.T) {
	testCases := []struct {
		name               string
		args               []string // DO NOT INCLUDE -F; it is automatically set to a project file
		p                  morc.Project
		expectErr          string // set if command.Execute expected to fail, with a string that would be in the error message
		expectStderrOutput string // set with expected output to stderr
		expectStdoutOutput string // set with expected output to stdout
	}{
		{
			name:               "no reqs present - empty project",
			args:               []string{"reqs"},
			p:                  morc.Project{},
			expectStdoutOutput: "(none)\n",
		},
		{
			name:               "no reqs present - empty RequsetTemplates",
			args:               []string{"reqs"},
			p:                  morc.Project{Templates: map[string]morc.RequestTemplate{}},
			expectStdoutOutput: "(none)\n",
		},
		{
			name:               "one req present",
			args:               []string{"reqs"},
			p:                  testProject_nRequests(1),
			expectStdoutOutput: "GET req1\n",
		},
		{
			name: "two reqs present - one has no method",
			args: []string{"reqs"},
			p: morc.Project{Templates: map[string]morc.RequestTemplate{
				"req1": {Name: "req1", Method: "GET"},
				"req2": {Name: "req2"},
			}},
			expectStdoutOutput: "GET req1\n??? req2\n",
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
			resetReqsFlags()

			// create project and dump config to a temp dir
			projFilePath := createTestProjectFiles(t, tc.p)
			// set up the root command and run
			output, outputErr, err := runTestCommand(reqsCmd, projFilePath, tc.args)

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

			assert_projectInFileMatches(assert, tc.p, projFilePath)
		})
	}
}

func resetReqsFlags() {
	flagReqsNew = ""
	flagReqsDelete = ""
	flagReqsGet = ""
	flagReqsGetHeader = ""
	flagReqsRemoveHeaders = nil
	flagReqsRemoveBody = false
	flagReqsBodyData = ""
	flagReqsHeaders = nil
	flagReqsMethod = ""
	flagReqsURL = ""
	flagReqsName = ""
	flagReqsDeleteForce = false

	reqsCmd.Flags().VisitAll(func(fl *pflag.Flag) {
		fl.Changed = false
	})
}
