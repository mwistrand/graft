package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mwistrand/graft/internal/provider"
)

// outputAIReview writes the AI review to console or a file.
// If severityFilter is set, only comments at or above that severity level are shown.
func outputAIReview(resp *provider.ReviewResponse, outputPath string, severityFilter provider.ReviewSeverity) error {
	if resp == nil || (resp.Content == "" && resp.Structured == nil) {
		return fmt.Errorf("AI review content is empty")
	}

	// Apply severity filter to structured review if present
	structured := resp.Structured
	if structured != nil && severityFilter != "" {
		structured = structured.FilterBySeverity(severityFilter)
	}

	if outputPath != "" {
		// Write markdown content to file
		var content string
		if structured != nil {
			content = provider.GenerateMarkdownFromReview(structured)
		} else {
			content = resp.Content
		}

		// Create parent directory if it doesn't exist
		dir := filepath.Dir(outputPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("creating output directory: %w", err)
		}
		if err := os.WriteFile(outputPath, []byte(content), 0600); err != nil {
			return fmt.Errorf("writing review to file: %w", err)
		}
		fmt.Printf("AI review written to: %s\n\n", outputPath)
	} else {
		// Console output
		fmt.Println("\n" + strings.Repeat("=", 60))
		fmt.Println("AI CODE REVIEW")
		if severityFilter != "" {
			fmt.Printf("(filtered: %s and above)\n", severityFilter)
		}
		fmt.Println(strings.Repeat("=", 60))

		if structured != nil {
			outputStructuredReview(structured)
		} else {
			fmt.Println()
			fmt.Println(resp.Content)
		}

		fmt.Println(strings.Repeat("=", 60) + "\n")
	}
	return nil
}

// outputStructuredReview renders a structured review to the console.
func outputStructuredReview(review *provider.StructuredReview) {
	// Summary
	if review.Summary != "" {
		fmt.Println()
		fmt.Println(review.Summary)
	}

	// Count by severity for stats
	counts := review.CountBySeverity()
	critical := counts[provider.SeverityCritical]
	suggestions := counts[provider.SeveritySuggestion]
	nits := counts[provider.SeverityNit]

	if critical+suggestions+nits > 0 {
		fmt.Printf("\nFindings: %d critical, %d suggestions, %d nits\n",
			critical, suggestions, nits)
	}

	// Group comments by category
	byCategory := review.CommentsByCategory()

	// Output in standard category order
	for _, cat := range provider.AllReviewCategories() {
		comments := byCategory[cat]
		if len(comments) == 0 {
			continue
		}

		// Category header with counts
		catCritical, catSuggestions, catNits := 0, 0, 0
		for _, c := range comments {
			switch c.Severity {
			case provider.SeverityCritical:
				catCritical++
			case provider.SeveritySuggestion:
				catSuggestions++
			case provider.SeverityNit:
				catNits++
			}
		}

		fmt.Println()
		fmt.Println(strings.Repeat("-", 60))
		header := strings.ToUpper(provider.CategoryDisplayName(cat))
		if cat == provider.CategoryPraise {
			fmt.Printf("%s (%d items)\n", header, len(comments))
		} else {
			parts := []string{}
			if catCritical > 0 {
				parts = append(parts, fmt.Sprintf("%d critical", catCritical))
			}
			if catSuggestions > 0 {
				parts = append(parts, fmt.Sprintf("%d suggestions", catSuggestions))
			}
			if catNits > 0 {
				parts = append(parts, fmt.Sprintf("%d nits", catNits))
			}
			if len(parts) > 0 {
				fmt.Printf("%s (%s)\n", header, strings.Join(parts, ", "))
			} else {
				fmt.Println(header)
			}
		}
		fmt.Println(strings.Repeat("-", 60))

		for _, c := range comments {
			// Severity indicator
			var indicator string
			switch c.Severity {
			case provider.SeverityCritical:
				indicator = "[!]"
			case provider.SeveritySuggestion:
				indicator = "[*]"
			case provider.SeverityNit:
				indicator = "[-]"
			}

			// For praise, use checkmark
			if cat == provider.CategoryPraise {
				indicator = "[+]"
			}

			// Title with optional file/line reference
			if c.File != "" {
				if c.Line > 0 {
					fmt.Printf("\n%s %s (%s:%d)\n", indicator, c.Title, c.File, c.Line)
				} else {
					fmt.Printf("\n%s %s (%s)\n", indicator, c.Title, c.File)
				}
			} else {
				fmt.Printf("\n%s %s\n", indicator, c.Title)
			}

			// Description (indented)
			if c.Description != "" {
				lines := strings.Split(c.Description, "\n")
				for _, line := range lines {
					fmt.Printf("    %s\n", line)
				}
			}

			// Suggestion (indented, with label)
			if c.Suggestion != "" {
				fmt.Println()
				fmt.Println("    Suggestion:")
				lines := strings.Split(c.Suggestion, "\n")
				for _, line := range lines {
					fmt.Printf("      %s\n", line)
				}
			}
		}
	}
	fmt.Println()
}

// outputQuickReview renders the quick review verdict to the console.
func outputQuickReview(resp *provider.QuickReviewResponse) error {
	if resp == nil {
		return fmt.Errorf("quick review response is empty")
	}

	fmt.Println()
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println("QUICK ASSESSMENT")
	fmt.Println(strings.Repeat("=", 60))

	// Verdict with visual indicator
	var verdictIcon, verdictLabel string
	switch resp.Verdict {
	case provider.VerdictApprove:
		verdictIcon = "[OK]"
		verdictLabel = "APPROVE"
	case provider.VerdictConcerns:
		verdictIcon = "[?]"
		verdictLabel = "CONCERNS"
	case provider.VerdictBlocker:
		verdictIcon = "[!]"
		verdictLabel = "BLOCKER"
	default:
		verdictIcon = "[?]"
		verdictLabel = string(resp.Verdict)
	}

	fmt.Printf("\nVerdict: %s %s\n", verdictIcon, verdictLabel)
	fmt.Println()

	// Summary
	if resp.Summary != "" {
		fmt.Println(resp.Summary)
		fmt.Println()
	}

	// Concerns
	if len(resp.Concerns) > 0 {
		fmt.Println("Concerns:")
		for _, c := range resp.Concerns {
			fmt.Printf("  - %s\n", c)
		}
		fmt.Println()
	}

	// Proceed indicator
	if resp.Proceed {
		fmt.Println("Proceeding with full review...")
	} else {
		fmt.Println("Review blocked. Address concerns before continuing.")
	}

	fmt.Println(strings.Repeat("=", 60))
	fmt.Println()

	return nil
}
