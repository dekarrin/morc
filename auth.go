package morc

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type AuthProofType string

const (
	AuthProofHTTPBasic AuthProofType = "http-basic"
	AuthProofCustom    AuthProofType = "custom"
)

type AuthProof interface {
	Apply(req *http.Request) error

	// Valid returns whether the AuthProof is still valid.
	Valid() bool

	// Export returns a JSON-encodable map that can be used to recreate this
	// AuthProof.
	Export() map[string]any

	// Type returns the type of AuthProof that this is. It is used for selecting
	// the correct constructor to recreate an AuthProof from an Exported string.
	Type() AuthProofType
}

type HTTPBasicCredentials struct {
	username string
	password string
}

func (b HTTPBasicCredentials) Apply(req *http.Request) error {
	req.SetBasicAuth(b.username, b.password)
	return nil
}

func (b HTTPBasicCredentials) Valid() bool {
	return true
}

func (b HTTPBasicCredentials) Export() map[string]any {
	return map[string]any{
		"username": b.username,
		"password": b.password,
	}
}

func (b HTTPBasicCredentials) Type() AuthProofType {
	return AuthProofHTTPBasic
}

func NewHTTPBasicCredentials(username, password string) AuthProof {
	return HTTPBasicCredentials{
		username: username,
		password: password,
	}
}

type ProofLocation string

const (
	ProofLocationHeader    ProofLocation = "header"
	ProofDestinationCookie ProofLocation = "cookie"
)

type ProofFormatType string

const (
	ProofFormatTypeNone   ProofFormatType = ""
	ProofFormatTypeBearer ProofFormatType = "bearer"
)

type dynamicProof struct {
	value     string
	dest      ProofDestination
	expiresAt time.Time
}

func (dp dynamicProof) Apply(req *http.Request) error {
	value := dp.value
	if dp.dest.Format == ProofFormatTypeBearer {
		value = "Bearer " + value
	}

	if dp.dest.Location == ProofLocationHeader {
		req.Header.Set(dp.dest.Key, value)
	} else if dp.dest.Location == ProofDestinationCookie {
		req.AddCookie(&http.Cookie{
			Name:  dp.dest.Key,
			Value: value,
		})
	} else {
		return errors.New("unknown destination")
	}

	return nil
}

func (dp dynamicProof) Valid() bool {
	if dp.expiresAt.IsZero() {
		return true
	}

	return time.Now().Before(dp.expiresAt)
}

func (dp dynamicProof) Export() map[string]any {
	return map[string]any{
		"expires_at": dp.expiresAt.Format(time.RFC3339),
		"value":      dp.value,
		"dest":       dp.dest.Export(),
	}
}

// Type returns the type of AuthProof that this is. It is used for selecting
// the correct constructor to recreate an AuthProof from an Exported string.
func (dp dynamicProof) Type() AuthProofType {
	return AuthProofCustom
}

func ImportDynamicProof(exported map[string]any) (AuthProof, error) {
	var expiresAt time.Time
	var value string
	var dest ProofDestination

	// ensure expected properties and types are present
	if rawValue, ok := exported["value"]; ok {
		if value, ok = rawValue.(string); !ok {
			return nil, errors.New("value must be a string")
		}
	} else {
		return nil, errors.New("missing value")
	}

	if rawDest, ok := exported["dest"]; ok {
		if destMap, ok := rawDest.(map[string]any); ok {
			var err error
			dest, err = ImportProofDest(destMap)
			if err != nil {
				return nil, fmt.Errorf("dest: %w", err)
			}
		} else {
			return nil, errors.New("dest must be an object")
		}
	} else {
		return nil, errors.New("missing dest")
	}

	if rawExp, ok := exported["expires_at"]; ok {
		var expStr string
		if expStr, ok = rawExp.(string); !ok {
			return nil, errors.New("expires_at must be a string containing an RFC-3339 date")
		}
		var err error
		if expiresAt, err = time.Parse(time.RFC3339, expStr); err != nil {
			return nil, fmt.Errorf("expires_at: %w", err)
		}
	}

	return dynamicProof{
		expiresAt: expiresAt,
		value:     value,
		dest:      dest,
	}, nil
}

func ImportHTTPBasicCreds(exported map[string]any) (AuthProof, error) {
	var username string
	var password string

	// ensure expected properties and types are present
	if rawUser, ok := exported["username"]; ok {
		if username, ok = rawUser.(string); !ok {
			return nil, errors.New("username must be a string")
		}
	} else {
		return nil, errors.New("missing username")
	}

	if rawPass, ok := exported["password"]; ok {
		if password, ok = rawPass.(string); !ok {
			return nil, errors.New("password must be a string")
		}
	} else {
		return nil, errors.New("missing password")
	}

	return HTTPBasicCredentials{
		username: username,
		password: password,
	}, nil
}

func ImportAuthProof(t AuthProofType, exported map[string]any) (AuthProof, error) {
	switch t {
	case AuthProofHTTPBasic:
		return ImportHTTPBasicCreds(exported)
	case AuthProofCustom:
		return ImportDynamicProof(exported)
	default:
		return nil, errors.New("unknown auth proof type")
	}
}

type AuthType string

const (
	AuthTypeHTTPBasic AuthType = "basic"
)

type Transformer func(string) (string, error)
type TimeTransformer func(string) (time.Time, error)

func NewIdentityStringTransformer() Transformer {
	return func(s string) (string, error) {
		return s, nil
	}
}

func NewParsedTimeTransformer(layout string) TimeTransformer {
	if layout == "" {
		layout = time.RFC3339
	}

	return func(s string) (time.Time, error) {
		return time.Parse(layout, s)
	}
}

func NewPairedCookieValueTransformer() Transformer {
	return func(s string) (string, error) {
		// for pulling value from string extracted by a CookieScraper that
		// returns both an expiration and a value. Attempts to detect if no
		// expiration was found, but might result in imperfect results if value
		// happens to contain the exact string used to delimit expiration. If no
		// expiration is detected, this will have the same result as the
		// identity transform.

		idx := strings.LastIndex(s, CookieScraperExpiresDelimiter)
		if idx > -1 {
			return s[:idx], nil
		}
		return s, nil
	}
}

func NewPairedCookieExpiresTransformer() TimeTransformer {
	return func(s string) (time.Time, error) {
		// for pulling expiration from string extracted by a CookieScraper that
		// returns both an expiration and a value. Attempts to detect if no
		// expiration was found; if one is not present or the value is empty,
		// the zero time is returned.

		idx := strings.LastIndex(s, CookieScraperExpiresDelimiter)
		if idx > -1 {
			expStr := s[idx+len(CookieScraperExpiresDelimiter):]
			if expStr == "" {
				return time.Time{}, nil
			}

			return time.Parse(time.RFC3339, expStr)
		}
		return time.Time{}, nil
	}
}

func NewJWTExpirationTransformer() TimeTransformer {
	return func(s string) (time.Time, error) {
		parts := strings.Split(s, ".")
		if len(parts) != 3 {
			return time.Time{}, fmt.Errorf("Not in xxxxx.yyyyy.zzzzz JWT format: %s", s)
		}
		encodedClaims := parts[1]
		claimsStr, err := base64.RawURLEncoding.DecodeString(encodedClaims)
		if err != nil {
			return time.Time{}, fmt.Errorf("Failed to decode JWT claims: %w", err)
		}

		type justExp struct {
			Exp int64 `json:"exp"`
		}

		var claims justExp
		if err := json.Unmarshal(claimsStr, &claims); err != nil {
			return time.Time{}, fmt.Errorf("Failed to parse JWT claims: %w", err)
		}

		if claims.Exp == 0 {
			return time.Time{}, nil
		}

		return time.Unix(claims.Exp, 0), nil
	}
}

// TODO: CLI transformer.

type ProofDestination struct {
	Location ProofLocation   `json:"location"`
	Key      string          `json:"key"`
	Format   ProofFormatType `json:"format,omitempty"`
}

func (pd ProofDestination) Export() map[string]any {
	m := map[string]any{
		"location": string(pd.Location),
		"key":      pd.Key,
	}

	if pd.Format != ProofFormatTypeNone {
		m["format"] = string(pd.Format)
	}

	return m
}

func ImportProofDest(exported map[string]any) (ProofDestination, error) {
	var loc ProofLocation
	var key string
	var destFormat ProofFormatType = ProofFormatTypeNone

	if rawType, ok := exported["location"]; ok {
		if destStr, ok := rawType.(string); ok {
			loc = ProofLocation(destStr)
		} else {
			return ProofDestination{}, errors.New("location: must be a string")
		}

		if loc != ProofLocationHeader && loc != ProofDestinationCookie {
			return ProofDestination{}, errors.New("location: must be 'header' or 'cookie'")
		}
	} else {
		return ProofDestination{}, errors.New("location: must be present")
	}

	if rawKey, ok := exported["key"]; ok {
		if key, ok = rawKey.(string); !ok {
			return ProofDestination{}, errors.New("key: must be a string")
		}
	} else {
		return ProofDestination{}, errors.New("key: must be present")
	}

	if rawFormat, ok := exported["format"]; ok {
		var destFormatStr string
		if destFormatStr, ok = rawFormat.(string); !ok {
			return ProofDestination{}, errors.New("format: must be a string")
		}
		destFormat = ProofFormatType(destFormatStr)
		if destFormat != ProofFormatTypeNone && destFormat != ProofFormatTypeBearer {
			return ProofDestination{}, errors.New("format: must be set to empty string or 'bearer'")
		}
	}

	return ProofDestination{
		Location: loc,
		Key:      key,
		Format:   destFormat,
	}, nil
}

// ScrapeExtractor refers to a value from a Scraper and translates it to its
// end string value using a Transformer.
type ScrapeExtractor struct {
	VarName   string
	Transform Transformer
}

// ScrapeTimeExtractor refers to a value from a Scraper and translates it to its
// end time.Time value using a TimeTransformer.
type ScrapeTimeExtractor struct {
	VarName   string
	Transform TimeTransformer
}

type Auth struct {
	proof AuthProof

	fetcher *AuthFetcher
}

func (a *Auth) GetAuth(p *Project, skipVerify bool, httpClient *http.Client, oc OutputControl) (AuthProof, error) {
	if a.Static() {
		return a.proof, nil
	}

	if a.proof == nil || !a.proof.Valid() {
		if a.fetcher == nil {
			return nil, errors.New("no static credentials or fetcher configured")
		}

		proof, err := a.fetcher.Fetch(p, skipVerify, httpClient, oc)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch auth proof: %w", err)
		}

		a.proof = proof
	}

	return a.proof, nil
}

func (a *Auth) IsSuccessfulAuth(resp *http.Response) bool {
	success := resp.StatusCode != http.StatusUnauthorized

	// invalidate proof immediately if auth failed
	if !success {
		a.proof = nil
	}

	return success
}

func (a *Auth) Static() bool {
	return a.fetcher == nil
}

func (a *Auth) Export() map[string]any {
	m := map[string]any{}
	if a.proof != nil {
		m["proof"] = a.proof.Export()
	}
	if a.fetcher != nil {

		reqType := "flow"
		reqName := a.fetcher.getAuthFlow
		if a.fetcher.getAuthTemplate != "" {
			reqType = "template"
			reqName = a.fetcher.getAuthTemplate
		}

		m["fetcher"] = map[string]any{
			"request": map[string]any{
				"type": reqType,
				"name": reqName,
			},
		}
	}

	return m
}

type RequestRef struct {
	Name   string
	IsFlow bool
}

func NewHTTPBasicAuth(creds HTTPBasicCredentials) Auth {
	return Auth{
		proof: creds,
	}
}

// NewLoginCookieAuth returns an Auth that is configured to pull a cookie
// containing the session ID from the response of the auth flow/template and use
// it in authorized requests. The cookieName is the name of the cookie to pull.
// Give flow or femplate, but not both. If expiration detection is set, this
// Auth will always attempt to detect expiration info from the initial
// set-cookie, but if it is not present, it will fallback to error response on
// the auth'd request's response to detect expiration.
func NewLoginCookieAuth(retrieval RequestRef, cookieName string, detectExpiration bool) (Auth, error) {
	var flow, femplate string

	if retrieval.IsFlow {
		flow = retrieval.Name
	} else {
		femplate = retrieval.Name
	}

	fetcher, err := NewLoginCookieFetcher(flow, femplate, cookieName, detectExpiration)
	if err != nil {
		return Auth{}, err
	}

	return Auth{
		fetcher: fetcher,
	}, nil
}

// NewTokenAuth returns am Auth that is configured to pull a simple token
// using another flow/template. Expiration is optional and is
// extracted via the expiresScraper, if present. If not present, expiration will
// be detected only by auth failure. Give flow or femplate, but not both. Only a
// single token value may be extracted. If expiresTimeLayout is set, it will be
// used for parsing expires time, and if set to an empty string, it will default
// to RFC3339.
func NewTokenAuth(retrieval RequestRef, tokenScraper Scraper, dest ProofDestination, expiresScraper *Scraper, expiresTimeLayout string) (Auth, error) {
	var flow, femplate string

	if retrieval.IsFlow {
		flow = retrieval.Name
	} else {
		femplate = retrieval.Name
	}

	fetcher, err := NewTokenFetcher(flow, femplate, tokenScraper, dest, expiresScraper, expiresTimeLayout)
	if err != nil {
		return Auth{}, err
	}

	return Auth{
		fetcher: fetcher,
	}, nil
}

// NewJWTAuth returns an Auth that is configured to pull a JWT token from
// the body of the response of the auth flow/template and place it in an
// Authorization header with the Bearer scheme. The scraper must point to a
// valid JWT token in the response.
func NewJWTAuth(retrieval RequestRef, scraper Scraper) (Auth, error) {
	var flow, femplate string

	if retrieval.IsFlow {
		flow = retrieval.Name
	} else {
		femplate = retrieval.Name
	}

	fetcher, err := NewJWTFetcher(flow, femplate, scraper)
	if err != nil {
		return Auth{}, err
	}

	return Auth{
		fetcher: fetcher,
	}, nil
}

// AuthFetcher is used to get a new AuthProof in an Auth. This is used for
// any Auth mechanism that requires a proof that may change, such as a token or
// a session ID.
type AuthFetcher struct {
	getAuthFlow     string // either execFlow or execTemplate must be set
	getAuthTemplate string

	caps []Scraper

	valueExtractor   ScrapeExtractor
	expiresExtractor ScrapeTimeExtractor

	dest ProofDestination
}

// NewJWTFetcher returns an AuthFetcher that is configured to pull a JWT token from
// the body of the response of the auth flow/template and place it in an
// Authorization header with the Bearer scheme. The scraper must point to a
// valid JWT token in the response body. Give flow or femplate, but not
// both.
func NewJWTFetcher(flow, femplate string, scraper Scraper) (*AuthFetcher, error) {
	if flow != "" && femplate != "" {
		return &AuthFetcher{}, errors.New("flow and template cannot both be set")
	}

	if flow == "" && femplate == "" {
		return &AuthFetcher{}, errors.New("either flow or template must be set")
	}

	// TODO: validate flow, template actually exist in caller.

	return &AuthFetcher{
		getAuthFlow:     flow,
		getAuthTemplate: femplate,
		caps:            []Scraper{scraper},
		valueExtractor: ScrapeExtractor{
			VarName:   scraper.Name,
			Transform: NewIdentityStringTransformer(),
		},
		expiresExtractor: ScrapeTimeExtractor{
			VarName:   scraper.Name,
			Transform: NewJWTExpirationTransformer(),
		},
		dest: ProofDestination{
			Location: ProofLocationHeader,
			Key:      "Authorization",
			Format:   ProofFormatTypeBearer,
		},
	}, nil
}

// NewTokenFetcher returns an AuthFetcher that is configured to pull a simple token
// using another flow/template. Expiration is optional and is
// extracted via the expiresScraper, if present. If not present, expiration will
// be detected only by auth failure. Give flow or femplate, but not both. Only a
// single token value may be extracted. If expiresTimeLayout is set, it will be
// used for parsing expires time, and if set to an empty string, it will default
// to RFC3339.
func NewTokenFetcher(flow, femplate string, tokenScraper Scraper, dest ProofDestination, expiresScraper *Scraper, expiresTimeLayout string) (*AuthFetcher, error) {
	if flow != "" && femplate != "" {
		return &AuthFetcher{}, errors.New("flow and template cannot both be set")
	}

	if flow == "" && femplate == "" {
		return &AuthFetcher{}, errors.New("either flow or template must be set")
	}

	// TODO: validate flow, template actually exist in caller.

	da := &AuthFetcher{
		getAuthFlow:     flow,
		getAuthTemplate: femplate,
		caps:            []Scraper{tokenScraper},
		valueExtractor: ScrapeExtractor{
			VarName:   tokenScraper.Name,
			Transform: NewIdentityStringTransformer(),
		},
		dest: dest,
	}

	if expiresScraper != nil {
		da.expiresExtractor = ScrapeTimeExtractor{
			VarName:   expiresScraper.Name,
			Transform: NewParsedTimeTransformer(expiresTimeLayout),
		}
		da.caps = append(da.caps, *expiresScraper)
	}

	return da, nil
}

// NewLoginCookieFetcher returns an AuthFetcher that is configured to pull a cookie
// containing the session ID from the response of the auth flow/template and use
// it in authorized requests. The cookieName is the name of the cookie to pull.
// Give flow or femplate, but not both. If expiration detection is set, this
// Auth will always attempt to detect expiration info from the initial
// set-cookie, but if it is not present, it will fallback to error response on
// the auth'd request's response to detect expiration.
func NewLoginCookieFetcher(flow, femplate string, cookieName string, detectExpiration bool) (*AuthFetcher, error) {
	if flow != "" && femplate != "" {
		return nil, errors.New("flow and template cannot both be set")
	}

	if flow == "" && femplate == "" {
		return nil, errors.New("either flow or template must be set")
	}

	const (
		cookieVarName = "session-cookie"
	)

	da := &AuthFetcher{
		getAuthFlow:     flow,
		getAuthTemplate: femplate,
		caps: []Scraper{
			Scraper{
				Type:             SpecCookie,
				Name:             cookieVarName,
				CookieName:       cookieName,
				CookieExpiration: detectExpiration,
			},
		},
		valueExtractor: ScrapeExtractor{
			VarName:   cookieVarName,
			Transform: NewIdentityStringTransformer(),
		},
		dest: ProofDestination{
			Location: ProofDestinationCookie,
			Key:      cookieName,
			Format:   ProofFormatTypeNone,
		},
	}

	if detectExpiration {
		da.expiresExtractor = ScrapeTimeExtractor{
			VarName:   cookieVarName,
			Transform: NewPairedCookieExpiresTransformer(),
		}
		da.valueExtractor.Transform = NewPairedCookieValueTransformer()
	}

	return da, nil
}

func (da *AuthFetcher) Fetch(p *Project, skipVerify bool, httpClient *http.Client, oc OutputControl) (AuthProof, error) {
	var lastResult SendResult
	var err error
	if da.getAuthFlow != "" {
		// TODO: pre-examine flow to ensure it doesn't itself end up calling
		// itself recursively.

		res, err := p.Exec(da.getAuthFlow, nil, skipVerify, "", httpClient, oc)
		if err != nil {
			return nil, fmt.Errorf("auth flow %q failed: %w", da.getAuthFlow, err)
		}
		if len(res) == 0 {
			return nil, fmt.Errorf("auth flow %q did not return any results", da.getAuthFlow)
		}
		lastResult = res[0]
	} else if da.getAuthTemplate != "" {
		lastResult, err = p.Send(da.getAuthTemplate, nil, skipVerify, "", httpClient, oc)
		if err != nil {
			return nil, fmt.Errorf("auth request template %q failed: %w", da.getAuthTemplate, err)
		}
	}

	// make sure the response is not an error one
	if lastResult.Response.StatusCode >= 400 {
		return nil, fmt.Errorf("auth request failed with status %d", lastResult.Response.StatusCode)
	}

	// pull out the values
	scrapes := map[string]string{}
	for _, scraper := range da.caps {
		var err error
		scrapes[scraper.Name], err = scraper.Scrape(lastResult.Response, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to scrape %q: %w", scraper.Name, err)
		}
	}

	valueEx := da.valueExtractor
	value, ok := scrapes[valueEx.VarName]
	if !ok {
		return nil, fmt.Errorf("value scrape %q not found", valueEx.VarName)
	}
	value, err = valueEx.Transform(value)
	if err != nil {
		return nil, fmt.Errorf("failed to transform value %q: %w", valueEx.VarName, err)
	}

	ap := dynamicProof{
		value: value,
		dest:  da.dest,
	}

	// okay, do we have an expiration?
	if da.expiresExtractor.VarName != "" {
		expiresEx := da.expiresExtractor

		expStr, ok := scrapes[expiresEx.VarName]
		if !ok {
			return nil, fmt.Errorf("expiration scrape %q not found", expiresEx.VarName)
		}
		expTime, err := expiresEx.Transform(expStr)
		if err != nil {
			return nil, fmt.Errorf("failed to transform expiration %q: %w", expiresEx.VarName, err)
		}
		ap.expiresAt = expTime
	}

	return ap, nil

	// TODO: fallback needs to be implemented at some level to decide that a
	// previously valid proof is not valid and could be re-obtained, but that's
	// probably going to need to be at caller-level.
}
