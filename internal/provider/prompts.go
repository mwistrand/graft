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
      "priority": 1
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

## Grouping Strategy

1. **Identify features**: Look for related changes that form a cohesive unit:
   - Files that implement a single feature together (handler + service + model + test)
   - A refactoring that spans multiple related files
   - Configuration changes that belong together

2. **Name groups meaningfully**: Use action-oriented names like:
   - "User Authentication" (not "auth.go changes")
   - "API Error Handling" (not "misc fixes")
   - "Database Migration" (not "db stuff")

3. **Order groups**: Put foundational/dependency changes first, then features that build on them

4. **Handle miscellaneous files**: Group standalone config files, docs, or unrelated small changes into a "Configuration" or "Miscellaneous" group

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
