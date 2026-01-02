// Package mock provides a mock AI provider for testing.
package mock

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/mwistrand/graft/internal/git"
	"github.com/mwistrand/graft/internal/provider"
)

// Provider is a mock AI provider for testing.
type Provider struct {
	// SummarizeFunc allows customizing the SummarizeChanges behavior.
	SummarizeFunc func(ctx context.Context, req *provider.SummarizeRequest) (*provider.SummarizeResponse, error)

	// OrderFunc allows customizing the OrderFiles behavior.
	OrderFunc func(ctx context.Context, req *provider.OrderRequest) (*provider.OrderResponse, error)

	// SummarizeCalls tracks calls to SummarizeChanges.
	SummarizeCalls []*provider.SummarizeRequest

	// OrderCalls tracks calls to OrderFiles.
	OrderCalls []*provider.OrderRequest
}

// New creates a new mock provider with default behavior.
func New() *Provider {
	return &Provider{}
}

// Name returns "mock".
func (p *Provider) Name() string {
	return "mock"
}

// SummarizeChanges returns a mock summary or calls the custom function.
func (p *Provider) SummarizeChanges(ctx context.Context, req *provider.SummarizeRequest) (*provider.SummarizeResponse, error) {
	p.SummarizeCalls = append(p.SummarizeCalls, req)

	if p.SummarizeFunc != nil {
		return p.SummarizeFunc(ctx, req)
	}

	// Default mock response
	return &provider.SummarizeResponse{
		Overview: "Mock summary of changes",
		KeyChanges: []string{
			"Changed " + pluralize(len(req.Files), "file"),
			"Made various modifications",
		},
		Concerns: nil,
		FileGroups: []provider.FileGroup{
			{
				Name:        "All Changes",
				Description: "All modified files",
				Files:       extractPaths(req.Files),
			},
		},
	}, nil
}

// OrderFiles returns files in alphabetical order or calls the custom function.
func (p *Provider) OrderFiles(ctx context.Context, req *provider.OrderRequest) (*provider.OrderResponse, error) {
	p.OrderCalls = append(p.OrderCalls, req)

	if p.OrderFunc != nil {
		return p.OrderFunc(ctx, req)
	}

	// Default: order files alphabetically with basic categorization
	files := make([]provider.OrderedFile, len(req.Files))
	for i, f := range req.Files {
		files[i] = provider.OrderedFile{
			Path:        f.Path,
			Category:    categorizeFile(f.Path),
			Priority:    i + 1,
			Description: describeFile(f),
		}
	}

	// Sort by path
	sort.Slice(files, func(i, j int) bool {
		return files[i].Path < files[j].Path
	})

	// Update priorities after sort
	for i := range files {
		files[i].Priority = i + 1
	}

	return &provider.OrderResponse{
		Files:     files,
		Reasoning: "Mock ordering: files sorted alphabetically",
	}, nil
}

// Reset clears recorded calls.
func (p *Provider) Reset() {
	p.SummarizeCalls = nil
	p.OrderCalls = nil
}

// extractPaths returns the paths from a slice of FileDiffs.
func extractPaths(files []git.FileDiff) []string {
	paths := make([]string, len(files))
	for i, f := range files {
		paths[i] = f.Path
	}
	return paths
}

func categorizeFile(path string) string {
	switch {
	case strings.Contains(path, "_test.go") || strings.Contains(path, "_test."):
		return provider.CategoryTest
	case strings.Contains(path, "cmd/") || strings.Contains(path, "main.go"):
		return provider.CategoryEntryPoint
	case strings.Contains(path, "internal/") || strings.Contains(path, "pkg/"):
		return provider.CategoryBusinessLogic
	case strings.Contains(path, "adapter") || strings.Contains(path, "repository"):
		return provider.CategoryAdapter
	case strings.Contains(path, "model") || strings.Contains(path, "entity"):
		return provider.CategoryModel
	case strings.Contains(path, "config") || strings.Contains(path, ".json") || strings.Contains(path, ".yaml"):
		return provider.CategoryConfig
	case strings.Contains(path, ".md") || strings.Contains(path, "doc"):
		return provider.CategoryDocs
	default:
		return provider.CategoryOther
	}
}

// describeFile generates a simple description.
func describeFile(f git.FileDiff) string {
	switch f.Status {
	case git.StatusAdded:
		return "New file"
	case git.StatusDeleted:
		return "Deleted file"
	case git.StatusRenamed:
		return "Renamed from " + f.OldPath
	default:
		return "Modified file"
	}
}

// pluralize adds "s" for plural counts.
func pluralize(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return fmt.Sprintf("%d %ss", n, word)
}
