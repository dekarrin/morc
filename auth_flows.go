package morc

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
)

type AuthFlowResult struct {
	AuthProof AuthProof
	Error    error
}

type AuthFlowExecutor struct {
	project *Project
}

func NewAuthFlowExecutor(project *Project) *AuthFlowExecutor {
	return &AuthFlowExecutor{
		project: project,
	}
}

func (e *AuthFlowExecutor) ExecuteAuthFlow(auth *Auth) (AuthFlowResult, error) {
	if !auth.Sendable() {
		return AuthFlowResult{}, fmt.Errorf("auth %q is not configured properly", auth.Name)
	}

	if auth.Static() {
		return AuthFlowResult{AuthProof: auth.Proof}, nil
	}

	// Execute the auth flow/template
	result, err := e.project.SendSequence(auth.RetrievalSequence())
	if err != nil {
		return AuthFlowResult{}, fmt.Errorf("failed to execute auth flow: %w", err)
	}

	// Scrape the auth proof from the result
	proof, err := auth.Fetcher.ScrapeFromResult(result)
	if err != nil {
		return AuthFlowResult{}, fmt.Errorf("failed to scrape auth proof: %w", err)
	}

	// Store the auth proof in the project
	if err := e.project.StoreAuthProof(auth.Name, proof); err != nil {
		return AuthFlowResult{}, fmt.Errorf("failed to store auth proof: %w", err)
	}

	return AuthFlowResult{AuthProof: proof}, nil
}

func (e *AuthFlowExecutor) CheckAuthNeeded(req *Request) (bool, error) {
	// Check if auth is needed based on request
	if req.Auth == "" {
		return false, nil
	}

	auth, err := e.project.GetAuth(req.Auth)
	if err != nil {
		return false, fmt.Errorf("failed to get auth %q: %w", req.Auth, err)
	}

	if auth.SecretExpiration().Before(time.Now()) {
		return true, nil
	}

	return false, nil
}

func (e *AuthFlowExecutor) ApplyAuthToRequest(req *Request, authProof AuthProof) error {
	if authProof == nil {
		return nil
	}

	return authProof.Apply(req.HTTPRequest)
}

func (p *Project) StoreAuthProof(authName string, proof AuthProof) error {
	// Store the auth proof in the project's auths
	if err := p.LoadAuths(); err != nil {
		return fmt.Errorf("failed to load auths: %w", err)
	}

	if auth, ok := p.Auths[authName]; ok {
		auth.Proof = proof
		if err := p.SaveAuths(); err != nil {
			return fmt.Errorf("failed to save auths: %w", err)
		}
		return nil
	}

	return fmt.Errorf("auth %q not found", authName)
}

func (p *Project) GetAuth(authName string) (*Auth, error) {
	if err := p.LoadAuths(); err != nil {
		return nil, fmt.Errorf("failed to load auths: %w", err)
	}

	if auth, ok := p.Auths[authName]; ok {
		return &auth, nil
	}

	return nil, fmt.Errorf("auth %q not found", authName)
}

func (p *Project) LoadAuths() error {
	// Load auths from project file
	if err := p.loadFromFile(p.AuthsPath()); err != nil {
		return fmt.Errorf("failed to load auths: %w", err)
	}
	return nil
}

func (p *Project) SaveAuths() error {
	// Save auths to project file
	if err := p.saveToFile(p.AuthsPath()); err != nil {
		return fmt.Errorf("failed to save auths: %w", err)
	}
	return nil
}

func (p *Project) AuthsPath() string {
	return filepath.Join(p.Path, "auths.json")
}

func (p *Project) loadFromFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read file: %w", err)
	}

	var auths map[string]Auth
	if err := json.Unmarshal(data, &auths); err != nil {
		return fmt.Errorf("failed to unmarshal auths: %w", err)
	}

	p.Auths = auths
	return nil
}

func (p *Project) saveToFile(path string) error {
	data, err := json.MarshalIndent(p.Auths, "", "\t")
	if err != nil {
		return fmt.Errorf("failed to marshal auths: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}
