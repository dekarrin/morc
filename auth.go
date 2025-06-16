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

	// Secret returns the secret value of the AuthProof.
	Secret() string

	// Expiration returns the expiration time of the AuthProof. If it doesn't
	// have one, the zero time will be returned.
	Expiration() time.Time
}

type HTTPBasicCredentials struct {
	Username string
	Password string
}

func (b HTTPBasicCredentials) Apply(req *http.Request) error {
	req.SetBasicAuth(b.Username, b.Password)
	return nil
}

func (b HTTPBasicCredentials) Valid() bool {
	return true
}

func (b HTTPBasicCredentials) Export() map[string]any {
	return map[string]any{
		"username": b.Username,
		"password": b.Password,
	}
}

func (b HTTPBasicCredentials) Type() AuthProofType {
	return AuthProofHTTPBasic
}

func (b HTTPBasicCredentials) Secret() string {
	return fmt.Sprintf("%s:%s", b.Username, b.Password)
}

func (b HTTPBasicCredentials) Expiration() time.Time {
	return time.Time{}
}

func NewHTTPBasicCredentials(username, password string) AuthProof {
	return HTTPBasicCredentials{
		Username: username,
		Password: password,
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

type ProofFormat string

const (
	ProofFormatTypeNone   ProofFormat = ""
	ProofFormatTypeBearer ProofFormat = "bearer"
	ProofFormatTypeBasic  ProofFormat = "basic"
)

func (pf ProofFormat) String() string {
	return string(pf)
}

func ParseProofFormat(s string) (ProofFormat, error) {
	switch strings.ToLower(s) {
	case "":
		return ProofFormatTypeNone, nil
	case "bearer":
		return ProofFormatTypeBearer, nil
	case "basic":
		return ProofFormatTypeBasic, nil
	default:
		return "", fmt.Errorf("unknown proof format %q", s)
	}
}

type DynamicProof struct {
	Value     string
	Dest      ProofDestination
	ExpiresAt time.Time
}

func (dp DynamicProof) Secret() string {
	return dp.Value
}

func (dp DynamicProof) Expiration() time.Time {
	return dp.ExpiresAt
}

func (dp DynamicProof) Apply(req *http.Request) error {
	value := dp.Value
	if dp.Dest.Format == ProofFormatTypeBearer {
		value = "Bearer " + value
	}

	if dp.Dest.Location == ProofLocationHeader {
		req.Header.Set(dp.Dest.Key, value)
	} else if dp.Dest.Location == ProofLocationCookie {
		req.AddCookie(&http.Cookie{
			Name:  dp.Dest.Key,
			Value: value,
		})
	} else {
		return errors.New("unknown destination")
	}

	return nil
}

func (dp DynamicProof) Valid() bool {
	if dp.ExpiresAt.IsZero() {
		return true
	}

	return time.Now().Before(dp.ExpiresAt)
}

func (dp DynamicProof) Export() map[string]any {
	return map[string]any{
		"expires_at": dp.ExpiresAt.Format(time.RFC3339),
		"value":      dp.Value,
		"dest":       dp.Dest.Export(),
	}
}

// Type returns the type of AuthProof that this is. It is used for selecting
// the correct constructor to recreate an AuthProof from an Exported string.
func (dp DynamicProof) Type() AuthProofType {
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

	return DynamicProof{
		ExpiresAt: expiresAt,
		Value:     value,
		Dest:      dest,
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
		Username: username,
		Password: password,
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

var AuthTypes = []AuthType{
	AuthTypeNone,
	AuthTypeHTTPBasic,
	AuthTypeSession,
	AuthTypeToken,
	AuthTypeJWT,
}

func ParseAuthType(s string) (AuthType, error) {
	lower := strings.ToLower(s)
	for _, t := range AuthTypes {
		if strings.ToLower(string(t)) == lower {
			return t, nil
		}
	}
	return "", fmt.Errorf("unknown auth type %q", s)
}

func (at AuthType) String() string {
	return string(at)
}

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

		return time.Unix(claims.Exp, 0).UTC(), nil
	}
}

// TODO: CLI transformer.

type ProofDestination struct {
	Location ProofLocation `json:"location"`
	Key      string        `json:"key"`
	Format   ProofFormat   `json:"format,omitempty"`
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
	var destFormat ProofFormat = ProofFormatTypeNone

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
		destFormat = ProofFormat(destFormatStr)
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

// SecretExpiration returns the current expiration of the secret in the Auth's
// auth proof. If the Auth does not currently have an AuthProof set, the zero
// time will be returned.
func (a Auth) SecretExpiration() time.Time {
	if a.Proof == nil {
		return time.Time{}
	}

	return a.Proof.Expiration()
}

// Secret returns the current value of an AuthProof. If the Auth does not
// currently have one set, for instance due to being Dynamic type that hasn't
// yet requested one, an empty string is returned.
func (a Auth) Secret() string {
	if a.Proof == nil {
		return ""
	}

	return a.Proof.Secret()
}

// Password returns the currently-configured password for the Auth. If the Auth
// is not of type HTTPBasic, an empty string will be returned. If the Auth is
// HTTPBasic but the proof is nil, an empty string will be returned.
func (a Auth) Password() string {
	if a.Type != AuthTypeHTTPBasic {
		return ""
	}

	if a.Proof == nil {
		return ""
	}

	if creds, ok := a.Proof.(HTTPBasicCredentials); ok {
		return creds.Password
	} else {
		panic("auth proof is not HTTPBasicCredentials; should never happen")
	}
}

// SetPassword is a helper function that sets the password for an Auth. If the
// Auth is not of type HTTPBasic, an error will be returned. If the Auth is
// HTTPBasic but the proof is nil, an error will be returned.
func (a *Auth) SetPassword(s string) error {
	if a.Type != AuthTypeHTTPBasic {
		return fmt.Errorf("cannot set password on non-HTTPBasic auth")
	}

	if a.Proof == nil {
		a.Proof = NewHTTPBasicCredentials("", s)
		return nil
	}

	if creds, ok := a.Proof.(HTTPBasicCredentials); ok {
		creds.Password = s
		a.Proof = creds
	} else {
		panic("auth proof is not HTTPBasicCredentials; should never happen")
	}

	return nil
}

// Username returns the currently-configured username for the Auth. If the Auth
// is not of type HTTPBasic, an empty string will be returned. If the Auth is
// HTTPBasic but the proof is nil, an empty string will be returned.
func (a Auth) Username() string {
	if a.Type != AuthTypeHTTPBasic {
		return ""
	}

	if a.Proof == nil {
		return ""
	}

	if creds, ok := a.Proof.(HTTPBasicCredentials); ok {
		return creds.Username
	} else {
		panic("auth proof is not HTTPBasicCredentials; should never happen")
	}
}

// SetUsername is a helper function that sets the username for an Auth. If the
// Auth is not of type HTTPBasic, an error will be returned. If the Auth is
// HTTPBasic but the proof is nil, an error will be returned.
func (a *Auth) SetUsername(s string) error {
	if a.Type != AuthTypeHTTPBasic {
		return fmt.Errorf("cannot set username on non-HTTPBasic auth")
	}

	if a.Proof == nil {
		a.Proof = NewHTTPBasicCredentials(s, "")
		return nil
	}

	if creds, ok := a.Proof.(HTTPBasicCredentials); ok {
		creds.Username = s
		a.Proof = creds
	} else {
		panic("auth proof is not HTTPBasicCredentials; should never happen")
	}

	return nil
}

// ValueScraper returns the scraper used to extract the value of an AuthProof in
// authentication requests. If the Auth is static, an empty Scraper will be
// returned.
func (a Auth) ValueScraper() Scraper {
	if a.Type == AuthTypeHTTPBasic || a.Type == AuthTypeNone {
		return Scraper{}
	}

	if a.Fetcher == nil {
		return Scraper{}
	}

	valVarName := a.Fetcher.Value.VarName
	if valVarName == "" {
		return Scraper{}
	}

	// find the scraper for the value var
	for _, scraper := range a.Fetcher.Caps {
		if scraper.Name == valVarName {
			return scraper
		}
	}

	panic(fmt.Sprintf("scraper for value var %q not found; should never happen", valVarName))
}

func (a *Auth) SetValueScraper(scraper Scraper) error {
	if a.Type == AuthTypeNone {
		return fmt.Errorf("cannot set value scraper on auth with no type set")
	}
	if a.Type == AuthTypeHTTPBasic {
		return fmt.Errorf("cannot set value scraper on HTTP Basic auth")
	}

	if a.Fetcher == nil {
		a.Fetcher = newEmptyFetcher(a.Type)
	}

	valVarName := a.Fetcher.Value.VarName
	if valVarName == "" {
		panic("value var name not set; should never happen")
	}

	// find the scraper for the value var and exclude it in a copied list
	newCaps := make([]Scraper, 0, len(a.Fetcher.Caps))
	for _, sc := range a.Fetcher.Caps {
		if sc.Name != valVarName {
			newCaps = append(newCaps, sc)
		}
	}
	scraper.Name = valVarName
	newCaps = append(newCaps, scraper)
	a.Fetcher.Caps = newCaps

	return nil
}

// ExpirationScraper returns the scraper used to extract the expiration of an
// AuthProof in authentication requests. If the Auth is static or does not have
// an expiration scraper, an empty Scraper will be returned.
func (a Auth) ExpirationScraper() Scraper {
	if a.Type == AuthTypeHTTPBasic || a.Type == AuthTypeNone {
		return Scraper{}
	}

	if a.Fetcher == nil {
		return Scraper{}
	}

	expVarName := a.Fetcher.Expires.VarName
	if expVarName == "" {
		return Scraper{}
	}

	// find the scraper for the expiration var
	for _, scraper := range a.Fetcher.Caps {
		if scraper.Name == expVarName {
			return scraper
		}
	}

	panic(fmt.Sprintf("scraper for expiration var %q not found; should never happen", expVarName))
}

func (a *Auth) DisableExpirationDetection() error {
	if a.Fetcher == nil {
		return nil
	}

	err := a.RemoveExpirationScraper()
	if err != nil {
		return err
	}

	// if this is a session cookie auth, we need to update transforms
	if a.Type == AuthTypeSession {
		a.Fetcher.Value.Transform.FuncName = TransformerFuncIdentity
		a.Fetcher.Expires.Transform.FuncName = ""
	}

	return nil
}

// RemoveExpirationScraper removes the expiration scraper from the Auth. Note
// that this will still persist any transforms and formatting settings for
// expiration value extraction; to ensure that these are also removed, call
// DisableExpirationDetection() instead.
func (a *Auth) RemoveExpirationScraper() error {
	if a.Type == AuthTypeNone {
		return fmt.Errorf("cannot remove expiration scraper from auth with no type set")
	}

	if a.Type == AuthTypeHTTPBasic {
		return fmt.Errorf("cannot remove expiration scraper from HTTP Basic auth")
	}

	if a.Fetcher == nil {
		return nil
	}

	expVarName := a.Fetcher.Expires.VarName
	if expVarName == "" {
		return nil
	}

	// find the scraper for the expiration var and exclude it in a copied list
	// unless it also is the value var, in which case we simply ensure any
	// expiration marking is removed.

	newCaps := make([]Scraper, 0, len(a.Fetcher.Caps))
	for _, sc := range a.Fetcher.Caps {
		if expVarName == a.Fetcher.Value.VarName {
			// if the expiration var is the same as the value var, we don't
			// remove it, but we do ensure that any possible expiration enabling
			// markers are removed

			sc.CookieExpiration = false

			newCaps = append(newCaps, sc)
		} else if sc.Name != expVarName {
			newCaps = append(newCaps, sc)
		}
	}
	a.Fetcher.Caps = newCaps
	a.Fetcher.Expires.VarName = ""

	return nil
}

// SetExpirationScraper sets the expiration scraper for the Auth. If scraper is
// nil, the expiration scraper is set to an automatically-configured one; this
// can only be done if the Auth is a JWT or session Auth, otherwise this will
// cause an error to be returned.
func (a *Auth) SetExpirationScraper(scraper *Scraper) error {
	if a.Type == AuthTypeNone {
		return fmt.Errorf("cannot set expiration scraper on auth with no type set")
	}
	if a.Type == AuthTypeHTTPBasic {
		return fmt.Errorf("cannot set expiration scraper on HTTP Basic auth")
	}

	if scraper == nil {
		if a.Type == AuthTypeToken {
			return fmt.Errorf("cannot automatically infer expiration scraper for token auth")
		} else if a.Type == AuthTypeSession {
			a.Fetcher = NewSessionCookieFetcher(a.RetrievalSequence(), a.CookieName(), true)
		} else if a.Type == AuthTypeJWT {
			a.Fetcher = NewJWTFetcher(a.RetrievalSequence(), a.ValueScraper())
		} else {
			return fmt.Errorf("unknown auth type %q", a.Type)
		}
		return nil
	}

	sVal := *scraper

	if a.Fetcher == nil {
		a.Fetcher = newEmptyFetcher(a.Type)
	}

	// if a is session cookie type, make sure Fetcher is initialized with
	// expiration detection enabled.
	if a.Type == AuthTypeSession {
		a.Fetcher = NewSessionCookieFetcher(a.RetrievalSequence(), a.CookieName(), true)
	} else if a.Type == AuthTypeToken {
		// simple case; just re-init with existing values
		a.Fetcher = NewTokenFetcher(a.RetrievalSequence(), a.ValueScraper(), a.Destination(), &sVal, a.ExpirationLayout())
		return nil
	}

	// all types should be handled with an expires detection scraper now.
	expVarName := a.Fetcher.Expires.VarName
	if expVarName == "" {
		panic("expires var name not set; should never happen")
	}

	// find the scraper for the expires var and exclude it in a copied list
	newCaps := make([]Scraper, 0, len(a.Fetcher.Caps))
	for _, sc := range a.Fetcher.Caps {
		if sc.Name != expVarName {
			newCaps = append(newCaps, sc)
		}
	}
	sVal.Name = expVarName
	newCaps = append(newCaps, sVal)
	a.Fetcher.Caps = newCaps

	return nil
}

// ExpirationLayout returns the layout used for the expiration scraper in
// token auths. If the Auth is static or does not have an expiration scraper, an
// empty string will be returned.
func (a Auth) ExpirationLayout() string {
	if a.Type != AuthTypeToken {
		return ""
	}

	if a.Fetcher == nil {
		return ""
	}

	if a.Fetcher.Expires.Transform.Params == nil {
		return ""
	}

	val, ok := a.Fetcher.Expires.Transform.Params["layout"]
	if !ok {
		return ""
	}
	return val.(string)
}

func (a *Auth) SetExpirationLayout(layout string) error {
	if a.Type != AuthTypeToken {
		return fmt.Errorf("cannot set expiration layout on non-token auth")
	}

	var oldExpScraper *Scraper
	sc := a.ExpirationScraper()
	if sc.Name != "" {
		oldExpScraper = &sc
	}
	a.Fetcher = NewTokenFetcher(a.RetrievalSequence(), a.ValueScraper(), a.Destination(), oldExpScraper, layout)

	return nil
}

// Destination returns the destination for AuthProofs in authenticated requests.
// For static auths, one will be created and returned; otherwise, the
// destination from the Auth's Fetcher will be returned. If no Fetcher is set,
// an empty ProofDestination will be returned.
func (a Auth) Destination() ProofDestination {
	if a.Type == AuthTypeHTTPBasic {
		return ProofDestination{
			Location: ProofLocationHeader,
			Key:      "Authorization",
			Format:   ProofFormatTypeBasic,
		}
	}

	if a.Fetcher == nil {
		return ProofDestination{}
	}

	return a.Fetcher.Dest
}

// SetDestination is a helper function that sets the destination for AuthProofs.
// It is an error to alter the destination for HTTP Basic type auths.
func (a *Auth) SetDestination(pd ProofDestination) error {
	if a.Type == AuthTypeHTTPBasic {
		return fmt.Errorf("cannot set destination on HTTP Basic auth")
	} else if a.Type == AuthTypeNone {
		return fmt.Errorf("cannot set destination on auth with no type set")
	}

	if a.Fetcher == nil {
		a.Fetcher = newEmptyFetcher(a.Type)
	}

	a.Fetcher.Dest = pd
	return nil
}

// CookieName is a helper function that pulls the name of the cookie out of the
// Fetcher if it is a session Auth. If it is not a session Auth, it will return
// an empty string.
func (a Auth) CookieName() string {
	if a.Type != AuthTypeSession {
		return ""
	}

	if a.Fetcher == nil {
		return ""
	}

	// get var name of cookie cap
	cookieVarName := a.Fetcher.Value.VarName
	if cookieVarName == "" {
		panic("cookie var name not set; should never happen")
	}

	// find the scraper for the cookie var
	for _, scraper := range a.Fetcher.Caps {
		if scraper.Name == cookieVarName {
			return scraper.CookieName
		}
	}

	panic(fmt.Sprintf("scraper for cookie var %q not found; should never happen", cookieVarName))
}

// SetCookieName is a helper function that sets the name of the cookie in the
// Fetcher if it is a session Auth.
func (a *Auth) SetCookieName(name string) error {
	if a.Type != AuthTypeSession {
		return fmt.Errorf("cannot set cookie name on non-session auth")
	}

	if a.Fetcher == nil {
		a.Fetcher = NewSessionCookieFetcher(RequestSequence{}, name, false)
		return nil
	}

	// get var name of cookie cap
	cookieVarName := a.Fetcher.Value.VarName
	if cookieVarName == "" {
		panic("cookie var name not set; should never happen")
	}

	// find the scraper for the cookie var
	var matchedIdx int = -1
	for idx, scraper := range a.Fetcher.Caps {
		if scraper.Name == cookieVarName {
			matchedIdx = idx
			break
		}
	}
	if matchedIdx < 0 {
		panic(fmt.Sprintf("scraper for cookie var %q not found; should never happen", cookieVarName))
	}

	s := a.Fetcher.Caps[matchedIdx]
	s.CookieName = name
	a.Fetcher.Dest.Key = name
	a.Fetcher.Caps[matchedIdx] = s

	return nil
}

func (a Auth) IsDetectingExpiration() bool {
	switch a.Type {
	case AuthTypeNone:
		return false
	case AuthTypeHTTPBasic:
		return false
	case AuthTypeSession:
		if a.Fetcher == nil {
			return false
		}

		// get var name of cookie cap
		cookieVarName := a.Fetcher.Value.VarName
		if cookieVarName == "" {
			panic("cookie var name not set; should never happen")
		}

		// find the scraper for the cookie var
		for _, scraper := range a.Fetcher.Caps {
			if scraper.Name == cookieVarName {
				return scraper.CookieExpiration
			}
		}

		panic(fmt.Sprintf("scraper for cookie var %q not found; should never happen", cookieVarName))
	case AuthTypeJWT:
		return true
	case AuthTypeToken:
		return a.Fetcher.Expires.VarName != ""
	}

	panic(fmt.Sprintf("unknown auth type %q", a.Type))
}

func (a Auth) RetrievalSequence() RequestSequence {
	if a.Fetcher == nil {
		return RequestSequence{}
	}
	return a.Fetcher.Seq
}

func (a *Auth) SetRetrievalSequence(seq RequestSequence) error {
	if a.Type == AuthTypeNone {
		return fmt.Errorf("cannot set retrieval sequence on auth with no type set")
	}
	if a.Type == AuthTypeHTTPBasic {
		return fmt.Errorf("cannot set retrieval sequence on HTTP Basic auth")
	}

	if a.Fetcher == nil {
		a.Fetcher = newEmptyFetcher(a.Type)
	}
	a.Fetcher.Seq = seq
	return nil
}

// Sendable returns whether the Auth is able to be used to retrieve an auth
// proof. This will be true if the Auth is all necessary config based on its
// type has been set.
func (a Auth) Sendable() bool {
	switch a.Type {
	case AuthTypeNone:
		return false
	case AuthTypeHTTPBasic:
		// always sendable, unless Proof is nil
		return a.Proof != nil
	case AuthTypeSession:
		// needs to have cookie name and request sequence set
		return a.RetrievalSequence().Name != "" && a.CookieName() != ""
	case AuthTypeJWT:
		// needs to have request sequence set and scraper set
		return a.RetrievalSequence().Name != "" && a.ValueScraper().IsUsable()
	case AuthTypeToken:
		// needs to have request sequence, scraper, and dest set
		return a.RetrievalSequence().Name != "" && a.ValueScraper().IsUsable() && a.Destination().Key != ""
	}

	return false
}

// CheckSuccessfulAuthUse returns whether the given response from an authenticated
// request is considered successful. This is generally always the case unless the
// returned status is 401 Unauthorized.
func (a *Auth) CheckSuccessfulAuthUse(resp *http.Response) bool {
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

func newEmptyFetcher(t AuthType) *AuthFetcher {
	switch t {
	case AuthTypeNone:
		return nil
	case AuthTypeHTTPBasic:
		return nil
	case AuthTypeSession:
		return NewSessionCookieFetcher(RequestSequence{}, "", false)
	case AuthTypeToken:
		return NewTokenFetcher(RequestSequence{}, Scraper{}, ProofDestination{}, nil, "")
	case AuthTypeJWT:
		return NewJWTFetcher(RequestSequence{}, Scraper{})
	default:
		panic(fmt.Sprintf("unknown auth type %q", t))
	}
}

// NewJWTFetcher returns an AuthFetcher that is configured to pull a JWT token from
// the body of the response of the auth flow/template and place it in an
// Authorization header with the Bearer scheme. The scraper must point to a
// valid JWT token in the response body. Give flow or femplate, but not
// both.
func NewJWTFetcher(seq RequestSequence, scraper Scraper) *AuthFetcher {
	// override whatever caller had set
	scraper.Name = "token"

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
	}
}

// NewTokenFetcher returns an AuthFetcher that is configured to pull a simple token
// using another flow/template. Expiration is optional and is
// extracted via the expiresScraper, if present. If not present, expiration will
// be detected only by auth failure. Give flow or femplate, but not both. Only a
// single token value may be extracted. If expiresTimeLayout is set, it will be
// used for parsing expires time, and if set to an empty string, it will default
// to RFC3339.
func NewTokenFetcher(seq RequestSequence, tokenScraper Scraper, dest ProofDestination, expiresScraper *Scraper, expiresTimeLayout string) *AuthFetcher {
	// override whatever caller had set
	tokenScraper.Name = "token"
	if expiresScraper != nil {
		// copy expires scraper to avoid modifying caller's
		scr := *expiresScraper
		expiresScraper = &scr
		expiresScraper.Name = "expires"
	}

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

	if expiresScraper != nil || expiresTimeLayout != "" {
		da.Expires = ScrapeTimeExtractor{
			Transform: TimeTransformer{
				FuncName: TransformerFuncParsedTime,
				Params:   map[string]any{"layout": expiresTimeLayout},
			},
		}

		if expiresScraper != nil {
			da.Expires.VarName = expiresScraper.Name
			da.Caps = append(da.Caps, *expiresScraper)
		}
	}

	return da
}

// NewSessionCookieFetcher returns an AuthFetcher that is configured to pull a cookie
// containing the session ID from the response of the auth flow/template and use
// it in authorized requests. The cookieName is the name of the cookie to pull.
// Give flow or femplate, but not both. If expiration detection is set, this
// Auth will always attempt to detect expiration info from the initial
// set-cookie, but if it is not present, it will fallback to error response on
// the auth'd request's response to detect expiration.
func NewSessionCookieFetcher(seq RequestSequence, cookieName string, detectExpiration bool) *AuthFetcher {
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

	return da
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

	ap := DynamicProof{
		Value: value,
		Dest:  da.Dest,
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
		ap.ExpiresAt = expTime
	}

	return ap, nil
}
