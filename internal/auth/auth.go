package auth

import (
	"fmt"
	"net/http"
	"strings"
	"time"
)

// AuthMethod represents a method for obtaining and managing authentication
// proofs in morc.
type AuthMethod struct {
	Name            string
	AuthType        AuthType
	Config          AuthConfig
	LastProof       *AuthProof
	LastProofTime   time.Time
	LastProofValid  bool
	ProofExpiration time.Duration
}

// AuthType represents the type of authentication method.
type AuthType string

const (
	AuthTypeStatic AuthType = "static"
	AuthTypeDynamic AuthType = "dynamic"
)

// AuthConfig contains the configuration for an authentication method.
type AuthConfig struct {
	// Static credentials configuration
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	
	// Dynamic auth configuration
	FlowName     string            `json:"flow_name,omitempty"`
	ProofCapture string            `json:"proof_capture,omitempty"`
	ProofHeader  string            `json:"proof_header,omitempty"`
	ProofQuery   string            `json:"proof_query,omitempty"`
	ProofCookie  string            `json:"proof_cookie,omitempty"`
	ProofType    string            `json:"proof_type,omitempty"`
	ProofFormat  string            `json:"proof_format,omitempty"`
	ProofExpiry  time.Duration     `json:"proof_expiry,omitempty"`
	InvalidCodes []int             `json:"invalid_codes,omitempty"`
	Headers      map[string]string `json:"headers,omitempty"`
}

// AuthProof represents a proof of authentication that can be attached to requests.
type AuthProof struct {
	Value     string
	Type      string
	ExpiresAt time.Time
}

// ApplyToRequest applies the authentication proof to the given HTTP request.
func (p *AuthProof) ApplyToRequest(r *http.Request) error {
	switch strings.ToLower(p.Type) {
	case "bearer":
		r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", p.Value))
	case "basic":
		r.Header.Set("Authorization", fmt.Sprintf("Basic %s", p.Value))
	case "cookie":
		r.AddCookie(&http.Cookie{Name: p.Type, Value: p.Value})
	case "query":
		if r.URL.Query().Get(p.Type) != "" {
			r.URL.Query().Set(p.Type, p.Value)
		} else {
			r.URL.Query().Add(p.Type, p.Value)
		}
		return nil
	default:
		return fmt.Errorf("unsupported auth type: %s", p.Type)
	}
	return nil
}

// IsExpired checks if the auth proof has expired.
func (p *AuthProof) IsExpired() bool {
	return time.Now().After(p.ExpiresAt)
}
