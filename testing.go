package morc

import "github.com/stretchr/testify/assert"

func AssertProjectInFileMatches(assert *assert.Assertions, expected Project, projFilePath string) bool {
	updatedProj, err := LoadProjectFromDisk(projFilePath, true)
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

	AssertHistoriesMatch(assert, expected.History, updatedProj.History)
	AssertSessionsMatch(assert, expected.Session, updatedProj.Session)

	// unset histories and sessions on both as they are separately checked above
	expected.History = nil
	expected.Session = Session{}

	return assert.Equal(expected, updatedProj, "project in file does not match expected")
}

// TODO: move all this to a custom asserter.
func AssertHistoriesMatch(assert *assert.Assertions, expected, actual []HistoryEntry) bool {
	if !assert.Len(actual, len(expected), "history entry count does not match expected") {
		return false
	}

	var failed bool

	for i := range actual {
		if !AssertHistEntryMatches(assert, expected, actual, i) {
			failed = true
		}
	}

	return !failed
}

func AssertSessionsMatch(assert *assert.Assertions, expected, actual Session) bool {
	if !assert.Len(actual.Cookies, len(expected.Cookies), "session set-cookie-call count does not match expected") {
		return false
	}

	var failed bool

	for i := range actual.Cookies {
		if !AssertSetCookiesMatches(assert, expected.Cookies, actual.Cookies, i) {
			failed = true
		}
	}

	return !failed
}

// note: does not check time.
func AssertSetCookiesMatches(assert *assert.Assertions, expectedCookies []SetCookiesCall, actualCookies []SetCookiesCall, idx int) bool {
	var failed bool

	expected := expectedCookies[idx]
	actual := actualCookies[idx]

	if !assert.Equalf(expected.URL, actual.URL, "set-cookie[%d] URL does not match expected", idx) {
		failed = true
	}
	if !assert.Equalf(expected.Cookies, actual.Cookies, "set-cookie[%d] cookies does not match expected", idx) {
		failed = true
	}

	return !failed
}

// note: does not check time.
func AssertHistEntryMatches(assert *assert.Assertions, expectedHist []HistoryEntry, actualHist []HistoryEntry, idx int) bool {

	var failed bool

	expected := expectedHist[idx]
	actual := actualHist[idx]

	if !assert.Equalf(expected.Template, actual.Template, "history entry[%d] template does not match expected", idx) {
		failed = true
	}
	if !assert.Equalf(expected.Request, actual.Request, "history entry[%d] request does not match expected", idx) {
		failed = true
	}
	if !assert.Equalf(expected.Response, actual.Response, "history entry[%d] response does not match expected", idx) {
		failed = true
	}
	if !assert.Equalf(expected.Captures, actual.Captures, "history entry[%d] captures do not match expected", idx) {
		failed = true
	}

	return !failed
}
