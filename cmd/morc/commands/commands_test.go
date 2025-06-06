package commands

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/dekarrin/morc"
	"github.com/spf13/cobra"
)

type MorcIOAssertions struct {
	morc.Assertions

	ProjFilePath string
	fio          *morcFileIO
}

// NewAssertionsForInMemoryProject creates a new MorcIOAssertions struct for
// testing that wraps the given assertion and sets file readers/writers to point
// to in-memory locations.
func NewAssertionsForInMemoryProject(t *testing.T, p morc.Project, fio *morcFileIO) *MorcIOAssertions {
	if fio == nil {
		panic("fio must be non-nil")
	}

	m := &MorcIOAssertions{
		Assertions: *morc.NewAssertions(t),
		fio:        fio,
	}

	m.fio.proj.Reader = nil
	m.fio.proj.Writer = nil
	m.fio.hist.Reader = nil
	m.fio.hist.Writer = nil
	m.fio.sesh.Reader = nil
	m.fio.sesh.Writer = nil

	m.ProjFilePath = "(in-memory)"

	projBuf := &bytes.Buffer{}

	// set the proj file path in project at this point or there will be issues
	// on persistence
	p.Config.ProjFile = m.ProjFilePath

	if err := p.Dump(projBuf); err != nil {
		t.Fatal(err)
		return nil
	}

	m.fio.proj.Reader = projBuf
	m.fio.proj.Writer = &bytes.Buffer{}

	// next do hist file, if one is given
	if p.Config.HistFile != "" {
		if !strings.HasPrefix(p.Config.HistFile, morc.ProjDirVar) {
			t.Fatal("hist file path must start with " + morc.ProjDirVar + " if present in tests")
			return nil
		}

		histBuf := &bytes.Buffer{}

		if err := p.DumpHistory(histBuf); err != nil {
			t.Fatal(err)
			return nil
		}

		m.fio.hist.Reader = histBuf
		m.fio.hist.Writer = &bytes.Buffer{}
	}

	// next do sesh file, if one is given
	if p.Config.SeshFile != "" {
		if !strings.HasPrefix(p.Config.SeshFile, morc.ProjDirVar) {
			t.Fatal("session file path must start with " + morc.ProjDirVar + " if present in tests")
			return nil
		}

		seshBuf := &bytes.Buffer{}

		if err := p.Session.Dump(seshBuf); err != nil {
			t.Fatal(err)
			return nil
		}

		m.fio.sesh.Reader = seshBuf
		m.fio.sesh.Writer = &bytes.Buffer{}
	}

	return m
}

func (m *MorcIOAssertions) NoProjectMutations() bool {
	m.T.Helper()

	if m.fio.proj.Writer == nil {
		panic("project buffer was never set up")
	}

	projBuf := m.fio.proj.Writer.(*bytes.Buffer)
	if projBuf.Len() > 0 {
		return m.Fail("project buffer was written to")
	}

	if m.fio.hist.Writer != nil {
		histBuf := m.fio.hist.Writer.(*bytes.Buffer)
		if histBuf.Len() > 0 {
			return m.Fail("history buffer was written to")
		}
	}

	if m.fio.sesh.Writer != nil {
		seshBuf := m.fio.sesh.Writer.(*bytes.Buffer)
		if seshBuf.Len() > 0 {
			return m.Fail("session buffer was written to")
		}
	}

	return true
}

// ProjectPersistedToBuffer checks that the project writer buffer was
// initially created (will be true if createTestProjectIO was called in the test
// this comes from) that it was written to, and that reading from it results in
// the expected Project. Note that this check specifically does *not* do loading
// of any history or session data that may have been written, and the checks
// will ignore expected.History and expected.Session; to check those at the same
// time, use assert_projectFilesInBuffersMatch. Additionally, all project file
// paths in expected.Config are ignored.
func (m *MorcIOAssertions) ProjectPersistedToBuffer(expected morc.Project) bool {
	m.T.Helper()

	// we just did writes so assume they hold *bytes.Buffers and use it as the
	// input
	var projR io.Reader

	if m.fio.proj.Writer == nil {
		return m.Fail("project buffer was not set up\nMake sure to call createTestProjectIO() in same test first")
	}

	projBuf := m.fio.proj.Writer.(*bytes.Buffer)

	// it exists, but was not necessarily written to. all writes should result
	// in at least two chars being written for an empty list/object, so we will
	// rely on that fact here glub.
	if !m.Greater(projBuf.Len(), 0, "project was not persisted") {
		return false
	}

	projR = projBuf

	updatedProj, err := morc.LoadProject(projR, nil, nil)
	if !m.NoError(err, "error loading project to check expectations: %v", err) {
		return false
	}

	// ignore project file paths
	expected.Config.ProjFile = ""
	expected.Config.HistFile = ""
	expected.Config.SeshFile = ""
	updatedProj.Config.ProjFile = ""
	updatedProj.Config.HistFile = ""
	updatedProj.Config.SeshFile = ""

	// also ignore actual history and session data
	expected.History = []morc.HistoryEntry{{Template: "HISTORY IGNORED FOR THIS CHECK"}}
	updatedProj.History = []morc.HistoryEntry{{Template: "HISTORY IGNORED FOR THIS CHECK"}}
	expected.Session = morc.Session{Cookies: []morc.SetCookiesCall{{Cookies: []*http.Cookie{{Name: "SESSION IGNORED FOR THIS CHECK"}}}}}
	updatedProj.Session = morc.Session{Cookies: []morc.SetCookiesCall{{Cookies: []*http.Cookie{{Name: "SESSION IGNORED FOR THIS CHECK"}}}}}

	return m.Equal(expected, updatedProj, "project in buffer does not match expected")
}

// SessionPersistedToBuffer checks that the session writer buffer was
// initially created (as creation depends on same conditions as writting to
// file, TODO: upd8 that!!!!!!!! It is super 8ad), that it was written to, and
// that reading from it results in the expected session data.
func (m *MorcIOAssertions) SessionPersistedToBuffer(expected morc.Session) bool {
	m.T.Helper()

	// we just did writes so assume they hold *bytes.Buffers and use it as the
	// input
	var seshR io.Reader

	if m.fio.sesh.Writer == nil {
		return m.Fail("session buffer was not set up\nMake sure to call createTestProjectIO() in same test first")
	}

	seshBuf := m.fio.sesh.Writer.(*bytes.Buffer)

	// it exists, but was not necessarily written to. all writes should result
	// in at least two chars being written for an empty list/object, so we will
	// rely on that fact here glub.
	if !m.Greater(seshBuf.Len(), 0, "session was not persisted") {
		return false
	}

	seshR = seshBuf

	updatedSesh, err := morc.LoadSession(seshR)
	if !m.NoError(err, "error loading session to check expectations: %v", err) {
		return false
	}

	return m.SessionsMatch(expected, updatedSesh)
}

// HistoryPersistedToBuffer checks that the history writer buffer was
// initially created (as creation depends on same conditions as writting to
// file, TODO: upd8 that!!!!!!!! It is super 8ad), that it was written to, and
// that reading from it results in the expected list of entries. The dates of
// the entries are not checked.
func (m *MorcIOAssertions) HistoryPersistedToBuffer(expected []morc.HistoryEntry) bool {
	m.T.Helper()

	// we just did writes so assume they hold *bytes.Buffers and use it as the
	// input
	var histR io.Reader

	if m.fio.hist.Writer == nil {
		return m.Fail("history buffer was not set up\nMake sure to call createTestProjectIO() in same test first")
	}

	histBuf := m.fio.hist.Writer.(*bytes.Buffer)

	// it exists, but was not necessarily written to. all writes should result
	// in at least two chars being written for an empty list/object, so we will
	// rely on that fact here glub.
	if !m.Greater(histBuf.Len(), 0, "history was not persisted") {
		return false
	}

	histR = histBuf

	updatedHist, err := morc.LoadHistory(histR)
	if !m.NoError(err, "error loading history to check expectations: %v", err) {
		return false
	}

	return m.HistoriesMatch(expected, updatedHist)
}

// specifically ensures that the project file was not written.
func (m *MorcIOAssertions) NoProjectFileMutations() bool {
	m.T.Helper()

	if m.fio.proj.Writer == nil {
		panic("project IO buffers were never set up")
	}

	projBuf := m.fio.proj.Writer.(*bytes.Buffer)
	if projBuf.Len() > 0 {
		return m.Fail("project buffer was written to")
	}

	return true
}

// specifically ensures that the history file was not written.
func (m *MorcIOAssertions) NoHistoryFileMutations() bool {
	m.T.Helper()

	if m.fio.proj.Writer == nil {
		panic("project IO buffers were never set up")
	}

	if m.fio.hist.Writer != nil {
		histBuf := m.fio.hist.Writer.(*bytes.Buffer)
		if histBuf.Len() > 0 {
			return m.Fail("history buffer was written to")
		}
	}

	return true
}

// specifically ensures that the session file was not written.
func (m *MorcIOAssertions) NoSessionFileMutations() bool {
	m.T.Helper()

	if m.fio.proj.Writer == nil {
		panic("project IO buffers were never set up")
	}

	if m.fio.sesh.Writer != nil {
		seshBuf := m.fio.sesh.Writer.(*bytes.Buffer)
		if seshBuf.Len() > 0 {
			return m.Fail("session buffer was written to")
		}
	}

	return true
}

func (m *MorcIOAssertions) ProjectFilesInBuffersMatch(expected morc.Project) bool {
	m.T.Helper()

	// we just did writes so assume they hold *bytes.Buffers and use it as the
	// input
	var projR, histR, seshR io.Reader

	if m.fio.proj.Writer == nil {
		panic("nothing to read; project writer buffer is nil")
	}

	projBuf := m.fio.proj.Writer.(*bytes.Buffer)
	projR = projBuf

	if m.fio.hist.Writer != nil {
		histBuf := m.fio.hist.Writer.(*bytes.Buffer)
		histR = histBuf
	}

	if m.fio.sesh.Writer != nil {
		seshBuf := m.fio.sesh.Writer.(*bytes.Buffer)
		seshR = seshBuf
	}

	updatedProj, err := morc.LoadProject(projR, seshR, histR)
	if !m.NoError(err, "error loading project to check expectations: %v", err) {
		return false
	}

	// ignore project file paths
	expected.Config.ProjFile = ""
	expected.Config.HistFile = ""
	expected.Config.SeshFile = ""
	updatedProj.Config.ProjFile = ""
	updatedProj.Config.HistFile = ""
	updatedProj.Config.SeshFile = ""
	return m.Equal(expected, updatedProj, "project in file does not match expected")
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

func (f *cliFlags) resetOutputControl() {
	f.BHeaders = false
	f.BCaptures = false
	f.BNoBody = false
	f.BRequest = false
	f.BHideAuth = false
	f.Format = "pretty" // TODO: make this default not be magic but rather have the cmd flag init and the reset use it
}

func testVarStore(curEnv string, vars map[string]map[string]string) morc.VarStore {
	vs := morc.NewVarStore()

	// make sure that the default env exists as that matches how it loads
	vs.Set("fake", "")
	vs.Unset("fake")

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
		Body: []byte("{\n    \"username\": \"grimAuxiliatrix\"\n}"),
		Auth: "auth1",
		Captures: map[string]morc.Scraper{
			"var1": {
				Name:        "var1",
				Type:        morc.SpecBodyOffset,
				OffsetStart: 1,
				OffsetEnd:   3,
			},
			"var2": {
				Name: "var2",
				Type: morc.SpecBodyJSON,
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

type seqType int

const (
	seqTemplate seqType = iota
	seqFlow
)

type expDetect int

const (
	enableExpiration expDetect = iota
	disableExpiration
)

func testAuths(auths ...morc.Auth) map[string]morc.Auth {
	m := make(map[string]morc.Auth)
	for _, a := range auths {
		m[a.Name] = a
	}
	return m
}

func testAuth_basic(name, user, pass string) morc.Auth {
	return morc.Auth{
		Name: name,
		Type: morc.AuthTypeHTTPBasic,
		Proof: morc.HTTPBasicCredentials{
			Username: user,
			Password: pass,
		},
	}
}

func testAuth_session(name string, seqType seqType, seqName, cookie string, expDetect expDetect, proof morc.AuthProof) morc.Auth {
	return morc.Auth{
		Name: name,
		Type: morc.AuthTypeSession,
		Fetcher: morc.NewSessionCookieFetcher(
			morc.RequestSequence{Name: seqName, IsFlow: seqType == seqFlow},
			cookie,
			expDetect == enableExpiration,
		),
		Proof: proof,
	}
}

func testAuth_jwt(name, seqName, tokenVar string, proof ...morc.AuthProof) morc.Auth {
	var p morc.AuthProof
	if len(proof) > 0 {
		p = proof[0]
	}

	return morc.Auth{
		Name:  name,
		Type:  morc.AuthTypeJWT,
		Proof: p,
		Fetcher: morc.NewJWTFetcher(
			morc.RequestSequence{
				Name:   seqName,
				IsFlow: false,
			},
			morc.Scraper{
				Name: tokenVar,
				Type: morc.SpecBodyJSON,
				Steps: []morc.TraversalStep{
					{Key: "token"},
				},
			},
		),
	}
}

func testAuth_token(name, seqName, tokenVar string, proof ...morc.AuthProof) morc.Auth {
	var p morc.AuthProof
	if len(proof) > 0 {
		p = proof[0]
	}

	return morc.Auth{
		Name:  name,
		Type:  morc.AuthTypeToken,
		Proof: p,
		Fetcher: morc.NewTokenFetcher(
			morc.RequestSequence{
				Name:   seqName,
				IsFlow: false,
			},
			morc.Scraper{
				Name: tokenVar,
				Type: morc.SpecBodyJSON,
				Steps: []morc.TraversalStep{
					{Key: "access_token"},
				},
			},
			morc.ProofDestination{
				Location: morc.ProofLocationHeader,
				Key:      "Authorization",
				Format:   "bearer",
			},
			&morc.Scraper{
				Type: morc.SpecBodyJSON,
				Steps: []morc.TraversalStep{
					{Key: "expiration"},
				},
			},
			time.RFC1123,
		),
	}
}

func testProof_jwt(claims testJWTData, secretKey string) morc.AuthProof {
	token := claims.Token(secretKey)
	return morc.DynamicProof{
		Dest: morc.ProofDestination{
			Location: morc.ProofLocationHeader,
			Key:      "Authorization",
			Format:   morc.ProofFormatTypeBearer,
		},
		Value:     token,
		ExpiresAt: claims.Expiration,
	}
}

func testProof_token(name string, tokenExp time.Time) morc.AuthProof {
	return morc.DynamicProof{
		Dest: morc.ProofDestination{
			Location: morc.ProofLocationHeader,
			Key:      "Authorization",
			Format:   "bearer",
		},
		Value:     name,
		ExpiresAt: tokenExp,
	}
}

type Creds struct {
	User string `json:"user"`
	Pass string `json:"pass"`
}

func testRequests_withProtectedResource_basic(authName string) map[string]morc.RequestTemplate {
	return map[string]morc.RequestTemplate{
		"resource": {
			Name:   "resource",
			Method: "GET",
			URL:    "/protected",
			Headers: http.Header{
				"Content-Type": []string{"application/json"},
			},
			Auth: authName,
		},
	}
}

// testRequests_withProtectedResource_session returns a set of request
// templates suitable for use with the server handler returned in
// serverHandler_withProtectedResource_session. "login" will POST to the
// login endpoint and "resource" will GET the protected resource. The resource
// request uses the Auth with the given name.
func testRequests_withProtectedResource_session(creds Creds, auth string) map[string]morc.RequestTemplate {
	body, err := json.Marshal(creds)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal creds: %v", err))
	}

	reqs := map[string]morc.RequestTemplate{
		"login": {
			Name:    "login",
			Method:  "POST",
			URL:     "/login",
			Body:    []byte(body),
			Headers: http.Header{"Content-Type": []string{"application/json"}},
		},
		"resource": {
			Name:    "resource",
			Method:  "GET",
			URL:     "/protected",
			Headers: http.Header{"Content-Type": []string{"application/json"}},
			Auth:    auth,
		},
	}
	return reqs
}

func testRequests_withProtectedResource_jwt(creds Creds, authName string) map[string]morc.RequestTemplate {
	return map[string]morc.RequestTemplate{
		"login": {
			Name:   "login",
			Method: "POST",
			URL:    "/login",
			Body:   []byte(fmt.Sprintf(`{"user":%q,"pass":%q}`, creds.User, creds.Pass)),
			Headers: http.Header{
				"Content-Type": []string{"application/json"},
			},
		},
		"resource": {
			Name:   "resource",
			Method: "GET",
			URL:    "/protected",
			Headers: http.Header{
				"Content-Type": []string{"application/json"},
			},
			Auth: authName,
		},
	}
}

func testRequests_withProtectedResource_token(creds Creds, authName string) map[string]morc.RequestTemplate {
	return map[string]morc.RequestTemplate{
		"login": {
			Name:   "login",
			Method: "POST",
			URL:    "/login",
			Body:   []byte(fmt.Sprintf(`{"user":%q,"pass":%q}`, creds.User, creds.Pass)),
			Headers: http.Header{
				"Content-Type": []string{"application/json"},
			},
		},
		"resource": {
			Name:   "resource",
			Method: "GET",
			URL:    "/protected",
			Headers: http.Header{
				"Content-Type": []string{"application/json"},
			},
			Auth: authName,
		},
	}
}

func serverHandler_withProtectedResource_basic(creds Creds, resource testResource) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		// suppress date header
		w.Header()["Date"] = nil

		if r.URL.Path == "/protected" {
			if r.Method != "GET" {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}

			// check basic auth
			u, p, ok := r.BasicAuth()
			if !ok || u != creds.User || p != creds.Pass {
				w.Header().Set("WWW-Authenticate", `Basic realm="test"`)
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			// send resource
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			data, _ := json.Marshal(resource)
			w.Write(data)
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}
}

// serverHandler_withProtectedResource_session returns a handler that can be
// used to test login cookie auth. It has a login endpoint at /login and a
// protected resource endpoint at /protected. When /login is POST'd to with a
// JSON body that unmarshals to a Creds object matching validCredentials, the
// server returns a Set-Cookie containing the provided cookie for the session.
// When /protected is GET'd with the proper session cookie, the server will
// return the protected resource as a JSON response.
func serverHandler_withProtectedResource_session(validCredentials Creds, cookie *http.Cookie, protected interface{}) func(w http.ResponseWriter, r *http.Request) {
	requiredValue := cookie.Value

	resourceBytes := []byte{}
	if protected != nil {
		var err error
		resourceBytes, err = json.Marshal(protected)
		if err != nil {
			panic("failed to marshal protected resource")
		}
	}

	return func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		w.Header()["Date"] = nil

		switch path {
		case "/login":
			switch r.Method {
			case http.MethodPost:
				bodyBytes, err := io.ReadAll(r.Body)
				if err != nil {
					w.WriteHeader(http.StatusInternalServerError)
				}

				var c Creds
				if err := json.Unmarshal(bodyBytes, &c); err != nil {
					w.WriteHeader(http.StatusBadRequest)
					return
				}

				if c != validCredentials {
					w.WriteHeader(http.StatusForbidden)
					return
				}

				http.SetCookie(w, cookie)
				w.WriteHeader(http.StatusNoContent)
			default:
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
		case "/protected":
			switch r.Method {
			case http.MethodGet:
				sessionCookie, err := r.Cookie(cookie.Name)
				if err != nil {
					w.WriteHeader(http.StatusForbidden)
					return
				}

				if sessionCookie.Value != requiredValue {
					w.WriteHeader(http.StatusForbidden)
					return
				}

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				w.Write(resourceBytes)
				return
			default:
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
		default:
			w.WriteHeader(http.StatusNotFound)
			return
		}

	}
}

func serverHandler_withProtectedResource_jwt(creds Creds, token testJWTData, resource testResource, secretKey string) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		expSubject := token.Subject

		// suppress date header
		w.Header()["Date"] = nil

		if r.URL.Path == "/login" {
			if r.Method != "POST" {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}

			// check credentials
			var reqCreds Creds
			if err := json.NewDecoder(r.Body).Decode(&reqCreds); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			if reqCreds != creds {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			// send token
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(fmt.Sprintf(`{"token":%q}`, token.Token(secretKey))))
			return
		}

		if r.URL.Path == "/protected" {
			if r.Method != "GET" {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}

			// check auth header
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				log.Printf("no auth header")
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			// check token
			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || parts[0] != "Bearer" {
				log.Printf("not a bearer token")
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			jwtPart := parts[1]
			parts = strings.Split(jwtPart, ".")
			if len(parts) != 3 {
				log.Printf("not a valid JWT")
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			encodedHeader := parts[0]
			encodedClaims := parts[1]
			encodedSignature := parts[2]

			signatureBytes, err := base64.StdEncoding.DecodeString(encodedSignature)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			// verify signature
			h := hmac.New(sha256.New, []byte(secretKey))
			h.Write([]byte(encodedHeader + "." + encodedClaims))
			if !hmac.Equal(signatureBytes, h.Sum(nil)) {
				log.Printf("invalid signature")
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			// decode header and validate it is typ=JWT and alg=HS256
			headerBytes, err := base64.StdEncoding.DecodeString(encodedHeader)
			if err != nil {
				log.Printf("cant read header: %s", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			var header map[string]string
			if err := json.Unmarshal(headerBytes, &header); err != nil {
				log.Printf("cant unmarshal header %s", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			if header["typ"] != "JWT" || header["alg"] != "HS256" {
				log.Printf("typ!=JWT || alg!=HS256")
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			// finally, read claims and verify expiration
			claimsBytes, err := base64.StdEncoding.DecodeString(encodedClaims)
			if err != nil {
				log.Printf("cant read claims: %s", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			var claims testJWTData
			var claimsMap map[string]any
			if err := json.Unmarshal(claimsBytes, &claimsMap); err != nil {
				log.Printf("cant unmarshal claims: %s", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			claims.Expiration = time.Unix(int64(claimsMap["exp"].(float64)), 0).UTC()
			claims.Subject = claimsMap["sub"].(string)

			if claims.Expiration.Before(time.Now()) {
				log.Printf("expired")
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			if claims.Subject != expSubject {
				log.Printf("subject mismatch")
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			// send resource
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			data, _ := json.Marshal(resource)
			w.Write(data)
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}
}

func serverHandler_withProtectedResource_token(creds Creds, token string, tokenExp time.Time, resource testResource) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		// suppress date header
		w.Header()["Date"] = nil

		if r.URL.Path == "/login" {
			if r.Method != "POST" {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}

			// check credentials
			var reqCreds Creds
			if err := json.NewDecoder(r.Body).Decode(&reqCreds); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			if reqCreds != creds {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			// send token
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(fmt.Sprintf(`{"access_token":%q,"expiration":%q}`, token, tokenExp.Format(time.RFC1123))))
			return
		}

		if r.URL.Path == "/protected" {
			if r.Method != "GET" {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}

			// check auth header
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			// check token
			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || parts[0] != "Bearer" || parts[1] != token {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			// verify token is not expired
			if time.Now().After(tokenExp) {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			// send resource
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			data, _ := json.Marshal(resource)
			w.Write(data)
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}
}
