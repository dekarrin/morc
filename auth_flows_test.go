package morc

import (
	"testing"
	"time"
)

func TestAuthFlowExecutor_ExecuteAuthFlow(t *testing.T) {
	// Setup test project
	project := NewProject()
	project.Path = t.TempDir()

	// Create test auth
	auth := Auth{
		Name: "test-auth",
		Type: AuthTypeSession,
		Fetcher: &AuthFetcher{
			Seq: RequestSequence{ // Mock sequence
				Requests: []*Request{
					{
						URL: "http://example.com/auth",
					},
				},
			},
			Caps: []Scraper{
				NewBodyScraper("session_id", "session_id", "\"session_id\":\"(.*?)\""),
			},
			Value: ScrapeExtractor{
				VarName: "session_id",
			},
			Dest: ProofDestination{
				Location: ProofLocationCookie,
				Key:      "session",
			},
		},
	}

	// Add auth to project
	project.Auths = map[string]Auth{auth.Name: auth}

	// Create executor
	executor := NewAuthFlowExecutor(project)

	// Test execution
	result, err := executor.ExecuteAuthFlow(&auth)
	if err != nil {
		t.Errorf("ExecuteAuthFlow() error = %v", err)
		return
	}

	if result.AuthProof == nil {
		t.Errorf("ExecuteAuthFlow() returned nil auth proof")
	}

	// Test auth proof storage
	storedAuth, err := project.GetAuth(auth.Name)
	if err != nil {
		t.Errorf("GetAuth() error = %v", err)
	}

	if storedAuth.Proof == nil {
		t.Errorf("Auth proof not stored in project")
	}
}

func TestAuthFlowExecutor_CheckAuthNeeded(t *testing.T) {
	project := NewProject()
	project.Path = t.TempDir()

	// Create test auth
	auth := Auth{
		Name: "test-auth",
		Type: AuthTypeSession,
		Proof: DynamicProof{
			ExpiresAt: time.Now().Add(-time.Hour), // Expired
		},
	}

	project.Auths = map[string]Auth{auth.Name: auth}

	// Create test request
	req := Request{
		Auth: auth.Name,
	}

	executor := NewAuthFlowExecutor(project)

	// Test auth needed
	needed, err := executor.CheckAuthNeeded(&req)
	if err != nil {
		t.Errorf("CheckAuthNeeded() error = %v", err)
	}
	if !needed {
		t.Errorf("CheckAuthNeeded() = %v, want %v", needed, true)
	}

	// Test auth not needed
	auth.Proof.ExpiresAt = time.Now().Add(time.Hour)
	project.Auths[auth.Name] = auth

	needed, err = executor.CheckAuthNeeded(&req)
	if err != nil {
		t.Errorf("CheckAuthNeeded() error = %v", err)
	}
	if needed {
		t.Errorf("CheckAuthNeeded() = %v, want %v", needed, false)
	}
}

func TestAuthFlowExecutor_ApplyAuthToRequest(t *testing.T) {
	project := NewProject()
	project.Path = t.TempDir()

	// Create test auth
	auth := Auth{
		Name: "test-auth",
		Type: AuthTypeSession,
		Proof: DynamicProof{
			Value: "test-session",
		},
		Fetcher: &AuthFetcher{
			Dest: ProofDestination{
				Location: ProofLocationCookie,
				Key:      "session",
			},
		},
	}

	project.Auths = map[string]Auth{auth.Name: auth}

	// Create test request
	req := Request{
		Auth: auth.Name,
	}

	executor := NewAuthFlowExecutor(project)

	// Test auth application
	if err := executor.ApplyAuthToRequest(&req, auth.Proof); err != nil {
		t.Errorf("ApplyAuthToRequest() error = %v", err)
	}

	// Verify cookie was set
	cookie := req.HTTPRequest.Header.Get("Cookie")
	if cookie != "session=test-session" {
		t.Errorf("Cookie not set correctly: got %v, want %v", cookie, "session=test-session")
	}
}
