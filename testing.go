package morc

import (
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

// note: does not check time.
func (m *Assertions) HistEntryMatches(expectedHist []HistoryEntry, actualHist []HistoryEntry, idx int) bool {
	m.T.Helper()

	var failed bool

	expected := expectedHist[idx]
	actual := actualHist[idx]

	if !m.Equalf(expected.Template, actual.Template, "history entry[%d] template does not match expected", idx) {
		failed = true
	}
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
