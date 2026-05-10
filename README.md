# Graft CLI

**AI-powered code review CLI that presents diffs in logical order**

Graft helps you review git branches by:
1. Ordering files by architectural layers (entry points → business logic → adapters)
2. Providing a beautiful diff view powered by [Delta](https://github.com/dandavison/delta)
3. Optionally summarizing changes using AI (`--summarize`)

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
- [GitHub CLI](https://cli.github.com/) (optional, required for PR URL reviews)

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

### Reviewing GitHub Pull Requests

You can review a GitHub pull request directly by providing its URL:

```bash
# Review a GitHub PR
graft review https://github.com/owner/repo/pull/123
```

**Requirements:**
- [GitHub CLI](https://cli.github.com/) must be installed and authenticated
- You must be in a local clone of the repository

**Setup:**
```bash
# Install GitHub CLI
brew install gh

# Authenticate with GitHub
gh auth login
```

**How it works:**
1. Graft parses the PR URL and fetches metadata via `gh`
2. Validates that your local repo matches the PR's repository
3. Fetches the PR's commits if not available locally
4. Reviews the diff between the PR's base and head branches

**Enterprise GitHub:** Enterprise instances are supported automatically. Just use your enterprise PR URL and ensure `gh` is authenticated for that host.

**Merged/Closed PRs:** Graft can review merged or closed PRs. It will display the PR state and use the exact commit SHA from the PR.

```
PR #123 [MERGED]: Add user authentication
  feature/auth -> main
  Note: Reviewing based on commit abc123def456
```

### Full Codebase Scan

Scan the entire repository, treating every tracked file as newly added. Useful for onboarding to a new codebase, architectural review, or auditing a project.

```bash
# Scan the full codebase
graft scan

# Scan with a specific model
graft scan --model gpt-4o

# Scan with detailed AI code review
graft scan --ai-review

# Skip minor files (config, docs, etc.)
graft scan --major-only
```

The scan command uses the same AI pipeline as `graft review` but diffs HEAD against an empty tree so all files appear as new additions. In non-git directories, it falls back to filesystem scanning.

### Options

```bash
# Include AI summary of changes
graft review main --summarize

# Skip AI ordering (use default order)
graft review main --no-order

# Disable Delta rendering
graft review main --no-delta

# Use a specific AI provider
graft review main --provider claude

# Use a specific model for both reviews and ordering (skips interactive selection)
graft review main --model gpt-4o

# Use a specific model for review and quick review tasks
graft review main --review-model gpt-4o

# Use a specific model for ordering and summary tasks
graft review main --order-model gpt-4o

# Show tests before implementation files
graft review main --tests-first

# Show test files alongside their implementation
graft review main --inline-tests

# Force refresh (bypass cache and re-analyze)
graft review main --refresh

# Generate detailed AI code review
graft review main --ai-review

# Write AI review to a file
graft review main --ai-review=review.md

# Focus review on specific categories
graft review main --ai-review --review-categories design,functionality,tests

# Filter review output by severity
graft review main --ai-review --review-severity critical

# Set prompt timeout (in minutes, 0 to disable)
graft review main --prompt-timeout 60

# Only review core and supporting groups, skip minor changes
graft review main --major-only

# Perform a quick initial assessment before full review
graft review main --quick
```

### AI Code Review

The `--ai-review` flag generates a detailed code review using the configured AI provider. The review is structured into categories and severity levels:

**Categories:** design, functionality, complexity, tests, naming, comments, style, documentation, praise

**Severity Levels:**
- `critical`: Must-fix issues (bugs, security, design flaws)
- `suggestion`: Should-consider improvements
- `nit`: Minor/optional issues (style preferences)

```bash
# Display review in console
graft review main --ai-review

# Save review to a file
graft review main --ai-review=review.md

# Focus on specific categories
graft review main --ai-review --review-categories design,functionality

# Show only critical issues
graft review main --ai-review --review-severity critical
```

**Custom Review Prompt:** Place a custom system prompt at `.graft/code-reviewer.md` in your repository to override the default review approach.

**Caching:** AI reviews are cached alongside summaries and ordering. Request the same review with `--ai-review` (no output file) to display a previously generated review in the console.

### Response Caching

Graft caches AI responses to speed up subsequent reviews of the same commits. For branch reviews, the cache is keyed by the base branch reference and commit hashes. For full-codebase scans, the key is derived from the HEAD commit hash (or a content fingerprint in non-git directories).

**How it works:**
- First review: AI generates summary, ordering, and review (if requested), results are cached
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
| `review-model` | Model for review tasks (review, quick review) | `GRAFT_REVIEW_MODEL` |
| `order-model` | Model for ordering and summary tasks | `GRAFT_ORDER_MODEL` |
| `anthropic-api-key` | Anthropic API key | `ANTHROPIC_API_KEY` |
| `copilot-base-url` | Copilot proxy URL (default: http://localhost:4141) | `COPILOT_BASE_URL` |
| `delta-path` | Path to Delta binary | `GRAFT_DELTA_PATH` |
| `summarize` | Include AI summary of changes | `GRAFT_SUMMARIZE` |
| `prompt-timeout` | Timeout in minutes for interactive prompts (default: 30, 0 to disable) | `GRAFT_PROMPT_TIMEOUT` |

## How It Works

1. **Analyze Changes**: Graft gets the diff between your branch and the base branch, along with all commit messages.

2. **Intelligent Grouping & Ordering**: Graft determines the best order to review files:
   - Groups related files by feature (e.g., "User Authentication", "API Refactor")
   - Orders files within each group by architectural flow:
     - Entry points (main, handlers, CLI commands)
     - Core business logic
     - Adapters (databases, external services)
     - Tests

3. **AI Summary** (optional, via `--summarize`): The AI analyzes the changes and provides:
   - A high-level overview
   - Key changes (bullet points)
   - Potential concerns or risks

4. **Continue Prompt**: Graft prompts you to continue:
   ```
   Continue reviewing diffs? [Y/n] (timeout in 30m)
   ```
   Press Enter or `y` to proceed, or `n` to cancel the review. The prompt times out after 30 minutes by default to prevent orphaned processes. Configure with `--prompt-timeout` or `prompt-timeout` config.

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

When the AI identifies multiple feature groups in your changes, you'll see an interactive selector. Groups are classified by significance tier:

- **Core**: Major logic changes (new features, API changes, business logic)
- **Supporting**: Tests, utilities, helpers
- **Minor**: Config files, docs, formatting, dependency updates

```
Select groups to review
Space to toggle, Enter to confirm. Core/supporting selected by default.

> [x] [core] User Authentication - Adds login and session management (3 files)
  [x] [supporting] Unit Tests - Test coverage for auth (2 files)
  [ ] [minor] Documentation - README updates (1 files)
```

| Key | Action |
|-----|--------|
| `↑` / `↓` | Navigate between groups |
| `Space` | Toggle group selection |
| `Enter` | Confirm and start review |

**Tips:**
- Core and supporting groups are selected by default; minor groups are deselected
- Use `--major-only` to skip minor groups entirely (won't appear in the selector)
- Files are displayed in group order, so you review one feature completely before the next

### Diff Viewer Controls

Graft displays diffs in an interactive TUI with file navigation:

**File Navigation:**

| Key | Action |
|-----|--------|
| `n` | Next file |
| `p` | Previous file |
| `Ctrl+J` | Open file picker (jump to any file) |
| `Ctrl+B` | Jump back to previous position |
| `q` / `Ctrl+C` | Quit review |

**Scrolling:**

| Key | Action |
|-----|--------|
| `↑` / `↓` | Scroll up/down one line |
| `Page Up` / `Page Down` | Scroll up/down one page |
| `Home` / `g` | Go to start of file |
| `End` / `G` | Go to end of file |

**File Picker (Ctrl+J):**

| Key | Action |
|-----|--------|
| `↑` / `↓` | Navigate file list |
| Type | Filter files by path |
| `Enter` | Jump to selected file |
| `Esc` | Cancel and return to current file |

The jump-back feature (`Ctrl+B`) remembers your position history, so you can quickly check a related file and return to where you were.

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
│   ├── filescan/       # Filesystem scanning for non-git directories
│   ├── git/            # Git operations
│   ├── pr/             # Pull request URL parsing and resolution
│   ├── prompt/         # Interactive terminal prompts
│   ├── provider/       # AI provider abstraction
│   │   ├── claude/     # Claude implementation
│   │   ├── copilot/    # Copilot implementation (via copilot-api proxy)
│   │   ├── mock/       # Mock for testing
│   │   └── testpair/   # Test/implementation file pairing
│   ├── render/         # Output rendering
│   └── tui/            # Interactive terminal UI
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

Graft is designed to support multiple AI providers. See [docs/providers.md](docs/providers.md) for instructions on adding new providers (e.g. additional hosted LLMs or local models).

## License

MIT License - see LICENSE file for details.

## Contributing

Contributions are welcome! Please read the contributing guidelines before submitting a PR.
