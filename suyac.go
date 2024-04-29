package suyac

import (
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"regexp"
	"strings"
	"time"
)

type RESTClient struct {
	HTTP      *http.Client
	Vars      map[string]string
	VarPrefix string
}

// NewRESTClient creates a new RESTClient.
func NewRESTClient() *RESTClient {
	cookies, cookieErr := cookiejar.New(nil)
	if cookieErr != nil {
		panic(cookieErr)
	}

	return &RESTClient{
		HTTP: &http.Client{
			Transport:     http.DefaultTransport,
			CheckRedirect: nil,
			Jar:           cookies,
			Timeout:       30 * time.Second,
		},
		Vars:      make(map[string]string),
		VarPrefix: "$",
	}
}

func (r *RESTClient) Request(method string, url string, data []byte) (*http.Response, error) {
	// find every variable in url of  and replace it with the value from r.Vars (or return error if not)
	url, err := r.Substitute(url)
	if err != nil {
		return nil, fmt.Errorf("substitute vars in URL: %w", err)
	}

	var payload io.Reader
	// find every variable in data and replace it with the value from r.Vars (or return error if not)
	if data != nil {
		dataStr := string(data)
		dataStr, err = r.Substitute(dataStr)
		if err != nil {
			return nil, fmt.Errorf("substitute vars in data: %w", err)
		}

		payload = strings.NewReader(dataStr)
	}

	req, err := http.NewRequest(method, url, payload)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	return r.HTTP.Do(req)
}

func (r *RESTClient) Substitute(s string) (string, error) {
	// find every variable in s and replace it with the value from r.Vars (or return error if not)
	expr := regexp.QuoteMeta(r.VarPrefix) + `{[a-zA-Z0-9_]+}`
	expr += `([a-zA-Z0-9_]+)`
	expr += regexp.QuoteMeta("}")
	rx, err := regexp.Compile(expr)
	if err != nil {
		return "", fmt.Errorf("compile regular expression: %w", err)
	}

	updated := strings.Builder{}
	var lastSearchEnd int

	// find all matches
	matchPairs := rx.FindAllStringIndex(s, -1)
	for _, pair := range matchPairs {
		// check if it begins with a doubled prefix; if so, skip it
		prefixLen := len(r.VarPrefix)

		if pair[0]-prefixLen >= 0 {
			prevSequence := s[pair[0]-prefixLen : pair[0]]
			if prevSequence == r.VarPrefix {
				// ignore it
				continue
			}
		}

		// get the variable name
		varName := s[pair[0]+prefixLen+1 : pair[1]-1]
		// get the value from r.Vars
		varValue, ok := r.Vars[varName]
		if !ok {
			return "", fmt.Errorf("variable %s not found", varName)
		}

		// add replaced value and any prior content to updated
		updated.WriteString(s[:pair[0]])
		updated.WriteString(varValue)

		lastSearchEnd = pair[1]
	}

	if len(s) > lastSearchEnd {
		updated.WriteString(s[lastSearchEnd:])
	}

	return updated.String(), nil
}
