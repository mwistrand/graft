// Package provider defines the interface for AI providers used in code review.
// Implementations can use different backends (Claude, OpenAI, etc.) while
// presenting a consistent interface to the rest of the application.
package provider

import (
	"context"

	"github.com/mwistrand/graft/internal/git"
)

// Provider defines the interface for AI-powered code review operations.
// Implementations exist for Claude, OpenAI, and other LLM providers.
type Provider interface {
	// Name returns the provider identifier (e.g., "claude", "openai").
	Name() string

	// SummarizeChanges analyzes a diff and returns a structured summary.
	SummarizeChanges(ctx context.Context, req *SummarizeRequest) (*SummarizeResponse, error)

	// OrderFiles determines the logical review order for changed files.
	OrderFiles(ctx context.Context, req *OrderRequest) (*OrderResponse, error)

	// ReviewChanges performs a detailed code review of the changes.
	ReviewChanges(ctx context.Context, req *ReviewRequest) (*ReviewResponse, error)
}

// SummarizeRequest contains the diff context for summarization.
type SummarizeRequest struct {
	// Files contains the changed files with their metadata.
	Files []git.FileDiff

	// Commits contains the commits being reviewed.
	Commits []git.Commit

	// FullDiff contains the complete diff content for analysis.
	FullDiff string

	// Options allows customizing summarization behavior.
	Options SummarizeOptions
}

// SummarizeOptions allows customizing summarization behavior.
type SummarizeOptions struct {
	// MaxTokens limits the response length.
	MaxTokens int

	// Temperature controls response randomness (0.0-1.0).
	Temperature float64

	// Focus optionally narrows the analysis (e.g., "security", "performance").
	Focus string
}

// SummarizeResponse contains the AI-generated summary.
type SummarizeResponse struct {
	// Overview is a high-level description of the changes (1-2 sentences).
	Overview string `json:"overview"`

	// KeyChanges lists the main changes in bullet point form.
	KeyChanges []string `json:"key_changes"`

	// Concerns lists potential issues or areas needing careful review.
	Concerns []string `json:"concerns,omitempty"`

	// FileGroups organizes files into logical groups.
	FileGroups []FileGroup `json:"file_groups,omitempty"`
}

// FileGroup represents a logical grouping of related files.
type FileGroup struct {
	// Name is the group name (e.g., "API Layer", "Database Models").
	Name string `json:"name"`

	// Description explains what this group of changes does.
	Description string `json:"description"`

	// Files lists the file paths in this group.
	Files []string `json:"files"`
}

// OrderRequest contains files to be ordered for review.
type OrderRequest struct {
	// Files contains the changed files with their metadata.
	Files []git.FileDiff

	// Commits contains the commits being reviewed (for context).
	Commits []git.Commit

	// RepoContext contains repository analysis context (optional).
	RepoContext string

	// TestsFirst indicates tests should be shown before implementation.
	TestsFirst bool
}

// OrderResponse contains the AI-determined ordering of files.
type OrderResponse struct {
	// Files contains the files in recommended review order.
	Files []OrderedFile `json:"files"`

	// Groups contains metadata about feature groups (optional).
	// When present, files are organized by logical feature/change unit.
	Groups []OrderGroup `json:"groups,omitempty"`

	// Reasoning explains the ordering strategy used.
	Reasoning string `json:"reasoning"`
}

// OrderGroup represents a feature group of related files.
type OrderGroup struct {
	// Name is the group identifier (matches OrderedFile.Group).
	Name string `json:"name"`

	// Description explains what this feature/change accomplishes.
	Description string `json:"description"`

	// Priority determines group review order (1 = first).
	Priority int `json:"priority"`
}

// OrderedFile represents a file with its review priority and metadata.
type OrderedFile struct {
	// Path is the file path relative to repository root.
	Path string `json:"path"`

	// Category classifies the file's architectural role.
	// Values: "entry_point", "business_logic", "adapter", "model", "config", "test", "docs", "other"
	Category string `json:"category"`

	// Priority determines review order (1 = first, higher = later).
	Priority int `json:"priority"`

	// Description briefly explains what this file does in context.
	Description string `json:"description"`

	// Group is the name of the feature group this file belongs to (optional).
	// Must match the Name field of an OrderGroup in the response.
	Group string `json:"group,omitempty"`
}

// Category constants for file classification.
const (
	CategoryEntryPoint    = "entry_point"
	CategoryBusinessLogic = "business_logic"
	CategoryAdapter       = "adapter"
	CategoryModel         = "model"
	CategoryConfig        = "config"
	CategoryTest          = "test"
	CategoryDocs          = "docs"
	CategoryRouting       = "routing"
	CategoryComponent     = "component"
	CategoryOther         = "other"
)

// ModelInfo describes an available AI model.
type ModelInfo struct {
	// ID is the model identifier to use in API calls.
	ID string `json:"id"`

	// Name is a human-readable display name (may equal ID if not provided).
	Name string `json:"name,omitempty"`

	// Description provides additional context about the model.
	Description string `json:"description,omitempty"`
}

// ModelLister is an optional interface for providers that can list available models.
// Use type assertion to check if a provider supports this: if lister, ok := p.(ModelLister); ok { ... }
type ModelLister interface {
	// ListModels returns the available models for this provider.
	ListModels(ctx context.Context) ([]ModelInfo, error)
}

// ModelSelector is an optional interface for providers that allow changing the model after creation.
type ModelSelector interface {
	// SetModel updates the model used by this provider.
	SetModel(model string)

	// Model returns the currently configured model.
	Model() string
}

// DefaultSummarizeOptions returns sensible defaults for summarization.
func DefaultSummarizeOptions() SummarizeOptions {
	return SummarizeOptions{
		MaxTokens:   2048,
		Temperature: 0.3,
	}
}

// ReviewRequest contains the context for a detailed code review.
type ReviewRequest struct {
	// Files contains the changed files with their metadata.
	Files []git.FileDiff

	// Commits contains the commits being reviewed.
	Commits []git.Commit

	// FullDiff contains the complete diff content for analysis.
	FullDiff string

	// SystemPrompt is the review expert system prompt.
	SystemPrompt string

	// Options allows customizing review behavior.
	Options ReviewOptions
}

// ReviewOptions allows customizing review behavior.
type ReviewOptions struct {
	// MaxTokens limits the response length.
	MaxTokens int
}

// ReviewResponse contains the AI-generated detailed code review.
type ReviewResponse struct {
	// Content is the full markdown-formatted review.
	Content string `json:"content"`
}

// DefaultReviewOptions returns sensible defaults for reviews.
func DefaultReviewOptions() ReviewOptions {
	return ReviewOptions{
		MaxTokens: 8192,
	}
}
