# Graft CLI

**AI-powered code review CLI that presents diffs in logical order**

Graft helps you review git branches by:
1. Summarizing changes using AI (with commit message context)
2. Ordering files by architectural layers and feature groups
3. Providing a beautiful diff view powered by [delta](https://github.com/dandavison/delta)
4. Tracking your review progress with bookmarks and annotations

## The Problem

When reviewing a PR with 30 changed files, you usually see them in alphabetical order:
```
adapters/user_repository.rb
controllers/users_controller.rb
models/user.rb
services/user_service.rb
```

But that's backwards! You want to understand *what* the change does before diving into *how*.

## The Solution

Graft reorders files by **architectural flow**:
```
▸ Feature: User Registration API
  → controllers/users_controller.rb    [Entry Point]
  ◆ services/user_service.rb           [Application]
  ● models/user.rb                      [Domain]
  ◇ adapters/user_repository.rb        [Adapter]
```

And groups them by feature, so related changes are reviewed together.

## Installation

### Prerequisites

- Go 1.21+
- [delta](https://github.com/dandavison/delta) (recommended for beautiful diffs)
- Git

### From Source

```bash
git clone https://github.com/mwistrand/graft
cd graft
go build -o graft ./cmd/graft
sudo mv graft /usr/local/bin/
```

Or use `go install`:

```bash
go install github.com/mwistrand/graft/cmd/graft@latest
```
