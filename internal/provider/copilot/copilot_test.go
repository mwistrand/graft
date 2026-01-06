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
