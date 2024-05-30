package commands

import (
	"net/http"
	"strings"
	"testing"

	"github.com/dekarrin/morc"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
)

func Test_Reqs_Get(t *testing.T) {
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
			expectErr: "no request named test exists in project",
		},
		{
			name:               "get name",
			args:               []string{"reqs", "req1", "--get", "name"},
			p:                  testProject_singleReqWillAllPropertiesSet(),
			expectStdoutOutput: "req1\n",
		},
		{
			name:               "get method",
			args:               []string{"reqs", "req1", "--get", "METHOD"},
			p:                  testProject_singleReqWillAllPropertiesSet(),
			expectStdoutOutput: "GET\n",
		},
		{
			name:               "get URL",
			args:               []string{"reqs", "req1", "--get", "UrL"},
			p:                  testProject_singleReqWillAllPropertiesSet(),
			expectStdoutOutput: "https://example.com\n",
		},
		{
			name:               "get headers (all)",
			args:               []string{"reqs", "req1", "--get", "headers"},
			p:                  testProject_singleReqWillAllPropertiesSet(),
			expectStdoutOutput: "Content-Type: application/json\nUser-Agent: morc/0.0.0\nUser-Agent: test/0.0.0\n",
		},
		{
			name:               "get specific header, single-valued",
			args:               []string{"reqs", "req1", "--get-header", "Content-Type"},
			p:                  testProject_singleReqWillAllPropertiesSet(),
			expectStdoutOutput: "application/json\n",
		},
		{
			name:               "get specific header, multi-valued",
			args:               []string{"reqs", "req1", "--get-header", "user-agent"},
			p:                  testProject_singleReqWillAllPropertiesSet(),
			expectStdoutOutput: "morc/0.0.0\ntest/0.0.0\n",
		},
		{
			name:               "get auth flow",
			args:               []string{"reqs", "req1", "--get-header", "user-agent"},
			p:                  testProject_singleReqWillAllPropertiesSet(),
			expectStdoutOutput: "morc/0.0.0\ntest/0.0.0\n",
		},
		// {
		// 	name:      "error: get 0th step",
		// 	args:      []string{"flows", "test", "--get", "0"},
		// 	p:         testProject_singleFlowWithNSteps(2),
		// 	expectErr: "does not exist",
		// },
		// {
		// 	name:      "get too big errors",
		// 	args:      []string{"flows", "test", "--get", "3"},
		// 	p:         testProject_singleFlowWithNSteps(2),
		// 	expectErr: "does not exist",
		// },
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
			expectErr: "no request named test exists in project",
		},
		{
			name:      "req is explicitly blank",
			args:      []string{"reqs", ""},
			p:         morc.Project{},
			expectErr: "no request named \"\" exists in project",
		},
		{
			name: "req is present",
			args: []string{"reqs", "req1"},
			p:    testProject_nRequests(1),
			expectStdoutOutput: "" +
				"GET https://example.com\n" +
				"\n" +
				"HEADERS: (none)\n" +
				"\n" +
				"BODY: (none)\n" +
				"\n" +
				"VAR CAPTURES: (none)\n" +
				"\n" +
				"AUTH FLOW: (none)\n",
		},
		{
			name: "req is present, has only name set",
			args: []string{"reqs", "req1"},
			p: morc.Project{
				Templates: map[string]morc.RequestTemplate{
					"req1": {Name: "req1"},
				},
			},
			expectStdoutOutput: "" +
				"(no-method) (no-url)\n" +
				"\n" +
				"HEADERS: (none)\n" +
				"\n" +
				"BODY: (none)\n" +
				"\n" +
				"VAR CAPTURES: (none)\n" +
				"\n" +
				"AUTH FLOW: (none)\n",
		},
		{
			name: "req is present, with body",
			args: []string{"reqs", "req1"},
			p: morc.Project{
				Templates: map[string]morc.RequestTemplate{
					"req1": {Name: "req1", Body: []byte("{\n    \"username\": \"grimAuxiliatrix\"\n}")},
				},
			},
			expectStdoutOutput: "" +
				"(no-method) (no-url)\n" +
				"\n" +
				"HEADERS: (none)\n" +
				"\n" +
				"BODY:\n" +
				"{\n" +
				"    \"username\": \"grimAuxiliatrix\"\n" +
				"}\n" +
				"\n" +
				"VAR CAPTURES: (none)\n" +
				"\n" +
				"AUTH FLOW: (none)\n",
		},
		{
			name: "req is present, with headers",
			args: []string{"reqs", "req1"},
			p: morc.Project{
				Templates: map[string]morc.RequestTemplate{
					"req1": {Name: "req1", Headers: http.Header(map[string][]string{"Content-Type": {"application/json"}})},
				},
			},
			expectStdoutOutput: "" +
				"(no-method) (no-url)\n" +
				"\n" +
				"HEADERS:\n" +
				"Content-Type: application/json\n" +
				"\n" +
				"BODY: (none)\n" +
				"\n" +
				"VAR CAPTURES: (none)\n" +
				"\n" +
				"AUTH FLOW: (none)\n",
		},
		{
			name: "req is present, with caps",
			args: []string{"reqs", "req1"},
			p: morc.Project{
				Templates: map[string]morc.RequestTemplate{
					"req1": {Name: "req1", Captures: map[string]morc.VarScraper{"test": {Name: "test", OffsetStart: 3, OffsetEnd: 5}}},
				},
			},
			expectStdoutOutput: "" +
				"(no-method) (no-url)\n" +
				"\n" +
				"HEADERS: (none)\n" +
				"\n" +
				"BODY: (none)\n" +
				"\n" +
				"VAR CAPTURES:\n" +
				"$TEST from offset 3,5\n" +
				"\n" +
				"AUTH FLOW: (none)\n",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert := assert.New(t)
			resetReqsFlags()

			// create project and dump config to a temp dir
			profFilePath := createTestProjectFiles(t, tc.p)
			// set up the root command and run
			output, outputErr, err := runTestCommand(reqsCmd, profFilePath, tc.args)

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

			assert_projectInFileMatches(assert, tc.p, profFilePath)
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
