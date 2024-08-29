package morc

import (
	"net/http"
)

type AuthProofType string

const (
	AuthProofHTTPBasic AuthProofType = "http-basic"
)

type AuthProof interface {
	Apply(req *http.Request) error

	// Valid returns false in the future.
	Valid() bool

	// Export returns a JSON-encodable map that can be used to recreate this
	// AuthProof.
	Export() map[string]any

	// Type returns the type of AuthProof that this is. It is used for selecting
	// the correct constructor to recreate an AuthProof from an Exported string.
	Type() AuthProofType
}

type basicAuth struct {
	username string
	password string
}

func (b basicAuth) Apply(req *http.Request) error {
	req.SetBasicAuth(b.username, b.password)
	return nil
}

func (b basicAuth) Valid() bool {
	return true
}

func (b basicAuth) Export() map[string]any {
	return map[string]any{
		"username": b.username,
		"password": b.password,
	}
}

func (b basicAuth) Type() AuthProofType {
	return AuthProofHTTPBasic
}

func NewHTTPBasicAuth(username, password string) AuthProof {
	return basicAuth{
		username: username,
		password: password,
	}
}

func ImportHTTPBasicAuth(exported map[string]any) AuthProof {
	return basicAuth{
		username: exported["username"].(string),
		password: exported["password"].(string),
	}
}

func ImportAuthProof(t AuthProofType, exported map[string]any) AuthProof {
	switch t {
	case AuthProofHTTPBasic:
		return ImportHTTPBasicAuth(exported)
	default:
		return nil
	}
}

type Auth interface {
	GetAuth() AuthProof // runs an auth flow if dynamic, or returns the static proof if static
}

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
// - if not custom, other props that are then used for initing the auth
// - if custom, need placement target and value.

type HTTPBasicAuth struct{}
