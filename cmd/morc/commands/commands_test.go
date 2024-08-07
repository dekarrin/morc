package commands

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/dekarrin/morc"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

type MorcIOAssertions struct {
	assert.Assertions
}

func assert_noProjectMutations(assert *assert.Assertions) bool {
	if projWriter == nil {
		panic("project buffer was never set up")
	}

	projBuf := projWriter.(*bytes.Buffer)
	if projBuf.Len() > 0 {
		return assert.Fail("project buffer was written to")
	}

	if histWriter != nil {
		histBuf := histWriter.(*bytes.Buffer)
		if histBuf.Len() > 0 {
			return assert.Fail("history buffer was written to")
		}
	}

	if seshWriter != nil {
		seshBuf := seshWriter.(*bytes.Buffer)
		if seshBuf.Len() > 0 {
			return assert.Fail("session buffer was written to")
		}
	}

	return true
}

// assert_projectPersistedToBuffer checks that the project writer buffer was
// initially created (will be true if createTestProjectIO was called in the test
// this comes from) that it was written to, and that reading from it results in
// the expected Project. Note that this check specifically does *not* do loading
// of any history or session data that may have been written, and the checks
// will ignore expected.History and expected.Session; to check those at the same
// time, use assert_projectFilesInBuffersMatch. Additionally, all project file
// paths in expected.Config are ignored.
func assert_projectPersistedToBuffer(assert *assert.Assertions, expected morc.Project) bool {
	// we just did writes so assume they hold *bytes.Buffers and use it as the
	// input
	var projR io.Reader

	if projWriter == nil {
		return assert.Fail("project buffer was not set up\nMake sure to call createTestProjectIO() in same test first")
	}

	projBuf := projWriter.(*bytes.Buffer)

	// it exists, but was not necessarily written to. all writes should result
	// in at least two chars being written for an empty list/object, so we will
	// rely on that fact here glub.
	if !assert.Greater(projBuf.Len(), 0, "project was not persisted") {
		return false
	}

	projR = projBuf

	updatedProj, err := morc.LoadProject(projR, nil, nil)
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

	return assert.Equal(expected, updatedProj, "project in buffer does not match expected")
}

// assert_sessionPersistedToBuffer checks that the history writer buffer was
// initially created (as creation depends on same conditions as writting to
// file, TODO: upd8 that!!!!!!!! It is super 8ad), that it was written to, and
// that reading from it results in the expected session data.
func assert_sessionPersistedToBuffer(assert *assert.Assertions, expected morc.Session) bool {
	// we just did writes so assume they hold *bytes.Buffers and use it as the
	// input
	var seshR io.Reader

	if seshWriter == nil {
		return assert.Fail("session buffer was not set up\nMake sure to call createTestProjectIO() in same test first")
	}

	seshBuf := seshWriter.(*bytes.Buffer)

	// it exists, but was not necessarily written to. all writes should result
	// in at least two chars being written for an empty list/object, so we will
	// rely on that fact here glub.
	if !assert.Greater(seshBuf.Len(), 0, "session was not persisted") {
		return false
	}

	seshR = seshBuf

	updatedSesh, err := morc.LoadSession(seshR)
	if !assert.NoError(err, "error loading session to check expectations: %v", err) {
		return false
	}

	return morc.AssertSessionsMatch(assert, expected, updatedSesh)
}

// assert_historyPersistedToBuffer checks that the history writer buffer was
// initially created (as creation depends on same conditions as writting to
// file, TODO: upd8 that!!!!!!!! It is super 8ad), that it was written to, and
// that reading from it results in the expected list of entries. The dates of
// the entries are not checked.
func assert_historyPersistedToBuffer(assert *assert.Assertions, expected []morc.HistoryEntry) bool {
	// we just did writes so assume they hold *bytes.Buffers and use it as the
	// input
	var histR io.Reader

	if histWriter == nil {
		return assert.Fail("history buffer was not set up\nMake sure to call createTestProjectIO() in same test first")
	}

	histBuf := histWriter.(*bytes.Buffer)

	// it exists, but was not necessarily written to. all writes should result
	// in at least two chars being written for an empty list/object, so we will
	// rely on that fact here glub.
	if !assert.Greater(histBuf.Len(), 0, "history was not persisted") {
		return false
	}

	histR = histBuf

	updatedHist, err := morc.LoadHistory(histR)
	if !assert.NoError(err, "error loading history to check expectations: %v", err) {
		return false
	}

	return morc.AssertHistoriesMatch(assert, expected, updatedHist)
}

// specifically ensures that the project file was not written.
func assert_noProjectFileMutations(assert *assert.Assertions) bool {
	if projWriter == nil {
		panic("project IO buffers were never set up")
	}

	projBuf := projWriter.(*bytes.Buffer)
	if projBuf.Len() > 0 {
		return assert.Fail("project buffer was written to")
	}

	return true
}

// specifically ensures that the project file was not written.
func assert_noHistoryFileMutations(assert *assert.Assertions) bool {
	if projWriter == nil {
		panic("project IO buffers were never set up")
	}

	if histWriter != nil {
		histBuf := histWriter.(*bytes.Buffer)
		if histBuf.Len() > 0 {
			return assert.Fail("history buffer was written to")
		}
	}

	return true
}

// specifically ensures that the project file was not written.
func assert_noSessionFileMutations(assert *assert.Assertions) bool {
	if projWriter == nil {
		panic("project IO buffers were never set up")
	}

	if seshWriter != nil {
		seshBuf := seshWriter.(*bytes.Buffer)
		if seshBuf.Len() > 0 {
			return assert.Fail("session buffer was written to")
		}
	}

	return true
}

func assert_projectFilesInBuffersMatch(assert *assert.Assertions, expected morc.Project) bool {
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
