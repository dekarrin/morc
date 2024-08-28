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
		// * req does not exist
		// * var already exists
		// * var is new, but spec not given
		// * var is new, bad spec errors
		// * var is new, good spec
		// * quiet mode variants
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
		// * req does not exist
		// * var does not exist
		// * var and req exists
		// * quiet mode variants
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
		// * req does not exist
		// * var does not exist
		// * var exists, change spec
		// * var exists, change var
		// * var exists, change both spec and var
		// * quiet mode variants
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
		// * req does not exist
		// * var does not exist
		// * req and var exists, path spec
		// * req and var exists, offset spec
		// * quiet mode variants
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
		// * req does not exist
		// * var does not exist
		// * get var name
		// * get var spec
		// * quiet mode variants
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
