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
	ProofLocationHeader ProofLocation = "header"
	ProofLocationCookie ProofLocation = "cookie"
)

func (pl ProofLocation) String() string {
	return string(pl)
}

func ParseProofLocation(s string) (ProofLocation, error) {
	switch strings.ToLower(s) {
	case "header":
		return ProofLocationHeader, nil
	case "cookie":
		return ProofLocationCookie, nil
	default:
		return "", fmt.Errorf("unknown proof location %q", s)
	}
}

type ProofFormatType string

const (
	ProofFormatTypeNone   ProofFormatType = ""
	ProofFormatTypeBearer ProofFormatType = "bearer"
)

func (pf ProofFormatType) String() string {
	return string(pf)
}

func ParseProofFormatType(s string) (ProofFormatType, error) {
	switch strings.ToLower(s) {
	case "":
		return ProofFormatTypeNone, nil
	case "bearer":
		return ProofFormatTypeBearer, nil
	default:
		return "", fmt.Errorf("unknown proof format %q", s)
	}
}

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
	} else if dp.dest.Location == ProofLocationCookie {
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
	AuthTypeNone      AuthType = ""
	AuthTypeHTTPBasic AuthType = "basic"
	AuthTypeSession   AuthType = "session"
	AuthTypeToken     AuthType = "token"
	AuthTypeJWT       AuthType = "jwt"
)

type TransformerFuncName string

const (
	TransformerFuncIdentity               TransformerFuncName = "identity"
	TransformerFuncPairedCookieValue      TransformerFuncName = "paired-cookie-value"
	TransformerFuncPairedCookieExpiration TransformerFuncName = "paired-cookie-expiration"
	TransformerFuncJWTExpiration          TransformerFuncName = "jwt-expiration"
	TransformerFuncParsedTime             TransformerFuncName = "parsed-time"
)

type TransformerFunc func(string) (string, error)
type TimeTransformerFunc func(string) (time.Time, error)

type Transformer struct {
	FuncName TransformerFuncName
	Params   map[string]any

	fn TransformerFunc
}

func (t *Transformer) Apply(s string) (string, error) {
	if t.fn == nil {
		var err error
		t.fn, err = NewTransformerFuncFromParams(t.FuncName, t.Params)
		if err != nil {
			return "", fmt.Errorf("init func: %w", err)
		}
	}

	return t.fn(s)
}

type TimeTransformer struct {
	FuncName TransformerFuncName
	Params   map[string]any

	fn TimeTransformerFunc
}

func (t *TimeTransformer) Apply(s string) (time.Time, error) {
	if t.fn == nil {
		var err error
		t.fn, err = NewTimeTransformerFuncFromParams(t.FuncName, t.Params)
		if err != nil {
			return time.Time{}, fmt.Errorf("init func: %w", err)
		}
	}
	return t.fn(s)
}

func NewTransformerFuncFromParams(name TransformerFuncName, params map[string]any) (TransformerFunc, error) {
	switch name {
	case TransformerFuncIdentity:
		return NewIdentityStringTransformer(), nil
	case TransformerFuncPairedCookieValue:
		return NewPairedCookieValueTransformer(), nil
	case TransformerFuncPairedCookieExpiration:
		return nil, fmt.Errorf("%v is a time-transformer", name)
	case TransformerFuncJWTExpiration:
		return nil, fmt.Errorf("%v is a time-transformer", name)
	case TransformerFuncParsedTime:
		return nil, fmt.Errorf("%v is a time-transformer", name)
	default:
		return nil, fmt.Errorf("unknown transformer %q", name)
	}
}

func NewTimeTransformerFuncFromParams(name TransformerFuncName, params map[string]any) (TimeTransformerFunc, error) {
	switch name {

	case TransformerFuncIdentity:
		return nil, fmt.Errorf("%v is a string-transformer", name)
	case TransformerFuncPairedCookieValue:
		return nil, fmt.Errorf("%v is a string-transformer", name)
	case TransformerFuncPairedCookieExpiration:
		return NewPairedCookieExpiresTransformer(), nil
	case TransformerFuncJWTExpiration:
		return NewJWTExpirationTransformer(), nil
	case TransformerFuncParsedTime:
		var layout string

		if rawLayout, ok := params["layout"]; ok {
			if layout, ok = rawLayout.(string); !ok {
				return nil, errors.New("layout must be a string")
			}
		} else {
			return nil, errors.New("layout missing from params")
		}
		return NewParsedTimeTransformer(layout), nil
	default:
		return nil, fmt.Errorf("unknown transformer %q", name)
	}
}

func NewIdentityStringTransformer() TransformerFunc {
	return func(s string) (string, error) {
		return s, nil
	}
}

func NewParsedTimeTransformer(layout string) TimeTransformerFunc {
	if layout == "" {
		layout = time.RFC3339
	}

	return func(s string) (time.Time, error) {
		return time.Parse(layout, s)
	}
}

func NewPairedCookieValueTransformer() TransformerFunc {
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

func NewPairedCookieExpiresTransformer() TimeTransformerFunc {
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

func NewJWTExpirationTransformer() TimeTransformerFunc {
	return func(s string) (time.Time, error) {
		parts := strings.Split(s, ".")
		if len(parts) != 3 {
			return time.Time{}, fmt.Errorf("not in xxxxx.yyyyy.zzzzz JWT format: %s", s)
		}
		encodedClaims := parts[1]
		claimsStr, err := base64.RawURLEncoding.DecodeString(encodedClaims)
		if err != nil {
			return time.Time{}, fmt.Errorf("failed to decode JWT claims: %w", err)
		}

		type justExp struct {
			Exp int64 `json:"exp"`
		}

		var claims justExp
		if err := json.Unmarshal(claimsStr, &claims); err != nil {
			return time.Time{}, fmt.Errorf("failed to parse JWT claims: %w", err)
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

func (pd ProofDestination) String() string {
	s := fmt.Sprintf("%s %q", pd.Location, pd.Key)

	if pd.Format != ProofFormatTypeNone {
		s += fmt.Sprintf(" (%s)", pd.Format)
	}

	return s
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

		if loc != ProofLocationHeader && loc != ProofLocationCookie {
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
	Name string

	// Type is the type of authentication method that this Auth uses. It mostly
	// is used to refer to the method of creation and for a CLI interface;
	// actual library usage of Type is fairly minimal and an Auth can be created
	// and used without the AuthType explicitly set.
	Type    AuthType
	Proof   AuthProof
	Fetcher *AuthFetcher
}

// IsSuccessfulAuthUse returns whether the given response from an authenticated
// request is considered successful. This is generally always the case unless the
// returned status is 401 Unauthorized.
func (a *Auth) IsSuccessfulAuthUse(resp *http.Response) bool {
	success := resp.StatusCode != http.StatusUnauthorized

	// invalidate proof immediately if auth failed and proof is dynamic
	if !success {
		if a.Proof != nil && a.Proof.Type() == AuthProofCustom {
			a.Proof = nil
		}
	}

	return success
}

func (a Auth) Static() bool {
	return a.Fetcher == nil
}

type marshaledAuth struct {
	Name      string
	Type      AuthType
	Proof     map[string]any
	ProofType AuthProofType `json:",omitempty"`
	Fetcher   *AuthFetcher  `json:",omitempty"`
}

func (a Auth) MarshalJSON() ([]byte, error) {
	ma := marshaledAuth{
		Name: a.Name,
		Type: a.Type,
	}
	if a.Proof != nil {
		ma.Proof = a.Proof.Export()
		ma.ProofType = a.Proof.Type()
	}

	if a.Fetcher != nil {
		ma.Fetcher = a.Fetcher
	}

	return json.Marshal(ma)
}

func (a *Auth) UnmarshalJSON(b []byte) error {
	var ma marshaledAuth
	if err := json.Unmarshal(b, &ma); err != nil {
		return err
	}

	a.Name = ma.Name
	a.Type = ma.Type

	if ma.Proof != nil {
		t, err := ImportAuthProof(ma.ProofType, ma.Proof)
		if err != nil {
			return err
		}
		a.Proof = t
	}

	if ma.Fetcher != nil {
		a.Fetcher = ma.Fetcher
	}
	return nil
}

// NewHTTPBasicAuth is a command that needs defining.
func NewHTTPBasicAuth(name string, creds HTTPBasicCredentials) Auth {
	return Auth{
		Name:  name,
		Type:  AuthTypeHTTPBasic,
		Proof: creds,
	}
}

// NewSessionAuth returns an Auth that is configured to pull a cookie
// containing the session ID from the response of the auth flow/template and use
// it in authorized requests. The cookieName is the name of the cookie to pull.
// Give flow or femplate, but not both. If expiration detection is set, this
// Auth will always attempt to detect expiration info from the initial
// set-cookie, but if it is not present, it will fallback to error response on
// the auth'd request's response to detect expiration.
func NewSessionAuth(name string, retrieval RequestSequence, cookieName string, detectExpiration bool) (Auth, error) {
	fetcher, err := NewSessionCookieFetcher(retrieval, cookieName, detectExpiration)
	if err != nil {
		return Auth{}, err
	}

	return Auth{
		Name:    name,
		Type:    AuthTypeSession,
		Fetcher: fetcher,
	}, nil
}

// NewTokenAuth returns am Auth that is configured to pull a simple token
// using another flow/template. Expiration is optional and is
// extracted via the expiresScraper, if present. If not present, expiration will
// be detected only by auth failure. Give flow or femplate, but not both. Only a
// single token value may be extracted. If expiresTimeLayout is set, it will be
// used for parsing expires time, and if set to an empty string, it will default
// to RFC3339.
func NewTokenAuth(name string, retrieval RequestSequence, tokenScraper Scraper, dest ProofDestination, expiresScraper *Scraper, expiresTimeLayout string) (Auth, error) {
	fetcher, err := NewTokenFetcher(retrieval, tokenScraper, dest, expiresScraper, expiresTimeLayout)
	if err != nil {
		return Auth{}, err
	}

	return Auth{
		Name:    name,
		Type:    AuthTypeToken,
		Fetcher: fetcher,
	}, nil
}

// NewJWTAuth returns an Auth that is configured to pull a JWT token from
// the body of the response of the auth flow/template and place it in an
// Authorization header with the Bearer scheme. The scraper must point to a
// valid JWT token in the response.
func NewJWTAuth(name string, retrieval RequestSequence, scraper Scraper) (Auth, error) {
	fetcher, err := NewJWTFetcher(retrieval, scraper)
	if err != nil {
		return Auth{}, err
	}

	return Auth{
		Name:    name,
		Type:    AuthTypeJWT,
		Fetcher: fetcher,
	}, nil
}

// AuthFetcher is used to get a new AuthProof in an Auth. This is used for
// any Auth mechanism that requires a proof that may change, such as a token or
// a session ID.
type AuthFetcher struct {
	Seq RequestSequence

	Caps []Scraper

	Value   ScrapeExtractor
	Expires ScrapeTimeExtractor

	Dest ProofDestination
}

// NewJWTFetcher returns an AuthFetcher that is configured to pull a JWT token from
// the body of the response of the auth flow/template and place it in an
// Authorization header with the Bearer scheme. The scraper must point to a
// valid JWT token in the response body. Give flow or femplate, but not
// both.
func NewJWTFetcher(seq RequestSequence, scraper Scraper) (*AuthFetcher, error) {
	if seq.Name == "" {
		return &AuthFetcher{}, errors.New("flow/template name must be set")
	}

	// TODO: validate flow, template actually exist in caller.

	return &AuthFetcher{
		Seq:  seq,
		Caps: []Scraper{scraper},
		Value: ScrapeExtractor{
			VarName: scraper.Name,
			Transform: Transformer{
				FuncName: TransformerFuncIdentity,
			},
		},
		Expires: ScrapeTimeExtractor{
			VarName: scraper.Name,
			Transform: TimeTransformer{
				FuncName: TransformerFuncJWTExpiration,
			},
		},
		Dest: ProofDestination{
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
func NewTokenFetcher(seq RequestSequence, tokenScraper Scraper, dest ProofDestination, expiresScraper *Scraper, expiresTimeLayout string) (*AuthFetcher, error) {
	if seq.Name == "" {
		return &AuthFetcher{}, errors.New("flow/template name must be set")
	}

	// TODO: validate flow, template actually exist in caller.

	da := &AuthFetcher{
		Seq:  seq,
		Caps: []Scraper{tokenScraper},
		Value: ScrapeExtractor{
			VarName: tokenScraper.Name,
			Transform: Transformer{
				FuncName: TransformerFuncIdentity,
			},
		},
		Dest: dest,
	}

	if expiresScraper != nil {
		da.Expires = ScrapeTimeExtractor{
			VarName: expiresScraper.Name,
			Transform: TimeTransformer{
				FuncName: TransformerFuncParsedTime,
				Params:   map[string]any{"layout": expiresTimeLayout},
			},
		}
		da.Caps = append(da.Caps, *expiresScraper)
	}

	return da, nil
}

// NewSessionCookieFetcher returns an AuthFetcher that is configured to pull a cookie
// containing the session ID from the response of the auth flow/template and use
// it in authorized requests. The cookieName is the name of the cookie to pull.
// Give flow or femplate, but not both. If expiration detection is set, this
// Auth will always attempt to detect expiration info from the initial
// set-cookie, but if it is not present, it will fallback to error response on
// the auth'd request's response to detect expiration.
func NewSessionCookieFetcher(seq RequestSequence, cookieName string, detectExpiration bool) (*AuthFetcher, error) {
	if seq.Name == "" {
		return &AuthFetcher{}, errors.New("flow/template name must be set")
	}

	const (
		cookieVarName = "session-cookie"
	)

	da := &AuthFetcher{
		Seq: seq,
		Caps: []Scraper{
			{
				Type:             SpecCookie,
				Name:             cookieVarName,
				CookieName:       cookieName,
				CookieExpiration: detectExpiration,
			},
		},
		Value: ScrapeExtractor{
			VarName: cookieVarName,
			Transform: Transformer{
				FuncName: TransformerFuncIdentity,
			},
		},
		Dest: ProofDestination{
			Location: ProofLocationCookie,
			Key:      cookieName,
			Format:   ProofFormatTypeNone,
		},
	}

	if detectExpiration {
		da.Expires = ScrapeTimeExtractor{
			VarName: cookieVarName,
			Transform: TimeTransformer{
				FuncName: TransformerFuncPairedCookieExpiration,
			},
		}
		da.Value.Transform = Transformer{
			FuncName: TransformerFuncPairedCookieValue,
		}
	}

	return da, nil
}

func (da *AuthFetcher) ScrapeFromResult(r SendResult) (AuthProof, error) {
	var err error

	// make sure the response is not an error one
	if r.Response.StatusCode >= 400 {
		return nil, fmt.Errorf("auth request failed with status %d", r.Response.StatusCode)
	}

	// pull out the values
	scrapes := map[string]string{}
	for _, scraper := range da.Caps {
		var err error
		scrapes[scraper.Name], err = scraper.Scrape(r.Response, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to scrape %q: %w", scraper.Name, err)
		}
	}

	valueEx := da.Value
	value, ok := scrapes[valueEx.VarName]
	if !ok {
		return nil, fmt.Errorf("value scrape %q not found", valueEx.VarName)
	}
	value, err = valueEx.Transform.Apply(value)
	if err != nil {
		return nil, fmt.Errorf("failed to transform value %q: %w", valueEx.VarName, err)
	}

	ap := dynamicProof{
		value: value,
		dest:  da.Dest,
	}

	// okay, do we have an expiration?
	if da.Expires.VarName != "" {
		expiresEx := da.Expires

		expStr, ok := scrapes[expiresEx.VarName]
		if !ok {
			return nil, fmt.Errorf("expiration scrape %q not found", expiresEx.VarName)
		}
		expTime, err := expiresEx.Transform.Apply(expStr)
		if err != nil {
			return nil, fmt.Errorf("failed to transform expiration %q: %w", expiresEx.VarName, err)
		}
		ap.expiresAt = expTime
	}

	return ap, nil
}
