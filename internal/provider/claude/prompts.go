package claude

import (
	"fmt"
	"strings"

	"github.com/mwistrand/graft/internal/provider"
)

// buildSummaryPrompt constructs the prompt for change summarization.
func buildSummaryPrompt(req *provider.SummarizeRequest) string {
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

// buildOrderPrompt constructs the prompt for file ordering.
func buildOrderPrompt(req *provider.OrderRequest) string {
	var b strings.Builder

	b.WriteString(`You are an expert code reviewer determining the optimal order to review files in a pull request. Files should be ordered to maximize understanding.

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

Determine the optimal review order. Respond with a JSON object in this exact format:
{
  "files": [
    {
      "path": "path/to/file.go",
      "category": "entry_point|business_logic|adapter|model|config|test|docs|routing|component|other",
      "priority": 1,
      "description": "Brief description of what this file does in the context of this PR"
    }
  ],
  "reasoning": "Brief explanation of the ordering strategy used"
}

## Ordering Strategy

Adapt your ordering based on the project type:

**For frontend projects:**
1. Routing (pages, routes)
2. Container/smart components (state management)
3. Presentational components (UI building blocks)
4. Models/types (data shapes)
5. Services (API clients, state stores)
6. Tests

**For backend projects:**
1. Entry points (main, cmd, handlers)
2. Routes/controllers (request handling)
3. Business logic (services, use cases)
4. Models/entities (data structures)
5. Adapters (databases, external services)
6. Tests

**For fullstack/mixed projects:**
1. Backend changes first (APIs shape frontend)
2. Then frontend changes following the frontend ordering above
3. Tests last

`)

	if req.TestsFirst {
		b.WriteString(`**IMPORTANT:** The user has requested tests-first ordering. Place ALL test files at the BEGINNING of the review (priority 1-N) so the reviewer understands intent before seeing implementation.

`)
	}

	b.WriteString(`Keep descriptions brief (under 15 words).
Priority 1 = review first, higher numbers = later.
Return ONLY valid JSON, no additional text.`)

	return b.String()
}
