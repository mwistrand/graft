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
  cli/               → Cobra commands (review.go is the main command)
  config/            → Config loading from ~/.config/graft/config.json
  git/               → Git operations (shells out to git binary)
  analysis/          → Repository structure analysis for smarter ordering
  provider/          → AI provider abstraction
    claude/          → Anthropic Claude API implementation
    copilot/         → GitHub Copilot via copilot-api proxy
    mock/            → Testing mock
  render/            → Output rendering (Delta subprocess or fallback)
```

### Key Patterns

**Provider Interface**: All AI providers implement `provider.Provider`:
```go
type Provider interface {
    Name() string
    SummarizeChanges(ctx, req) (*SummarizeResponse, error)
    OrderFiles(ctx, req) (*OrderResponse, error)
}
```

**Repository Analysis**: The `analysis` package scans repo structure to detect project type (frontend/backend/fullstack) and frameworks, caching results at `.graft/analysis.json`.

**Copilot Proxy**: The copilot provider auto-starts `npx copilot-api@latest` if not running, with a 2-minute timeout for GitHub authentication.

### Adding a New Provider

1. Create `internal/provider/newprovider/newprovider.go`
2. Implement the `Provider` interface
3. Add case in `cli/review.go:initProvider()`
4. Add config keys in `config/config.go`

## Configuration

Config file: `~/.config/graft/config.json`

Key settings:
- `provider`: "claude" or "copilot"
- `anthropic-api-key`: For Claude
- `copilot-base-url`: For Copilot proxy (default: http://localhost:4141)

Environment overrides: `ANTHROPIC_API_KEY`, `COPILOT_BASE_URL`, `GRAFT_PROVIDER`
