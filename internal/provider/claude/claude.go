// Package claude provides an AI provider implementation using Anthropic's Claude API.
package claude

import (
	"context"
	"errors"
	"fmt"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"github.com/mwistrand/graft/internal/provider"
)

// AvailableModels defines the Claude models available for selection.
var AvailableModels = []provider.ModelInfo{
	{ID: "claude-opus-4-5-20250514", Name: "Claude Opus 4.5", Description: "Most capable model for complex tasks"},
	{ID: "claude-sonnet-4-20250514", Name: "Claude Sonnet 4", Description: "Balanced performance and speed"},
	{ID: "claude-haiku-3-5-20241022", Name: "Claude Haiku 3.5", Description: "Fast and efficient for simpler tasks"},
}

// Provider implements the provider.Provider interface using Claude.
type Provider struct {
	client anthropic.Client
	model  anthropic.Model
}

// New creates a new Claude provider with the given API key and model.
// Model can be empty initially and set later via SetModel.
func New(apiKey, model string) (*Provider, error) {
	if apiKey == "" {
		return nil, errors.New("anthropic API key is required")
	}

	client := anthropic.NewClient(option.WithAPIKey(apiKey))

	return &Provider{
		client: client,
		model:  anthropic.Model(model),
	}, nil
}

// ListModels returns the available Claude models.
func (p *Provider) ListModels(ctx context.Context) ([]provider.ModelInfo, error) {
	return AvailableModels, nil
}

// SetModel updates the model used by this provider.
func (p *Provider) SetModel(model string) {
	p.model = anthropic.Model(model)
}

// Model returns the currently configured model.
func (p *Provider) Model() string {
	return string(p.model)
}

// effectiveModel returns the model to use for a request.
// Per-request model overrides the provider default.
func (p *Provider) effectiveModel(reqModel string) anthropic.Model {
	if reqModel != "" {
		return anthropic.Model(reqModel)
	}
	return p.model
}

// Name returns "claude".
func (p *Provider) Name() string {
	return "claude"
}

// SummarizeChanges analyzes a diff and returns a structured summary.
func (p *Provider) SummarizeChanges(ctx context.Context, req *provider.SummarizeRequest) (*provider.SummarizeResponse, error) {
	prompt := provider.BuildSummaryPrompt(req)

	maxTokens := req.Options.MaxTokens
	if maxTokens == 0 {
		maxTokens = 2048
	}

	resp, err := p.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     p.effectiveModel(req.Model),
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
	if err := provider.ParseJSONResponse(text, &summary); err != nil {
		return nil, fmt.Errorf("parsing summary response: %w", err)
	}

	return &summary, nil
}

// OrderFiles determines the logical review order for changed files.
func (p *Provider) OrderFiles(ctx context.Context, req *provider.OrderRequest) (*provider.OrderResponse, error) {
	prompt := provider.BuildOrderPrompt(req)

	resp, err := p.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     p.effectiveModel(req.Model),
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
	if err := provider.ParseJSONResponse(text, &order); err != nil {
		return nil, fmt.Errorf("parsing order response: %w", err)
	}

	return &order, nil
}

// ReviewChanges performs a detailed code review of the changes.
func (p *Provider) ReviewChanges(ctx context.Context, req *provider.ReviewRequest) (*provider.ReviewResponse, error) {
	prompt := provider.BuildReviewPrompt(req)

	maxTokens := req.Options.MaxTokens
	if maxTokens == 0 {
		maxTokens = 8192
	}

	params := anthropic.MessageNewParams{
		Model:     p.effectiveModel(req.Model),
		MaxTokens: int64(maxTokens),
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(prompt)),
		},
	}

	// Add system prompt if provided
	if req.SystemPrompt != "" {
		params.System = []anthropic.TextBlockParam{
			{Text: req.SystemPrompt},
		}
	}

	resp, err := p.client.Messages.New(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("claude API error: %w", err)
	}

	text := extractTextContent(resp)
	if text == "" {
		return nil, errors.New("empty response from Claude")
	}

	// Parse structured review (falls back to raw content on parse error)
	return provider.ParseStructuredReview(text), nil
}

// QuickReview performs a fast initial assessment of changes.
func (p *Provider) QuickReview(ctx context.Context, req *provider.QuickReviewRequest) (*provider.QuickReviewResponse, error) {
	prompt := provider.BuildQuickReviewPrompt(req)

	resp, err := p.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     p.effectiveModel(req.Model),
		MaxTokens: int64(1024),
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

	return provider.ParseQuickReviewResponse(text)
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
