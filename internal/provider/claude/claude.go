// Package claude provides an AI provider implementation using Anthropic's Claude API.
package claude

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/mwistrand/graft/internal/provider"
)

// DefaultModel is the default Claude model to use.
const DefaultModel = "claude-sonnet-4-20250514"

// Provider implements the provider.Provider interface using Claude.
type Provider struct {
	client anthropic.Client
	model  anthropic.Model
}

// New creates a new Claude provider with the given API key and model.
// If model is empty, DefaultModel is used.
func New(apiKey, model string) (*Provider, error) {
	if apiKey == "" {
		return nil, errors.New("anthropic API key is required")
	}

	if model == "" {
		model = DefaultModel
	}

	client := anthropic.NewClient(option.WithAPIKey(apiKey))

	return &Provider{
		client: client,
		model:  anthropic.Model(model),
	}, nil
}

// Name returns "claude".
func (p *Provider) Name() string {
	return "claude"
}

// SummarizeChanges analyzes a diff and returns a structured summary.
func (p *Provider) SummarizeChanges(ctx context.Context, req *provider.SummarizeRequest) (*provider.SummarizeResponse, error) {
	prompt := buildSummaryPrompt(req)

	maxTokens := req.Options.MaxTokens
	if maxTokens == 0 {
		maxTokens = 2048
	}

	resp, err := p.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     p.model,
		MaxTokens: int64(maxTokens),
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(prompt)),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("claude API error: %w", err)
	}

	// Extract text content from response
	text := extractTextContent(resp)
	if text == "" {
		return nil, errors.New("empty response from Claude")
	}

	// Parse JSON response
	var summary provider.SummarizeResponse
	if err := parseJSONResponse(text, &summary); err != nil {
		return nil, fmt.Errorf("parsing summary response: %w", err)
	}

	return &summary, nil
}

// OrderFiles determines the logical review order for changed files.
func (p *Provider) OrderFiles(ctx context.Context, req *provider.OrderRequest) (*provider.OrderResponse, error) {
	prompt := buildOrderPrompt(req)

	resp, err := p.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     p.model,
		MaxTokens: int64(2048),
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(prompt)),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("claude API error: %w", err)
	}

	text := extractTextContent(resp)
	if text == "" {
		return nil, errors.New("empty response from Claude")
	}

	var order provider.OrderResponse
	if err := parseJSONResponse(text, &order); err != nil {
		return nil, fmt.Errorf("parsing order response: %w", err)
	}

	return &order, nil
}

// extractTextContent extracts the text content from a Claude response.
func extractTextContent(resp *anthropic.Message) string {
	for _, block := range resp.Content {
		if block.Type == "text" {
			return block.Text
		}
	}
	return ""
}

// parseJSONResponse extracts and parses JSON from Claude's response.
// It handles cases where JSON is wrapped in markdown code blocks.
func parseJSONResponse(text string, v any) error {
	// Try to extract JSON from markdown code blocks
	jsonStr := extractJSON(text)

	if err := json.Unmarshal([]byte(jsonStr), v); err != nil {
		return fmt.Errorf("invalid JSON: %w\nResponse was: %s", err, text)
	}

	return nil
}

func extractJSON(text string) string {
	// Look for JSON code block
	start := strings.Index(text, "```json")
	if start != -1 {
		start += 7 // len("```json")
		end := strings.Index(text[start:], "```")
		if end != -1 {
			return strings.TrimSpace(text[start : start+end])
		}
	}

	// Look for generic code block
	start = strings.Index(text, "```")
	if start != -1 {
		start += 3 // len("```")
		// Skip language identifier if present
		if nl := strings.Index(text[start:], "\n"); nl != -1 {
			start += nl + 1
		}
		end := strings.Index(text[start:], "```")
		if end != -1 {
			return strings.TrimSpace(text[start : start+end])
		}
	}

	// Look for raw JSON (starts with { or [)
	for i := 0; i < len(text); i++ {
		if text[i] == '{' || text[i] == '[' {
			return strings.TrimSpace(text[i:])
		}
	}

	return strings.TrimSpace(text)
}
