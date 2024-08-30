package commands

import (
	"strings"
	"testing"

	"github.com/dekarrin/morc"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
)

func Test_Caps_List(t *testing.T) {
	testCases := []struct {
		name               string
		args               []string // DO NOT INCLUDE -F; it is automatically set to a project file
		p                  morc.Project
		expectErr          string // set if command.Execute expected to fail, with a string that would be in the error message
		expectStderrOutput string // set with expected output to stderr
		expectStdoutOutput string // set with expected output to stdout
	}{
		{
			name:      "req does not exist",
			args:      []string{"caps", "req1"},
			p:         morc.Project{},
			expectErr: "no request template req1",
		},
		{
			name:      "req does not exist, quiet still errors",
			args:      []string{"caps", "req1", "-q"},
			p:         morc.Project{},
			expectErr: "no request template req1",
		},
		{
			name:               "req has no caps",
			args:               []string{"caps", "req1"},
			p:                  testProject_withRequests(morc.RequestTemplate{Name: "req1"}),
			expectStdoutOutput: "(none)\n",
		},
		{
			name:               "req has no caps, quiet",
			args:               []string{"caps", "req1", "-q"},
			p:                  testProject_withRequests(morc.RequestTemplate{Name: "req1"}),
			expectStdoutOutput: "",
		},
		{
			name: "req has 1 cap",
			args: []string{"caps", "req1"},
			p: testProject_withRequests(morc.RequestTemplate{
				Name: "req1",
				Captures: map[string]morc.VarScraper{
					"troll": {
						Name: "troll",
						Steps: []morc.TraversalStep{
							{Key: "data"},
							{Key: "people"},
							{Index: 0},
							{Key: "name"},
							{Key: "first"},
						},
					},
				},
			}),
			expectStdoutOutput: "TROLL from .data.people[0].name.first\n",
		},
		{
			name: "req has 1 cap, full request",
			args: []string{"caps", "req1"},
			p: testProject_withRequests(morc.RequestTemplate{
				Name: "req1",
				Captures: map[string]morc.VarScraper{
					"troll": {
						Name: "troll",
					},
				},
			}),
			expectStdoutOutput: "TROLL from entire response\n",
		},
		{
			name: "req has 1 cap, omitted end",
			args: []string{"caps", "req1"},
			p: testProject_withRequests(morc.RequestTemplate{
				Name: "req1",
				Captures: map[string]morc.VarScraper{
					"troll": {
						Name:        "troll",
						OffsetStart: 12,
					},
				},
			}),
			expectStdoutOutput: "TROLL from offset 12,<END>\n",
		},
		{
			name: "req has 1 cap, quiet still prints",
			args: []string{"caps", "req1", "-q"},
			p: testProject_withRequests(morc.RequestTemplate{
				Name: "req1",
				Captures: map[string]morc.VarScraper{
					"troll": {
						Name: "troll",
						Steps: []morc.TraversalStep{
							{Key: "data"},
							{Key: "people"},
							{Index: 0},
							{Key: "name"},
							{Key: "first"},
						},
					},
				},
			}),
			expectStdoutOutput: "TROLL from .data.people[0].name.first\n",
		},
		{
			name: "req has multiple caps",
			args: []string{"caps", "req1"},
			p: testProject_withRequests(morc.RequestTemplate{
				Name: "req1",
				Captures: map[string]morc.VarScraper{
					"troll": {
						Name: "troll",
						Steps: []morc.TraversalStep{
							{Key: "data"},
							{Key: "people"},
							{Index: 0},
							{Key: "name"},
							{Key: "first"},
						},
					},
					"villain": {
						Name:        "villain",
						OffsetStart: 28,
						OffsetEnd:   36,
					},
					"lakhs": {
						Name: "lakhs",
						Steps: []morc.TraversalStep{
							{Key: "data"},
							{Key: "people"},
							{Index: 13},
							{Key: "salary"},
						},
					},
				},
			}),
			expectStdoutOutput: "" +
				"LAKHS from .data.people[13].salary\n" +
				"TROLL from .data.people[0].name.first\n" +
				"VILLAIN from offset 28,36\n",
		},
		{
			name: "req has multiple caps, quiet still prints",
			args: []string{"caps", "req1", "-q"},
			p: testProject_withRequests(morc.RequestTemplate{
				Name: "req1",
				Captures: map[string]morc.VarScraper{
					"troll": {
						Name: "troll",
						Steps: []morc.TraversalStep{
							{Key: "data"},
							{Key: "people"},
							{Index: 0},
							{Key: "name"},
							{Key: "first"},
						},
					},
					"villain": {
						Name:        "villain",
						OffsetStart: 28,
						OffsetEnd:   36,
					},
					"lakhs": {
						Name: "lakhs",
						Steps: []morc.TraversalStep{
							{Key: "data"},
							{Key: "people"},
							{Index: 13},
							{Key: "salary"},
						},
					},
				},
			}),
			expectStdoutOutput: "" +
				"LAKHS from .data.people[13].salary\n" +
				"TROLL from .data.people[0].name.first\n" +
				"VILLAIN from offset 28,36\n",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert := assert.New(t)
			resetCapsFlags()

			// create project and dump config to a temp dir
			projFilePath := createTestProjectIO(t, tc.p)
			// set up the root command and run
			output, outputErr, err := runTestCommand(capsCmd, projFilePath, tc.args)

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

			assert.Equal(tc.expectStdoutOutput, output)
			assert.Equal(tc.expectStderrOutput, outputErr)

			assert_noProjectMutations(assert)
		})
	}
}

func Test_Caps_New(t *testing.T) {
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
			name:      "req does not exist",
			args:      []string{"caps", "req1", "-N", "troll", "-s", ".data.people[0].name.first"},
			p:         morc.Project{},
			expectErr: "no request template req1",
		},
		{
			name:      "req does not exist, quiet still errors",
			args:      []string{"caps", "req1", "-N", "troll", "-s", ".data.people[0].name.first", "-q"},
			p:         morc.Project{},
			expectErr: "no request template req1",
		},
		{
			name: "var already exists",
			args: []string{"caps", "req1", "-N", "troll", "-s", ".data.people[0].name.first"},
			p: testProject_withRequests(
				morc.RequestTemplate{
					Name: "req1",
					Captures: map[string]morc.VarScraper{
						"troll": {
							Name: "troll",
							Steps: []morc.TraversalStep{
								{Key: "data"},
								{Key: "people"},
								{Index: 0},
								{Key: "name"},
								{Key: "first"},
							},
						},
					},
				},
			),
			expectErr: "request req1 already captures to $TROLL",
		},
		{
			name: "var already exists, quiet still errors",
			args: []string{"caps", "req1", "-N", "troll", "-s", ".data.people[0].name.first", "-q"},
			p: testProject_withRequests(
				morc.RequestTemplate{
					Name: "req1",
					Captures: map[string]morc.VarScraper{
						"troll": {
							Name: "troll",
							Steps: []morc.TraversalStep{
								{Key: "data"},
								{Key: "people"},
								{Index: 0},
								{Key: "name"},
								{Key: "first"},
							},
						},
					},
				},
			),
			expectErr: "request req1 already captures to $TROLL",
		},
		{
			name: "spec is required",
			args: []string{"caps", "req1", "-N", "troll"},
			p: testProject_withRequests(
				morc.RequestTemplate{
					Name:     "req1",
					Captures: map[string]morc.VarScraper{},
				},
			),
			expectErr: "--new/-N requires --spec/-s",
		},
		{
			name: "spec is required, quiet still errors",
			args: []string{"caps", "req1", "-N", "troll", "-q"},
			p: testProject_withRequests(
				morc.RequestTemplate{
					Name:     "req1",
					Captures: map[string]morc.VarScraper{},
				},
			),
			expectErr: "--new/-N requires --spec/-s",
		},
		{
			name: "invalid spec",
			args: []string{"caps", "req1", "-N", "troll", "-s", ".data["},
			p: testProject_withRequests(
				morc.RequestTemplate{
					Name:     "req1",
					Captures: map[string]morc.VarScraper{},
				},
			),
			expectErr: "unterminated index at end of string",
		},
		{
			name: "invalid spec, quiet still errors",
			args: []string{"caps", "req1", "-N", "troll", "-s", ".data[", "-q"},
			p: testProject_withRequests(
				morc.RequestTemplate{
					Name:     "req1",
					Captures: map[string]morc.VarScraper{},
				},
			),
			expectErr: "unterminated index at end of string",
		},
		{
			name: "happy path - json path",
			args: []string{"caps", "req1", "-N", "troll", "-s", ".data.people[0].name.first"},
			p: testProject_withRequests(
				morc.RequestTemplate{
					Name:     "req1",
					Captures: map[string]morc.VarScraper{},
				},
			),
			expectP: testProject_withRequests(
				morc.RequestTemplate{
					Name: "req1",
					Captures: map[string]morc.VarScraper{
						"TROLL": {
							Name: "TROLL",
							Steps: []morc.TraversalStep{
								{Key: "data"},
								{Key: "people"},
								{Index: 0},
								{Key: "name"},
								{Key: "first"},
							},
						},
					},
				},
			),
			expectStdoutOutput: "Added capture from path .data.people[0].name.first to $TROLL on req1\n",
		},
		{
			name: "happy path - json path, quiet mode",
			args: []string{"caps", "req1", "-N", "troll", "-s", ".data.people[0].name.first", "-q"},
			p: testProject_withRequests(
				morc.RequestTemplate{
					Name:     "req1",
					Captures: map[string]morc.VarScraper{},
				},
			),
			expectP: testProject_withRequests(
				morc.RequestTemplate{
					Name: "req1",
					Captures: map[string]morc.VarScraper{
						"TROLL": {
							Name: "TROLL",
							Steps: []morc.TraversalStep{
								{Key: "data"},
								{Key: "people"},
								{Index: 0},
								{Key: "name"},
								{Key: "first"},
							},
						},
					},
				},
			),
			expectStdoutOutput: "",
		},
		{
			name: "happy path - offset missing end",
			args: []string{"caps", "req1", "-N", "troll", "-s", ":28,"},
			p: testProject_withRequests(
				morc.RequestTemplate{
					Name:     "req1",
					Captures: map[string]morc.VarScraper{},
				},
			),
			expectP: testProject_withRequests(
				morc.RequestTemplate{
					Name: "req1",
					Captures: map[string]morc.VarScraper{
						"TROLL": {
							Name:        "TROLL",
							OffsetStart: 28,
							OffsetEnd:   0,
						},
					},
				},
			),
			expectStdoutOutput: "Added capture from offset 28,<END> to $TROLL on req1\n",
		},
		{
			name: "happy path - offset missing start",
			args: []string{"caps", "req1", "-N", "troll", "-s", ":,32"},
			p: testProject_withRequests(
				morc.RequestTemplate{
					Name:     "req1",
					Captures: map[string]morc.VarScraper{},
				},
			),
			expectP: testProject_withRequests(
				morc.RequestTemplate{
					Name: "req1",
					Captures: map[string]morc.VarScraper{
						"TROLL": {
							Name:        "TROLL",
							OffsetStart: 0,
							OffsetEnd:   32,
						},
					},
				},
			),
			expectStdoutOutput: "Added capture from offset 0,32 to $TROLL on req1\n",
		},
		{
			name: "happy path - offset missing both",
			args: []string{"caps", "req1", "-N", "troll", "-s", ":,"},
			p: testProject_withRequests(
				morc.RequestTemplate{
					Name:     "req1",
					Captures: map[string]morc.VarScraper{},
				},
			),
			expectP: testProject_withRequests(
				morc.RequestTemplate{
					Name: "req1",
					Captures: map[string]morc.VarScraper{
						"TROLL": {
							Name: "TROLL",
						},
					},
				},
			),
			expectStdoutOutput: "Added capture from entire response to $TROLL on req1\n",
		},
		{
			name: "happy path - raw",
			args: []string{"caps", "req1", "-N", "troll", "-s", "raw"},
			p: testProject_withRequests(
				morc.RequestTemplate{
					Name:     "req1",
					Captures: map[string]morc.VarScraper{},
				},
			),
			expectP: testProject_withRequests(
				morc.RequestTemplate{
					Name: "req1",
					Captures: map[string]morc.VarScraper{
						"TROLL": {
							Name: "TROLL",
						},
					},
				},
			),
			expectStdoutOutput: "Added capture from entire response to $TROLL on req1\n",
		},
		{
			name: "happy path - offset",
			args: []string{"caps", "req1", "-N", "troll", "-s", ":28,32"},
			p: testProject_withRequests(
				morc.RequestTemplate{
					Name:     "req1",
					Captures: map[string]morc.VarScraper{},
				},
			),
			expectP: testProject_withRequests(
				morc.RequestTemplate{
					Name: "req1",
					Captures: map[string]morc.VarScraper{
						"TROLL": {
							Name:        "TROLL",
							OffsetStart: 28,
							OffsetEnd:   32,
						},
					},
				},
			),
			expectStdoutOutput: "Added capture from offset 28,32 to $TROLL on req1\n",
		},
		{
			name: "happy path - offset, quiet mode",
			args: []string{"caps", "req1", "-N", "troll", "-s", ":28,32", "-q"},
			p: testProject_withRequests(
				morc.RequestTemplate{
					Name:     "req1",
					Captures: map[string]morc.VarScraper{},
				},
			),
			expectP: testProject_withRequests(
				morc.RequestTemplate{
					Name: "req1",
					Captures: map[string]morc.VarScraper{
						"TROLL": {
							Name:        "TROLL",
							OffsetStart: 28,
							OffsetEnd:   32,
						},
					},
				},
			),
			expectStdoutOutput: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert := assert.New(t)
			resetCapsFlags()

			// create project and dump config to a temp dir
			projFilePath := createTestProjectIO(t, tc.p)
			// set up the root command and run
			output, outputErr, err := runTestCommand(capsCmd, projFilePath, tc.args)

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

			if !tc.expectNoModify {
				assert_projectFilesInBuffersMatch(assert, tc.expectP)
			} else {
				assert_noProjectMutations(assert)
			}
		})
	}
}

func Test_Caps_Delete(t *testing.T) {
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
			name: "req does not exist",
			args: []string{"caps", "req1", "-D", "troll"},
			p: testProject_withRequests(
				morc.RequestTemplate{
					Name:     "req2",
					Captures: map[string]morc.VarScraper{},
				},
			),
			expectErr: "no request template req1",
		},
		{
			name: "req does not exist, quiet still errors",
			args: []string{"caps", "req1", "-D", "troll", "-q"},
			p: testProject_withRequests(
				morc.RequestTemplate{
					Name:     "req2",
					Captures: map[string]morc.VarScraper{},
				},
			),
			expectErr: "no request template req1",
		},
		{
			name: "var does not exist",
			args: []string{"caps", "req1", "-D", "troll"},
			p: testProject_withRequests(
				morc.RequestTemplate{
					Name:     "req1",
					Captures: map[string]morc.VarScraper{},
				},
			),
			expectErr: "no capture defined for $TROLL in req1",
		},
		{
			name: "var does not exist, quiet still errors",
			args: []string{"caps", "req1", "-D", "troll", "-q"},
			p: testProject_withRequests(
				morc.RequestTemplate{
					Name:     "req1",
					Captures: map[string]morc.VarScraper{},
				},
			),
			expectErr: "no capture defined for $TROLL in req1",
		},
		{
			name: "var exists",
			args: []string{"caps", "REQ1", "-D", "troll"},
			p: testProject_withRequests(
				morc.RequestTemplate{
					Name: "req1",
					Captures: map[string]morc.VarScraper{
						"TROLL": {
							Name:        "TROLL",
							OffsetStart: 28,
							OffsetEnd:   32,
						},
					},
				},
			),
			expectP: testProject_withRequests(
				morc.RequestTemplate{
					Name:     "req1",
					Captures: map[string]morc.VarScraper{},
				},
			),
			expectStdoutOutput: "Deleted capture to $TROLL from req1\n",
		},
		{
			name: "var exists, quiet mode",
			args: []string{"caps", "REQ1", "-D", "troll", "-q"},
			p: testProject_withRequests(
				morc.RequestTemplate{
					Name: "req1",
					Captures: map[string]morc.VarScraper{
						"TROLL": {
							Name:        "TROLL",
							OffsetStart: 28,
							OffsetEnd:   32,
						},
					},
				},
			),
			expectP: testProject_withRequests(
				morc.RequestTemplate{
					Name:     "req1",
					Captures: map[string]morc.VarScraper{},
				},
			),
			expectStdoutOutput: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert := assert.New(t)
			resetCapsFlags()

			// create project and dump config to a temp dir
			projFilePath := createTestProjectIO(t, tc.p)
			// set up the root command and run
			output, outputErr, err := runTestCommand(capsCmd, projFilePath, tc.args)

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

			if !tc.expectNoModify {
				assert_projectFilesInBuffersMatch(assert, tc.expectP)
			} else {
				assert_noProjectMutations(assert)
			}
		})
	}
}

func Test_Caps_Edit(t *testing.T) {
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
			name: "req does not exist",
			args: []string{"caps", "req1", "troll", "-s", ":28,32"},
			p: testProject_withRequests(
				morc.RequestTemplate{
					Name:     "req2",
					Captures: map[string]morc.VarScraper{},
				},
			),
			expectErr: "no request template req1",
		},
		{
			name: "req does not exist, quiet still errors",
			args: []string{"caps", "req1", "troll", "-s", ":28,32", "-q"},
			p: testProject_withRequests(
				morc.RequestTemplate{
					Name:     "req2",
					Captures: map[string]morc.VarScraper{},
				},
			),
			expectErr: "no request template req1",
		},
		{
			name: "var does not exist",
			args: []string{"caps", "req1", "troll", "-s", ":28,32"},
			p: testProject_withRequests(
				morc.RequestTemplate{
					Name:     "req1",
					Captures: map[string]morc.VarScraper{},
				},
			),
			expectErr: "no capture to variable $TROLL exists in request req1",
		},
		{
			name: "var does not exist, quiet still errors",
			args: []string{"caps", "req1", "troll", "-s", ":28,32", "-q"},
			p: testProject_withRequests(
				morc.RequestTemplate{
					Name:     "req1",
					Captures: map[string]morc.VarScraper{},
				},
			),
			expectErr: "no capture to variable $TROLL exists in request req1",
		},
		{
			name: "alter spec",
			args: []string{"caps", "req1", "troll", "-s", ":28,32"},
			p: testProject_withRequests(
				morc.RequestTemplate{
					Name: "req1",
					Captures: map[string]morc.VarScraper{
						"TROLL": {
							Name: "TROLL",
							Steps: []morc.TraversalStep{
								{Key: "data"},
								{Key: "people"},
								{Index: 0},
								{Key: "name"},
								{Key: "first"},
							},
						},
					},
				},
			),
			expectP: testProject_withRequests(
				morc.RequestTemplate{
					Name: "req1",
					Captures: map[string]morc.VarScraper{
						"TROLL": {
							Name:        "TROLL",
							OffsetStart: 28,
							OffsetEnd:   32,
						},
					},
				},
			),
			expectStdoutOutput: "Set capture specification to offset 28,32\n",
		},
		{
			name: "alter spec, quiet",
			args: []string{"caps", "req1", "troll", "-s", ":28,32", "-q"},
			p: testProject_withRequests(
				morc.RequestTemplate{
					Name: "req1",
					Captures: map[string]morc.VarScraper{
						"TROLL": {
							Name: "TROLL",
							Steps: []morc.TraversalStep{
								{Key: "data"},
								{Key: "people"},
								{Index: 0},
								{Key: "name"},
								{Key: "first"},
							},
						},
					},
				},
			),
			expectP: testProject_withRequests(
				morc.RequestTemplate{
					Name: "req1",
					Captures: map[string]morc.VarScraper{
						"TROLL": {
							Name:        "TROLL",
							OffsetStart: 28,
							OffsetEnd:   32,
						},
					},
				},
			),
			expectStdoutOutput: "",
		},
		{
			name: "alter var",
			args: []string{"caps", "req1", "troll", "-V", "troll_name"},
			p: testProject_withRequests(
				morc.RequestTemplate{
					Name: "req1",
					Captures: map[string]morc.VarScraper{
						"TROLL": {
							Name: "TROLL",
							Steps: []morc.TraversalStep{
								{Key: "data"},
								{Key: "people"},
								{Index: 0},
								{Key: "name"},
								{Key: "first"},
							},
						},
					},
				},
			),
			expectP: testProject_withRequests(
				morc.RequestTemplate{
					Name: "req1",
					Captures: map[string]morc.VarScraper{
						"TROLL_NAME": {
							Name: "TROLL_NAME",
							Steps: []morc.TraversalStep{
								{Key: "data"},
								{Key: "people"},
								{Index: 0},
								{Key: "name"},
								{Key: "first"},
							},
						},
					},
				},
			),
			expectStdoutOutput: "Set captured-to variable to $TROLL_NAME\n",
		},
		{
			name: "alter var, quiet",
			args: []string{"caps", "req1", "troll", "-V", "troll_name", "-q"},
			p: testProject_withRequests(
				morc.RequestTemplate{
					Name: "req1",
					Captures: map[string]morc.VarScraper{
						"TROLL": {
							Name: "TROLL",
							Steps: []morc.TraversalStep{
								{Key: "data"},
								{Key: "people"},
								{Index: 0},
								{Key: "name"},
								{Key: "first"},
							},
						},
					},
				},
			),
			expectP: testProject_withRequests(
				morc.RequestTemplate{
					Name: "req1",
					Captures: map[string]morc.VarScraper{
						"TROLL_NAME": {
							Name: "TROLL_NAME",
							Steps: []morc.TraversalStep{
								{Key: "data"},
								{Key: "people"},
								{Index: 0},
								{Key: "name"},
								{Key: "first"},
							},
						},
					},
				},
			),
			expectStdoutOutput: "",
		},
		{
			name: "alter var and spec",
			args: []string{"caps", "req1", "troll", "-V", "troll_name", "-s", ":28,32"},
			p: testProject_withRequests(
				morc.RequestTemplate{
					Name: "req1",
					Captures: map[string]morc.VarScraper{
						"TROLL": {
							Name: "TROLL",
							Steps: []morc.TraversalStep{
								{Key: "data"},
								{Key: "people"},
								{Index: 0},
								{Key: "name"},
								{Key: "first"},
							},
						},
					},
				},
			),
			expectP: testProject_withRequests(
				morc.RequestTemplate{
					Name: "req1",
					Captures: map[string]morc.VarScraper{
						"TROLL_NAME": {
							Name:        "TROLL_NAME",
							OffsetStart: 28,
							OffsetEnd:   32,
						},
					},
				},
			),
			expectStdoutOutput: "Set captured-to variable to $TROLL_NAME and capture specification to offset 28,32\n",
		},
		{
			name: "alter var and spec, quiet",
			args: []string{"caps", "req1", "troll", "-V", "troll_name", "-s", ":28,32", "-q"},
			p: testProject_withRequests(
				morc.RequestTemplate{
					Name: "req1",
					Captures: map[string]morc.VarScraper{
						"TROLL": {
							Name: "TROLL",
							Steps: []morc.TraversalStep{
								{Key: "data"},
								{Key: "people"},
								{Index: 0},
								{Key: "name"},
								{Key: "first"},
							},
						},
					},
				},
			),
			expectP: testProject_withRequests(
				morc.RequestTemplate{
					Name: "req1",
					Captures: map[string]morc.VarScraper{
						"TROLL_NAME": {
							Name:        "TROLL_NAME",
							OffsetStart: 28,
							OffsetEnd:   32,
						},
					},
				},
			),
			expectStdoutOutput: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert := assert.New(t)
			resetCapsFlags()

			// create project and dump config to a temp dir
			projFilePath := createTestProjectIO(t, tc.p)
			// set up the root command and run
			output, outputErr, err := runTestCommand(capsCmd, projFilePath, tc.args)

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

func Test_Caps_Show(t *testing.T) {
	testCases := []struct {
		name               string
		args               []string // DO NOT INCLUDE -F; it is automatically set to a project file
		p                  morc.Project
		expectErr          string // set if command.Execute expected to fail, with a string that would be in the error message
		expectStderrOutput string // set with expected output to stderr
		expectStdoutOutput string // set with expected output to stdout
	}{
		{
			name: "req does not exist",
			args: []string{"caps", "req1", "troll"},
			p: testProject_withRequests(
				morc.RequestTemplate{
					Name:     "req2",
					Captures: map[string]morc.VarScraper{},
				},
			),
			expectErr: "no request template req1",
		},
		{
			name: "req does not exist, quiet still errors",
			args: []string{"caps", "req1", "troll"},
			p: testProject_withRequests(
				morc.RequestTemplate{
					Name:     "req2",
					Captures: map[string]morc.VarScraper{},
				},
			),
			expectErr: "no request template req1",
		},
		{
			name: "var does not exist",
			args: []string{"caps", "req1", "troll"},
			p: testProject_withRequests(
				morc.RequestTemplate{
					Name:     "req1",
					Captures: map[string]morc.VarScraper{},
				},
			),
			expectErr: "no capture to $TROLL exists on request template req1",
		},
		{
			name: "var does not exist, quiet still errors",
			args: []string{"caps", "req1", "troll"},
			p: testProject_withRequests(
				morc.RequestTemplate{
					Name:     "req1",
					Captures: map[string]morc.VarScraper{},
				},
			),
			expectErr: "no capture to $TROLL exists on request template req1",
		},
		{
			name: "happy path - json path",
			args: []string{"caps", "req1", "troll"},
			p: testProject_withRequests(
				morc.RequestTemplate{
					Name: "req1",
					Captures: map[string]morc.VarScraper{
						"TROLL": {
							Name: "TROLL",
							Steps: []morc.TraversalStep{
								{Key: "data"},
								{Key: "people"},
								{Index: 0},
								{Key: "name"},
								{Key: "first"},
							},
						},
					},
				},
			),
			expectStdoutOutput: "$TROLL from .data.people[0].name.first\n",
		},
		{
			name: "happy path - json path, quiet mode",
			args: []string{"caps", "req1", "troll", "-q"},
			p: testProject_withRequests(
				morc.RequestTemplate{
					Name: "req1",
					Captures: map[string]morc.VarScraper{
						"TROLL": {
							Name: "TROLL",
							Steps: []morc.TraversalStep{
								{Key: "data"},
								{Key: "people"},
								{Index: 0},
								{Key: "name"},
								{Key: "first"},
							},
						},
					},
				},
			),
			expectStdoutOutput: "$TROLL from .data.people[0].name.first\n",
		},
		{
			name: "happy path - offset",
			args: []string{"caps", "req1", "troll"},
			p: testProject_withRequests(
				morc.RequestTemplate{
					Name: "req1",
					Captures: map[string]morc.VarScraper{
						"TROLL": {
							Name:        "TROLL",
							OffsetStart: 28,
							OffsetEnd:   32,
						},
					},
				},
			),
			expectStdoutOutput: "$TROLL from offset 28,32\n",
		},
		{
			name: "happy path - offset, quiet mode",
			args: []string{"caps", "req1", "troll", "-q"},
			p: testProject_withRequests(
				morc.RequestTemplate{
					Name: "req1",
					Captures: map[string]morc.VarScraper{
						"TROLL": {
							Name:        "TROLL",
							OffsetStart: 28,
							OffsetEnd:   32,
						},
					},
				},
			),
			expectStdoutOutput: "$TROLL from offset 28,32\n",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert := assert.New(t)
			resetCapsFlags()

			// create project and dump config to a temp dir
			projFilePath := createTestProjectIO(t, tc.p)
			// set up the root command and run
			output, outputErr, err := runTestCommand(capsCmd, projFilePath, tc.args)

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

			assert_noProjectMutations(assert)
		})
	}
}

func Test_Caps_Get(t *testing.T) {
	testCases := []struct {
		name               string
		args               []string // DO NOT INCLUDE -F; it is automatically set to a project file
		p                  morc.Project
		expectErr          string // set if command.Execute expected to fail, with a string that would be in the error message
		expectStderrOutput string // set with expected output to stderr
		expectStdoutOutput string // set with expected output to stdout
	}{
		{
			name: "req does not exist",
			args: []string{"caps", "req1", "troll", "-G", "VAR"},
			p: testProject_withRequests(
				morc.RequestTemplate{
					Name:     "req2",
					Captures: map[string]morc.VarScraper{},
				},
			),
			expectErr: "no request template req1",
		},
		{
			name: "req does not exist, quiet still errors",
			args: []string{"caps", "req1", "troll", "-G", "VAR", "-q"},
			p: testProject_withRequests(
				morc.RequestTemplate{
					Name:     "req2",
					Captures: map[string]morc.VarScraper{},
				},
			),
			expectErr: "no request template req1",
		},
		{
			name: "var does not exist",
			args: []string{"caps", "req1", "troll", "-G", "VAR"},
			p: testProject_withRequests(
				morc.RequestTemplate{
					Name:     "req1",
					Captures: map[string]morc.VarScraper{},
				},
			),
			expectErr: "no capture to $TROLL exists on request template req1",
		},
		{
			name: "var does not exist, quiet still errors",
			args: []string{"caps", "req1", "troll", "-G", "VAR", "-q"},
			p: testProject_withRequests(
				morc.RequestTemplate{
					Name:     "req1",
					Captures: map[string]morc.VarScraper{},
				},
			),
			expectErr: "no capture to $TROLL exists on request template req1",
		},
		{
			name: "get var name",
			args: []string{"caps", "req1", "troll", "-G", "var"},
			p: testProject_withRequests(
				morc.RequestTemplate{
					Name: "req1",
					Captures: map[string]morc.VarScraper{
						"TROLL": {
							Name:        "TROLL",
							OffsetStart: 28,
							OffsetEnd:   32,
						},
					},
				},
			),
			expectStdoutOutput: "TROLL\n",
		},
		{
			name: "get var name, quiet still prints",
			args: []string{"caps", "req1", "troll", "-G", "var", "-q"},
			p: testProject_withRequests(
				morc.RequestTemplate{
					Name: "req1",
					Captures: map[string]morc.VarScraper{
						"TROLL": {
							Name:        "TROLL",
							OffsetStart: 28,
							OffsetEnd:   32,
						},
					},
				},
			),
			expectStdoutOutput: "TROLL\n",
		},
		{
			name: "get var spec, path spec",
			args: []string{"caps", "req1", "troll", "-G", "SpEc"},
			p: testProject_withRequests(
				morc.RequestTemplate{
					Name: "req1",
					Captures: map[string]morc.VarScraper{
						"troll": {
							Name: "troll",
							Steps: []morc.TraversalStep{
								{Key: "data"},
								{Key: "people"},
								{Index: 0},
								{Key: "name"},
								{Key: "first"},
							},
						},
					},
				},
			),
			expectStdoutOutput: ".data.people[0].name.first\n",
		},
		{
			name: "get var spec, path spec, quiet still prints",
			args: []string{"caps", "req1", "troll", "-G", "SpEc", "-q"},
			p: testProject_withRequests(
				morc.RequestTemplate{
					Name: "req1",
					Captures: map[string]morc.VarScraper{
						"troll": {
							Name: "troll",
							Steps: []morc.TraversalStep{
								{Key: "data"},
								{Key: "people"},
								{Index: 0},
								{Key: "name"},
								{Key: "first"},
							},
						},
					},
				},
			),
			expectStdoutOutput: ".data.people[0].name.first\n",
		},
		{
			name: "get var spec, offset spec",
			args: []string{"caps", "req1", "troll", "-G", "SPEC"},
			p: testProject_withRequests(
				morc.RequestTemplate{
					Name: "req1",
					Captures: map[string]morc.VarScraper{
						"TROLL": {
							Name:        "TROLL",
							OffsetStart: 28,
							OffsetEnd:   32,
						},
					},
				},
			),
			expectStdoutOutput: "offset 28,32\n",
		},
		{
			name: "get var spec, raw spec",
			args: []string{"caps", "req1", "troll", "-G", "SPEC"},
			p: testProject_withRequests(
				morc.RequestTemplate{
					Name: "req1",
					Captures: map[string]morc.VarScraper{
						"TROLL": {
							Name: "TROLL",
						},
					},
				},
			),
			expectStdoutOutput: "entire response\n",
		},
		{
			name: "get var spec, offset spec, quiet still prints",
			args: []string{"caps", "req1", "troll", "-G", "SPEC", "-q"},
			p: testProject_withRequests(
				morc.RequestTemplate{
					Name: "req1",
					Captures: map[string]morc.VarScraper{
						"TROLL": {
							Name:        "TROLL",
							OffsetStart: 28,
							OffsetEnd:   32,
						},
					},
				},
			),
			expectStdoutOutput: "offset 28,32\n",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert := assert.New(t)
			resetCapsFlags()

			// create project and dump config to a temp dir
			projFilePath := createTestProjectIO(t, tc.p)
			// set up the root command and run
			output, outputErr, err := runTestCommand(capsCmd, projFilePath, tc.args)

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

			assert_noProjectMutations(assert)
		})
	}
}

func resetCapsFlags() {
	flags.New = ""
	flags.Delete = ""
	flags.Get = ""
	flags.Spec = ""
	flags.VarName = ""
	flags.BQuiet = false

	capsCmd.Flags().VisitAll(func(fl *pflag.Flag) {
		fl.Changed = false
	})
}
