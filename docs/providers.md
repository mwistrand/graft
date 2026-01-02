# Adding New AI Providers

This guide explains how to add a new AI provider to Graft.

## Provider Interface

All providers must implement the `Provider` interface defined in `internal/provider/provider.go`:

```go
type Provider interface {
    // Name returns the provider identifier (e.g., "claude", "openai")
    Name() string

    // SummarizeChanges analyzes a diff and returns a structured summary
    SummarizeChanges(ctx context.Context, req *SummarizeRequest) (*SummarizeResponse, error)

    // OrderFiles determines the logical review order for changed files
    OrderFiles(ctx context.Context, req *OrderRequest) (*OrderResponse, error)
}
```

## Step-by-Step Guide

### 1. Create Provider Package

Create a new directory under `internal/provider/`:

```
internal/provider/openai/
├── openai.go      # Main implementation
├── openai_test.go # Tests
└── prompts.go     # Provider-specific prompts (optional)
```

### 2. Implement the Provider

```go
// internal/provider/openai/openai.go
package openai

import (
    "context"
    "github.com/mwistrand/graft/internal/provider"
)

type Provider struct {
    client *openai.Client
    model  string
}

func New(apiKey, model string) (*Provider, error) {
    if apiKey == "" {
        return nil, errors.New("OpenAI API key is required")
    }
    // Initialize client...
    return &Provider{client: client, model: model}, nil
}

func (p *Provider) Name() string {
    return "openai"
}

func (p *Provider) SummarizeChanges(ctx context.Context, req *provider.SummarizeRequest) (*provider.SummarizeResponse, error) {
    // Build prompt from req.Files, req.Commits, req.FullDiff
    // Call OpenAI API
    // Parse response into SummarizeResponse
}

func (p *Provider) OrderFiles(ctx context.Context, req *provider.OrderRequest) (*provider.OrderResponse, error) {
    // Build prompt from req.Files, req.Commits
    // Call OpenAI API
    // Parse response into OrderResponse
}
```

### 3. Add Configuration Support

Update `internal/config/config.go`:

```go
type Config struct {
    // ... existing fields ...
    OpenAIAPIKey string `json:"openai_api_key,omitempty"`
}

func (c *Config) applyEnvOverrides() {
    // ... existing overrides ...
    if v := os.Getenv("OPENAI_API_KEY"); v != "" {
        c.OpenAIAPIKey = v
    }
}

func (c *Config) Validate() error {
    switch c.Provider {
    // ... existing cases ...
    case "openai":
        if c.OpenAIAPIKey == "" {
            return errors.New("OpenAI API key not set")
        }
    }
}
```

### 4. Register Provider in CLI

Update `internal/cli/review.go`:

```go
import (
    "github.com/mwistrand/graft/internal/provider/openai"
)

func initProvider(cfg *config.Config) (provider.Provider, error) {
    switch cfg.Provider {
    case "claude", "":
        return claude.New(cfg.AnthropicAPIKey, cfg.Model)
    case "openai":
        return openai.New(cfg.OpenAIAPIKey, cfg.Model)
    default:
        return nil, fmt.Errorf("unknown provider %q", cfg.Provider)
    }
}
```

### 5. Write Tests

Create comprehensive tests in `openai_test.go`:

```go
func TestNew(t *testing.T) {
    // Test with valid API key
    // Test with empty API key
    // Test with custom model
}

func TestSummarizeChanges(t *testing.T) {
    // Use httptest to mock API responses
    // Test successful response parsing
    // Test error handling
}

func TestOrderFiles(t *testing.T) {
    // Similar to SummarizeChanges tests
}
```

## Response Format

Providers must return responses that match these structures:

### SummarizeResponse

```go
type SummarizeResponse struct {
    Overview   string      `json:"overview"`
    KeyChanges []string    `json:"key_changes"`
    Concerns   []string    `json:"concerns,omitempty"`
    FileGroups []FileGroup `json:"file_groups,omitempty"`
}

type FileGroup struct {
    Name        string   `json:"name"`
    Description string   `json:"description"`
    Files       []string `json:"files"`
}
```

### OrderResponse

```go
type OrderResponse struct {
    Files     []OrderedFile `json:"files"`
    Reasoning string        `json:"reasoning"`
}

type OrderedFile struct {
    Path        string `json:"path"`
    Category    string `json:"category"`  // Use constants from provider package
    Priority    int    `json:"priority"`
    Description string `json:"description"`
}
```

## Prompt Guidelines

When designing prompts for your provider:

1. **Request JSON output** - Include explicit JSON schema in the prompt
2. **Handle truncation** - Large diffs may need to be truncated
3. **Include context** - Provide commit messages for better understanding
4. **Be specific** - Define what each field should contain

See `internal/provider/claude/prompts.go` for reference implementations.

## Testing Checklist

Before submitting a new provider:

- [ ] All tests pass
- [ ] Handles API errors gracefully
- [ ] Respects context cancellation
- [ ] Validates API key before making requests
- [ ] Parses responses correctly (including edge cases)
- [ ] Works with `--provider` flag
- [ ] Configuration documented in help text
- [ ] Added to provider list in error messages
