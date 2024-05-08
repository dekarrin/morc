package suyac

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
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
	ProjDirVar = "::PROJ_DIR::"

	DefaultProjectPath = ".suyac/project.json"
	DefaultSessionPath = ProjDirVar + "/session.json"
	DefaultHistoryPath = ProjDirVar + "/history.json"

	FiletypeProject = "SUYAC/PROJECT"
	FiletypeSession = "SUYAC/SESSION"
	FiletypeHistory = "SUYAC/HISTORY"

	CurFileVersion = 1
)

type Settings struct {
	ProjFile       string        `json:"project_file"`
	HistFile       string        `json:"history_file"`
	SeshFile       string        `json:"session_file"`
	CookieLifetime time.Duration `json:"cookie_lifetime"`
	RecordHistory  bool          `json:"record_history"`
}

// HistoryFSPath returns the file-system compatible path to the history file. If
// s.HistFile contains ProjDirVar, it will be replaced with the directory that
// the project file is in. If s.HistFile is empty, or if s.ProjFile is referred
// to with ProjDirVar and is itself empty, this will return the empty string.
func (s Settings) HistoryFSPath() string {
	if strings.Contains(s.HistFile, ProjDirVar) {
		if s.ProjFile == "" {
			return ""
		}

		projDir := filepath.Dir(s.ProjFile)

		fullDir := strings.ReplaceAll(s.HistFile, ProjDirVar, projDir)
		if fullDir == projDir {
			// if it is ONLY the proj dir, that is not valid. return empty
			// string
			return ""
		}

		return fullDir
	}

	return s.HistFile
}

// SessionFSPath returns the file-system compatible path to the session file. If
// s.SeshFile contains ProjDirVar, it will be replaced with the directory that
// the project file is in. If s.SeshFile is empty, or if s.ProjFile is referred
// to with ProjDirVar and is itself empty, this will return the empty string.
func (s Settings) SessionFSPath() string {
	if strings.Contains(s.SeshFile, ProjDirVar) {
		if s.ProjFile == "" {
			return ""
		}

		projDir := filepath.Dir(s.ProjFile)

		fullDir := strings.ReplaceAll(s.SeshFile, ProjDirVar, projDir)
		if fullDir == projDir {
			// if it is ONLY the proj dir, that is not valid. return empty
			// string
			return ""
		}

		return fullDir
	}

	return s.SeshFile
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
	Filetype  string                     `json:"filetype"`
	Version   int                        `json:"version"`
	Name      string                     `json:"name"`
	Templates map[string]RequestTemplate `json:"templates"`
	Flows     map[string]Flow            `json:"flows"`
	Vars      VarStore                   `json:"vars"`
	Config    Settings                   `json:"config"`
}

func (p Project) PersistHistoryToDisk() error {
	histPath := p.Config.HistoryFSPath()
	if histPath == "" {
		return fmt.Errorf("history file path is not set")
	}

	m := marshaledHistory{
		Filetype: FiletypeHistory,
		Version:  CurFileVersion,
		Entries:  p.History,
	}

	histDataBytes, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("marshal history data: %w", err)
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
	seshPath := p.Config.HistoryFSPath()
	if seshPath == "" {
		return fmt.Errorf("session file path is not set")
	}

	seshDataBytes, err := json.Marshal(p.Session)
	if err != nil {
		return fmt.Errorf("marshal session data: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(seshPath), 0755); err != nil {
		return fmt.Errorf("create dir for session file: %w", err)
	}

	if err := os.WriteFile(seshPath, seshDataBytes, 0644); err != nil {
		return fmt.Errorf("write session file: %w", err)
	}

	return nil
}

// PersistToDisk writes up to 3 files; one for the suite, one for the session,
// and one for the history. If p.ProjFile is empty, it will be written to the
// current working directory at path .suyac/suite.json. If p.SeshFile is
// empty, it will be written to the current working directory at path
// .suyac/session.json. If p.HistFile is empty, it will be written to the
// current working directory at path .suyac/history.json.
func (p Project) PersistToDisk(all bool) error {
	// get data to persist
	m := marshaledProject{
		Filetype:  FiletypeProject,
		Version:   CurFileVersion,
		Name:      p.Name,
		Templates: p.Templates,
		Flows:     p.Flows,
		Vars:      p.Vars,
		Config:    p.Config,
	}

	projDataBytes, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("marshal suite data: %w", err)
	}

	// check file paths and see if they need to be defaulted
	projPath := p.Config.ProjFile
	if projPath == "" {
		projPath = DefaultProjectPath
	}

	// call mkdir -p on the paths
	if err := os.MkdirAll(filepath.Dir(projPath), 0755); err != nil {
		return fmt.Errorf("create dir for suite file: %w", err)
	}

	// write out the data for project, session, and history
	if err := os.WriteFile(projPath, projDataBytes, 0644); err != nil {
		return fmt.Errorf("write suite file: %w", err)
	}

	if all {
		if p.Config.SessionFSPath() != "" {
			if err := p.PersistSessionToDisk(); err != nil {
				return fmt.Errorf("persist session: %w", err)
			}
		}

		if p.Config.HistoryFSPath() != "" {
			if err := p.PersistHistoryToDisk(); err != nil {
				return fmt.Errorf("persist history: %w", err)
			}
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
		if p.Config.SessionFSPath() != "" {
			p.Session, err = LoadSessionFromDisk(p.Config.SessionFSPath())
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				return Project{}, fmt.Errorf("load session: %w", err)
			}
		}

		if p.Config.HistoryFSPath() != "" {
			p.History, err = LoadHistoryFromDisk(p.Config.HistoryFSPath())
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				return Project{}, fmt.Errorf("load history: %w", err)
			}
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

	var m marshaledHistory
	if err := json.Unmarshal(histData, &m); err != nil {
		return nil, fmt.Errorf("unmarshal history data: %w", err)
	}

	return m.Entries, nil
}

type Session struct {
	Cookies []SetCookiesCall
}

// TotalCookieSets returns the total number of individual cookies that this
// Session has a record of being set across all URLs. This may include the same
// cookie being set multiple times.
func (s *Session) TotalCookieSets() int {
	total := 0
	for _, c := range s.Cookies {
		total += len(c.Cookies)
	}
	return total
}

type marshaledSession struct {
	Filetype string   `json:"filetype"`
	Version  int      `json:"version"`
	Cookies  []string `json:"cookies"`
}

func (s Session) MarshalJSON() ([]byte, error) {
	ms := marshaledSession{
		Filetype: FiletypeSession,
		Version:  CurFileVersion,
	}
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
	Captures map[string]VarScraper
	Body     []byte
	URL      string
	Method   string
	Headers  http.Header
	AuthFlow string
}

func (r RequestTemplate) Sendable() bool {
	return r.URL != "" && r.Method != ""
}

// VarStore is a collection of variables that can be accessed by name within
// multiple environments. The zero value of this type is not valid; create a
// new VarStore with NewVarStore().
type VarStore struct {
	Environment string

	envs map[string]map[string]string
}

func NewVarStore() VarStore {
	return VarStore{
		envs: make(map[string]map[string]string),
	}
}

type marshaledVarStore struct {
	Current string                       `json:"current_environment"`
	Envs    map[string]map[string]string `json:"environments"`
}

func (v VarStore) MarshalJSON() ([]byte, error) {
	m := marshaledVarStore{
		Current: v.Environment,
		Envs:    v.envs,
	}

	return json.Marshal(m)
}

func (v *VarStore) UnmarshalJSON(data []byte) error {
	var m marshaledVarStore
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}

	v.Environment = m.Current
	v.envs = m.Envs

	return nil
}

// Count returns the number of variables accessible from the current
// environment. This includes any in the default environment that are not
// overridden by the current environment. This will match the number of elements
// returned by All().
func (v *VarStore) Count() int {
	if v.envs == nil {
		return 0
	}

	env := v.envs[""]
	if env != nil {
		return len(env)
	}

	return 0
}

func (v *VarStore) EnvCount() int {
	if len(v.envs) == 0 {
		return 1 // default env is always considered to exist
	}

	return len(v.envs)
}

func (v *VarStore) EnvNames() []string {
	if v.envs == nil {
		return []string{""}
	}

	var names []string
	for k := range v.envs {
		names = append(names, k)
	}

	if _, ok := v.envs[""]; !ok {
		names = append(names, "") // default env is always considered to exist
	}

	return names
}

func (v *VarStore) IsDefined(key string) bool {
	if v.envs == nil {
		return false
	}

	envUpper := strings.ToUpper(v.Environment)
	env := v.envs[envUpper]
	if env == nil {
		return false
	}

	k := strings.ToUpper(key)
	if _, ok := env[k]; ok {
		return true
	}

	if v.Environment != "" {
		env = v.envs[""]
		if env != nil {
			if _, ok := env[k]; ok {
				return true
			}
		}
	}

	return false
}

func (v *VarStore) IsDefinedIn(key, env string) bool {
	if v.envs == nil {
		return false
	}

	envUpper := strings.ToUpper(env)
	varEnv := v.envs[envUpper]
	if varEnv == nil {
		return false
	}

	k := strings.ToUpper(key)
	if _, ok := varEnv[k]; ok {
		return true
	}

	return false
}

// Defined returns the names of all variables defined in the current
// environment. It does not include any vars that are only defined in the
// default environment.
func (v *VarStore) Defined() []string {
	if v.envs == nil {
		return nil
	}

	envUpper := strings.ToUpper(v.Environment)
	env := v.envs[envUpper]
	if env == nil {
		return nil
	}

	var keys []string
	for k := range env {
		keys = append(keys, k)
	}

	return keys
}

// DefinedIn returns the names of all variables defined in the given
// environment. It does not include any vars that are only defined in the
// default environment unless "" is given as env.
func (v *VarStore) DefinedIn(env string) []string {
	if v.envs == nil {
		return nil
	}

	envUpper := strings.ToUpper(env)
	varEnv := v.envs[envUpper]
	if varEnv == nil {
		return nil
	}

	var keys []string
	for k := range varEnv {
		keys = append(keys, k)
	}

	return keys
}

// All returns the names of all variables defined between the current
// environment and the default environment. If a variable is defined in both
// environments, it will only be included once.
func (v *VarStore) All() []string {
	if v.envs == nil {
		return nil
	}

	seenKeys := map[string]struct{}{}
	keys := []string{}

	envUpper := strings.ToUpper(v.Environment)
	if env, ok := v.envs[envUpper]; ok {
		for k := range env {
			seenKeys[k] = struct{}{}
			keys = append(keys, k)
		}
	}

	// now include any missing from the default env, if we didn't just do that
	if v.Environment != "" {
		if env, ok := v.envs[""]; ok {
			for k := range env {
				if _, ok := seenKeys[k]; !ok {
					keys = append(keys, k)
				}
			}
		}
	}

	return keys
}

func (v *VarStore) Get(key string) string {
	if v.envs == nil {
		v.envs = make(map[string]map[string]string)
	}

	envUpper := strings.ToUpper(v.Environment)
	env := v.envs[envUpper]
	k := strings.ToUpper(key)
	if env != nil {
		if value, ok := env[k]; ok {
			return value
		}
	}

	// for some reason, the var doesn't exist. see if it does in the default env
	if v.Environment != "" {
		env = v.envs[""]
		if env != nil {
			if value, ok := env[k]; ok {
				return value
			}
		}
	}

	// couldn't find it, return empty
	return ""
}

// GetFrom has no fallback to default, unlike Get.
func (v *VarStore) GetFrom(key, env string) string {
	if v.envs == nil {
		v.envs = make(map[string]map[string]string)
	}

	envUpper := strings.ToUpper(env)
	varEnv := v.envs[envUpper]
	k := strings.ToUpper(key)
	if varEnv != nil {
		return varEnv[k]
	}

	// couldn't find it, return empty
	return ""
}

func (v *VarStore) Set(key, value string) {
	if v.envs == nil {
		v.envs = make(map[string]map[string]string)
	}

	envUpper := strings.ToUpper(v.Environment)
	env := v.envs[envUpper]
	if env == nil {
		env = make(map[string]string)
		v.envs[envUpper] = env
	}

	k := strings.ToUpper(key)
	env[k] = value

	// also make shore var exists in default env
	if v.Environment != "" {
		env = v.envs[""]
		if env == nil {
			env = make(map[string]string)
			v.envs[""] = env
		}

		if _, ok := env[k]; !ok {
			env[k] = ""
		}
	}
}

func (v *VarStore) SetIn(key, value, env string) {
	if v.envs == nil {
		v.envs = make(map[string]map[string]string)
	}

	envUpper := strings.ToUpper(env)
	varEnv := v.envs[envUpper]
	if varEnv == nil {
		varEnv = make(map[string]string)
		v.envs[envUpper] = varEnv
	}

	k := strings.ToUpper(key)
	varEnv[k] = value

	// also make shore var exists in default env
	if envUpper != "" {
		defEnv := v.envs[""]
		if defEnv == nil {
			defEnv = make(map[string]string)
			v.envs[""] = defEnv
		}

		if _, ok := defEnv[k]; !ok {
			defEnv[k] = ""
		}
	}
}

// Unset removes the variable from the current environemnt. If the current
// environment is not the default environment, the variable will not be removed
// from the default environment. Use Remove to remove the variable from all
// environments.
//
// If the current environment *is* the default environment, calling this method
// has the same effect as calling Remove, as variables are not allowed to exist
// in only a non-default environment.
func (v *VarStore) Unset(key string) {
	if v.envs == nil {
		return
	}

	if v.Environment == "" {
		v.Remove(key)
		return
	}

	envUpper := strings.ToUpper(v.Environment)
	env := v.envs[envUpper]
	if env != nil {
		k := strings.ToUpper(key)
		delete(env, k)
	}
}

func (v *VarStore) UnsetIn(key, env string) {
	if v.envs == nil {
		return
	}

	if env == "" {
		v.Remove(key)
		return
	}

	envUpper := strings.ToUpper(v.Environment)
	varEnv := v.envs[envUpper]
	if varEnv != nil {
		k := strings.ToUpper(key)
		delete(varEnv, k)
	}
}

// Remove removes the variable from all environments, including the default one.
func (v *VarStore) Remove(key string) {
	if v.envs == nil {
		return
	}

	for _, env := range v.envs {
		if env != nil {
			k := strings.ToUpper(key)
			delete(env, k)
		}
	}
}

type Flow struct {
	Name     string
	Requests []RequestTemplate
}

type marshaledHistory struct {
	Filetype string         `json:"filetype"`
	Version  int            `json:"version"`
	Entries  []HistoryEntry `json:"history"`
}

type HistoryEntry struct {
	Template string
	ReqTime  time.Time
	RespTime time.Time
	Request  *http.Request
	Response *http.Response
	Captures map[string]string
}

type marshaledHistoryEntry struct {
	Template string               `json:"template"`
	ReqTime  int64                `json:"request_time"`
	RespTime int64                `json:"response_time"`
	Request  clientRequestRecord  `json:"request"`
	Response clientResponseRecord `json:"response"`
	Captures map[string]string    `json:"captures,omitempty"`
}

func (h HistoryEntry) MarshalJSON() ([]byte, error) {
	// convert the http.Request and http.Response into marshaledHistoryEntry
	// structs
	reqRec := httpRequestToRecord(h.Request)
	respRec := httpResponseToRecord(h.Response)

	// marshal the marshaledHistoryEntry struct
	m := marshaledHistoryEntry{
		Template: h.Template,
		ReqTime:  h.ReqTime.Unix(),
		RespTime: h.RespTime.Unix(),
		Request:  reqRec,
		Response: respRec,
		Captures: h.Captures,
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
	h.Request = req
	h.Response = resp
	h.Captures = m.Captures

	return nil
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
