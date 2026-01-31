// Package provider defines the interface for AI providers used in code review.
// Implementations can use different backends (Claude, OpenAI, etc.) while
// presenting a consistent interface to the rest of the application.
package provider

import (
	"context"
	"strings"

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

// Significance indicates the importance tier of a group change.
type Significance string

const (
	// SignificanceCore indicates major logic changes, new features, API changes.
	SignificanceCore Significance = "core"
	// SignificanceSupporting indicates tests, utilities, helpers.
	SignificanceSupporting Significance = "supporting"
	// SignificanceMinor indicates config, docs, formatting, dependency updates.
	SignificanceMinor Significance = "minor"
)

// AllSignificanceTiers returns all significance tiers in priority order.
func AllSignificanceTiers() []Significance {
	return []Significance{
		SignificanceCore,
		SignificanceSupporting,
		SignificanceMinor,
	}
}

// SignificanceDisplayName returns a human-readable name for a significance tier.
func SignificanceDisplayName(s Significance) string {
	switch s {
	case SignificanceCore:
		return "Core Changes"
	case SignificanceSupporting:
		return "Supporting"
	case SignificanceMinor:
		return "Minor"
	default:
		return string(s)
	}
}

// SignificancePriority returns the sort priority for a significance tier (lower = first).
func SignificancePriority(s Significance) int {
	switch s {
	case SignificanceCore:
		return 1
	case SignificanceSupporting:
		return 2
	case SignificanceMinor:
		return 3
	default:
		return 4
	}
}

// NormalizeSignificance returns the significance, defaulting to "core" if empty.
func NormalizeSignificance(s Significance) Significance {
	if s == "" {
		return SignificanceCore
	}
	return s
}

// OrderGroup represents a feature group of related files.
type OrderGroup struct {
	// Name is the group identifier (matches OrderedFile.Group).
	Name string `json:"name"`

	// Description explains what this feature/change accomplishes.
	Description string `json:"description"`

	// Priority determines group review order (1 = first).
	Priority int `json:"priority"`

	// Significance indicates the importance tier (core, supporting, minor).
	// Defaults to "core" if not specified for backwards compatibility.
	Significance Significance `json:"significance,omitempty"`
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

	// Categories limits the review to specific categories (empty = all).
	Categories []ReviewCategory
}

// ReviewSeverity indicates the importance of a review comment.
type ReviewSeverity string

const (
	// SeverityCritical indicates a must-fix issue (bugs, security, design flaws).
	SeverityCritical ReviewSeverity = "critical"
	// SeveritySuggestion indicates a should-consider improvement.
	SeveritySuggestion ReviewSeverity = "suggestion"
	// SeverityNit indicates a minor/optional issue (style, preferences).
	SeverityNit ReviewSeverity = "nit"
)

// ReviewCategory represents a Google code review checklist category.
type ReviewCategory string

const (
	CategoryDesign        ReviewCategory = "design"
	CategoryFunctionality ReviewCategory = "functionality"
	CategoryComplexity    ReviewCategory = "complexity"
	CategoryTests         ReviewCategory = "tests"
	CategoryNaming        ReviewCategory = "naming"
	CategoryComments      ReviewCategory = "comments"
	CategoryStyle         ReviewCategory = "style"
	CategoryDocumentation ReviewCategory = "documentation"
	CategoryPraise        ReviewCategory = "praise"
)

// AllReviewCategories returns all review categories in display order.
func AllReviewCategories() []ReviewCategory {
	return []ReviewCategory{
		CategoryDesign,
		CategoryFunctionality,
		CategoryComplexity,
		CategoryTests,
		CategoryNaming,
		CategoryComments,
		CategoryStyle,
		CategoryDocumentation,
		CategoryPraise,
	}
}

// ReviewComment represents a single finding from the code review.
// Each comment belongs to a category and has a severity level.
type ReviewComment struct {
	// Category is the review category this comment belongs to.
	Category ReviewCategory `json:"category"`
	// Severity indicates the importance of this finding.
	Severity ReviewSeverity `json:"severity"`
	// File is the file path (optional for general comments).
	File string `json:"file,omitempty"`
	// Line is the line number (0 if not applicable).
	Line int `json:"line,omitempty"`
	// Title is a short summary (1 line).
	Title string `json:"title"`
	// Description is the detailed explanation.
	Description string `json:"description"`
	// Suggestion is an optional code suggestion.
	Suggestion string `json:"suggestion,omitempty"`
}

// StructuredReview contains categorized review findings.
type StructuredReview struct {
	// Summary is an executive summary (2-3 sentences).
	Summary string `json:"summary"`
	// Comments contains all findings, categorized.
	Comments []ReviewComment `json:"comments"`
}

// CommentsByCategory returns comments grouped by category.
func (s *StructuredReview) CommentsByCategory() map[ReviewCategory][]ReviewComment {
	result := make(map[ReviewCategory][]ReviewComment)
	for _, c := range s.Comments {
		result[c.Category] = append(result[c.Category], c)
	}
	return result
}

// CountBySeverity returns the count of comments for each severity level.
func (s *StructuredReview) CountBySeverity() map[ReviewSeverity]int {
	result := make(map[ReviewSeverity]int)
	for _, c := range s.Comments {
		result[c.Severity]++
	}
	return result
}

// FilterBySeverity returns a new StructuredReview with only comments at or above the minimum severity.
// Severity order: critical > suggestion > nit
func (s *StructuredReview) FilterBySeverity(minSeverity ReviewSeverity) *StructuredReview {
	if minSeverity == "" {
		return s
	}

	minLevel := severityLevel(minSeverity)
	filtered := &StructuredReview{
		Summary:  s.Summary,
		Comments: make([]ReviewComment, 0),
	}

	for _, c := range s.Comments {
		if severityLevel(c.Severity) >= minLevel {
			filtered.Comments = append(filtered.Comments, c)
		}
	}

	return filtered
}

// severityLevel returns a numeric level for severity comparison (higher = more severe).
func severityLevel(s ReviewSeverity) int {
	switch s {
	case SeverityCritical:
		return 3
	case SeveritySuggestion:
		return 2
	case SeverityNit:
		return 1
	default:
		return 0
	}
}

// ParseReviewCategories parses a comma-separated list of category names.
func ParseReviewCategories(s string) []ReviewCategory {
	if s == "" {
		return nil
	}

	parts := splitAndTrim(s, ",")
	categories := make([]ReviewCategory, 0, len(parts))

	for _, p := range parts {
		cat := ReviewCategory(p)
		// Validate it's a known category
		switch cat {
		case CategoryDesign, CategoryFunctionality, CategoryComplexity,
			CategoryTests, CategoryNaming, CategoryComments,
			CategoryStyle, CategoryDocumentation, CategoryPraise:
			categories = append(categories, cat)
		}
	}

	return categories
}

// ParseReviewSeverity parses a severity string.
func ParseReviewSeverity(s string) ReviewSeverity {
	switch ReviewSeverity(s) {
	case SeverityCritical, SeveritySuggestion, SeverityNit:
		return ReviewSeverity(s)
	default:
		return ""
	}
}

// splitAndTrim splits a string and trims whitespace from each part.
func splitAndTrim(s, sep string) []string {
	var result []string
	for _, part := range strings.Split(s, sep) {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// ReviewResponse contains the AI-generated detailed code review.
type ReviewResponse struct {
	// Content is the full markdown-formatted review (legacy, for backwards compat).
	Content string `json:"content"`

	// Structured contains categorized review findings.
	// Will be nil for legacy cached reviews.
	Structured *StructuredReview `json:"structured,omitempty"`
}

// DefaultReviewOptions returns sensible defaults for reviews.
func DefaultReviewOptions() ReviewOptions {
	return ReviewOptions{
		MaxTokens: 8192,
	}
}
