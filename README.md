# Graft CLI

**AI-powered code review CLI that presents diffs in logical order**

Graft helps you review git branches by:
1. Summarizing changes using AI (with commit message context)
2. Ordering files by architectural layers (entry points → business logic → adapters)
3. Providing a beautiful diff view powered by [Delta](https://github.com/dandavison/delta)

## The Problem

When reviewing a PR with 30 changed files, you usually see them in alphabetical order:
```
adapters/user_repository.go
controllers/users_controller.go
models/user.go
services/user_service.go
```

But that's backwards! You want to understand *what* the change does before diving into *how*.

## The Solution

Graft reorders files by **architectural flow**:
```
=== Review Order ===

Files ordered by architectural flow: entry points first, then business logic, then adapters.

  1. → controllers/users_controller.go
      Main HTTP handler for user endpoints
  2. ◆ services/user_service.go
      Core user business logic
  3. ● models/user.go
      User domain model
  4. ◇ adapters/user_repository.go
      Database adapter for user persistence
```

## Installation

### Prerequisites

- Go 1.21+
- [Delta](https://github.com/dandavison/delta) (recommended for beautiful diffs)
- Git
- One of the following AI backends:
  - Claude API key from [Anthropic](https://console.anthropic.com/)
  - GitHub Copilot subscription with [copilot-api](https://github.com/ericc-ch/copilot-api) proxy

### From Source

```bash
git clone https://github.com/mwistrand/graft
cd graft
make build
sudo mv graft /usr/local/bin/
```

Or use `go install`:

```bash
go install github.com/mwistrand/graft/cmd/graft@latest
```

### Install Delta (recommended)

```bash
# macOS
brew install git-delta

# Ubuntu/Debian
sudo apt install git-delta

# Arch Linux
sudo pacman -S git-delta
```

## Quick Start

### Option A: Using Claude (default)

1. **Set your API key:**
   ```bash
   graft config set anthropic-api-key sk-ant-...
   # Or use environment variable:
   export ANTHROPIC_API_KEY=sk-ant-...
   ```

2. **Review a branch:**
   ```bash
   graft review main
   ```

### Option B: Using GitHub Copilot

1. **Set the provider:**
   ```bash
   graft config set provider copilot
   ```

2. **Review a branch:**
   ```bash
   graft review main
   ```

   On first run, graft will:
   - Automatically start the copilot-api proxy (requires Node.js)
   - Prompt you to authenticate with GitHub if needed
   - Display an interactive model selector if no model is configured

3. **Select a model** (if prompted):
   ```
   Select a model
   Use arrow keys to navigate, enter to select

   > gpt-4o
     gpt-4
     claude-3.5-sonnet
     o1-mini
   ```

Graft will wait for your selection before proceeding with the review.

## Usage

### Basic Review

```bash
# Review current branch against main
graft review main

# Review against a specific branch
graft review origin/develop

# Review the last 5 commits
graft review HEAD~5
```

### Options

```bash
# Skip AI summary (faster)
graft review main --no-summary

# Skip AI ordering (use default order)
graft review main --no-order

# Disable Delta rendering
graft review main --no-delta

# Use a specific AI provider
graft review main --provider claude

# Use a specific model (skips interactive selection)
graft review main --model gpt-4o

# Show tests before implementation files
graft review main --tests-first
```

### Interactive Model Selection

When using the Copilot provider without a configured model, graft displays an interactive model selector after the proxy is ready. The selector:

- Lists all available models from the Copilot API
- Waits indefinitely for your selection (no timeout)
- Can be bypassed by setting a model via `--model` flag, config file, or `GRAFT_MODEL` environment variable

### Configuration

```bash
# Show current configuration
graft config

# Set a configuration value
graft config set provider claude
graft config set anthropic-api-key sk-ant-...

# Get a configuration value
graft config get provider

# Show config file path
graft config path
```

### Available Configuration Keys

| Key | Description | Environment Variable |
|-----|-------------|---------------------|
| `provider` | AI provider (claude, copilot) | `GRAFT_PROVIDER` |
| `model` | Model name | `GRAFT_MODEL` |
| `anthropic-api-key` | Anthropic API key | `ANTHROPIC_API_KEY` |
| `copilot-base-url` | Copilot proxy URL (default: http://localhost:4141) | `COPILOT_BASE_URL` |
| `delta-path` | Path to Delta binary | `GRAFT_DELTA_PATH` |

## How It Works

1. **Analyze Changes**: Graft gets the diff between your branch and the base branch, along with all commit messages.

2. **AI Summary**: Claude analyzes the changes and provides:
   - A high-level overview
   - Key changes (bullet points)
   - Potential concerns or risks
   - Logical file groupings

3. **Intelligent Ordering**: While you read the summary, graft determines the best order to review files based on:
   - Configuration and constants first (set context)
   - Types and interfaces (understand the domain)
   - Entry points (main, handlers, CLI commands)
   - Core business logic
   - Adapters (databases, external services)
   - Tests last

4. **Continue Prompt**: After displaying the summary, graft prompts you to continue:
   ```
   Continue reviewing diffs? [Y/n]
   ```
   Press Enter or `y` to proceed, or `n` to cancel the review.

5. **Beautiful Diffs**: Each file is displayed through Delta with syntax highlighting and side-by-side view (if configured).

## Navigating the Review

### Review Flow

```
┌─────────────────────────────┐
│     AI Summary Displayed    │
│  (ordering runs in background)
└──────────────┬──────────────┘
               │
               ▼
┌─────────────────────────────┐
│  Continue reviewing? [Y/n]  │
└──────────────┬──────────────┘
               │
       ┌───────┴───────┐
       │               │
       ▼               ▼
    [Enter/y]         [n]
       │               │
       ▼               ▼
┌──────────────┐  ┌──────────┐
│ Show file    │  │  Cancel  │
│ ordering &   │  │  review  │
│ display diffs│  └──────────┘
└──────────────┘
```

### Delta Pager Controls

When viewing diffs through Delta, use standard pager controls:

| Key | Action |
|-----|--------|
| `Space` / `Page Down` | Scroll down one page |
| `b` / `Page Up` | Scroll up one page |
| `j` / `↓` | Scroll down one line |
| `k` / `↑` | Scroll up one line |
| `g` | Go to start of file |
| `G` | Go to end of file |
| `q` | Quit current file (proceed to next) |
| `/pattern` | Search for pattern |
| `n` | Next search match |
| `N` | Previous search match |

Files are displayed sequentially in the AI-determined order. After viewing each file's diff, press `q` to proceed to the next file.

## File Category Icons

| Icon | Category | Description |
|------|----------|-------------|
| → | Entry Point | Main functions, handlers, CLI commands |
| ◆ | Business Logic | Core application logic |
| ◇ | Adapter | Database, API clients, external services |
| ● | Model | Domain models, entities |
| ⚙ | Config | Configuration files |
| ✓ | Test | Test files |
| 📄 | Docs | Documentation |
| ○ | Other | Everything else |

## Project Structure

```
graft/
├── cmd/graft/          # Application entry point
├── internal/
│   ├── analysis/       # Repository structure analysis
│   ├── cli/            # Cobra CLI commands
│   ├── config/         # Configuration management
│   ├── git/            # Git operations
│   ├── prompt/         # Interactive terminal prompts
│   ├── provider/       # AI provider abstraction
│   │   ├── claude/     # Claude implementation
│   │   ├── copilot/    # Copilot implementation (via copilot-api proxy)
│   │   └── mock/       # Mock for testing
│   └── render/         # Output rendering
├── docs/               # Documentation
├── Makefile
└── README.md
```

## Development

```bash
# Build
make build

# Run tests
make test

# Run tests with coverage
make test-coverage

# Format code
make fmt

# Run linter (requires golangci-lint)
make lint
```

## Adding New Providers

Graft is designed to support multiple AI providers. See [docs/providers.md](docs/providers.md) for instructions on adding new providers like OpenAI, Copilot, or local models.

## License

MIT License - see LICENSE file for details.

## Contributing

Contributions are welcome! Please read the contributing guidelines before submitting a PR.
