package morc

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type AuthProofType string

const (
	AuthProofHTTPBasic AuthProofType = "http-basic"
	AuthProofOAuth2    AuthProofType = "oauth2"
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

type Oauth2Token struct {
	accessToken  string
	tokenType    string // TODO: should support "bearer" and "mac".
	refreshToken string
	expiresAt    time.Time // might not be given, TODO: config to get this somewhere else like default, etc.
	Scope        []string
}

func (t Oauth2Token) Apply(req *http.Request) error {
	if t.tokenType == "bearer" {
		req.Header.Set("Authorization", "Bearer "+t.accessToken)
	} else if t.tokenType == "mac" {
		// TODO: support this
		return errors.New("MAC access token type not supported")
	} else {
		return errors.New("Unknown token type: " + t.tokenType)
	}
	return nil
}

func (t Oauth2Token) Valid() bool {
	if t.expiresAt.IsZero() {
		return true
	}

	return time.Now().Before(t.expiresAt)
}

func (t Oauth2Token) Export() map[string]any {
	m := map[string]any{
		"access_token": t.accessToken,
		"token_type":   t.tokenType,
		"expires_at":   t.expiresAt.Format(time.RFC3339),
	}

	if t.refreshToken != "" {
		m["refresh_token"] = t.refreshToken
	}

	if len(t.Scope) > 0 {
		m["scope"] = t.Scope
	}

	return m
}

func (t Oauth2Token) Type() AuthProofType {
	return AuthProofOAuth2
}

func NewOAuth2Token(accessToken, tokenType string, expiresAt time.Time, refreshToken string, scope []string) AuthProof {
	return Oauth2Token{
		accessToken:  accessToken,
		tokenType:    tokenType,
		expiresAt:    expiresAt,
		refreshToken: refreshToken,
		Scope:        scope,
	}
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

func ImportOAuth2Token(exported map[string]any) (AuthProof, error) {
	var accessToken string
	var refreshToken string
	var tokenType string
	var expTime time.Time
	var scope []string

	// ensure expected properties are present.
	if rawAccess, ok := exported["access_token"]; ok {
		if accessToken, ok = rawAccess.(string); !ok {
			return nil, errors.New("access_token must be a string")
		}
	} else {
		return nil, errors.New("missing access_token")
	}

	if rawType, ok := exported["token_type"]; ok {
		if tokenType, ok = rawType.(string); !ok {
			return nil, errors.New("token_type must be a string")
		}
	} else {
		return nil, errors.New("missing token_type")
	}

	if rawExp, ok := exported["expires_at"]; ok {
		var expStr string
		if expStr, ok = rawExp.(string); !ok {
			return nil, errors.New("expires_at must be a string containing an RFC-3339 date")
		}
		var err error
		if expTime, err = time.Parse(time.RFC3339, expStr); err != nil {
			return nil, fmt.Errorf("expires_at: %w", err)
		}
	} else {
		return nil, errors.New("missing expires_at")
	}

	if rawRefresh, ok := exported["refresh_token"]; ok {
		if refreshToken, ok = rawRefresh.(string); !ok {
			return nil, errors.New("refresh_token must be a string")
		}
	}

	if rawScope, ok := exported["scope"]; ok {
		if scope, ok = rawScope.([]string); !ok {

			// fallback to 'any'
			if scopeAny, ok := rawScope.([]any); ok {
				scope = make([]string, len(scopeAny))
				for i := range scopeAny {
					if scope[i], ok = scopeAny[i].(string); !ok {
						return nil, errors.New("scope must be a list of strings")
					}
				}
			} else {
				return nil, errors.New("scope must be a list of strings")
			}
		}
	}

	return Oauth2Token{
		accessToken:  accessToken,
		tokenType:    tokenType,
		expiresAt:    expTime,
		refreshToken: refreshToken,
		Scope:        scope,
	}, nil
}

func ImportAuthProof(t AuthProofType, exported map[string]any) (AuthProof, error) {
	switch t {
	case AuthProofHTTPBasic:
		return ImportHTTPBasicCreds(exported)
	case AuthProofOAuth2:
		return ImportOAuth2Token(exported)
	default:
		return nil, errors.New("unknown auth proof type")
	}
}

type Auth interface {
	GetAuth() (AuthProof, error) // runs an auth flow if dynamic, or returns the static proof if static
	Static() bool
}

type HTTPBasicAuth struct {
	proof HTTPBasicCredentials
}

func (ba HTTPBasicAuth) Static() bool {
	return true
}

func (sa HTTPBasicAuth) GetAuth() (AuthProof, error) {
	return sa.proof, nil
}

func NewHTTPBasicAuth(creds HTTPBasicCredentials) HTTPBasicAuth {
	return HTTPBasicAuth{
		proof: creds,
	}
}

// TODO: when adding other grant types, consider if they can be combined into a
// single type
//
// oauth flow doesn't really work great with CLI, look into grant types
// that are betta and do those first
type OAuth2AuthCodeGrantAuth struct {
	clientID     string
	clientSecret string
}

func (oa2 OAuth2AuthCodeGrantAuth) Static() bool {
	return false
}

func (oa2 OAuth2AuthCodeGrantAuth) GetAuth() (AuthProof, error) {
	return nil, fmt.Errorf("not implemented")
}

// ALGO FLOW for basic auth -
//
// * if basic auth, send the creds every time.
//
// ALGO FLOW for password to token (NON OAuth2)-
//

type SpecType int

const (
	SpecBodyJSON SpecType = iota
	SpecBodyOffset
	SpecHeader
	SpecCookie
	SpecTrailer
)

// TODO: make everything that takes a BodyScraper actually take a Scraper. This
// will be fairly non-trivial, make sure all types are accounted for.
type Scraper interface {
	String() string
	Spec() string
	Scrape(resp *http.Response, preReadBody []byte) (string, error)
	Type() SpecType
	EqualSpec(other Scraper) bool
	VarName() string
}

type HeaderScraper struct {
	Name  string
	Key   string
	Index int
}

func (hs HeaderScraper) VarName() string {
	return hs.Name
}

func (hs HeaderScraper) String() string {
	s := fmt.Sprintf("%s from ", strings.ToUpper(hs.Name))
	s += hs.Spec()
	return s
}

func (hs HeaderScraper) Spec() string {
	return fmt.Sprintf("header %s[%d]", hs.Key, hs.Index)
}

func (hs HeaderScraper) Type() SpecType {
	return SpecHeader
}

func (hs HeaderScraper) Scrape(resp *http.Response, preReadBody []byte) (string, error) {
	vals := resp.Header.Values(hs.Key)

	if len(vals) < 1 {
		return "", fmt.Errorf("header %s is not present in response", hs.Key)
	}

	// neg header indexes specify from the end of the list
	actualIndex := hs.Index
	if actualIndex < 0 {
		actualIndex = len(vals) + actualIndex
	}

	if len(vals) <= actualIndex {
		return "", fmt.Errorf("header %s does not have a %d'th value; only %d values are present", hs.Key, hs.Index, len(vals))
	}

	return vals[actualIndex], nil
}

type DynamicAuth struct {
	proof AuthProof // TODO: fill concrete type later

	getAuthFlow     string // either execFlow or execTemplate must be set
	getAuthTemplate string
}

func (da DynamicAuth) Static() bool {
	return false
}

func (da DynamicAuth) GetAuth(p *Project, skipVerify bool, httpClient *http.Client, oc OutputControl) (AuthProof, error) {
	if da.proof.Valid() {
		return da.proof, nil
	}

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

	// TODO: fallback needs to be implemented at some level to decide that a
	// previously valid proof is not valid and could be re-obtained, but that's
	// probably going to need to be at caller-level.
	return nil, fmt.Errorf("not implemented")
}

// BASIC ALGO FLOW -
//
// 1. Does my proof need renewal?
//    * That is, is the proof missing? OR is it present but expired?

// req data model, flow-based:
// - static: false (or omitted)
// - flow name
// - retrieval target (header, cookie, body path-spec, body offset)
// - transform retreival to value
// - transform retreival to expiration
// - transform retrieval to target-key. optional.
// - placement target (header, cookie, query param)

// req data model, static-based:
// - static true
// - type: "http basic" or such or "custom"
// - credentials: gives the credentials, custom obj.
// - if custom, need placement target.

/*
"auth": {
	"static": true,
	"type": "http-basic",
	"credentials": {
		"username": "foo",
		"password": "bar"
	},
}

OR

"auth": {
	"static": true,
	"type": "custom",
	"credentials": {
		"key": "value"
	},
	"target": {
		"location": "header",
		// key not needed and is ignored as it is from creds in this case
	}
}

OR

"auth": {
	"static": false,
	"type": "api-key",
	"expiration": {
		"resource-status": [
			401,
		]
		AND/OR:
		"from": {
			// from syntax
			"format": "ISO8601",
		}
		AND/OR:
		"duration": "1h",
		// implicitly - don't have it.
	},
	"retrieval": {
		"template": "the-thing",
		OR:
		"flow": "the-thing",
		OR:
		"template params": {

		},
		-------
		"from": {
			"var": "name", // if var is auto-captured
			OR:
			"header": "name",
			OR:
			"cookie": "name",
			OR:
			"body": {
				// offset, path-spec
			}
		}
	}
	"target": {
		"location": "header"/"cookie"/"query",
	}
}

OR

// oauth2 w 4 different grant types

// * auth code - client 'tells' owner to go to auth server and get an auth code, then owner goes back to client with it,
// THEN, client takes the auth code, and...

// * implicit - client gets the access token right away by being authorized by owner. Somehow. THEN...

// * Resource owner password credentials - client uses the owner's username and password directly to get the access token. THEN...

// * Client credentials - separate credentials just for the client are used to get the access token. THEN...


// ALL (?) flows require use of a client_id.
// SOME flows will use a client authentication method (like client_secret, or a client certificate).


// NOTE:
// from RFC-6749: "Additional authentication credentials, which are beyond
   the scope of this specification, may be required in order for the
   client to use a[n access] token."

// NOTE:
// methods of usage of access token are defined in RFC-6750.


// If refresh token exists, it will be given when an access token is given as well.
// If refresh token exists, then when access token is detected as expired, it is
// used for getting a new access token and the grant flow is not repeated (unless the refresh token is ALSO expired).

"auth": {
	"static": false,
// 	"type": "oauth2",

// */

// type marshaledAuthConfig struct {
// 	Static      bool
// 	Type        string
// 	Credentials map[string]any
// 	Target      AuthLocation

// 	// TODO: flow-based things

// }

// type AuthLocation string

// const (
// 	AuthLocationHeader AuthLocation = "header"
// 	AuthLocationCookie AuthLocation = "cookie"
// 	AuthLocationQuery  AuthLocation = "query"
// )

// func unmarshalAuthConfig(data map[string]any) (Auth, error) {
// 	if data == nil {
// 		return nil
// 	}

// 	// check for well-known field names
// 	var static bool
// 	var typeStr string

// 	for k, v := range data {
// 		switch strings.ToLower(k) {
// 		case "static":
// 			boolV, ok := v.(bool)
// 			if !ok {
// 				return nil, fmt.Errorf("static: must be a boolean")
// 			}
// 			static = boolV
// 		case "type":
// 			strV, ok := v.(string)
// 			if !ok {
// 				return nil, fmt.Errorf("type: must be a string")
// 			}

// 			typeStr = strV
// 		}
// 	}
// }
