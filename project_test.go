package morc

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_LoadProjectFromDisk(t *testing.T) {
	// create a project with some templates and vars
	p := Project{
		Name: "Test Project",
		Templates: map[string]RequestTemplate{
			"template1": {
				Name:   "template1",
				Method: "GET",
				URL:    "http://example.com",
				Headers: http.Header{
					"Accept": []string{"application/json"},
				},
			},
			"template2": {
				Name:   "template2",
				Method: "POST",
				URL:    "http://example.com",
				Headers: http.Header{
					"Accept": []string{"application/json"},
				},
			},
		},
		Flows: map[string]Flow{
			"flow1": {
				Name: "flow1",
				Steps: []FlowStep{
					{
						Template: "template1",
					},
					{
						Template: "template2",
					},
				},
			},
		},
		Vars: NewVarStore(),
		History: []HistoryEntry{
			{
				Template: "template2",
				Request: &http.Request{
					URL:    mustParseURL("http://example.com"),
					Method: "POST",
					Body:   http.NoBody,
				},
				Response: &http.Response{
					StatusCode: 200,
					Body:       http.NoBody,
				},
			},
		},
		Session: Session{
			Cookies: []SetCookiesCall{
				{
					URL: mustParseURL("http://example.com"),
					Cookies: []*http.Cookie{
						{
							Name:  "cookie1",
							Value: "value1",
							Raw:   "cookie1=value1",
						},
					},
				},
			},
		},
		Config: Settings{
			CookieLifetime: 24,
			ProjFile:       "project.json",
			HistFile:       "::PROJ_DIR::/history.json",
			SeshFile:       "::PROJ_DIR::/session.json",
			RecordHistory:  true,
			RecordSession:  true,
		},
	}

	assert := assert.New(t)

	projFilePath := createTestProjectFiles(t, p)
	if projFilePath == "" {
		t.Fatal("failed to create test project files")
		return
	}

	// compare loaded to original
	AssertProjectInFileMatches(assert, p, projFilePath)
}

func mustParseURL(s string) *url.URL {
	u, err := url.Parse(s)
	if err != nil {
		panic(err)
	}
	return u
}

func createTestProjectFiles(t *testing.T, p Project) string {
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
		if !strings.HasPrefix(p.Config.HistFile, ProjDirVar) {
			t.Fatal("hist file path must start with " + ProjDirVar + " if present in tests")
			return ""
		}

		relativePath := strings.TrimPrefix(p.Config.HistFile, ProjDirVar)
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
		if !strings.HasPrefix(p.Config.SeshFile, ProjDirVar) {
			t.Fatal("session file path must start with " + ProjDirVar + " if present in tests")
			return ""
		}

		relativePath := strings.TrimPrefix(p.Config.SeshFile, ProjDirVar)
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
