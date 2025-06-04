package morc

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"time"
)

// AuthMethod represents a method for obtaining and validating authentication
// credentials.
type AuthMethod struct {
	Name            string `json:"name"`
	AuthType        string `json:"auth_type"`
	Credentials     *Credentials `json:"credentials,omitempty"`
	Flow            string `json:"flow,omitempty"`
	CaptureFrom     *CaptureConfig `json:"capture_from,omitempty"`
	ExpirationCheck *ExpirationConfig `json:"expiration_check,omitempty"`
	AuthProof       *AuthProofConfig `json:"auth_proof,omitempty"`
	InvalidCodes    []int `json:"invalid_codes,omitempty"`
}

// Credentials represents static authentication credentials
type Credentials struct {
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	// Additional fields can be added for other auth types
}

// CaptureConfig specifies how to capture authentication proof from a response
type CaptureConfig struct {
	HeaderName string `json:"header_name,omitempty"`
	BodyPath   string `json:"body_path,omitempty"` // JSON path for body capture
}

// ExpirationConfig specifies how to determine if auth is expired
type ExpirationConfig struct {
	HeaderName string `json:"header_name,omitempty"`
	BodyPath   string `json:"body_path,omitempty"` // JSON path for body check
	Value      string `json:"value,omitempty"`     // Expected value for expiration
}

// AuthProofConfig specifies how to use the auth proof in requests
type AuthProofConfig struct {
	HeaderName string `json:"header_name,omitempty"`
	BodyPath   string `json:"body_path,omitempty"` // JSON path for body insertion
}





// AuthProof represents a piece of authentication proof
type AuthProof struct {
	Value    string    `json:"value"`
	Expires  time.Time `json:"expires,omitempty"`
	AuthType string    `json:"auth_type"`
}

// ProjectWithAuth adds auth methods to the Project struct
type ProjectWithAuth struct {
	Project
	AuthMethods map[string]AuthMethod `json:"auth_methods,omitempty"`
	Flows      map[string]*Flow       `json:"flows,omitempty"`
}

// GetAuthProof retrieves an authentication proof for the given auth method
func (p *ProjectWithAuth) GetAuthProof(authName string) (*AuthProof, error) {
	if auth, ok := p.AuthMethods[authName]; ok {
		return p.executeAuthFlow(&auth)
	}
	return nil, fmt.Errorf("auth method %s not found", authName)
}

// executeAuthFlow executes the auth flow and returns the proof
func (p *ProjectWithAuth) executeAuthFlow(auth *AuthMethod) (*AuthProof, error) {
	if auth.AuthType == "basic" {
		if auth.Credentials == nil {
			return nil, errors.New("basic auth requires credentials")
		}
		proof := &AuthProof{
			Value:    base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", auth.Credentials.Username, auth.Credentials.Password))),
			AuthType: "basic",
		}
		return proof, nil
	}

	if auth.Flow == "" {
		return nil, errors.New("auth flow not specified")
	}

	// Execute the flow to get the auth proof
	if _, ok := p.Flows[auth.Flow]; !ok {
		return nil, fmt.Errorf("flow %s not found", auth.Flow)
	}

	// Execute the flow here (implementation needed)
	// This is a placeholder for the actual flow execution
	
	// Capture the auth proof from the response
	if auth.CaptureFrom != nil {
		// Implementation needed to capture from response
	}

	return nil, errors.New("not implemented")
}

// ApplyAuthProof applies the authentication proof to a request
func (p *ProjectWithAuth) ApplyAuthProof(req *http.Request, proof *AuthProof) error {
	if proof == nil {
		return errors.New("auth proof is nil")
	}

	switch proof.AuthType {
	case "basic":
		req.Header.Set("Authorization", "Basic "+proof.Value)
	case "bearer":
		req.Header.Set("Authorization", "Bearer "+proof.Value)
	default:
		return fmt.Errorf("unsupported auth type: %s", proof.AuthType)
	}

	return nil
}

// IsAuthValid checks if the auth proof is still valid
func (p *ProjectWithAuth) IsAuthValid(proof *AuthProof, response *http.Response) bool {
	if proof == nil {
		return false
	}

	// Check if auth has expired
	if !proof.Expires.IsZero() && time.Now().After(proof.Expires) {
		return false
	}

	// Check if response indicates invalid auth
	if response.StatusCode >= 400 {
		return false
	}

	return true
}
