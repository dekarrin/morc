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

func testRequestsN(n int) map[string]morc.RequestTemplate {
	methods := []string{"GET", "POST", "PATCH", "DELETE", "PUT"}
	urlAppend := 0

	reqs := make(map[string]morc.RequestTemplate)

	for i := 0; i < n; i++ {
		if i > 0 && i%len(methods) == 0 {
			urlAppend++
		}

		tmpl := morc.RequestTemplate{
			Name:   fmt.Sprintf(testRequestBaseName+"%d", i+1),
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
			Template: fmt.Sprintf(testRequestBaseName+"%d", req),
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

func Test_Flows_Edit(t *testing.T) {
	testCases := []struct {
		name               string
		args               []string // DO NOT INCLUDE -F; it is automatically set to a project file
		p                  morc.Project
		expectP            morc.Project
		expectErr          string // set if command.Execute expected to fail, with a string that would be in the error message
		expectStderrOutput string // set with expected output to stderr
		expectOutput       string // set with expected output to stdout
	}{
		{
			name: "set name",
			args: []string{"flows", "test", "name", "nepeta"},
			p: morc.Project{
				Flows:     testFlows_singleFlowWithNSteps(2),
				Templates: testRequestsN(2),
			},
			expectP: morc.Project{
				Flows:     testFlows_singleFlowWithNameAndNSteps("nepeta", 2),
				Templates: testRequestsN(2),
			},
			expectOutput: "Set flow name to nepeta\n",
		},
		{
			name: "replace 2nd step",
			args: []string{"flows", "test", "2", "req1"},
			p: morc.Project{
				Flows:     testFlows_singleFlowWithSequence(1, 2, 3),
				Templates: testRequestsN(3),
			},
			expectP: morc.Project{
				Flows:     testFlows_singleFlowWithSequence(1, 1, 3),
				Templates: testRequestsN(3),
			},
			expectOutput: "Set step #2 to req1\n",
		},
		{
			name: "replace no-op",
			args: []string{"flows", "test", "2", "req2"},
			p: morc.Project{
				Flows:     testFlows_singleFlowWithNSteps(3),
				Templates: testRequestsN(3),
			},
			expectP: morc.Project{
				Flows:     testFlows_singleFlowWithNSteps(3),
				Templates: testRequestsN(3),
			},
			expectStderrOutput: "No change to step #2; already set to req2\n",
		},
		{
			name: "add at start",
			args: []string{"flows", testFlowName, "-a", "1:req2"},
			p: morc.Project{
				Flows:     testFlows_singleFlowWithNSteps(3),
				Templates: testRequestsN(3),
			},
			expectP: morc.Project{
				Flows:     testFlows_singleFlowWithSequence(2, 1, 2, 3),
				Templates: testRequestsN(3),
			},
			expectOutput: "Set step #1 to req2 (added)\n",
		},
		// {
		// 	name: "add at end (implied position)",
		// 	p: morc.Project{
		// 		Flows: map[string]morc.Flow{
		// 			"test": {
		// 				Name: "test",
		// 				Steps: []morc.FlowStep{
		// 					{Template: "req1"},
		// 					{Template: "req2"},
		// 					{Template: "req3"},
		// 				},
		// 			},
		// 		},
		// 		Templates: map[string]morc.RequestTemplate{
		// 			"req1": {Name: "req1", Method: "GET", URL: "https://example.com"},
		// 			"req2": {Name: "req2", Method: "POST", URL: "https://example.com"},
		// 			"req3": {Name: "req2", Method: "POST", URL: "https://example.com"},
		// 		},
		// 	},
		// 	args: []string{"flows", "test", "-a", "req2"},
		// 	expectP: morc.Project{
		// 		Flows: map[string]morc.Flow{
		// 			"test": {
		// 				Name: "test",
		// 				Steps: []morc.FlowStep{
		// 					{Template: "req1"},
		// 					{Template: "req2"},
		// 					{Template: "req3"},
		// 					{Template: "req2"},
		// 				},
		// 			},
		// 		},
		// 		Templates: map[string]morc.RequestTemplate{
		// 			"req1": {Name: "req1", Method: "GET", URL: "https://example.com"},
		// 			"req2": {Name: "req2", Method: "POST", URL: "https://example.com"},
		// 			"req3": {Name: "req3", Method: "POST", URL: "https://example.com"},
		// 		},
		// 	},
		// 	expectOutput: "Set step #4 to req2 (added)\n",
		// },
		{
			name: "move first to third",
			args: []string{"flows", "test", "-m", "1:3"},
			p: morc.Project{
				Flows:     testFlows_singleFlowWithNSteps(3),
				Templates: testRequestsN(3),
			},
			expectP: morc.Project{
				Flows:     testFlows_singleFlowWithSequence(2, 3, 1),
				Templates: testRequestsN(3),
			},
			expectOutput: "Set step #1 to position #3\n",
		},
		{
			name: "move second to end",
			args: []string{"flows", "test", "-m", "2:"},
			p: morc.Project{
				Flows:     testFlows_singleFlowWithNSteps(3),
				Templates: testRequestsN(3),
			},
			expectP: morc.Project{
				Flows:     testFlows_singleFlowWithSequence(1, 3, 2),
				Templates: testRequestsN(3),
			},
			expectOutput: "Set step #2 to position #3\n",
		},
		{
			name: "move third to first",
			p: morc.Project{
				Flows:     testFlows_singleFlowWithNSteps(3),
				Templates: testRequestsN(3),
			},
			args: []string{"flows", "test", "-m", "3:1"},
			expectP: morc.Project{
				Flows:     testFlows_singleFlowWithSequence(3, 1, 2),
				Templates: testRequestsN(3),
			},
			expectOutput: "Set step #3 to position #1\n",
		},
		{
			name: "no-op move",
			args: []string{"flows", "test", "-m", "3:"},
			p: morc.Project{
				Flows:     testFlows_singleFlowWithNSteps(3),
				Templates: testRequestsN(3),
			},
			expectP: morc.Project{
				Flows:     testFlows_singleFlowWithNSteps(3),
				Templates: testRequestsN(3),
			},
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

			assert.Equal(tc.expectOutput, output, "stdout output mismatch")
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
			expectErr: "no flow named test exists in project",
		},

		{
			name: "get first request",
			p: morc.Project{
				Flows: map[string]morc.Flow{
					"test": {
						Name: "test",
						Steps: []morc.FlowStep{
							{Template: "req1"},
							{Template: "req2"},
						},
					},
				},
				Templates: map[string]morc.RequestTemplate{
					"req1": {Name: "req1", Method: "GET", URL: "https://example.com"},
					"req2": {Name: "req2", Method: "POST", URL: "https://example.com"},
					"req3": {Name: "req3", Method: "PATCH", URL: "https://example.com"},
				},
			},
			args:         []string{"flows", "test", "1"},
			expectOutput: "req1\n",
		},
		{
			name: "get second request",
			p: morc.Project{
				Flows: map[string]morc.Flow{
					"test": {
						Name: "test",
						Steps: []morc.FlowStep{
							{Template: "req1"},
							{Template: "req2"},
						},
					},
				},
				Templates: map[string]morc.RequestTemplate{
					"req1": {Name: "req1", Method: "GET", URL: "https://example.com"},
					"req2": {Name: "req2", Method: "POST", URL: "https://example.com"},
					"req3": {Name: "req3", Method: "PATCH", URL: "https://example.com"},
				},
			},
			args:         []string{"flows", "test", "2"},
			expectOutput: "req2\n",
		},
		{
			name: "get name",
			p: morc.Project{
				Flows: map[string]morc.Flow{
					"test": {
						Name: "test",
						Steps: []morc.FlowStep{
							{Template: "req1"},
							{Template: "req2"},
						},
					},
				},
				Templates: map[string]morc.RequestTemplate{
					"req1": {Name: "req1", Method: "GET", URL: "https://example.com"},
					"req2": {Name: "req2", Method: "POST", URL: "https://example.com"},
					"req3": {Name: "req3", Method: "PATCH", URL: "https://example.com"},
				},
			},
			args:         []string{"flows", "test", "NAME"},
			expectOutput: "test\n",
		},
		{
			name: "get 0th errors",
			p: morc.Project{
				Flows: map[string]morc.Flow{
					"test": {
						Name: "test",
						Steps: []morc.FlowStep{
							{Template: "req1"},
							{Template: "req2"},
						},
					},
				},
				Templates: map[string]morc.RequestTemplate{
					"req1": {Name: "req1", Method: "GET", URL: "https://example.com"},
					"req2": {Name: "req2", Method: "POST", URL: "https://example.com"},
					"req3": {Name: "req3", Method: "PATCH", URL: "https://example.com"},
				},
			},
			args:      []string{"flows", "test", "0"},
			expectErr: "does not exist",
		},
		{
			name: "get too big errors",
			p: morc.Project{
				Flows: map[string]morc.Flow{
					"test": {
						Name: "test",
						Steps: []morc.FlowStep{
							{Template: "req1"},
							{Template: "req2"},
						},
					},
				},
				Templates: map[string]morc.RequestTemplate{
					"req1": {Name: "req1", Method: "GET", URL: "https://example.com"},
					"req2": {Name: "req2", Method: "POST", URL: "https://example.com"},
					"req3": {Name: "req3", Method: "PATCH", URL: "https://example.com"},
				},
			},
			args:      []string{"flows", "test", "3"},
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

			assert.Equal(tc.expectOutput, output)
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
		p                  morc.Project
		args               []string // DO NOT INCLUDE -F; it is automatically set to a project file
		expectP            morc.Project
		expectErr          string // set if command.Execute expected to fail, with a string that would be in the error message
		expectStderrOutput string // set with expected output to stderr
		expectOutput       string // set with expected output to stdout
	}{
		{
			name: "happy path - 2 requests",
			p: morc.Project{
				Templates: map[string]morc.RequestTemplate{
					"req1": {Name: "req1", Method: "GET", URL: "https://example.com"},
					"req2": {Name: "req2", Method: "POST", URL: "https://example.com"},
				},
			},
			expectP: morc.Project{
				Flows: map[string]morc.Flow{
					"test": {
						Name: "test",
						Steps: []morc.FlowStep{
							{Template: "req1"},
							{Template: "req2"},
						},
					},
				},
				Templates: map[string]morc.RequestTemplate{
					"req1": {Name: "req1", Method: "GET", URL: "https://example.com"},
					"req2": {Name: "req2", Method: "POST", URL: "https://example.com"},
				},
			},
			args:         []string{"flows", "--new", "test", "req1", "req2"},
			expectOutput: "Created new flow test with 2 steps\n",
		},
		{
			name: "happy path - 3 requests",
			p: morc.Project{
				Templates: map[string]morc.RequestTemplate{
					"req1": {Name: "req1", Method: "GET", URL: "https://example.com"},
					"req2": {Name: "req2", Method: "POST", URL: "https://example.com"},
					"req3": {Name: "req3", Method: "PATCH", URL: "https://example.com"},
				},
			},
			args:         []string{"flows", "test", "req1", "req2", "req3", "--new"},
			expectOutput: "Created new flow test with 3 steps\n",
			expectP: morc.Project{
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
					"req1": {Name: "req1", Method: "GET", URL: "https://example.com"},
					"req2": {Name: "req2", Method: "POST", URL: "https://example.com"},
					"req3": {Name: "req3", Method: "PATCH", URL: "https://example.com"},
				},
			},
		},
		{
			name: "need more than 1 request",
			p: morc.Project{
				Templates: map[string]morc.RequestTemplate{
					"req1": {Name: "req1", Method: "GET", URL: "https://example.com"},
					"req2": {Name: "req2", Method: "POST", URL: "https://example.com"},
				},
			},
			args:      []string{"flows", "--new", "test", "req1"},
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

			assert.Equal(tc.expectOutput, output)
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
			expectErr: "no flow named test exists in project",
		},
		{
			name:      "flow is explicitly blank",
			p:         morc.Project{},
			args:      []string{"flows", ""},
			expectErr: "no flow named \"\" exists in project",
		},
		{
			name: "flow is present - no steps",
			p: morc.Project{
				Flows: map[string]morc.Flow{
					"test": {
						Name: "test",
					},
				},
			},
			args:         []string{"flows", "test"},
			expectOutput: "(no steps in flow)\n",
		},
		{
			name: "flow is present - one step is missing",
			p: morc.Project{
				Flows: map[string]morc.Flow{
					"test": {
						Name: "test",
						Steps: []morc.FlowStep{
							{Template: "req1"},
							{Template: "req2"},
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
			args:         []string{"flows", "test"},
			expectOutput: "1: req1 (GET https://example.com)\n2:! req2 (!non-existent req)\n",
		},
		{
			name: "flow is present - one step is unsendable",
			p: morc.Project{
				Flows: map[string]morc.Flow{
					"test": {
						Name: "test",
						Steps: []morc.FlowStep{
							{Template: "req1"},
							{Template: "req2"},
						},
					},
				},
				Templates: map[string]morc.RequestTemplate{
					"req1": {
						Name:   "req1",
						Method: "",
						URL:    "https://example.com",
					},
					"req2": {
						Name:   "req2",
						Method: "POST",
						URL:    "https://example.com",
					},
				},
			},
			args:         []string{"flows", "test"},
			expectOutput: "1:! req1 (??? https://example.com)\n2: req2 (POST https://example.com)\n",
		},
		{
			name: "flow is present - all steps are valid",
			p: morc.Project{
				Flows: map[string]morc.Flow{
					"test": {
						Name: "test",
						Steps: []morc.FlowStep{
							{Template: "req1"},
							{Template: "req2"},
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
						Method: "POST",
						URL:    "https://example.com",
					},
				},
			},
			args:         []string{"flows", "test"},
			expectOutput: "1: req1 (GET https://example.com)\n2: req2 (POST https://example.com)\n",
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

			assert.Equal(tc.expectOutput, output)
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
