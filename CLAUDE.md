# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Graft is an AI-powered code review CLI that presents diffs in logical order. It uses AI (Claude or GitHub Copilot) to summarize changes and determine optimal file review order based on project architecture.

## Common Commands

```bash
# Build
make build              # Build binary to ./graft
go build ./...          # Compile all packages

# Test
make test               # Run all tests
go test ./...           # Same as above
go test ./internal/analysis/...  # Run tests for specific package
go test -v -run TestName ./internal/cli  # Run single test

# Lint/Format
make fmt                # Format code
make lint               # Run golangci-lint (must be installed)
```

## Architecture

The codebase follows a clean layered architecture:

```
cmd/graft/           → Entry point
internal/
  cli/               → Cobra commands
    review.go        → Review command flags and entry point
    review_runner.go → reviewRunner workflow orchestration
    review_helpers.go→ Provider init, file ordering, repo analysis helpers
    review_output.go → AI review and quick review output formatting
  config/            → Config loading from ~/.config/graft/config.json
  git/               → Git operations (shells out to git binary)
  analysis/          → Repository structure analysis for smarter ordering
  pr/                → PR URL parsing and GitHub CLI integration
  prompt/            → Interactive terminal prompts
  provider/          → AI provider abstraction
    claude/          → Anthropic Claude API implementation
    copilot/         → GitHub Copilot via copilot-api proxy
    mock/            → Testing mock
    testpair/        → Test/implementation file pairing utilities
  render/            → Output rendering (Delta subprocess or fallback)
```

### Key Patterns

**Provider Interface**: All AI providers implement `provider.Provider`:
```go
type Provider interface {
    Name() string
    SummarizeChanges(ctx, req) (*SummarizeResponse, error)
    OrderFiles(ctx, req) (*OrderResponse, error)
    ReviewChanges(ctx, req) (*ReviewResponse, error)
    QuickReview(ctx, req) (*QuickReviewResponse, error)
}
```

**File Grouping**: The `OrderFiles` response groups related files by feature:
```go
type OrderResponse struct {
    Files     []OrderedFile  // Files with Group field
    Groups    []OrderGroup   // Group metadata (name, description, priority, significance)
    Reasoning string
}

type OrderGroup struct {
    Name         string       // Group identifier
    Description  string       // What this feature/change accomplishes
    Priority     int          // Group review order (1 = first)
    Significance Significance // Importance tier: core, supporting, minor
}
```
The AI identifies logical feature groups, assigns each file to a group, and classifies groups by significance:
- **core**: Major logic changes (new features, API changes, business logic)
- **supporting**: Tests, utilities, helpers
- **minor**: Config files, docs, formatting, dependency updates

Use `--major-only` to skip minor groups entirely.

**Repository Analysis**: The `analysis` package scans repo structure to detect project type (frontend/backend/fullstack) and frameworks, caching results at `.graft/analysis.json`.

**Review Caching**: AI responses (summary, ordering, and code reviews) are cached in `.graft/reviews/<key>.json` where the key is derived from commit hashes. This allows instant re-reviews of the same commits. Use `--refresh` to bypass the cache.
```go
// Cache key is generated from base ref + sorted commit hashes
cacheKey := provider.GenerateCacheKey(baseRef, commits)
```

**AI Code Review**: The `--ai-review` flag generates structured code reviews with categories (design, functionality, complexity, tests, naming, comments, style, documentation, praise) and severity levels (critical, suggestion, nit). Use `--ai-review` alone to output to console, or `--ai-review=path/to/file.md` to write to a file. Use `--review-categories` to focus on specific categories and `--review-severity` to filter output. Custom system prompts can be placed at `.graft/code-reviewer.md` to override the default review approach.

**Copilot Proxy**: The copilot provider auto-starts `npx copilot-api@latest` if not running, with a 2-minute timeout for GitHub authentication.

### Adding a New Provider

1. Create `internal/provider/newprovider/newprovider.go`
2. Implement the `Provider` interface
3. Add case in `cli/review_helpers.go:initProvider()`
4. Add config keys in `config/config.go`

## Configuration

Config file: `~/.config/graft/config.json`

### Provider Settings
- `provider`: "claude" or "copilot"
- `model`: Default model to use (if not set, interactive prompt appears)
- `review-model`: Model for review tasks (summarize, review, quick review)
- `order-model`: Model for file ordering
- `anthropic-api-key`: For Claude provider
- `openai-api-key`: For OpenAI provider
- `copilot-base-url`: For Copilot proxy (default: http://localhost:4141)

### Review Preferences

These CLI flags can be persisted in config to set defaults:

```bash
graft config set tests-first true       # Show test files before implementation
graft config set inline-tests true      # Show test files alongside implementation
graft config set no-delta true          # Disable Delta rendering
graft config set no-analyze true        # Skip repository analysis
graft config set major-only true        # Only review core/supporting groups
graft config set review-categories "design,functionality"  # Focus AI review
graft config set review-severity "critical"  # Filter review output
graft config set prompt-timeout 60      # Timeout in minutes (0 = disable)
```

CLI flags always override config values.

### Model Selection

There is no default model. If no model is configured:
- An interactive model selection prompt appears
- Use `--model <name>` to specify directly
- Use `--select-model` to force the prompt even when a model is configured

Per-task model overrides allow using different models for different operations:
- `--review-model <name>`: Model for summarize, review, and quick review tasks
- `--order-model <name>`: Model for file ordering

When using task-specific models, either `--model` or both task-specific models must be set so all tasks are covered. Skip flags (`--no-order`, `--no-summary`) reduce what's required.

```bash
graft review main                                          # Prompts for model if not configured
graft review main --model gpt-4o                           # Use specific model for all tasks
graft review main --select-model                           # Force model selection prompt
graft review main --review-model gpt-4o --order-model gpt-3.5  # Different models per task
graft review main --model gpt-4 --review-model gpt-4o     # Override just review tasks
graft config set review-model gpt-4o                       # Persist review model
graft config set order-model gpt-3.5                       # Persist order model
```

### Environment Variables

All config options can be overridden via environment variables:

| Config Key | Environment Variable |
|------------|---------------------|
| `provider` | `GRAFT_PROVIDER` |
| `model` | `GRAFT_MODEL` |
| `review-model` | `GRAFT_REVIEW_MODEL` |
| `order-model` | `GRAFT_ORDER_MODEL` |
| `anthropic-api-key` | `ANTHROPIC_API_KEY` |
| `openai-api-key` | `OPENAI_API_KEY` |
| `copilot-base-url` | `COPILOT_BASE_URL` |
| `delta-path` | `GRAFT_DELTA_PATH` |
| `prompt-timeout` | `GRAFT_PROMPT_TIMEOUT` |
| `tests-first` | `GRAFT_TESTS_FIRST` |
| `inline-tests` | `GRAFT_INLINE_TESTS` |
| `no-delta` | `GRAFT_NO_DELTA` |
| `no-analyze` | `GRAFT_NO_ANALYZE` |
| `major-only` | `GRAFT_MAJOR_ONLY` |
| `review-categories` | `GRAFT_REVIEW_CATEGORIES` |
| `review-severity` | `GRAFT_REVIEW_SEVERITY` |

### Prompt Timeout

Interactive prompts (like "Continue reviewing diffs?") will timeout after 30 minutes by default. This prevents orphaned graft processes when users forget to respond. Configure via:

```bash
# CLI flag (per-invocation)
graft review main --prompt-timeout 60  # 60 minutes
graft review main --prompt-timeout 0   # Disable timeout

# Config file (persistent)
graft config set prompt-timeout 60

# Environment variable
export GRAFT_PROMPT_TIMEOUT=60
```
