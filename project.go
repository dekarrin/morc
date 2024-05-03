package suyac

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dekarrin/rezi/v2"
)

const (
	DefaultProjectPath = ".suyac/project.json"
	DefaultSessionPath = ".suyac/session.json"
	DefaultHistoryPath = ".suyac/history.json"
)

type Settings struct {
	ProjFile       string        `json:"project_file"`
	HistFile       string        `json:"history_file"`
	SeshFile       string        `json:"session_file"`
	CookieLifetime time.Duration `json:"cookie_lifetime"`
}

type Project struct {
	Name      string
	Templates map[string]RequestTemplate // note: Names must be manually synched across Templates, Flows, and History
	Flows     map[string]Flow
	Vars      VarStore
	History   []HistoryEntry
	Session   Session
	Config    Settings
}

type marshaledProject struct {
	Name      string                     `json:"name"`
	Templates map[string]RequestTemplate `json:"templates"`
	Flows     map[string]Flow            `json:"flows"`
	Vars      VarStore                   `json:"vars"`
	Config    Settings                   `json:"config"`
}

func (p Project) PersistHistoryToDisk() error {
	histDataBytes, err := json.Marshal(p.History)
	if err != nil {
		return fmt.Errorf("marshal history data: %w", err)
	}

	histPath := p.Config.HistFile
	if histPath == "" {
		histPath = DefaultHistoryPath
	}

	if err := os.MkdirAll(filepath.Dir(histPath), 0755); err != nil {
		return fmt.Errorf("create dir for history file: %w", err)
	}

	if err := os.WriteFile(histPath, histDataBytes, 0644); err != nil {
		return fmt.Errorf("write history file: %w", err)
	}

	return nil
}

func (p Project) PersistSessionToDisk() error {
	seshDataBytes, err := json.Marshal(p.Session)
	if err != nil {
		return fmt.Errorf("marshal session data: %w", err)
	}

	seshPath := p.Config.SeshFile
	if seshPath == "" {
		seshPath = DefaultSessionPath
	}

	if err := os.MkdirAll(filepath.Dir(seshPath), 0755); err != nil {
		return fmt.Errorf("create dir for session file: %w", err)
	}

	if err := os.WriteFile(seshPath, seshDataBytes, 0644); err != nil {
		return fmt.Errorf("write session file: %w", err)
	}

	return nil
}

// PersistToDisk writes up to 3 files; one for the project, one for the session,
// and one for the history. If p.ProjFile is empty, it will be written to the
// current working directory at path .suyac/project.json. If p.SeshFile is
// empty, it will be written to the current working directory at path
// .suyac/session.json. If p.HistFile is empty, it will be written to the
// current working directory at path .suyac/history.json.
func (p Project) PersistToDisk(all bool) error {
	// get data to persist
	m := marshaledProject{
		Name:      p.Name,
		Templates: p.Templates,
		Flows:     p.Flows,
		Vars:      p.Vars,
		Config:    p.Config,
	}

	projDataBytes, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("marshal project data: %w", err)
	}

	// check file paths and see if they need to be defaulted
	projPath := p.Config.ProjFile
	if projPath == "" {
		projPath = DefaultProjectPath
	}

	// call mkdir -p on the paths
	if err := os.MkdirAll(filepath.Dir(projPath), 0755); err != nil {
		return fmt.Errorf("create dir for project file: %w", err)
	}

	// write out the data for project, session, and history
	if err := os.WriteFile(projPath, projDataBytes, 0644); err != nil {
		return fmt.Errorf("write project file: %w", err)
	}

	if all {
		if err := p.PersistSessionToDisk(); err != nil {
			return fmt.Errorf("persist session: %w", err)
		}

		if err := p.PersistHistoryToDisk(); err != nil {
			return fmt.Errorf("persist history: %w", err)
		}
	}

	return nil
}

func LoadProjectFromDisk(projFilename string, all bool) (Project, error) {
	projData, err := os.ReadFile(projFilename)
	if err != nil {
		return Project{}, fmt.Errorf("read project file: %w", err)
	}

	var m marshaledProject
	if err := json.Unmarshal(projData, &m); err != nil {
		return Project{}, fmt.Errorf("unmarshal project data: %w", err)
	}

	p := Project{
		Name:      m.Name,
		Templates: m.Templates,
		Flows:     m.Flows,
		Vars:      m.Vars,
		Config:    m.Config,
	}

	if all {
		p.Session, err = LoadSessionFromDisk(p.Config.SeshFile)
		if err != nil {
			return Project{}, fmt.Errorf("load session: %w", err)
		}

		p.History, err = LoadHistoryFromDisk(p.Config.HistFile)
		if err != nil {
			return Project{}, fmt.Errorf("load history: %w", err)
		}
	}

	return p, nil
}

func LoadSessionFromDisk(seshFilename string) (Session, error) {
	seshData, err := os.ReadFile(seshFilename)
	if err != nil {
		return Session{}, fmt.Errorf("read session file: %w", err)
	}

	var s Session
	if err := json.Unmarshal(seshData, &s); err != nil {
		return Session{}, fmt.Errorf("unmarshal session data: %w", err)
	}

	return s, nil
}

func LoadHistoryFromDisk(histFilename string) ([]HistoryEntry, error) {
	histData, err := os.ReadFile(histFilename)
	if err != nil {
		return nil, fmt.Errorf("read history file: %w", err)
	}

	var h []HistoryEntry
	if err := json.Unmarshal(histData, &h); err != nil {
		return nil, fmt.Errorf("unmarshal history data: %w", err)
	}

	return h, nil
}

type Session struct {
	Cookies []SetCookiesCall
}

type marshaledSession struct {
	Cookies []string
}

func (s Session) MarshalJSON() ([]byte, error) {
	var ms marshaledSession
	for _, c := range s.Cookies {
		buf := &bytes.Buffer{}
		rzw, err := rezi.NewWriter(buf, &rezi.Format{Compression: true})
		if err != nil {
			return nil, err
		}
		if err := rzw.Enc(c); err != nil {
			return nil, err
		}
		if err := rzw.Close(); err != nil {
			return nil, err
		}
		encoded := "rezi/b64:" + base64.StdEncoding.EncodeToString(buf.Bytes())

		ms.Cookies = append(ms.Cookies, encoded)
	}

	return json.Marshal(ms)
}

func (s *Session) UnmarshalJSON(data []byte) error {
	var ms marshaledSession
	if err := json.Unmarshal(data, &ms); err != nil {
		return err
	}

	for idx, c := range ms.Cookies {
		if !strings.HasPrefix(c, "rezi/b64:") {
			return fmt.Errorf("invalid cookie encoding in cookie index %d; not 'rezi/b64'", idx)
		}

		c = strings.TrimPrefix(c, "rezi/b64:")

		decoded, err := base64.StdEncoding.DecodeString(c)
		if err != nil {
			return err
		}

		buf := bytes.NewBuffer(decoded)
		rzr, err := rezi.NewReader(buf, nil)
		if err != nil {
			return err
		}

		var cookie SetCookiesCall
		if err := rzr.Dec(&cookie); err != nil {
			return err
		}

		s.Cookies = append(s.Cookies, cookie)
	}

	return nil
}

type RequestTemplate struct {
	Name     string
	Captures []VarScraper
	Body     []byte
	URL      string
	Method   string
	Headers  http.Header
	AuthFlow string
}

type VarStore struct {
	Environment string

	envs map[string]map[string]string
}

func (v *VarStore) Get(key string) string {
	if v.envs == nil {
		v.envs = make(map[string]map[string]string)
	}

	env := v.envs[v.Environment]
	if env != nil {
		if value, ok := env[key]; ok {
			return value
		}
	}

	// for some reason, the var doesn't exist. see if it does in the default env
	if v.Environment != "" {
		env = v.envs[""]
		if env != nil {
			if value, ok := env[key]; ok {
				return value
			}
		}
	}

	// couldn't find it, return empty
	return ""
}

func (v *VarStore) Set(key, value string) {
	if v.envs == nil {
		v.envs = make(map[string]map[string]string)
	}

	env := v.envs[v.Environment]
	if env == nil {
		env = make(map[string]string)
		v.envs[v.Environment] = env
	}

	env[key] = value
}

type Flow struct {
	Name     string
	Requests []RequestTemplate
}

type HistoryEntry struct {
	Template string
	ReqTime  time.Time
	RespTime time.Time
	Request  http.Request
	Response http.Response
}

func (h HistoryEntry) MarshalJSON() ([]byte, error) {
	// convert the http.Request and http.Response into marshaledHistoryEntry
	// structs
	reqRec := httpRequestToRecord(&h.Request)
	respRec := httpResponseToRecord(&h.Response)

	// marshal the marshaledHistoryEntry struct
	m := marshaledHistoryEntry{
		Template: h.Template,
		ReqTime:  h.ReqTime.Unix(),
		RespTime: h.RespTime.Unix(),
		Request:  reqRec,
		Response: respRec,
	}

	return json.Marshal(m)
}

func (h *HistoryEntry) UnmarshalJSON(data []byte) error {
	var m marshaledHistoryEntry
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}

	// convert the marshaledHistoryEntry structs back into http.Request and
	// http.Response structs
	req, err := reqRecordToHTTPRequest(m.Request)
	if err != nil {
		return fmt.Errorf("request: %w", err)
	}

	resp, err := respRecordToHTTPResponse(m.Response)
	if err != nil {
		return fmt.Errorf("response: %w", err)
	}

	// set the fields on the HistoryEntry struct
	h.Template = m.Template
	h.ReqTime = time.Unix(m.ReqTime, 0)
	h.RespTime = time.Unix(m.RespTime, 0)
	h.Request = *req
	h.Response = *resp

	return nil
}

type marshaledHistoryEntry struct {
	Template string               `json:"template"`
	ReqTime  int64                `json:"request_time"`
	RespTime int64                `json:"response_time"`
	Request  clientRequestRecord  `json:"request"`
	Response clientResponseRecord `json:"response"`
}

type clientRequestRecord struct {
	Method            string      `json:"method"`
	URL               string      `json:"url"`
	Proto             string      `json:"proto,omitempty"`
	ProtoMajor        int         `json:"proto_major,omitempty"`
	ProtoMinor        int         `json:"proto_minor,omitempty"`
	Headers           http.Header `json:"headers,omitempty"`
	Body              string      `json:"body,omitempty"` // base64 encoded
	ContentLength     int64       `json:"content_length,omitempty"`
	TransferEncodings []string    `json:"transfer_encodings,omitempty"`
	Host              string      `json:"host"`
	Trailers          http.Header `json:"trailers,omitempty"`
}

func httpRequestToRecord(req *http.Request) clientRequestRecord {
	var body string
	if req.Body != nil && req.Body != http.NoBody {
		bodyBytes, err := io.ReadAll(req.Body)
		if err != nil {
			panic(fmt.Sprintf("failed to read request body: %s", err))
		}
		body = base64.StdEncoding.EncodeToString(bodyBytes)
		req.Body.Close()
		req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	}

	return clientRequestRecord{
		Method:            req.Method,
		URL:               req.URL.String(),
		Proto:             req.Proto,
		ProtoMajor:        req.ProtoMajor,
		ProtoMinor:        req.ProtoMinor,
		Headers:           req.Header,
		Body:              body,
		ContentLength:     req.ContentLength,
		TransferEncodings: req.TransferEncoding,
		Host:              req.Host,
		Trailers:          req.Trailer,
	}
}

func reqRecordToHTTPRequest(rec clientRequestRecord) (*http.Request, error) {
	var bodyReader io.ReadCloser
	if len(rec.Body) > 0 {
		body, err := base64.StdEncoding.DecodeString(rec.Body)
		if err != nil {
			return nil, fmt.Errorf("body: %w", err)
		}
		bodyReader = io.NopCloser(bytes.NewBuffer(body))
	} else {
		bodyReader = http.NoBody
	}

	url, err := url.Parse(rec.URL)
	if err != nil {
		return nil, fmt.Errorf("url: %w", err)
	}

	req := &http.Request{
		Method:           rec.Method,
		URL:              url,
		Proto:            rec.Proto,
		ProtoMinor:       rec.ProtoMinor,
		ProtoMajor:       rec.ProtoMajor,
		Header:           rec.Headers,
		Body:             bodyReader,
		ContentLength:    rec.ContentLength,
		TransferEncoding: rec.TransferEncodings,
		Host:             rec.Host,
		Trailer:          rec.Trailers,
	}

	return req, nil
}

// Note: clientResponseRecord will not save TLS state, only whether it was
// indeed received over TLS.
type clientResponseRecord struct {
	Status            string      `json:"status"`
	StatusCode        int         `json:"status_code"`
	Proto             string      `json:"proto,omitempty"`
	ProtoMajor        int         `json:"proto_major,omitempty"`
	ProtoMinor        int         `json:"proto_minor,omitempty"`
	Headers           http.Header `json:"headers,omitempty"`
	Body              string      `json:"body,omitempty"` // base64 encoded
	ContentLength     int64       `json:"content_length,omitempty"`
	TransferEncodings []string    `json:"transfer_encodings,omitempty"`
	Uncompressed      bool        `json:"uncompressed,omitempty"`
	Trailers          http.Header `json:"trailers,omitempty"`
	TLS               bool        `json:"tls"`
}

func httpResponseToRecord(resp *http.Response) clientResponseRecord {
	var body string
	if resp.Body != nil && resp.Body != http.NoBody {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			panic(fmt.Sprintf("failed to read response body: %s", err))
		}
		body = base64.StdEncoding.EncodeToString(bodyBytes)
		resp.Body.Close()
		resp.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	}

	return clientResponseRecord{
		Status:            resp.Status,
		StatusCode:        resp.StatusCode,
		Proto:             resp.Proto,
		ProtoMajor:        resp.ProtoMajor,
		ProtoMinor:        resp.ProtoMinor,
		Headers:           resp.Header,
		Body:              body,
		ContentLength:     resp.ContentLength,
		TransferEncodings: resp.TransferEncoding,
		Uncompressed:      resp.Uncompressed,
		TLS:               resp.TLS != nil,
	}
}

func respRecordToHTTPResponse(rec clientResponseRecord) (*http.Response, error) {
	var bodyReader io.ReadCloser
	if len(rec.Body) > 0 {
		body, err := base64.StdEncoding.DecodeString(rec.Body)
		if err != nil {
			return nil, fmt.Errorf("body: %w", err)
		}
		bodyReader = io.NopCloser(bytes.NewBuffer(body))
	} else {
		bodyReader = http.NoBody
	}

	resp := &http.Response{
		Status:           rec.Status,
		StatusCode:       rec.StatusCode,
		Proto:            rec.Proto,
		ProtoMinor:       rec.ProtoMinor,
		ProtoMajor:       rec.ProtoMajor,
		Header:           rec.Headers,
		Body:             bodyReader,
		ContentLength:    rec.ContentLength,
		TransferEncoding: rec.TransferEncodings,
		Uncompressed:     rec.Uncompressed,
		Trailer:          rec.Trailers,
	}

	if rec.TLS {
		resp.TLS = &tls.ConnectionState{}
	}

	return resp, nil
}
