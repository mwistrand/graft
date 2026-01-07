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

Graft groups related files by **feature** and orders them by **architectural flow**:
```
=== Review Order ===

Files grouped by feature, ordered by architectural flow within each group.

Groups:
  1. User Authentication (3 files)
     Adds login and session management
  2. API Endpoints (2 files)
     New user management endpoints

  1. [User Authentication] → auth/handler.go
      HTTP handler for auth endpoints
  2. [User Authentication] ◆ auth/service.go
      Authentication business logic
  3. [User Authentication] ◇ auth/repository.go
      Session storage adapter
  4. [API Endpoints] → api/users.go
      User CRUD endpoints
  5. [API Endpoints] ◆ api/validation.go
      Request validation logic
```

This way, you review one complete feature before moving to the next.

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

# Force refresh (bypass cache and re-analyze)
graft review main --refresh
```

### Response Caching

Graft caches AI responses to speed up subsequent reviews of the same commits. The cache is keyed by:
- The base branch reference
- The commit hashes being reviewed

**How it works:**
- First review: AI generates summary and ordering, results are cached
- Subsequent reviews of same commits: Cached results are used instantly
- Use `--refresh` to bypass the cache and get fresh AI analysis

**Cache location:** `.graft/reviews/<cache-key>.json`

This is especially useful when:
- Reviewing the same branch multiple times during development
- Re-running a review after accidentally closing the terminal

### Cache Management

```bash
# Clear all cached reviews (with confirmation)
graft cache clear

# Clear only stale entries (older than one week)
graft cache clear --stale
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

3. **Intelligent Grouping & Ordering**: While you read the summary, graft determines the best order to review files:
   - Groups related files by feature (e.g., "User Authentication", "API Refactor")
   - Orders files within each group by architectural flow:
     - Entry points (main, handlers, CLI commands)
     - Core business logic
     - Adapters (databases, external services)
     - Tests

4. **Continue Prompt**: After displaying the summary, graft prompts you to continue:
   ```
   Continue reviewing diffs? [Y/n]
   ```
   Press Enter or `y` to proceed, or `n` to cancel the review.

5. **Group Selection**: If multiple feature groups are detected, an interactive selector appears:
   ```
   Select groups to review
   Space to toggle, Enter to confirm. All selected by default.

   > [x] User Authentication - Adds login and session management (3 files)
     [x] API Endpoints - New user management endpoints (2 files)
     [x] Configuration - Updates to app config (1 files)
   ```
   Use Space to toggle groups on/off, then Enter to confirm your selection.

6. **Beautiful Diffs**: Each file is displayed through Delta with syntax highlighting. The file header shows which group the file belongs to:
   ```
   [1/5] User Authentication -> → auth/handler.go
     HTTP handler for auth endpoints
   ```

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
       │               ▼
       │          ┌──────────┐
       │          │  Cancel  │
       │          │  review  │
       │          └──────────┘
       ▼
┌─────────────────────────────┐
│   Show Review Order         │
│   (with groups if detected) │
└──────────────┬──────────────┘
               │
               ▼
┌─────────────────────────────┐
│   Select Groups to Review   │
│   (if multiple groups)      │
│   Space=toggle, Enter=done  │
└──────────────┬──────────────┘
               │
               ▼
┌─────────────────────────────┐
│   Display Diffs             │
│   Files shown by group,     │
│   then by architecture      │
└─────────────────────────────┘
```

### Group Selection

When the AI identifies multiple feature groups in your changes, you'll see an interactive selector:

```
Select groups to review
Space to toggle, Enter to confirm. All selected by default.

> [x] User Authentication - Adds login and session management (3 files)
  [x] API Endpoints - New user management endpoints (2 files)
  [ ] Documentation - README updates (1 files)
```

| Key | Action |
|-----|--------|
| `↑` / `↓` | Navigate between groups |
| `Space` | Toggle group selection |
| `Enter` | Confirm and start review |

**Tips:**
- All groups are selected by default - just press Enter to review everything
- Deselect groups you want to skip (e.g., documentation-only changes)
- Files are displayed in group order, so you review one feature completely before the next

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
