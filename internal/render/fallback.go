package render

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/mwistrand/graft/internal/provider"
)

// fallbackRenderer renders diffs using basic git diff output.
type fallbackRenderer struct {
	output io.Writer
	color  bool
}

func newFallbackRenderer(opts Options) *fallbackRenderer {
	return &fallbackRenderer{
		output: opts.Output,
		color:  opts.ColorEnabled,
	}
}

// RenderSummary displays the AI-generated summary.
func (r *fallbackRenderer) RenderSummary(summary *provider.SummarizeResponse) error {
	w := r.output

	// Header
	r.writeLine(w, "")
	r.writeHeader(w, "Change Summary")
	r.writeLine(w, "")

	// Overview
	if summary.Overview != "" {
		r.writeLine(w, summary.Overview)
		r.writeLine(w, "")
	}

	// Key changes
	if len(summary.KeyChanges) > 0 {
		r.writeSubHeader(w, "Key Changes")
		for _, change := range summary.KeyChanges {
			r.writeBullet(w, change)
		}
		r.writeLine(w, "")
	}

	// Concerns
	if len(summary.Concerns) > 0 {
		r.writeSubHeader(w, "Concerns")
		for _, concern := range summary.Concerns {
			r.writeWarningBullet(w, concern)
		}
		r.writeLine(w, "")
	}

	// File groups
	if len(summary.FileGroups) > 0 {
		r.writeSubHeader(w, "File Groups")
		for _, group := range summary.FileGroups {
			r.writeLine(w, fmt.Sprintf("  %s: %s", group.Name, group.Description))
			for _, file := range group.Files {
				r.writeLine(w, fmt.Sprintf("    - %s", file))
			}
		}
		r.writeLine(w, "")
	}

	return nil
}

// RenderOrdering displays the file ordering with reasoning.
func (r *fallbackRenderer) RenderOrdering(order *provider.OrderResponse) error {
	w := r.output

	r.writeHeader(w, "Review Order")
	r.writeLine(w, "")

	if order.Reasoning != "" {
		r.writeLine(w, order.Reasoning)
		r.writeLine(w, "")
	}

	// If we have groups, display grouped view
	if len(order.Groups) > 0 {
		r.writeSubHeader(w, "Groups")
		for i, group := range order.Groups {
			fileCount := countFilesInGroup(order.Files, group.Name)
			r.writeLine(w, fmt.Sprintf("  %d. %s (%d files)", i+1, group.Name, fileCount))
			if group.Description != "" {
				r.writeLine(w, fmt.Sprintf("     %s", group.Description))
			}
		}
		r.writeLine(w, "")
	}

	// Show file list with group context
	for i, file := range order.Files {
		categoryIcon := getCategoryIcon(file.Category)
		if file.Group != "" {
			r.writeLine(w, fmt.Sprintf("  %2d. [%s] %s %s", i+1, file.Group, categoryIcon, file.Path))
		} else {
			r.writeLine(w, fmt.Sprintf("  %2d. %s %s", i+1, categoryIcon, file.Path))
		}
		if file.Description != "" {
			r.writeLine(w, fmt.Sprintf("      %s", file.Description))
		}
	}
	r.writeLine(w, "")

	return nil
}

// countFilesInGroup counts how many files belong to a specific group.
func countFilesInGroup(files []provider.OrderedFile, groupName string) int {
	count := 0
	for _, f := range files {
		if f.Group == groupName {
			count++
		}
	}
	return count
}

// RenderFileHeader displays a header for a file before its diff.
func (r *fallbackRenderer) RenderFileHeader(file *provider.OrderedFile, fileNum, totalFiles int) error {
	w := r.output

	r.writeLine(w, "")
	r.writeDivider(w)

	categoryIcon := getCategoryIcon(file.Category)
	var header string
	if file.Group != "" {
		header = fmt.Sprintf("[%d/%d] %s -> %s %s", fileNum, totalFiles, file.Group, categoryIcon, file.Path)
	} else {
		header = fmt.Sprintf("[%d/%d] %s %s", fileNum, totalFiles, categoryIcon, file.Path)
	}
	r.writeHighlight(w, header)

	if file.Description != "" {
		r.writeLine(w, fmt.Sprintf("  %s", file.Description))
	}

	r.writeDivider(w)
	r.writeLine(w, "")

	return nil
}

// RenderFileDiff displays the diff for a single file.
func (r *fallbackRenderer) RenderFileDiff(ctx context.Context, repoDir, baseRef, filePath string, fileNum, totalFiles int) error {
	colorFlag := "--color=never"
	if r.color {
		colorFlag = "--color=always"
	}

	cmd := exec.CommandContext(ctx, "git", "diff", colorFlag, baseRef+"...HEAD", "--", filePath)
	cmd.Dir = repoDir
	cmd.Stdout = r.output
	cmd.Stderr = r.output

	return cmd.Run()
}

func (r *fallbackRenderer) writeLine(w io.Writer, s string) {
	fmt.Fprintln(w, s)
}

func (r *fallbackRenderer) writeHeader(w io.Writer, s string) {
	if r.color {
		fmt.Fprintf(w, "\033[1;36m=== %s ===\033[0m\n", s)
	} else {
		fmt.Fprintf(w, "=== %s ===\n", s)
	}
}

func (r *fallbackRenderer) writeSubHeader(w io.Writer, s string) {
	if r.color {
		fmt.Fprintf(w, "\033[1m%s:\033[0m\n", s)
	} else {
		fmt.Fprintf(w, "%s:\n", s)
	}
}

func (r *fallbackRenderer) writeHighlight(w io.Writer, s string) {
	if r.color {
		fmt.Fprintf(w, "\033[1;33m%s\033[0m\n", s)
	} else {
		fmt.Fprintln(w, s)
	}
}

func (r *fallbackRenderer) writeBullet(w io.Writer, s string) {
	if r.color {
		fmt.Fprintf(w, "  \033[32m•\033[0m %s\n", s)
	} else {
		fmt.Fprintf(w, "  * %s\n", s)
	}
}

func (r *fallbackRenderer) writeWarningBullet(w io.Writer, s string) {
	if r.color {
		fmt.Fprintf(w, "  \033[33m⚠\033[0m %s\n", s)
	} else {
		fmt.Fprintf(w, "  ! %s\n", s)
	}
}

func (r *fallbackRenderer) writeDivider(w io.Writer) {
	if r.color {
		fmt.Fprintf(w, "\033[90m%s\033[0m\n", strings.Repeat("─", 60))
	} else {
		fmt.Fprintln(w, strings.Repeat("-", 60))
	}
}

// getCategoryIcon returns an icon for the file category.
func getCategoryIcon(category string) string {
	switch category {
	case provider.CategoryEntryPoint:
		return "→"
	case provider.CategoryBusinessLogic:
		return "◆"
	case provider.CategoryAdapter:
		return "◇"
	case provider.CategoryModel:
		return "●"
	case provider.CategoryConfig:
		return "⚙"
	case provider.CategoryTest:
		return "✓"
	case provider.CategoryDocs:
		return "📄"
	default:
		return "○"
	}
}
