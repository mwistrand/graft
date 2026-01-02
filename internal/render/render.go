// Package render provides output rendering for the graft CLI.
// It supports rendering diffs through Delta (when available) or falling back
// to basic git diff output, and formatting AI-generated summaries.
package render

import (
	"context"
	"io"
	"os"
	"os/exec"

	"github.com/mwistrand/graft/internal/provider"
)

// Renderer handles output display for code review.
type Renderer interface {
	// RenderSummary displays the AI-generated summary.
	RenderSummary(summary *provider.SummarizeResponse) error

	// RenderOrdering displays the file ordering with reasoning.
	RenderOrdering(order *provider.OrderResponse) error

	// RenderFileDiff displays the diff for a single file.
	RenderFileDiff(ctx context.Context, repoDir, baseRef, filePath string, fileNum, totalFiles int) error

	// RenderFileHeader displays a header for a file before its diff.
	RenderFileHeader(file *provider.OrderedFile, fileNum, totalFiles int) error
}

// Options configures the renderer.
type Options struct {
	// DeltaPath is the path to the delta binary. If empty, PATH is searched.
	DeltaPath string

	// UseDelta enables Delta rendering. If false, uses fallback.
	UseDelta bool

	// Output is where to write output. Defaults to os.Stdout.
	Output io.Writer

	// ColorEnabled controls whether ANSI colors are used.
	ColorEnabled bool
}

// DefaultOptions returns sensible defaults.
func DefaultOptions() Options {
	return Options{
		UseDelta:     true,
		Output:       os.Stdout,
		ColorEnabled: true,
	}
}

// New creates a new Renderer based on the options.
// If Delta is requested but not available, falls back to basic rendering.
func New(opts Options) Renderer {
	if opts.Output == nil {
		opts.Output = os.Stdout
	}

	if opts.UseDelta {
		deltaPath := opts.DeltaPath
		if deltaPath == "" {
			// Try to find delta in PATH
			var err error
			deltaPath, err = exec.LookPath("delta")
			if err != nil {
				// Delta not found, use fallback
				return newFallbackRenderer(opts)
			}
		}
		return newDeltaRenderer(deltaPath, opts)
	}

	return newFallbackRenderer(opts)
}

// IsDeltaAvailable checks if delta is available on the system.
func IsDeltaAvailable() bool {
	_, err := exec.LookPath("delta")
	return err == nil
}
