package render

import (
	"context"
	"os"
	"os/exec"

	"github.com/mwistrand/graft/internal/provider"
)

// deltaRenderer renders diffs through the Delta pager.
type deltaRenderer struct {
	deltaPath string
	fallback  *fallbackRenderer
}

func newDeltaRenderer(deltaPath string, opts Options) *deltaRenderer {
	return &deltaRenderer{
		deltaPath: deltaPath,
		fallback:  newFallbackRenderer(opts),
	}
}

// RenderSummary displays the AI-generated summary.
// Uses the fallback renderer since summaries don't need Delta.
func (r *deltaRenderer) RenderSummary(summary *provider.SummarizeResponse) error {
	return r.fallback.RenderSummary(summary)
}

// RenderOrdering displays the file ordering with reasoning.
// Uses the fallback renderer since ordering doesn't need Delta.
func (r *deltaRenderer) RenderOrdering(order *provider.OrderResponse) error {
	return r.fallback.RenderOrdering(order)
}

// RenderFileHeader displays a header for a file before its diff.
// Uses the fallback renderer for headers.
func (r *deltaRenderer) RenderFileHeader(file *provider.OrderedFile, fileNum, totalFiles int) error {
	return r.fallback.RenderFileHeader(file, fileNum, totalFiles)
}

// RenderFileDiff displays the diff for a single file through Delta.
func (r *deltaRenderer) RenderFileDiff(ctx context.Context, repoDir, baseRef, filePath string, fileNum, totalFiles int) error {
	gitCmd := exec.CommandContext(ctx, "git", "diff", "--color=always", baseRef+"...HEAD", "--", filePath)
	gitCmd.Dir = repoDir

	deltaCmd := exec.CommandContext(ctx, r.deltaPath)

	pipe, err := gitCmd.StdoutPipe()
	if err != nil {
		return r.fallback.RenderFileDiff(ctx, repoDir, baseRef, filePath, fileNum, totalFiles)
	}
	deltaCmd.Stdin = pipe
	deltaCmd.Stdout = os.Stdout
	deltaCmd.Stderr = os.Stderr

	if err := deltaCmd.Start(); err != nil {
		pipe.Close()
		return r.fallback.RenderFileDiff(ctx, repoDir, baseRef, filePath, fileNum, totalFiles)
	}

	if err := gitCmd.Run(); err != nil {
		deltaCmd.Wait()
		return err
	}

	return deltaCmd.Wait()
}

// RenderFullDiff renders the complete diff through Delta.
func (r *deltaRenderer) RenderFullDiff(ctx context.Context, repoDir, baseRef string) error {
	gitCmd := exec.CommandContext(ctx, "git", "diff", "--color=always", baseRef+"...HEAD")
	gitCmd.Dir = repoDir

	deltaCmd := exec.CommandContext(ctx, r.deltaPath)

	pipe, err := gitCmd.StdoutPipe()
	if err != nil {
		return err
	}
	deltaCmd.Stdin = pipe
	deltaCmd.Stdout = os.Stdout
	deltaCmd.Stderr = os.Stderr

	if err := deltaCmd.Start(); err != nil {
		pipe.Close()
		return err
	}

	if err := gitCmd.Run(); err != nil {
		deltaCmd.Wait()
		return err
	}

	return deltaCmd.Wait()
}
