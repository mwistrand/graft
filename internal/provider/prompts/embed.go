// Package prompts provides embedded prompt templates for AI providers.
package prompts

import (
	_ "embed"
)

// DefaultCodeReviewerPrompt is the default system prompt for AI code reviews.
// This prompt instructs the AI to act as a staff-level code review expert.
//
//go:embed code-reviewer.md
var DefaultCodeReviewerPrompt string
