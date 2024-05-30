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

func assert_projectInFileMatches(assert *assert.Assertions, expected morc.Project, projFilePath string) bool {
	updatedProj, err := morc.LoadProjectFromDisk(projFilePath, true)
	if !assert.NoError(err, "error loading project to check expectations: %v", err) {
		return false
	}

	// ignore the project file path
	expected.Config.ProjFile = ""
	updatedProj.Config.ProjFile = ""
	return assert.Equal(expected, updatedProj, "project in file does not match expected")
}

// DO NOT INCLUDE -F IN args!!! It is added automatically from projFilePath
func runTestCommand(cmd *cobra.Command, projFilePath string, args []string) (stdout string, stderr string, err error) {
	stdoutCapture := &bytes.Buffer{}
	stderrCapture := &bytes.Buffer{}

	// add -F immediately after the first arg
	if len(args) >= 1 {
		newArgs := make([]string, len(args)+2)
		newArgs[0] = args[0]
		newArgs[1] = "-F"
		newArgs[2] = projFilePath
		if len(args) >= 2 {
			copy(newArgs[3:], args[1:])
		}
		args = newArgs
	} else {
		args = append(args, "-F", projFilePath)
	}

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

func testProject_singleReqWillAllPropertiesSet() morc.Project {
	return morc.Project{
		Templates: map[string]morc.RequestTemplate{
			"req1": testRequest_withAllPropertiesSet(),
		},
	}
}

func testRequest_withAllPropertiesSet() morc.RequestTemplate {
	return morc.RequestTemplate{
		Name:   "req1",
		Method: "GET",
		URL:    "https://example.com",
		Headers: map[string][]string{
			"Content-Type": {"application/json"},
			"User-Agent":   {"morc/0.0.0", "test/0.0.0"},
		},
		Body:     []byte("{\n    \"username\": \"grimAuxiliatrix\"\n}"),
		AuthFlow: "auth1",
		Captures: map[string]morc.VarScraper{
			"var1": {
				Name:        "var1",
				OffsetStart: 1,
				OffsetEnd:   3,
			},
			"var2": {
				Name: "var2",
				Steps: []morc.TraversalStep{
					{Key: "key1"},
				},
			},
		},
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
