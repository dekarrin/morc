package commands

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dekarrin/morc"
	"github.com/dekarrin/morc/cmd/morc/commonflags"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
)

func Test_Reqs_Delete(t *testing.T) {
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
			name:      "no reqs present - empty project",
			args:      []string{"reqs", "-D", "req1"},
			p:         morc.Project{},
			expectErr: "no request named req1 exists",
		},
		{
			name:      "no reqs present - empty RequestTemplates",
			args:      []string{"reqs", "-D", "req1"},
			p:         morc.Project{Templates: map[string]morc.RequestTemplate{}},
			expectErr: "no request named req1 exists",
		},
		{
			name:      "delete needs value",
			args:      []string{"reqs", "-D"},
			p:         testProject_singleReqWillAllPropertiesSet(),
			expectErr: "flag needs an argument: 'D'",
		},
		{
			name: "normal delete",
			args: []string{"reqs", "-D", "req1"},
			p:    testProject_singleReqWillAllPropertiesSet(),
			expectP: morc.Project{
				Templates: map[string]morc.RequestTemplate{},
			},
			expectStdoutOutput: "Deleted request req1\n",
		},
		{
			name: "in a flow - can't delete",
			args: []string{"reqs", "-D", "req1"},
			p: morc.Project{
				Templates: map[string]morc.RequestTemplate{"req1": testRequest_withAllPropertiesSet()},
				Flows: map[string]morc.Flow{
					"testflow": {
						Name: "testflow",
						Steps: []morc.FlowStep{
							{Template: "req1"},
						},
					},
				},
			},
			expectErr: "req1 is used in flow testflow\nUse -f to force-delete",
		},
		{
			name: "in a flow - delete with force",
			args: []string{"reqs", "-D", "req1", "-f"},
			p: morc.Project{
				Templates: map[string]morc.RequestTemplate{"req1": testRequest_withAllPropertiesSet()},
				Flows: map[string]morc.Flow{
					"testflow": {
						Name: "testflow",
						Steps: []morc.FlowStep{
							{Template: "req1"},
						},
					},
				},
			},
			expectP: morc.Project{
				Templates: map[string]morc.RequestTemplate{},
				Flows: map[string]morc.Flow{
					"testflow": {
						Name: "testflow",
						Steps: []morc.FlowStep{
							{Template: "req1"},
						},
					},
				},
			},
			expectStdoutOutput: "Deleted request req1\n",
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

func Test_Reqs_Edit(t *testing.T) {
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
			args:               []string{"reqs", "req1", "-n", "nepeta"},
			p:                  testProject_withRequests(morc.RequestTemplate{Name: "req1"}),
			expectP:            testProject_withRequests(morc.RequestTemplate{Name: "nepeta"}),
			expectStdoutOutput: "Set request name to nepeta\n",
		},
		{
			name:               "set method",
			args:               []string{"reqs", "req1", "-X", "GET"},
			p:                  testProject_withRequests(morc.RequestTemplate{Name: "req1", Method: "POST"}),
			expectP:            testProject_withRequests(morc.RequestTemplate{Name: "req1", Method: "GET"}),
			expectStdoutOutput: "Set request method to GET\n",
		},
		{
			name:               "set url",
			args:               []string{"reqs", "req1", "-u", "https://test.example.com/"},
			p:                  testProject_withRequests(morc.RequestTemplate{Name: "req1", URL: "https://www.example.com/"}),
			expectP:            testProject_withRequests(morc.RequestTemplate{Name: "req1", URL: "https://test.example.com/"}),
			expectStdoutOutput: "Set request URL to https://test.example.com/\n",
		},
		{
			name:               "set body",
			args:               []string{"reqs", "req1", "-d", `{"name":"JACK NOIR"}`},
			p:                  testProject_withRequests(morc.RequestTemplate{Name: "req1"}),
			expectP:            testProject_withRequests(morc.RequestTemplate{Name: "req1", Body: []byte(`{"name":"JACK NOIR"}`)}),
			expectStdoutOutput: "Set request body to data with length 20\n",
		},
		{
			name:               "remove body",
			args:               []string{"reqs", "req1", "--remove-body"},
			p:                  testProject_withRequests(morc.RequestTemplate{Name: "req1", Body: []byte(`{"name":"JACK NOIR"}`)}),
			expectP:            testProject_withRequests(morc.RequestTemplate{Name: "req1"}),
			expectStdoutOutput: "Set request body to (none)\n",
		},
		{
			name: "add header (none present)",
			args: []string{"reqs", "req1", "-H", "User-Agent: morc/0.0.0"},
			p:    testProject_withRequests(morc.RequestTemplate{Name: "req1"}),
			expectP: testProject_withRequests(morc.RequestTemplate{
				Name: "req1",
				Headers: http.Header(map[string][]string{
					"User-Agent": {"morc/0.0.0"},
				}),
			}),
			expectStdoutOutput: "Set header User-Agent to have new value morc/0.0.0\n",
		},
		{
			name: "add header (one present)",
			args: []string{"reqs", "req1", "-H", "User-Agent: morc/0.0.0"},
			p: testProject_withRequests(morc.RequestTemplate{
				Name: "req1",
				Headers: http.Header(map[string][]string{
					"Content-Type": {"application/json"},
				}),
			}),
			expectP: testProject_withRequests(morc.RequestTemplate{
				Name: "req1",
				Headers: http.Header(map[string][]string{
					"Content-Type": {"application/json"},
					"User-Agent":   {"morc/0.0.0"},
				}),
			}),
			expectStdoutOutput: "Set header User-Agent to have new value morc/0.0.0\n",
		},
		{
			name: "add header (key already present)",
			args: []string{"reqs", "req1", "-H", "User-Agent: morc/0.0.0"},
			p: testProject_withRequests(morc.RequestTemplate{
				Name: "req1",
				Headers: http.Header(map[string][]string{
					"User-Agent": {"test/0.0.0"},
				}),
			}),
			expectP: testProject_withRequests(morc.RequestTemplate{
				Name: "req1",
				Headers: http.Header(map[string][]string{
					"User-Agent": {"test/0.0.0", "morc/0.0.0"},
				}),
			}),
			expectStdoutOutput: "Set header User-Agent to have new value morc/0.0.0\n",
		},
		{
			name: "remove header (one present)",
			args: []string{"reqs", "req1", "-r", "User-Agent"},
			p: testProject_withRequests(morc.RequestTemplate{
				Name: "req1",
				Headers: http.Header(map[string][]string{
					"User-Agent": {"morc/0.0.0"},
				}),
			}),
			expectP: testProject_withRequests(morc.RequestTemplate{
				Name:    "req1",
				Headers: http.Header{},
			}),
			expectStdoutOutput: "Set header User-Agent to no longer exist\n",
		},
		{
			name: "remove header (multi present)",
			args: []string{"reqs", "req1", "-r", "User-Agent"},
			p: testProject_withRequests(morc.RequestTemplate{
				Name: "req1",
				Headers: http.Header(map[string][]string{
					"User-Agent": {"test/0.0.0", "morc/0.0.0"},
				}),
			}),
			expectP: testProject_withRequests(morc.RequestTemplate{
				Name: "req1",
				Headers: http.Header{
					"User-Agent": {"test/0.0.0"},
				},
			}),
			expectStdoutOutput: "Set header User-Agent to no longer have value morc/0.0.0\n",
		},
		{
			name:               "remove header (not present)",
			args:               []string{"reqs", "req1", "-r", "User-Agent"},
			p:                  testProject_nRequests(1),
			expectP:            testProject_nRequests(1),
			expectStderrOutput: "No change to header User-Agent; already set to not exist\n",
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
				}
				return
			}

			// assertions

			assert.Equal(tc.expectStdoutOutput, output, "stdout output mismatch")
			assert.Equal(tc.expectStderrOutput, outputErr, "stderr output mismatch")

			assert_projectInFileMatches(assert, tc.expectP, projFilePath)
		})
	}

	t.Run("set body from file", func(t *testing.T) {
		assert := assert.New(t)
		resetReqsFlags()

		p := testProject_withRequests(morc.RequestTemplate{Name: "req1"})
		expectP := testProject_withRequests(morc.RequestTemplate{Name: "req1", Body: []byte(`{"name":"JACK NOIR"}`)})
		expectStdoutOutput := "Set request body to data with length 20\n"

		// create project and dump config to a temp dir
		projFilePath := createTestProjectFiles(t, p)
		projDir := filepath.Dir(projFilePath)
		bodyFilePath := filepath.Join(projDir, "body.json")
		err := os.WriteFile(bodyFilePath, []byte(`{"name":"JACK NOIR"}`), 0644)
		if err != nil {
			t.Fatalf("failed to write body file: %v", err)
		}

		args := []string{"reqs", "req1", "-d", "@" + bodyFilePath}

		// set up the root command and run
		output, outputErr, err := runTestCommand(reqsCmd, projFilePath, args)

		// assert and check stdout and stderr
		if !assert.NoError(err) {
			return
		}

		// assertions

		assert.Equal(expectStdoutOutput, output)
		assert.Equal("", outputErr)

		assert_projectInFileMatches(assert, expectP, projFilePath)
	})

}

func Test_Reqs_New(t *testing.T) {
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
			name:               "method and url defaulted",
			args:               []string{"reqs", "--new", "req1"},
			p:                  morc.Project{},
			expectP:            testProject_withRequests(morc.RequestTemplate{Name: "req1", Method: "GET", URL: "http://example.com"}),
			expectStdoutOutput: "Created new request req1\n",
		},
		{
			name:               "method and url explicitly blank",
			args:               []string{"reqs", "--new", "req1", "-X", "", "-u", ""},
			p:                  morc.Project{},
			expectP:            testProject_withRequests(morc.RequestTemplate{Name: "req1"}),
			expectStdoutOutput: "Created new request req1\n",
		},
		{
			name:               "body initially set",
			args:               []string{"reqs", "--new", "req1", "-d", `{"name":"JACK NOIR"}`},
			p:                  morc.Project{},
			expectP:            testProject_withRequests(morc.RequestTemplate{Name: "req1", Method: "GET", URL: "http://example.com", Body: []byte(`{"name":"JACK NOIR"}`)}),
			expectStdoutOutput: "Created new request req1\n",
		},
		{
			name: "headers initially set",
			args: []string{"reqs", "--new", "req1", "-H", "Content-Type: application/json", "-H", "User-Agent: morc/0.0.0", "-H", "User-Agent: test/0.0.0"},
			p:    morc.Project{},
			expectP: testProject_withRequests(morc.RequestTemplate{
				Name:   "req1",
				Method: "GET",
				URL:    "http://example.com",
				Headers: http.Header(map[string][]string{
					"Content-Type": {"application/json"},
					"User-Agent":   {"morc/0.0.0", "test/0.0.0"},
				}),
			}),
			expectStdoutOutput: "Created new request req1\n",
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
				}
				return
			}

			// assertions

			assert.Equal(tc.expectStdoutOutput, output)
			assert.Equal(tc.expectStderrOutput, outputErr)

			assert_projectInFileMatches(assert, tc.expectP, projFilePath)
		})
	}

	t.Run("body initially set from file", func(t *testing.T) {
		assert := assert.New(t)
		resetReqsFlags()

		p := morc.Project{}
		expectP := testProject_withRequests(morc.RequestTemplate{Name: "req1", Method: "GET", URL: "http://example.com", Body: []byte(`{"name":"JACK NOIR"}`)})
		expectStdoutOutput := "Created new request req1\n"

		// create project and dump config to a temp dir
		projFilePath := createTestProjectFiles(t, p)
		projDir := filepath.Dir(projFilePath)
		bodyFilePath := filepath.Join(projDir, "body.json")
		err := os.WriteFile(bodyFilePath, []byte(`{"name":"JACK NOIR"}`), 0644)
		if err != nil {
			t.Fatalf("failed to write body file: %v", err)
		}

		args := []string{"reqs", "--new", "req1", "-d", "@" + bodyFilePath}

		// set up the root command and run
		output, outputErr, err := runTestCommand(reqsCmd, projFilePath, args)

		// assert and check stdout and stderr
		if !assert.NoError(err) {
			return
		}

		// assertions

		assert.Equal(expectStdoutOutput, output)
		assert.Equal("", outputErr)

		assert_projectInFileMatches(assert, expectP, projFilePath)
	})
}

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
			expectStdoutOutput: "http://example.com\n",
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
			args:               []string{"reqs", "req1", "--get", "auth"},
			p:                  testProject_singleReqWillAllPropertiesSet(),
			expectStdoutOutput: "auth1\n",
		},
		{
			name:               "get data",
			args:               []string{"reqs", "req1", "--get", "data"},
			p:                  testProject_singleReqWillAllPropertiesSet(),
			expectStdoutOutput: "{\n    \"username\": \"grimAuxiliatrix\"\n}\n",
		},
		{
			name:               "get captures",
			args:               []string{"reqs", "req1", "--get", "captures"},
			p:                  testProject_singleReqWillAllPropertiesSet(),
			expectStdoutOutput: "$VAR1 from offset 1,3\n$VAR2 from .key1\n",
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
	commonflags.New = ""
	commonflags.Delete = ""
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
