package commands

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dekarrin/morc"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func assert_noProjectMutations(assert *assert.Assertions) bool {
	if projWriter == nil {
		panic("project buffer was never set up")
	}

	projBuf := projWriter.(*bytes.Buffer)
	if projBuf.Len() > 0 {
		return assert.Fail("project buffer was written")
	}

	if histWriter != nil {
		histBuf := histWriter.(*bytes.Buffer)
		if histBuf.Len() > 0 {
			return assert.Fail("history buffer was written")
		}
	}

	if seshWriter != nil {
		seshBuf := seshWriter.(*bytes.Buffer)
		if seshBuf.Len() > 0 {
			return assert.Fail("session buffer was written")
		}
	}

	return true
}

func assert_projectInBufferMatches(assert *assert.Assertions, expected morc.Project) bool {
	// we just did writes so assume they hold *bytes.Buffers and use it as the
	// input
	var projR, histR, seshR io.Reader

	if projWriter == nil {
		panic("nothing to read; project writer buffer is nil")
	}

	projBuf := projWriter.(*bytes.Buffer)
	projR = projBuf

	if histWriter != nil {
		histBuf := histWriter.(*bytes.Buffer)
		histR = histBuf
	}

	if seshWriter != nil {
		seshBuf := seshWriter.(*bytes.Buffer)
		seshR = seshBuf
	}

	updatedProj, err := morc.LoadProject(projR, seshR, histR)
	if !assert.NoError(err, "error loading project to check expectations: %v", err) {
		return false
	}

	// ignore project file paths
	expected.Config.ProjFile = ""
	expected.Config.HistFile = ""
	expected.Config.SeshFile = ""
	updatedProj.Config.ProjFile = ""
	updatedProj.Config.HistFile = ""
	updatedProj.Config.SeshFile = ""
	return assert.Equal(expected, updatedProj, "project in file does not match expected")
}

// TODO: make unit tests that actually cover loading from file. This func can be
// used there.
func assert_projectInFileMatches(assert *assert.Assertions, expected morc.Project, projFilePath string) bool {
	updatedProj, err := morc.LoadProjectFromDisk(projFilePath, true)
	if !assert.NoError(err, "error loading project to check expectations: %v", err) {
		return false
	}

	// ignore project file paths
	expected.Config.ProjFile = ""
	expected.Config.HistFile = ""
	expected.Config.SeshFile = ""
	updatedProj.Config.ProjFile = ""
	updatedProj.Config.HistFile = ""
	updatedProj.Config.SeshFile = ""
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

// TODO: make unit tests that actually cover loading from file. This func can be
// used there.
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
	defer f.Close()
	if err := p.Dump(f); err != nil {
		t.Fatal(err)
		return ""
	}

	// next do hist file, if one is given
	if p.Config.HistFile != "" {
		if !strings.HasPrefix(p.Config.HistFile, morc.ProjDirVar) {
			t.Fatal("hist file path must start with " + morc.ProjDirVar + " if present in tests")
			return ""
		}

		relativePath := strings.TrimPrefix(p.Config.HistFile, morc.ProjDirVar)
		histFilePath := filepath.Join(dir, relativePath)
		hf, err := os.Create(histFilePath)
		if err != nil {
			t.Fatal(err)
			return ""
		}
		defer hf.Close()

		if err := p.DumpHistory(hf); err != nil {
			t.Fatal(err)
			return ""
		}
	}

	// next do sesh file, if one is given
	if p.Config.SeshFile != "" {
		if !strings.HasPrefix(p.Config.SeshFile, morc.ProjDirVar) {
			t.Fatal("session file path must start with " + morc.ProjDirVar + " if present in tests")
			return ""
		}

		relativePath := strings.TrimPrefix(p.Config.SeshFile, morc.ProjDirVar)
		seshFilePath := filepath.Join(dir, relativePath)
		sf, err := os.Create(seshFilePath)
		if err != nil {
			t.Fatal(err)
			return ""
		}
		defer sf.Close()

		if err := p.Session.Dump(sf); err != nil {
			t.Fatal(err)
			return ""
		}
	}

	return projFilePath
}

func createTestProjectIO(t *testing.T, p morc.Project) string {
	projReader = nil
	projWriter = nil
	histReader = nil
	histWriter = nil
	seshReader = nil
	seshWriter = nil

	projFilePath := "(in-memory)"

	projBuf := &bytes.Buffer{}

	// set the proj file path in project at this point or there will be issues
	// on persistence
	p.Config.ProjFile = projFilePath

	if err := p.Dump(projBuf); err != nil {
		t.Fatal(err)
		return ""
	}

	projReader = projBuf
	projWriter = &bytes.Buffer{}

	// next do hist file, if one is given
	if p.Config.HistFile != "" {
		if !strings.HasPrefix(p.Config.HistFile, morc.ProjDirVar) {
			t.Fatal("hist file path must start with " + morc.ProjDirVar + " if present in tests")
			return ""
		}

		histBuf := &bytes.Buffer{}

		if err := p.DumpHistory(histBuf); err != nil {
			t.Fatal(err)
			return ""
		}

		histReader = histBuf
		histWriter = &bytes.Buffer{}
	}

	// next do sesh file, if one is given
	if p.Config.SeshFile != "" {
		if !strings.HasPrefix(p.Config.SeshFile, morc.ProjDirVar) {
			t.Fatal("session file path must start with " + morc.ProjDirVar + " if present in tests")
			return ""
		}

		seshBuf := &bytes.Buffer{}

		if err := p.Session.Dump(seshBuf); err != nil {
			t.Fatal(err)
			return ""
		}

		seshReader = seshBuf
		seshWriter = &bytes.Buffer{}
	}

	return projFilePath
}

func testVarStore(curEnv string, vars map[string]map[string]string) morc.VarStore {
	vs := morc.NewVarStore()

	for env, envVars := range vars {
		for k, v := range envVars {
			vs.SetIn(k, v, env)
		}
	}

	vs.Environment = curEnv

	return vs
}

func testProject_vars(curEnv string, vars map[string]map[string]string, moreVars ...map[string]map[string]string) morc.Project {
	if len(moreVars) > 0 {
		combined := make(map[string]map[string]string)
		for env, envVars := range vars {
			combined[env] = make(map[string]string)
			for k, v := range envVars {
				combined[env][k] = v
			}
		}
		for _, varsToMerge := range moreVars {
			for env, envVars := range varsToMerge {
				if _, ok := combined[env]; !ok {
					combined[env] = make(map[string]string)
				}
				for k, v := range envVars {
					combined[env][k] = v
				}
			}
		}
		vars = combined
	}

	return morc.Project{
		Vars: testVarStore(curEnv, vars),
	}
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

func testProject_withRequests(reqs ...morc.RequestTemplate) morc.Project {
	tmpls := make(map[string]morc.RequestTemplate, len(reqs))
	for _, r := range reqs {
		tmpls[strings.ToLower(r.Name)] = r
	}

	return morc.Project{
		Templates: tmpls,
	}
}

func testRequest_withAllPropertiesSet() morc.RequestTemplate {
	return morc.RequestTemplate{
		Name:   "req1",
		Method: "GET",
		URL:    "http://example.com",
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
