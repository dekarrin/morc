package commands

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dekarrin/morc"
	"github.com/dekarrin/morc/cmd/morc/cmdio"
	"github.com/stretchr/testify/assert"
)

func Test_Send(t *testing.T) {
	respFnNoBodyOK := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}

	testCases := []struct {
		name            string
		respFn          func(w http.ResponseWriter, r *http.Request)
		reqs            []morc.RequestTemplate // endpoints are relative to some server; do not include host
		args            []string               // DO NOT INCLUDE -F; it is automatically set to a project file
		expectErr       string                 // set if command.Execute expected to fail, with a string that would be in the error message
		expectErrOutput string                 // set with expected output to stderr
		expectOutput    string                 // set with expected output to stdout
	}{
		{
			name:   "minimal request/response",
			respFn: respFnNoBodyOK,
			reqs: []morc.RequestTemplate{
				{
					Name:   "testreq",
					Method: "GET",
					URL:    "/",
				},
			},
			args: []string{"send", "testreq"},
			expectOutput: `HTTP/1.1 200 OK
(no response body)
`,
		},
		{
			name:   "minimal request/response with headers",
			respFn: respFnNoBodyOK,
			reqs: []morc.RequestTemplate{
				{
					Name:   "testreq",
					Method: "GET",
					URL:    "/",
				},
			},
			args: []string{"send", "testreq", "--headers"},
			expectOutput: `HTTP/1.1 200 OK
------------------- HEADERS -------------------
Content-Length: 0
Date: Tue, 14 May 2024 15:27:36 GMT
-----------------------------------------------
(no response body)
`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert := assert.New(t)

			// setup test server
			srv := httptest.NewServer(http.HandlerFunc(tc.respFn))
			defer srv.Close()
			cmdio.HTTPClient = srv.Client()

			// create project and dump config to a temp dir
			dir := t.TempDir()
			projFilePath := filepath.Join(dir, "project.json")
			p := morc.Project{
				Name:      "Test",
				Templates: map[string]morc.RequestTemplate{},
				Config: morc.Settings{
					ProjFile: projFilePath,
				},
			}
			for _, req := range tc.reqs {
				req.URL = srv.URL + req.URL
				p.Templates[strings.ToLower(req.Name)] = req
			}

			f, err := os.Create(projFilePath)
			if err != nil {
				t.Fatal(err)
			}
			if err := p.Dump(f); err != nil {
				t.Fatal(err)
			}
			f.Close()

			// set up the root command and run
			stdoutCapture := &bytes.Buffer{}
			stderrCapture := &bytes.Buffer{}

			args := tc.args
			args = append(args, "-F", projFilePath)

			sendCmd.Root().SetOut(stdoutCapture)
			sendCmd.Root().SetErr(stderrCapture)
			sendCmd.Root().SetArgs(args)

			// SETUP COMPLETE, EXECUTE
			err = sendCmd.Execute()

			// assert and check stdout and stderr
			if err != nil {
				if tc.expectErr == "" {
					t.Fatalf("unexpected returned error: %v", err)
					return
				}
				if !strings.Contains(err.Error(), tc.expectErr) {
					t.Fatalf("expected returned error to contain %q, got %q", tc.expectErr, err)
					return
				}
			}

			// okay, check stdout and stderr
			output := stdoutCapture.String()
			outputErr := stderrCapture.String()

			assert.Equal(tc.expectOutput, output)
			assert.Equal(tc.expectErrOutput, outputErr)
		})
	}
}
