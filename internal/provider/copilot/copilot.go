// Package copilot provides an AI provider that connects to a copilot-api proxy server.
// The proxy exposes GitHub Copilot through OpenAI-compatible endpoints.
// See https://github.com/ericc-ch/copilot-api for proxy setup.
package copilot

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/mwistrand/graft/internal/provider"
)

const (
	// DefaultBaseURL is the default URL for the copilot-api proxy.
	DefaultBaseURL = "http://localhost:4141"

	// DefaultModel is the default model to use with the proxy.
	DefaultModel = "gpt-4"
)

// Provider implements the provider.Provider interface using a copilot-api proxy.
type Provider struct {
	baseURL      string
	model        string
	client       *http.Client
	proxyManager *ProxyManager
}

// New creates a new Copilot provider with the given base URL and model.
// If model is empty, it will remain empty to allow for interactive selection.
func New(baseURL, model string) (*Provider, error) {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	baseURL = strings.TrimSuffix(baseURL, "/")

	return &Provider{
		baseURL:      baseURL,
		model:        model,
		client:       &http.Client{},
		proxyManager: NewProxyManager(baseURL),
	}, nil
}

// EnsureProxyRunning starts the copilot-api proxy if it's not already running.
// The logFn is called with status messages. Returns true if the proxy was started.
func (p *Provider) EnsureProxyRunning(ctx context.Context, logFn func(string, ...any)) (bool, error) {
	return p.proxyManager.EnsureRunning(ctx, logFn)
}

// Close stops the proxy if it was started by this provider.
func (p *Provider) Close() {
	if p.proxyManager != nil && p.proxyManager.WasStarted() {
		p.proxyManager.Stop()
	}
}

// ListModels returns the available models from the copilot-api proxy.
// It uses the cached models from the proxy manager (populated during readiness check).
func (p *Provider) ListModels(ctx context.Context) ([]provider.ModelInfo, error) {
	models := p.proxyManager.Models()
	if len(models) == 0 {
		return nil, fmt.Errorf("no models available (proxy may not be ready)")
	}
	return models, nil
}

// Name returns "copilot".
func (p *Provider) Name() string {
	return "copilot"
}

// SetModel updates the model used by this provider.
func (p *Provider) SetModel(model string) {
	p.model = model
}

// Model returns the currently configured model.
func (p *Provider) Model() string {
	return p.model
}

// chatRequest represents an OpenAI-compatible chat completion request.
type chatRequest struct {
	Model     string        `json:"model"`
	Messages  []chatMessage `json:"messages"`
	MaxTokens int           `json:"max_tokens,omitempty"`
}

// chatMessage represents a message in the chat request.
type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// chatResponse represents an OpenAI-compatible chat completion response.
type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// SummarizeChanges analyzes a diff and returns a structured summary.
func (p *Provider) SummarizeChanges(ctx context.Context, req *provider.SummarizeRequest) (*provider.SummarizeResponse, error) {
	prompt := buildSummaryPrompt(req)

	maxTokens := req.Options.MaxTokens
	if maxTokens == 0 {
		maxTokens = 2048
	}

	text, err := p.chat(ctx, prompt, maxTokens)
	if err != nil {
		return nil, err
	}

	var summary provider.SummarizeResponse
	if err := parseJSONResponse(text, &summary); err != nil {
		return nil, fmt.Errorf("parsing summary response: %w", err)
	}

	return &summary, nil
}

// OrderFiles determines the logical review order for changed files.
func (p *Provider) OrderFiles(ctx context.Context, req *provider.OrderRequest) (*provider.OrderResponse, error) {
	prompt := buildOrderPrompt(req)

	text, err := p.chat(ctx, prompt, 2048)
	if err != nil {
		return nil, err
	}

	var order provider.OrderResponse
	if err := parseJSONResponse(text, &order); err != nil {
		return nil, fmt.Errorf("parsing order response: %w", err)
	}

	return &order, nil
}

// chat sends a message to the copilot-api proxy and returns the response text.
func (p *Provider) chat(ctx context.Context, prompt string, maxTokens int) (string, error) {
	reqBody := chatRequest{
		Model: p.model,
		Messages: []chatMessage{
			{Role: "user", Content: prompt},
		},
		MaxTokens: maxTokens,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshaling request: %w", err)
	}

	url := p.baseURL + "/v1/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("copilot API error: %w (is copilot-api proxy running at %s?)", err, p.baseURL)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("copilot API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var chatResp chatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return "", fmt.Errorf("parsing response: %w", err)
	}

	if chatResp.Error != nil {
		return "", fmt.Errorf("copilot API error: %s", chatResp.Error.Message)
	}

	if len(chatResp.Choices) == 0 {
		return "", errors.New("empty response from copilot API")
	}

	return chatResp.Choices[0].Message.Content, nil
}

// parseJSONResponse extracts and parses JSON from the response text.
func parseJSONResponse(text string, v any) error {
	jsonStr := extractJSON(text)
	if err := json.Unmarshal([]byte(jsonStr), v); err != nil {
		return fmt.Errorf("invalid JSON: %w\nResponse was: %s", err, text)
	}
	return nil
}

// extractJSON extracts JSON content from a string that may contain markdown.
func extractJSON(text string) string {
	// Look for JSON code block
	start := strings.Index(text, "```json")
	if start != -1 {
		start += 7
		end := strings.Index(text[start:], "```")
		if end != -1 {
			return strings.TrimSpace(text[start : start+end])
		}
	}

	// Look for generic code block
	start = strings.Index(text, "```")
	if start != -1 {
		start += 3
		if nl := strings.Index(text[start:], "\n"); nl != -1 {
			start += nl + 1
		}
		end := strings.Index(text[start:], "```")
		if end != -1 {
			return strings.TrimSpace(text[start : start+end])
		}
	}

	// Look for raw JSON
	for i := 0; i < len(text); i++ {
		if text[i] == '{' || text[i] == '[' {
			return strings.TrimSpace(text[i:])
		}
	}

	return strings.TrimSpace(text)
}
