package suyac

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode"
)

type TraversalStep struct {
	Key   string // if set, index is ignored
	Index int
}

func (t TraversalStep) String() string {
	if t.Key != "" {
		return "." + t.Key
	}
	return fmt.Sprintf("[%d]", t.Index)
}

func (t TraversalStep) Traverse(data interface{}) (interface{}, error) {
	switch data := data.(type) {
	case map[string]interface{}:
		return data[t.Key], nil
	case []interface{}:
		return data[t.Index], nil
	default:
		return nil, fmt.Errorf("can't traverse %T", data)
	}
}

type VarScraper struct {
	Name        string
	OffsetStart int
	OffsetEnd   int
	Steps       []TraversalStep // if non-nil, OffsetStart and OffsetEnd are ignored
}

func (v VarScraper) String() string {
	s := fmt.Sprintf("VarScraper{%s from ", v.Name)
	if len(v.Steps) > 0 {
		for _, step := range v.Steps {
			s += step.String()
		}
	} else {
		s += fmt.Sprintf("offset %d,%d", v.OffsetStart, v.OffsetEnd)
	}
	return s + "}"
}

func (v VarScraper) Scrape(data []byte) (string, error) {
	if len(v.Steps) < 1 {
		// binary offset only, just do a bounds check
		if v.OffsetEnd > len(data) {
			return "", fmt.Errorf("end offset is %d but data length is only %d", v.OffsetEnd, len(data))
		}
		return string(data[v.OffsetStart:v.OffsetEnd]), nil
	}

	// otherwise, perform the traversal. hopefully we got either a JSON map or a
	// JSON list or this is going to fail
	var jsonData interface{}

	// ...just look ahead and check if the first non-whitespace char is a '{' or '['
	var firstChar rune
	for _, b := range data {
		if unicode.IsSpace(rune(b)) {
			continue
		}
		firstChar = rune(b)
		break
	}

	// if first char is a '{', assume it's a map
	if firstChar == '{' {
		var jsonMap map[string]interface{}
		err := json.Unmarshal(data, &jsonMap)
		if err != nil {
			return "", fmt.Errorf("unmarshal JSON map: %w", err)
		}
		jsonData = jsonMap
	} else if firstChar == '[' {
		var jsonList []interface{}
		err := json.Unmarshal(data, &jsonList)
		if err != nil {
			return "", fmt.Errorf("unmarshal JSON list: %w", err)
		}
		jsonData = jsonList
	} else {
		return "", fmt.Errorf("data does not appear to be a JSON array or object")
	}

	// now that we have the parsed data, apply traversal steps
	var err error
	for idx, step := range v.Steps {
		jsonData, err = step.Traverse(jsonData)
		if err != nil {
			errSequence := ""
			for _, oldStep := range v.Steps[:idx+1] {
				errSequence += oldStep.String()
			}
			return "", fmt.Errorf("traversal error at %s: %w", errSequence, err)
		}
	}

	// assuming successful traversal, jsonData should be the value we want.
	switch typedData := jsonData.(type) {
	case string:
		return typedData, nil
	default:
		return fmt.Sprintf("%v", jsonData), nil
	}
}

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

type StateInfo struct {
	Variables map[string]string
}

func (r *RESTClient) WriteState(w io.Writer) error {

}

type SetCookiesCall struct {
	Time    time.Time
	URL     *url.URL
	Cookies []*http.Cookie
}

// TimedCookieJar wraps a net/http.CookieJar implementation and does quick and
// dirty recording of all cookies that are received. Because it cannot examine
// the policy of the wrapped jar, it simply records calls to SetCookies and
// stores enough information to reproduce all said calls, and additionally
// records the time that each call was made.
//
// This record can be persisted to bytes, and later played back to restore the
// state of the cookie jar. Note that depending on the policy of the wrapped
// jar, cookies that are valid at persistence time may be invalid at playback
// time.
//
// The time of each call is used for record eviction. At load time and
// periodically during calls to other methods, the jar will remove any records
// that are older than a certain threshold. This threshold is stored in the
// Lifetime member of the TimedCookieJar.
//
// The zero value of TimedCookieJar is not valid. Use NewTimedCookieJar to
// create one.
type TimedCookieJar struct {
	lifetime time.Duration
	wrapped  http.CookieJar

	calls               []SetCookiesCall
	numCalls            int
	callsBeforeEviction int
	mx                  *sync.Mutex
}

// NewTimedCookieJar creates a new TimedCookieJar with the given lifetime. If
// lifetime is 0, the default lifetime of 24 hours is used. If wrapped is nil,
// a new net/http/cookiejar.Jar is created with default options and used as
// wrapped.
func NewTimedCookieJar(wrapped http.CookieJar, lifetime time.Duration) *TimedCookieJar {
	if lifetime <= 0 {
		lifetime = 24 * time.Hour
	}
	if wrapped == nil {
		wrapped, _ = cookiejar.New(nil)
	}

	return &TimedCookieJar{
		lifetime:            lifetime,
		wrapped:             wrapped,
		calls:               make([]SetCookiesCall, 0),
		numCalls:            0,
		callsBeforeEviction: 20,
		mx:                  &sync.Mutex{},
	}
}

func (j *TimedCookieJar) SetCookies(u *url.URL, cookies []*http.Cookie) {
	j.wrapped.SetCookies(u, cookies)

	j.mx.Lock()
	defer j.mx.Unlock()
	j.calls = append(j.calls, SetCookiesCall{
		Time:    time.Now(),
		URL:     u,
		Cookies: cookies,
	})
	j.numCalls = (j.numCalls + 1) % j.callsBeforeEviction
	j.checkEviction()
}

func (j *TimedCookieJar) Cookies(u *url.URL) []*http.Cookie {
	return j.wrapped.Cookies(u)
}

func (j *TimedCookieJar) checkEviction() {
	if j.numCalls == 0 {
		// remove any calls that are older than Lifetime
		oldestTime := time.Now().Add(-j.lifetime)
		startIdx := -1
		for idx, call := range j.calls {
			if !call.Time.Before(oldestTime) {
				startIdx = idx
				break
			}
		}
		if startIdx >= 0 {
			j.calls = j.calls[startIdx:]
		}
	}
}
