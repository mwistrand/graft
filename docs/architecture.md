# Graft Architecture

This document describes the internal architecture of Graft, an AI-powered code review CLI.

## Package Overview

```
graft/
├── cmd/graft/          # Application entry point
│   └── main.go
├── internal/
│   ├── cli/            # Command-line interface (Cobra)
│   ├── config/         # Configuration management
│   ├── git/            # Git operations
│   ├── provider/       # AI provider abstraction
│   │   ├── claude/     # Claude/Anthropic implementation
│   │   └── mock/       # Mock provider for testing
│   └── render/         # Output rendering (Delta/fallback)
└── docs/               # Documentation
```

## Data Flow

```
                            ┌─────────────────┐
                            │  User Command   │
                            │ graft review X  │
                            └────────┬────────┘
                                     │
                                     ▼
┌─────────────────────────────────────────────────────────────────┐
│                         CLI Layer                                │
│  • Parse arguments (base branch)                                 │
│  • Load configuration                                            │
│  • Validate environment                                          │
└────────────────────────────┬────────────────────────────────────┘
                             │
              ┌──────────────┼──────────────┐
              │              │              │
              ▼              ▼              ▼
        ┌─────────┐    ┌─────────┐    ┌─────────┐
        │   Git   │    │Provider │    │ Render  │
        │  Layer  │    │  Layer  │    │  Layer  │
        └────┬────┘    └────┬────┘    └────┬────┘
             │              │              │
             ▼              ▼              ▼
      ┌───────────┐  ┌───────────┐  ┌───────────┐
      │ DiffResult│  │ Summary   │  │ Terminal  │
      │ Commits   │  │ Ordering  │  │ Output    │
      └───────────┘  └───────────┘  └───────────┘
```

## Package Details

### cmd/graft

The entry point. Sets build-time version info and calls the CLI executor.

### internal/cli

Cobra-based CLI implementation. Key files:
- `root.go` - Root command, global flags, config loading
- `review.go` - Main review command orchestration
- `config.go` - Configuration management subcommands
- `version.go` - Version display

### internal/config

Configuration management:
- Loads from `~/.config/graft/config.json`
- Supports environment variable overrides
- Validates provider-specific requirements

### internal/git

Git operations abstraction:
- `Repository` - Main interface to git commands
- `GetDiff` - Extracts diff between branches
- `GetCommits` - Retrieves commit messages
- Shell-based implementation (calls git binary)

### internal/provider

AI provider abstraction layer:

```go
type Provider interface {
    Name() string
    SummarizeChanges(ctx, req) (*SummarizeResponse, error)
    OrderFiles(ctx, req) (*OrderResponse, error)
}
```

Implementations:
- `claude/` - Anthropic Claude API
- `mock/` - Testing mock

### internal/render

Output rendering:
- `Renderer` interface for display operations
- `deltaRenderer` - Pipes output through Delta
- `fallbackRenderer` - Basic terminal output
- Summary and ordering display formatting

## Key Design Decisions

### 1. Provider Abstraction

The `Provider` interface allows swapping AI backends without changing application code. This enables:
- Adding new providers (OpenAI, Copilot, local models)
- Testing with mock providers
- User choice of backend

### 2. Shell-Based Git

We shell out to git rather than using go-git because:
- Simpler implementation
- Better compatibility with user's git config
- Easier Delta integration (needs git color output)

### 3. Delta as Subprocess

Delta is called as a subprocess with piped input:
```
git diff --color=always | delta
```

This respects user's Delta configuration and provides beautiful output without embedding Delta's rendering logic.

### 4. Graceful Degradation

The tool degrades gracefully when:
- Delta not installed → fallback to basic diff
- API key not set → skip AI analysis
- AI API fails → use default file ordering

## Extension Points

### Adding a New Provider

1. Create `internal/provider/newprovider/newprovider.go`
2. Implement the `Provider` interface
3. Add initialization in `cli/review.go:initProvider()`
4. Update config to support new API keys

See `docs/providers.md` for detailed instructions.

### Adding New Commands

1. Create `internal/cli/newcmd.go`
2. Define Cobra command
3. Add to root command in `init()`

### Customizing Rendering

Implement the `Renderer` interface in `internal/render/` for custom output formats.
