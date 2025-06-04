package morc

import (
	"testing"
	"time"
)

func TestAuthMethods(t *testing.T) {
	// Create a test project
	project := Project{
		Name:      "test",
		Templates: make(map[string]RequestTemplate),
		Flows:     make(map[string]Flow),
		AuthMethods: make(map[string]AuthMethod),
	}

	// Test adding basic auth
	basicAuth := AuthMethod{
		Name:     "basic",
		AuthType: "basic",
		Credentials: &Credentials{
			Username: "user",
			Password: "pass",
		},
	}

	if err := project.AddAuthMethod(basicAuth); err != nil {
		t.Errorf("Failed to add basic auth: %v", err)
	}

	// Test getting auth method
	auth, err := project.GetAuthMethod("basic")
	if err != nil {
		t.Errorf("Failed to get auth method: %v", err)
	}
	if auth == nil || auth.Name != "basic" {
		t.Errorf("Invalid auth method returned")
	}

	// Test updating auth method
	updatedAuth := *auth
	updatedAuth.Credentials.Username = "newuser"
	if err := project.UpdateAuthMethod("basic", updatedAuth); err != nil {
		t.Errorf("Failed to update auth method: %v", err)
	}

	// Test removing auth method
	if err := project.RemoveAuthMethod("basic"); err != nil {
		t.Errorf("Failed to remove auth method: %v", err)
	}

	// Test flow auth
	project.Flows["test-flow"] = Flow{
		Name: "test-flow",
		Steps: []FlowStep{{
			Template: "test-template",
		}},
	}

	flowAuth := AuthMethod{
		Name:     "flow",
		AuthType: "flow",
		Flow:     "test-flow",
		CaptureFrom: &CaptureConfig{
			HeaderName: "X-Token",
		},
		AuthProof: &AuthProofConfig{
			HeaderName: "Authorization",
		},
	}

	if err := project.AddAuthMethod(flowAuth); err != nil {
		t.Errorf("Failed to add flow auth: %v", err)
	}

	// Test request template with auth
	template := RequestTemplate{
		Name:    "test-template",
		Method:  "GET",
		URL:     "http://example.com",
		AuthFlow: "basic",
	}

	req, err := http.NewRequest("GET", "http://example.com", nil)
	if err != nil {
		t.Errorf("Failed to create request: %v", err)
	}

	if err := project.ApplyAuth(&template, req); err != nil {
		t.Errorf("Failed to apply auth: %v", err)
	}

	// Verify auth header
	if req.Header.Get("Authorization") == "" {
		t.Errorf("Auth header not set")
	}
}
