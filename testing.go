package morc

import (
	"bytes"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TODO: maybe move this to testing package or internal package so it doesn't
// show up in godoc
type Assertions struct {
	assert.Assertions

	T *testing.T
}

func NewAssertions(t *testing.T) *Assertions {
	if t == nil {
		panic("t must be non-nil")
	}

	return &Assertions{
		Assertions: *assert.New(t),
		T:          t,
	}
}

func (m *Assertions) ProjectInFileMatches(expected Project, projFilePath string) bool {
	m.T.Helper()

	updatedProj, err := LoadProjectFromDisk(projFilePath, true)
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

	m.HistoriesMatch(expected.History, updatedProj.History)
	m.SessionsMatch(expected.Session, updatedProj.Session)

	// unset histories and sessions on both as they are checked separately above
	expected.History = nil
	expected.Session = Session{}
	updatedProj.History = nil
	updatedProj.Session = Session{}

	return m.Equal(expected, updatedProj, "project in file does not match expected")
}

func (m *Assertions) HistoriesMatch(expected, actual []HistoryEntry) bool {
	m.T.Helper()

	if !m.Len(actual, len(expected), "history entry count does not match expected") {
		return m.Equal(expected, actual)
	}

	var failed bool

	for i := range actual {
		if !m.HistEntryMatches(expected, actual, i) {
			failed = true
		}
	}

	return !failed
}

func (m *Assertions) SessionsMatch(expected Session, actual Session) bool {
	m.T.Helper()

	if !m.Len(actual.Cookies, len(expected.Cookies), "session set-cookie-call count does not match expected") {
		return m.Equal(expected, actual)
	}

	var failed bool

	for i := range actual.Cookies {
		if !m.SetCookiesMatch(expected.Cookies, actual.Cookies, i) {
			failed = true
		}
	}

	return !failed
}

// note: does not check time. body contents are extracted and placed into
// equivalent Readers.
func (m *Assertions) HistEntryMatches(expectedHist []HistoryEntry, actualHist []HistoryEntry, idx int) bool {
	m.T.Helper()

	var failed bool

	expected := expectedHist[idx]
	actual := actualHist[idx]

	if !m.Equalf(expected.Template, actual.Template, "history entry[%d] template does not match expected", idx) {
		failed = true
	}

	// extract response/request bodies so we can mess with them
	oldExpReqBody := expected.Request.Body
	oldExpRespBody := expected.Response.Body
	oldActReqBody := actual.Request.Body
	oldActRespBody := actual.Response.Body
	defer func() {
		expected.Request.Body = oldExpReqBody
		expected.Response.Body = oldExpRespBody
		actual.Request.Body = oldActReqBody
		actual.Response.Body = oldActRespBody
	}()

	if expected.Request.Body == nil {
		expected.Request.Body = http.NoBody
	}
	if expected.Response.Body == nil {
		expected.Response.Body = http.NoBody
	}
	if actual.Request.Body == nil {
		actual.Request.Body = http.NoBody
	}
	if actual.Response.Body == nil {
		actual.Response.Body = http.NoBody
	}

	expReqBytes, err := io.ReadAll(expected.Request.Body)
	if err != nil {
		m.T.Fatalf("error reading expected request body: %v", err)
	}
	expected.Request.Body = io.NopCloser(bytes.NewReader(expReqBytes))

	expRespBytes, err := io.ReadAll(expected.Response.Body)
	if err != nil {
		m.T.Fatalf("error reading expected response body: %v", err)
	}
	expected.Response.Body = io.NopCloser(bytes.NewReader(expRespBytes))

	actReqBytes, err := io.ReadAll(actual.Request.Body)
	if err != nil {
		m.T.Fatalf("error reading actual request body: %v", err)
	}
	actual.Request.Body = io.NopCloser(bytes.NewReader(actReqBytes))

	actRespBytes, err := io.ReadAll(actual.Response.Body)
	if err != nil {
		m.T.Fatalf("error reading actual response body: %v", err)
	}
	actual.Response.Body = io.NopCloser(bytes.NewReader(actRespBytes))

	if !m.Equalf(expected.Request, actual.Request, "history entry[%d] request does not match expected", idx) {
		failed = true
	}
	if !m.Equalf(expected.Response, actual.Response, "history entry[%d] response does not match expected", idx) {
		failed = true
	}
	if !m.Equalf(expected.Captures, actual.Captures, "history entry[%d] captures do not match expected", idx) {
		failed = true
	}
	if !m.Equalf(expected.Initiator, actual.Initiator, "history entry[%d] initiator does not match expected", idx) {
		failed = true
	}

	return !failed
}

// note: does not check time.
func (m *Assertions) SetCookiesMatch(expectedCookies []SetCookiesCall, actualCookies []SetCookiesCall, idx int) bool {
	m.T.Helper()

	var failed bool

	expected := expectedCookies[idx]
	actual := actualCookies[idx]

	if !m.Equalf(expected.URL, actual.URL, "set-cookie[%d] URL does not match expected", idx) {
		failed = true
	}
	if !m.Equalf(expected.Cookies, actual.Cookies, "set-cookie[%d] cookies does not match expected", idx) {
		failed = true
	}

	return !failed
}
