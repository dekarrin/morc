package suyac

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/dekarrin/rezi/v2"
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

func ParseVarScraper(s string) (VarScraper, error) {
	// Parse var scraper specification strings of the form "NAME::START,END" for
	// byte offsets and "NAME:key1.key2[index1]...keyN" for JSON traversal with array
	// indexes and object keys in a syntax similar to jq.

	// first, split name from spec:
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return VarScraper{}, fmt.Errorf("not in NAME:SPEC format")
	}

	name := parts[0]

	// validate that name does not contain any invalid characters; it must be
	// alphanumeric or underscore
	if !regexp.MustCompile(`^[a-zA-Z0-9_]+$`).MatchString(name) {
		return VarScraper{}, fmt.Errorf("name %q contains invalid characters", name)
	}

	spec := parts[1]

	// okay, are we looking at a byte offset or a JSON traversal?
	if strings.HasPrefix(spec, ":") {
		// it is a byte offset of the form ":START,END"
		offsets := strings.SplitN(spec[1:], ",", 2)
		if len(offsets) != 2 {
			return VarScraper{}, fmt.Errorf("%q is not in :START,END format", spec)
		}

		start, err := strconv.Atoi(offsets[0])
		if err != nil {
			return VarScraper{}, fmt.Errorf("%q: start offset: %w", spec, err)
		}

		end, err := strconv.Atoi(offsets[1])
		if err != nil {
			return VarScraper{}, fmt.Errorf("%q: end offset: %w", spec, err)
		}

		if end <= start {
			return VarScraper{}, fmt.Errorf("end offset %d is less than or equal to start offset %d", end, start)
		}

		return VarScraper{
			Name:        name,
			OffsetStart: start,
			OffsetEnd:   end,
		}, nil
	}

	// otherwise, it must be a JSON traversal. Use . as the path separator, and
	// [index] to for array indexes. Space chars and dots in keys are only
	// allowed if key is quoted with double-quotes. Unquoted keys can contain
	// any other character. Indexes must be integers. Quoted keys may contain a
	// backslash to escape a quote or backslash.

	// to make it easier, ensure that spec starts with a dot.
	if !strings.HasPrefix(spec, ".") {
		spec = "." + spec
	}

	steps := []TraversalStep{}
	var currentStep TraversalStep

	type mode int64

	const (
		none mode = iota
		inKey
		inQuotedKey
		inIndex
	)

	var curMode mode

	var curSymbol strings.Builder

	specR := []rune(spec)
	for i := 0; i < len(specR); i++ {
		ch := specR[i]

		switch curMode {
		case none:
			if ch == '.' {
				// lookahead to see if in quote
				if i+1 < len(specR) && specR[i+1] == '"' {
					curMode = inQuotedKey
					i++
				} else {
					curMode = inKey
				}
			} else if ch == '[' {
				curMode = inIndex
			} else {
				return VarScraper{}, fmt.Errorf("invalid character %q at position %d; should be either '.' to specify a key or '[' to specify an index", ch, i)
			}
		case inKey:
			if ch == '.' || ch == '[' {
				// at end of the key, add it to the steps, reset mode, and continue
				// parsing at this index
				symStr := curSymbol.String()
				if symStr == "" {
					return VarScraper{}, fmt.Errorf("missing key at position %d", i)
				}
				currentStep.Key = symStr
				steps = append(steps, currentStep)
				currentStep = TraversalStep{}
				curSymbol.Reset()
				curMode = none
				i--
			} else if ch == '\\' {
				// escape character; consume next character
				i++
				if i >= len(specR) {
					return VarScraper{}, fmt.Errorf("escape character at end of string")
				}
				curSymbol.WriteRune(specR[i])
			} else if unicode.IsSpace(ch) {
				return VarScraper{}, fmt.Errorf("unescaped whitespace character in key at position %d; quote key name or escape whitespace with '\\'", i)
			} else {
				curSymbol.WriteRune(ch)
			}
		case inQuotedKey:
			if ch == '"' {
				// end of quoted key
				symStr := curSymbol.String()
				if symStr == "" {
					return VarScraper{}, fmt.Errorf("missing key at position %d", i)
				}
				currentStep.Key = symStr
				steps = append(steps, currentStep)
				currentStep = TraversalStep{}
				curSymbol.Reset()
				curMode = none
			} else if ch == '\\' {
				// escape character; consume next character
				i++
				if i >= len(specR) {
					return VarScraper{}, fmt.Errorf("escape character at end of string")
				}
				curSymbol.WriteRune(specR[i])
			} else {
				curSymbol.WriteRune(ch)
			}
		case inIndex:
			if ch == ']' {
				// end of index
				symStr := curSymbol.String()
				if symStr == "" {
					return VarScraper{}, fmt.Errorf("missing index at position %d", i)
				}
				index, err := strconv.Atoi(symStr)
				if err != nil {
					return VarScraper{}, fmt.Errorf("invalid index %q: %w", symStr, err)
				}
				currentStep.Index = index
				steps = append(steps, currentStep)
				currentStep = TraversalStep{}
				curSymbol.Reset()
				curMode = none
			}
		default:
			// should never happen
			return VarScraper{}, fmt.Errorf("invalid mode %d", curMode)
		}
	}

	// we should be in mode none at the end, but it is valid to be in mode inKey
	// as well
	if curMode == inKey {
		symStr := curSymbol.String()
		if symStr == "" {
			return VarScraper{}, fmt.Errorf("missing key at end of string")
		}
		currentStep.Key = symStr
		steps = append(steps, currentStep)
	} else if curMode == inQuotedKey {
		return VarScraper{}, fmt.Errorf("unterminated quoted key at end of string")
	} else if curMode == inIndex {
		return VarScraper{}, fmt.Errorf("unterminated index at end of string")
	}

	return VarScraper{
		Name:  name,
		Steps: steps,
	}, nil
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

// Do not use default RESTClient, call NewRESTClient instead.
type RESTClient struct {
	HTTP         *http.Client
	Vars         map[string]string
	VarOverrides map[string]string // Cleared after every call to SendRequest.
	VarPrefix    string

	Scrapers []VarScraper

	// cookie jar that records all SetCookies calls; this is a pointer to the
	// same jar that is passed to HTTP
	jar *TimedCookieJar
}

// NewRESTClient creates a new RESTClient. 0 for cookie lifetime will default it
// to 24 hours.
func NewRESTClient(cookieLifetime time.Duration) *RESTClient {
	cookies := newTimedCookieJar(nil, cookieLifetime)

	return &RESTClient{
		HTTP: &http.Client{
			Transport:     http.DefaultTransport,
			CheckRedirect: nil,
			Jar:           cookies,
			Timeout:       30 * time.Second,
		},
		Vars:      make(map[string]string),
		VarPrefix: "$",
		Scrapers:  make([]VarScraper, 0),
		jar:       cookies,
	}
}

// CreateRequest creates a request to the given endpoint. Values set in Vars and
// VarOverrides are used to fill any variables in the URL, data, and headers.
func (r *RESTClient) CreateRequest(method string, url string, data []byte, hdrs http.Header) (*http.Request, error) {
	// find every variable in url of  and replace it with the value from r.Vars (or return error if encountering invalid var)
	url, err := r.Substitute(url)
	if err != nil {
		return nil, fmt.Errorf("substitute vars in URL: %w", err)
	}

	var payload io.Reader
	// find every variable in data and replace it with the value from r.Vars (or return error if encountering invalid var)
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
		return nil, err
	}

	// find every variable in headers and replace it with the value from r.Vars (or return error if encountering invalid var)
	if len(hdrs) > 0 {
		req.Header = make(http.Header)
		for key, values := range hdrs {
			newKey, err := r.Substitute(key)
			if err != nil {
				return nil, fmt.Errorf("substitute header key %q: %w", key, err)
			}

			for _, value := range values {
				newValue, err := r.Substitute(value)
				if err != nil {
					return nil, fmt.Errorf("substitute header value %q: %w", value, err)
				}
				req.Header.Add(newKey, newValue)
			}
		}
	}

	return req, err
}

// SendRequest sends the given request and returns the response. VarOverrides
// will be cleared after this is called. Prior to returning, the response is
// scanned for var captures and those that are captured are stored in Vars and
// re
func (r *RESTClient) SendRequest(req *http.Request) (*http.Response, map[string]string, error) {
	resp, err := r.HTTP.Do(req)
	if err != nil {
		return resp, nil, err
	}

	// we need to load the entire response body into memory so we can scrape it
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp, nil, fmt.Errorf("read response body: %w", err)
	}
	resp.Body.Close()
	resp.Body = io.NopCloser(bytes.NewBuffer(respBody))

	// scrape vars from response
	capturedVars := make(map[string]string)
	for _, scraper := range r.Scrapers {
		value, err := scraper.Scrape(respBody)
		if err != nil {
			return resp, nil, fmt.Errorf("scrape %s: %w", scraper.Name, err)
		}
		capturedVars[scraper.Name] = value
		r.Vars[scraper.Name] = value
	}

	// clear var overrides
	r.VarOverrides = map[string]string{}

	return resp, capturedVars, nil
}

func (r *RESTClient) Substitute(s string) (string, error) {
	// find every variable in s and replace it with the value from r.Vars (or return error if not)
	expr := regexp.QuoteMeta(r.VarPrefix + "{")
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

		// get the value from r.VarOverrides followed by r.Vars
		var varValue string
		var ok bool
		if varValue, ok = r.VarOverrides[varName]; !ok {
			varValue, ok = r.Vars[varName]
			if !ok {
				return "", fmt.Errorf("variable %s not found", varName)
			}
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

// State holds all information in a saved state file.
type State struct {
	Cookies []SetCookiesCall
	Vars    map[string]string
}

func (r *RESTClient) WriteState(w io.Writer) error {
	rzw, err := rezi.NewWriter(w, nil)
	if err != nil {
		return fmt.Errorf("create REZI writer: %w", err)
	}

	r.jar.evictOld()
	state := State{
		Cookies: r.jar.calls,
		Vars:    r.Vars,
	}

	if err := rzw.Enc(state); err != nil {
		return fmt.Errorf("encode cookie jar: %w", err)
	}

	return rzw.Close()
}

func (r *RESTClient) ReadState(rd io.Reader) error {
	rzr, err := rezi.NewReader(rd, nil)
	if err != nil {
		return fmt.Errorf("create REZI reader: %w", err)
	}

	// first, get state object
	var state State
	if err := rzr.Dec(&state); err != nil {
		return fmt.Errorf("decode state: %w", err)
	}

	// create the cookie jar
	existingLifetime := 0 * time.Second
	if r.jar != nil {
		existingLifetime = r.jar.lifetime
	}
	jar := newTimedCookieJar(nil, existingLifetime)
	jar.calls = state.Cookies

	jar.evictOld()
	for _, call := range jar.calls {
		jar.wrapped.SetCookies(call.URL, call.Cookies)
	}

	r.jar = jar
	r.HTTP.Jar = jar
	r.Vars = state.Vars

	return rzr.Close()
}

type SetCookiesCall struct {
	Time    time.Time      `json:"time"`
	URL     *url.URL       `json:"url"`
	Cookies []*http.Cookie `json:"cookies"`
}

func (sc SetCookiesCall) String() string {
	return fmt.Sprintf("SetCookiesCall{Time: %s, URL: %s, Cookies: %v}", sc.Time, sc.URL, sc.Cookies)
}

func (sc SetCookiesCall) MarshalBinary() ([]byte, error) {
	var enc []byte

	enc = append(enc, rezi.MustEnc(sc.Time)...)
	enc = append(enc, rezi.MustEnc(sc.URL.String())...)
	enc = append(enc, rezi.MustEnc(sc.Cookies)...)

	return enc, nil
}

func (sc *SetCookiesCall) UnmarshalBinary(data []byte) error {
	var n, offset int
	var err error

	var decoded SetCookiesCall

	// Time
	n, err = rezi.Dec(data[offset:], &decoded.Time)
	if err != nil {
		return rezi.Wrapf(offset, "time: %w", err)
	}
	offset += n

	// URL
	var urlString string
	n, err = rezi.Dec(data[offset:], &urlString)
	if err != nil {
		return rezi.Wrapf(offset, "url: %w", err)
	}
	decoded.URL, err = url.Parse(urlString)
	if err != nil {
		return rezi.Wrapf(offset, "parse URL at offset: %w", err)
	}
	offset += n

	// Cookies
	_, err = rezi.Dec(data[offset:], &decoded.Cookies)
	if err != nil {
		return rezi.Wrapf(offset, "cookies: %w", err)
	}

	*sc = decoded

	return nil
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
//
// It uses trickiness inside of unmarshal that relies on assumption that it is
// being called on a valid one whose wrapped cookiejar hasn't yet been called.
type TimedCookieJar struct {
	lifetime time.Duration
	wrapped  http.CookieJar

	calls               []SetCookiesCall
	numCalls            int
	callsBeforeEviction int
	mx                  *sync.Mutex
}

// newTimedCookieJar creates a new TimedCookieJar with the given lifetime. If
// lifetime is 0, the default lifetime of 24 hours is used. If wrapped is nil,
// a new net/http/cookiejar.Jar is created with default options and used as
// wrapped.
func newTimedCookieJar(wrapped http.CookieJar, lifetime time.Duration) *TimedCookieJar {
	if lifetime <= 0 {
		lifetime = 24 * time.Hour
	}
	if wrapped == nil {
		var err error
		wrapped, err = cookiejar.New(nil)
		if err != nil {
			panic(err)
		}
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

func (j *TimedCookieJar) evictOld() {
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

func (j *TimedCookieJar) checkEviction() {
	if j.numCalls == 0 {
		j.evictOld()
	}
}
