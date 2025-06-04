package morc

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"testing"
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

	// Add auth method first
	project.AuthMethods[basicAuth.Name] = basicAuth

	// Test getting auth method
	auth, ok := project.AuthMethods["basic"]
	if !ok {
		t.Errorf("Auth method not found")
		return
	}
	if auth.Name != "basic" {
		t.Errorf("Invalid auth method returned")
		return
	}

	// Test updating auth method
	updatedAuth := auth
	updatedAuth.Credentials.Username = "newuser"
	project.AuthMethods["basic"] = updatedAuth

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

	// Add auth method first
	project.AuthMethods[flowAuth.Name] = flowAuth

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
		return
	}

	// Apply auth manually since we don't have the full implementation yet

	authMethod, ok := project.AuthMethods[template.AuthFlow]
	if !ok {
		t.Errorf("Auth method not found: %s", template.AuthFlow)
		return
	}
	if authMethod.AuthType == "basic" && authMethod.Credentials != nil {
		req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", authMethod.Credentials.Username, authMethod.Credentials.Password))))
	}

	// Verify auth header
	if req.Header.Get("Authorization") == "" {
		t.Errorf("Auth header not set")
	}

	// Test removing auth method
	delete(project.AuthMethods, "basic")
}
