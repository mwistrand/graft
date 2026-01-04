package copilot

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mwistrand/graft/internal/git"
	"github.com/mwistrand/graft/internal/provider"
)

func TestNew(t *testing.T) {
	p, err := New("", "")
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	if p.Name() != "copilot" {
		t.Errorf("Name() = %q, want %q", p.Name(), "copilot")
	}

	if p.baseURL != DefaultBaseURL {
		t.Errorf("baseURL = %q, want %q", p.baseURL, DefaultBaseURL)
	}

	if p.model != "" {
		t.Errorf("model = %q, want empty string", p.model)
	}
}

func TestNew_CustomValues(t *testing.T) {
	p, err := New("http://custom:8080", "gpt-4o")
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	if p.baseURL != "http://custom:8080" {
		t.Errorf("baseURL = %q, want %q", p.baseURL, "http://custom:8080")
	}

	if p.model != "gpt-4o" {
		t.Errorf("model = %q, want %q", p.model, "gpt-4o")
	}
}

func TestNew_TrailingSlash(t *testing.T) {
	p, err := New("http://localhost:4141/", "")
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	if p.baseURL != "http://localhost:4141" {
		t.Errorf("baseURL = %q, want %q (trailing slash should be removed)", p.baseURL, "http://localhost:4141")
	}
}

func TestSummarizeChanges(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("expected /v1/chat/completions, got %s", r.URL.Path)
		}

		resp := chatResponse{
			Choices: []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			}{
				{Message: struct {
					Content string `json:"content"`
				}{Content: `{"overview": "Test summary", "key_changes": ["Change 1"]}`}},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p, _ := New(server.URL, "")
	result, err := p.SummarizeChanges(context.Background(), &provider.SummarizeRequest{
		Files: []git.FileDiff{{Path: "test.go", Status: git.StatusModified}},
	})

	if err != nil {
		t.Fatalf("SummarizeChanges() failed: %v", err)
	}

	if result.Overview != "Test summary" {
		t.Errorf("Overview = %q, want %q", result.Overview, "Test summary")
	}
}

func TestOrderFiles(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := chatResponse{
			Choices: []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			}{
				{Message: struct {
					Content string `json:"content"`
				}{Content: `{"files": [{"path": "main.go", "category": "entry_point", "priority": 1, "description": "Main entry"}], "reasoning": "Test"}`}},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p, _ := New(server.URL, "")
	result, err := p.OrderFiles(context.Background(), &provider.OrderRequest{
		Files: []git.FileDiff{{Path: "main.go", Status: git.StatusModified}},
	})

	if err != nil {
		t.Fatalf("OrderFiles() failed: %v", err)
	}

	if len(result.Files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(result.Files))
	}

	if result.Files[0].Path != "main.go" {
		t.Errorf("Path = %q, want %q", result.Files[0].Path, "main.go")
	}
}

func TestChat_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	p, _ := New(server.URL, "")
	_, err := p.SummarizeChanges(context.Background(), &provider.SummarizeRequest{
		Files: []git.FileDiff{{Path: "test.go"}},
	})

	if err == nil {
		t.Error("expected error for server error response")
	}
}

func TestChat_EmptyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := chatResponse{Choices: []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		}{}}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p, _ := New(server.URL, "")
	_, err := p.SummarizeChanges(context.Background(), &provider.SummarizeRequest{
		Files: []git.FileDiff{{Path: "test.go"}},
	})

	if err == nil {
		t.Error("expected error for empty response")
	}
}

func TestExtractJSON(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "raw JSON",
			input: `{"key": "value"}`,
			want:  `{"key": "value"}`,
		},
		{
			name:  "JSON in code block",
			input: "```json\n{\"key\": \"value\"}\n```",
			want:  `{"key": "value"}`,
		},
		{
			name:  "JSON in generic code block",
			input: "```\n{\"key\": \"value\"}\n```",
			want:  `{"key": "value"}`,
		},
		{
			name:  "JSON with leading text",
			input: "Here is the response:\n{\"key\": \"value\"}",
			want:  `{"key": "value"}`,
		},
		{
			name:  "JSON array",
			input: `[1, 2, 3]`,
			want:  `[1, 2, 3]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractJSON(tt.input)
			if got != tt.want {
				t.Errorf("extractJSON() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseJSONResponse(t *testing.T) {
	input := `{"overview": "Test summary", "key_changes": ["Change 1"]}`

	var resp provider.SummarizeResponse
	err := parseJSONResponse(input, &resp)
	if err != nil {
		t.Fatalf("parseJSONResponse() failed: %v", err)
	}

	if resp.Overview != "Test summary" {
		t.Errorf("Overview = %q, want %q", resp.Overview, "Test summary")
	}
}

func TestParseJSONResponse_Invalid(t *testing.T) {
	var resp provider.SummarizeResponse
	err := parseJSONResponse("not valid json", &resp)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestBuildSummaryPrompt(t *testing.T) {
	req := &provider.SummarizeRequest{
		Files: []git.FileDiff{
			{Path: "main.go", Status: git.StatusModified, Additions: 10, Deletions: 5},
		},
		Commits: []git.Commit{
			{ShortHash: "abc123", Author: "Test User", Subject: "Add feature"},
		},
		FullDiff: "+line1\n-line2",
	}

	prompt := buildSummaryPrompt(req)

	if !strings.Contains(prompt, "main.go") {
		t.Error("prompt should contain main.go")
	}
	if !strings.Contains(prompt, "abc123") {
		t.Error("prompt should contain commit hash")
	}
	if !strings.Contains(prompt, "JSON") {
		t.Error("prompt should mention JSON format")
	}
}

func TestBuildOrderPrompt(t *testing.T) {
	req := &provider.OrderRequest{
		Files: []git.FileDiff{
			{Path: "cmd/main.go", Status: git.StatusModified},
		},
		Commits: []git.Commit{
			{Subject: "Implement feature X"},
		},
	}

	prompt := buildOrderPrompt(req)

	if !strings.Contains(prompt, "cmd/main.go") {
		t.Error("prompt should contain cmd/main.go")
	}
	if !strings.Contains(prompt, "Implement feature X") {
		t.Error("prompt should contain commit message")
	}
	if !strings.Contains(prompt, "entry_point") {
		t.Error("prompt should mention category options")
	}
}

func TestProxyManager_IsRunning(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/models" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"data": [{"id": "gpt-4o", "object": "model"}]}`))
		}
	}))
	defer server.Close()

	pm := NewProxyManager(server.URL)
	if !pm.IsRunning(context.Background()) {
		t.Error("IsRunning should return true when server responds")
	}
}

func TestProxyManager_IsRunning_NotRunning(t *testing.T) {
	pm := NewProxyManager("http://localhost:59999") // Non-existent server
	if pm.IsRunning(context.Background()) {
		t.Error("IsRunning should return false when server doesn't respond")
	}
}

func TestProxyManager_WasStarted(t *testing.T) {
	pm := NewProxyManager("")
	if pm.WasStarted() {
		t.Error("WasStarted should return false initially")
	}
}

func TestProvider_Close(t *testing.T) {
	p, _ := New("", "")
	// Close should not panic even when proxy wasn't started
	p.Close()
}

func TestChat_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := chatResponse{
			Error: &struct {
				Message string `json:"message"`
			}{Message: "rate limit exceeded"},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p, _ := New(server.URL, "")
	_, err := p.SummarizeChanges(context.Background(), &provider.SummarizeRequest{
		Files: []git.FileDiff{{Path: "test.go"}},
	})

	if err == nil {
		t.Error("expected error for API error response")
	}
	if !strings.Contains(err.Error(), "rate limit exceeded") {
		t.Errorf("error should contain API error message, got: %v", err)
	}
}

func TestEnsureProxyRunning_AlreadyRunning(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/models" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"models": []}`))
		}
	}))
	defer server.Close()

	p, _ := New(server.URL, "")

	var logMessages []string
	logFn := func(format string, args ...any) {
		logMessages = append(logMessages, format)
	}

	started, err := p.EnsureProxyRunning(context.Background(), logFn)
	if err != nil {
		t.Fatalf("EnsureProxyRunning() failed: %v", err)
	}
	if started {
		t.Error("EnsureProxyRunning() should return false when proxy is already running")
	}
	if len(logMessages) != 0 {
		t.Errorf("expected no log messages when proxy is already running, got: %v", logMessages)
	}
}

func TestProxyManager_EnsureRunning_AlreadyRunning(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/models" {
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	pm := NewProxyManager(server.URL)

	var logMessages []string
	logFn := func(format string, args ...any) {
		logMessages = append(logMessages, format)
	}

	started, err := pm.EnsureRunning(context.Background(), logFn)
	if err != nil {
		t.Fatalf("EnsureRunning() failed: %v", err)
	}
	if started {
		t.Error("should return false when proxy is already running")
	}
}

func TestProxyManager_Stop_NilCmd(t *testing.T) {
	pm := NewProxyManager("")
	// Stop should not panic when cmd is nil
	pm.Stop()
}

func TestSummarizeChanges_WithMaxTokens(t *testing.T) {
	var receivedMaxTokens int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req chatRequest
		json.NewDecoder(r.Body).Decode(&req)
		receivedMaxTokens = req.MaxTokens

		resp := chatResponse{
			Choices: []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			}{
				{Message: struct {
					Content string `json:"content"`
				}{Content: `{"overview": "Test", "key_changes": []}`}},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p, _ := New(server.URL, "")
	_, err := p.SummarizeChanges(context.Background(), &provider.SummarizeRequest{
		Files:   []git.FileDiff{{Path: "test.go"}},
		Options: provider.SummarizeOptions{MaxTokens: 4096},
	})

	if err != nil {
		t.Fatalf("SummarizeChanges() failed: %v", err)
	}
	if receivedMaxTokens != 4096 {
		t.Errorf("MaxTokens = %d, want 4096", receivedMaxTokens)
	}
}

func TestBuildSummaryPrompt_EdgeCases(t *testing.T) {
	t.Run("empty commits", func(t *testing.T) {
		req := &provider.SummarizeRequest{
			Files: []git.FileDiff{{Path: "main.go"}},
		}
		prompt := buildSummaryPrompt(req)
		if strings.Contains(prompt, "## Commits") {
			t.Error("prompt should not have Commits section when commits are empty")
		}
	})

	t.Run("commit with body", func(t *testing.T) {
		req := &provider.SummarizeRequest{
			Files: []git.FileDiff{{Path: "main.go"}},
			Commits: []git.Commit{
				{ShortHash: "abc", Author: "Test", Subject: "Subject", Body: "Detailed body"},
			},
		}
		prompt := buildSummaryPrompt(req)
		if !strings.Contains(prompt, "Detailed body") {
			t.Error("prompt should include commit body")
		}
	})

	t.Run("renamed file", func(t *testing.T) {
		req := &provider.SummarizeRequest{
			Files: []git.FileDiff{{Path: "new.go", OldPath: "old.go", Status: git.StatusRenamed}},
		}
		prompt := buildSummaryPrompt(req)
		if !strings.Contains(prompt, "old.go") {
			t.Error("prompt should include old path for renamed files")
		}
	})

	t.Run("with focus", func(t *testing.T) {
		req := &provider.SummarizeRequest{
			Files:   []git.FileDiff{{Path: "main.go"}},
			Options: provider.SummarizeOptions{Focus: "security implications"},
		}
		prompt := buildSummaryPrompt(req)
		if !strings.Contains(prompt, "security implications") {
			t.Error("prompt should include focus area")
		}
	})
}

func TestBuildOrderPrompt_EdgeCases(t *testing.T) {
	t.Run("empty commits", func(t *testing.T) {
		req := &provider.OrderRequest{
			Files: []git.FileDiff{{Path: "main.go"}},
		}
		prompt := buildOrderPrompt(req)
		if strings.Contains(prompt, "Brief Context from Commits") {
			t.Error("prompt should not have Commits section when commits are empty")
		}
	})

	t.Run("renamed file", func(t *testing.T) {
		req := &provider.OrderRequest{
			Files: []git.FileDiff{{Path: "new.go", OldPath: "old.go", Status: git.StatusRenamed}},
		}
		prompt := buildOrderPrompt(req)
		if !strings.Contains(prompt, "old.go") {
			t.Error("prompt should include old path for renamed files")
		}
	})
}

func TestListModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/models" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"data": [{"id": "gpt-4o", "object": "model"}, {"id": "gpt-4", "object": "model"}]}`))
		}
	}))
	defer server.Close()

	p, _ := New(server.URL, "")
	// Trigger IsRunning to cache models
	p.proxyManager.IsRunning(context.Background())

	models, err := p.ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels() failed: %v", err)
	}

	if len(models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(models))
	}
	if models[0].ID != "gpt-4o" {
		t.Errorf("first model ID = %q, want %q", models[0].ID, "gpt-4o")
	}
}

func TestListModels_NoModels(t *testing.T) {
	pm := NewProxyManager("http://localhost:59999")
	p := &Provider{proxyManager: pm}

	_, err := p.ListModels(context.Background())
	if err == nil {
		t.Error("expected error when no models available")
	}
}

func TestSetModel(t *testing.T) {
	p, _ := New("", "")

	if p.Model() != "" {
		t.Errorf("initial model should be empty, got %q", p.Model())
	}

	p.SetModel("gpt-4o")
	if p.Model() != "gpt-4o" {
		t.Errorf("Model() = %q, want %q", p.Model(), "gpt-4o")
	}

	p.SetModel("claude-3.5-sonnet")
	if p.Model() != "claude-3.5-sonnet" {
		t.Errorf("Model() = %q, want %q", p.Model(), "claude-3.5-sonnet")
	}
}

func TestProxyManager_Models(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/models" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"data": [{"id": "model-1", "object": "model"}, {"id": "model-2", "object": "model"}]}`))
		}
	}))
	defer server.Close()

	pm := NewProxyManager(server.URL)

	// Initially should return empty slice
	models := pm.Models()
	if len(models) != 0 {
		t.Errorf("expected empty models initially, got %d", len(models))
	}

	// Trigger IsRunning to cache models
	pm.IsRunning(context.Background())

	models = pm.Models()
	if len(models) != 2 {
		t.Fatalf("expected 2 models after IsRunning, got %d", len(models))
	}
	if models[0].ID != "model-1" {
		t.Errorf("first model ID = %q, want %q", models[0].ID, "model-1")
	}
}

func TestProxyManager_Models_ReturnsCopy(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/models" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"data": [{"id": "model-1", "object": "model"}]}`))
		}
	}))
	defer server.Close()

	pm := NewProxyManager(server.URL)
	pm.IsRunning(context.Background())

	models := pm.Models()
	// Modify the returned slice
	models[0].ID = "modified"

	// Original should be unchanged
	originalModels := pm.Models()
	if originalModels[0].ID != "model-1" {
		t.Error("Models() should return a copy, not the original slice")
	}
}
