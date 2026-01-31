package provider

import (
	"encoding/json"
	"fmt"
	"strings"
)

// BuildSummaryPrompt constructs the prompt for change summarization.
func BuildSummaryPrompt(req *SummarizeRequest) string {
	var b strings.Builder

	b.WriteString(`You are an expert code reviewer analyzing a pull request. Review the following diff and commit messages to provide a concise, actionable summary.

`)

	// Add commits section
	if len(req.Commits) > 0 {
		b.WriteString("## Commits\n")
		for _, c := range req.Commits {
			b.WriteString(fmt.Sprintf("### %s by %s\n", c.ShortHash, c.Author))
			b.WriteString(c.Subject + "\n")
			if c.Body != "" {
				b.WriteString(c.Body + "\n")
			}
			b.WriteString("\n")
		}
	}

	// Add changed files section
	b.WriteString("## Changed Files\n")
	for _, f := range req.Files {
		status := f.Status
		if f.OldPath != "" {
			status = fmt.Sprintf("%s from %s", status, f.OldPath)
		}
		b.WriteString(fmt.Sprintf("- %s (%s: +%d/-%d)\n", f.Path, status, f.Additions, f.Deletions))
	}
	b.WriteString("\n")

	// Add diff content if available (truncated for large diffs)
	if req.FullDiff != "" {
		diff := req.FullDiff
		const maxDiffLen = 50000
		if len(diff) > maxDiffLen {
			diff = diff[:maxDiffLen] + "\n\n... [diff truncated for length] ..."
		}
		b.WriteString("## Diff Content\n```diff\n")
		b.WriteString(diff)
		b.WriteString("\n```\n\n")
	}

	// Add focus instruction if specified
	if req.Options.Focus != "" {
		b.WriteString(fmt.Sprintf("Focus your analysis on: %s\n\n", req.Options.Focus))
	}

	b.WriteString(`---

Respond with a JSON object in this exact format:
{
  "overview": "A 1-2 sentence summary of what this change accomplishes",
  "key_changes": [
    "First key change or feature",
    "Second key change",
    "..."
  ],
  "concerns": [
    "Any potential issues, risks, or areas needing careful review",
    "..."
  ],
  "file_groups": [
    {
      "name": "Group name (e.g., 'API Layer', 'Database')",
      "description": "What this group of changes does",
      "files": ["path/to/file1.go", "path/to/file2.go"]
    }
  ]
}

Focus on:
- The "why" behind the changes, not just the "what"
- Architectural implications
- Potential side effects or risks
- Test coverage considerations

Return ONLY valid JSON, no additional text.`)

	return b.String()
}

// BuildOrderPrompt constructs the prompt for file ordering.
func BuildOrderPrompt(req *OrderRequest) string {
	var b strings.Builder

	b.WriteString(`You are an expert code reviewer determining the optimal order to review files in a pull request.

Your goals:
1. Identify related changes that form logical features or units of work
2. Group files by these features so reviewers can understand one feature completely before moving to the next
3. Order files within each group to maximize understanding (entry points -> business logic -> adapters -> tests)

`)

	// Include repository context if available
	if req.RepoContext != "" {
		b.WriteString("## Repository Context\n")
		b.WriteString(req.RepoContext)
		b.WriteString("\n")
	}

	b.WriteString("## Changed Files\n")
	for _, f := range req.Files {
		status := f.Status
		if f.OldPath != "" {
			status = fmt.Sprintf("%s from %s", status, f.OldPath)
		}
		b.WriteString(fmt.Sprintf("- %s (%s: +%d/-%d)\n", f.Path, status, f.Additions, f.Deletions))
	}

	if len(req.Commits) > 0 {
		b.WriteString("\n## Brief Context from Commits\n")
		for _, c := range req.Commits {
			b.WriteString(fmt.Sprintf("- %s\n", c.Subject))
		}
	}

	b.WriteString(`

---

Respond with a JSON object in this exact format:
{
  "groups": [
    {
      "name": "Short feature name (2-4 words)",
      "description": "Brief explanation of what this feature/change accomplishes",
      "priority": 1,
      "significance": "core|supporting|minor"
    }
  ],
  "files": [
    {
      "path": "path/to/file.go",
      "category": "entry_point|business_logic|adapter|model|config|test|docs|routing|component|other",
      "priority": 1,
      "description": "Brief description of what this file does",
      "group": "Short feature name (must match a group name)"
    }
  ],
  "reasoning": "Brief explanation of the grouping and ordering strategy"
}

## Significance Classification

Assign each group a significance level based on the nature of changes:

- **core**: Major logic changes that affect functionality
  - New features, API changes, business logic modifications
  - Data model changes, database schema updates
  - Security-related changes, authentication/authorization

- **supporting**: Changes that support or validate core logic
  - Test files (unit tests, integration tests)
  - Utility functions and helpers
  - Internal refactoring that doesn't change behavior

- **minor**: Low-impact changes that rarely need deep review
  - Configuration files, environment settings
  - Documentation, comments, README updates
  - Formatting, linting fixes, dependency updates
  - Generated code, lock files

## Grouping Strategy

1. **Identify features**: Look for related changes that form a cohesive unit:
   - Files that implement a single feature together (handler + service + model + test)
   - A refactoring that spans multiple related files
   - Configuration changes that belong together

2. **Name groups meaningfully**: Use action-oriented names like:
   - "User Authentication" (not "auth.go changes")
   - "API Error Handling" (not "misc fixes")
   - "Database Migration" (not "db stuff")

3. **Order groups by significance first, then by dependency**:
   - Core groups first (main features, logic changes)
   - Supporting groups next (tests, utilities)
   - Minor groups last (config, docs)
   - Within each significance level, put foundational changes before dependent ones

4. **Handle miscellaneous files**: Group standalone config files, docs, or unrelated small changes into a "Configuration" or "Miscellaneous" group with significance "minor"

## File Ordering Within Groups

Adapt ordering based on the project type:

**For backend projects:**
1. Entry points (main, cmd, handlers)
2. Routes/controllers (request handling)
3. Business logic (services, use cases)
4. Models/entities (data structures)
5. Adapters (databases, external services)
6. Tests

**For frontend projects:**
1. Routing (pages, routes)
2. Container/smart components (state management)
3. Presentational components (UI building blocks)
4. Models/types (data shapes)
5. Services (API clients, state stores)
6. Tests

**For fullstack/mixed projects:**
1. Backend changes first (APIs shape frontend)
2. Then frontend changes

`)

	if req.TestsFirst {
		b.WriteString(`**IMPORTANT:** The user has requested tests-first ordering. Within each group, place test files at the BEGINNING so the reviewer understands intent before seeing implementation.

`)
	}

	b.WriteString(`Keep descriptions brief (under 15 words).
Group names should be 2-4 words.
Priority 1 = review first, higher numbers = later.
Every file MUST have a group assigned.
Every group MUST have a significance level (core, supporting, or minor).
Return ONLY valid JSON, no additional text.`)

	return b.String()
}

// ParseJSONResponse extracts and parses JSON from an AI response.
// It handles cases where JSON is wrapped in markdown code blocks.
func ParseJSONResponse(text string, v any) error {
	jsonStr := ExtractJSON(text)

	if err := json.Unmarshal([]byte(jsonStr), v); err != nil {
		return fmt.Errorf("invalid JSON: %w\nResponse was: %s", err, text)
	}

	return nil
}

// ParseStructuredReview parses a JSON response into a StructuredReview.
// It also generates a markdown Content field for backwards compatibility.
// If parsing fails, it returns a ReviewResponse with just the raw content.
func ParseStructuredReview(text string) *ReviewResponse {
	var structured StructuredReview
	if err := ParseJSONResponse(text, &structured); err != nil {
		// Fall back to raw content if JSON parsing fails
		return &ReviewResponse{Content: text}
	}

	// Generate markdown from structured review for backwards compatibility
	content := GenerateMarkdownFromReview(&structured)

	return &ReviewResponse{
		Content:    content,
		Structured: &structured,
	}
}

// GenerateMarkdownFromReview converts a StructuredReview to markdown format.
func GenerateMarkdownFromReview(review *StructuredReview) string {
	var b strings.Builder

	// Summary
	if review.Summary != "" {
		b.WriteString("## Summary\n\n")
		b.WriteString(review.Summary)
		b.WriteString("\n\n")
	}

	// Group comments by category
	byCategory := review.CommentsByCategory()

	// Output in standard category order
	for _, cat := range AllReviewCategories() {
		comments := byCategory[cat]
		if len(comments) == 0 {
			continue
		}

		// Category header
		b.WriteString(fmt.Sprintf("## %s\n\n", CategoryDisplayName(cat)))

		for _, c := range comments {
			// Severity indicator
			switch c.Severity {
			case SeverityCritical:
				b.WriteString("**[CRITICAL]** ")
			case SeverityNit:
				b.WriteString("*[Nit]* ")
			}

			// Title with optional file/line reference
			if c.File != "" {
				if c.Line > 0 {
					b.WriteString(fmt.Sprintf("**%s** (`%s:%d`)\n\n", c.Title, c.File, c.Line))
				} else {
					b.WriteString(fmt.Sprintf("**%s** (`%s`)\n\n", c.Title, c.File))
				}
			} else {
				b.WriteString(fmt.Sprintf("**%s**\n\n", c.Title))
			}

			// Description
			if c.Description != "" {
				b.WriteString(c.Description)
				b.WriteString("\n\n")
			}

			// Suggestion
			if c.Suggestion != "" {
				b.WriteString("**Suggestion:**\n")
				// Check if suggestion looks like code
				if strings.Contains(c.Suggestion, "\n") || strings.Contains(c.Suggestion, "func ") || strings.Contains(c.Suggestion, "if ") {
					b.WriteString("```\n")
					b.WriteString(c.Suggestion)
					b.WriteString("\n```\n\n")
				} else {
					b.WriteString(c.Suggestion)
					b.WriteString("\n\n")
				}
			}
		}
	}

	return strings.TrimSpace(b.String())
}

// CategoryDisplayName returns a human-readable name for a review category.
func CategoryDisplayName(cat ReviewCategory) string {
	switch cat {
	case CategoryDesign:
		return "Design"
	case CategoryFunctionality:
		return "Functionality"
	case CategoryComplexity:
		return "Complexity"
	case CategoryTests:
		return "Tests"
	case CategoryNaming:
		return "Naming"
	case CategoryComments:
		return "Comments"
	case CategoryStyle:
		return "Style"
	case CategoryDocumentation:
		return "Documentation"
	case CategoryPraise:
		return "Praise"
	default:
		return string(cat)
	}
}

// ExtractJSON extracts JSON content from a string that may contain markdown.
func ExtractJSON(text string) string {
	// Look for JSON code block
	start := strings.Index(text, "```json")
	if start != -1 {
		start += 7 // len("```json")
		end := strings.Index(text[start:], "```")
		if end != -1 {
			return strings.TrimSpace(text[start : start+end])
		}
	}

	// Look for generic code block
	start = strings.Index(text, "```")
	if start != -1 {
		start += 3 // len("```")
		// Skip language identifier if present
		if nl := strings.Index(text[start:], "\n"); nl != -1 {
			start += nl + 1
		}
		end := strings.Index(text[start:], "```")
		if end != -1 {
			return strings.TrimSpace(text[start : start+end])
		}
	}

	// Look for raw JSON (starts with { or [)
	for i := 0; i < len(text); i++ {
		if text[i] == '{' || text[i] == '[' {
			return strings.TrimSpace(text[i:])
		}
	}

	return strings.TrimSpace(text)
}

// BuildReviewPrompt constructs the user prompt for a detailed code review.
// The system prompt should be passed separately to the AI provider.
func BuildReviewPrompt(req *ReviewRequest) string {
	var b strings.Builder

	b.WriteString(`Please review the following code changes and provide a detailed, constructive code review.

`)

	// Add commits section
	if len(req.Commits) > 0 {
		b.WriteString("## Commits\n")
		for _, c := range req.Commits {
			b.WriteString(fmt.Sprintf("### %s by %s\n", c.ShortHash, c.Author))
			b.WriteString(c.Subject + "\n")
			if c.Body != "" {
				b.WriteString(c.Body + "\n")
			}
			b.WriteString("\n")
		}
	}

	// Add changed files section
	b.WriteString("## Changed Files\n")
	for _, f := range req.Files {
		status := f.Status
		if f.OldPath != "" {
			status = fmt.Sprintf("%s from %s", status, f.OldPath)
		}
		b.WriteString(fmt.Sprintf("- %s (%s: +%d/-%d)\n", f.Path, status, f.Additions, f.Deletions))
	}
	b.WriteString("\n")

	// Add diff content
	if req.FullDiff != "" {
		diff := req.FullDiff
		const maxDiffLen = 80000
		if len(diff) > maxDiffLen {
			diff = diff[:maxDiffLen] + "\n\n... [diff truncated for length] ..."
		}
		b.WriteString("## Diff Content\n```diff\n")
		b.WriteString(diff)
		b.WriteString("\n```\n\n")
	}

	b.WriteString(`---

Respond with a JSON object in this exact format:
{
  "summary": "2-3 sentence executive summary of the changes and overall assessment",
  "comments": [
    {
      "category": "design|functionality|complexity|tests|naming|comments|style|documentation|praise",
      "severity": "critical|suggestion|nit",
      "file": "path/to/file.go",
      "line": 42,
      "title": "Short issue title (1 line)",
      "description": "Detailed explanation of the issue and why it matters",
      "suggestion": "Optional: suggested fix or code example"
    }
  ]
}

## Category Definitions
- **design**: Architecture fit, system integration, approach soundness
- **functionality**: Correctness, edge cases, error handling, concurrency
- **complexity**: Understandability, over-engineering, simplification opportunities
- **tests**: Test presence, validity, coverage, quality
- **naming**: Clarity and appropriateness of names
- **comments**: Comment necessity, accuracy, explaining "why" not "what"
- **style**: Codebase conventions, idioms, formatting
- **documentation**: Doc updates for user-facing changes
- **praise**: Good patterns, improvements, things done well

`)

	// Add focus instruction if specific categories are requested
	if len(req.Options.Categories) > 0 {
		catNames := make([]string, len(req.Options.Categories))
		for i, c := range req.Options.Categories {
			catNames[i] = string(c)
		}
		b.WriteString(fmt.Sprintf("**FOCUS**: Only provide feedback for these categories: %s. Skip other categories entirely.\n\n", strings.Join(catNames, ", ")))
	}

	b.WriteString(`## Guidelines
- Include file and line when referencing specific code
- Use "praise" category to acknowledge good work
- Omit file/line for general observations
- Prioritize critical issues over nits
- Be specific and actionable

Return ONLY valid JSON, no additional text.`)

	return b.String()
}
