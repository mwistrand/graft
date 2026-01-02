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

	b.WriteString(`You are an expert code reviewer determining the optimal order to review files in a pull request. Files should be ordered to maximize understanding - starting with entry points and high-level changes, then proceeding to implementation details.

## Changed Files
`)

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
      "category": "entry_point|business_logic|adapter|model|config|test|docs|other",
      "priority": 1,
      "description": "Brief description of what this file does in the context of this PR"
    }
  ],
  "reasoning": "Brief explanation of the ordering strategy used"
}

Ordering principles:
1. Configuration and constants first (context-setting)
2. Types/interfaces/models (understand the domain)
3. Entry points (main.go, handlers, CLI commands, API routes)
4. Core business logic (services, use cases)
5. Adapters and integrations (databases, external services)
6. Tests last (verify understanding)

Keep descriptions brief (under 15 words).
Priority 1 = review first, higher numbers = later.
Return ONLY valid JSON, no additional text.`)

	return b.String()
}
