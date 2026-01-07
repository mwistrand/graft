package render

import (
	"bytes"
	"context"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/mwistrand/graft/internal/provider"
)

func TestDefaultOptions(t *testing.T) {
	opts := DefaultOptions()

	if !opts.UseDelta {
		t.Error("UseDelta should be true by default")
	}
	if !opts.ColorEnabled {
		t.Error("ColorEnabled should be true by default")
	}
	if opts.Output == nil {
		t.Error("Output should not be nil")
	}
}

func TestNew_FallbackWhenNoDelta(t *testing.T) {
	opts := DefaultOptions()
	opts.UseDelta = false

	buf := new(bytes.Buffer)
	opts.Output = buf

	r := New(opts)

	// Should be a fallback renderer
	_, ok := r.(*fallbackRenderer)
	if !ok {
		t.Error("expected fallbackRenderer when UseDelta is false")
	}
}

func TestNew_DeltaWhenAvailable(t *testing.T) {
	// Skip if delta is not installed
	if _, err := exec.LookPath("delta"); err != nil {
		t.Skip("delta not installed")
	}

	opts := DefaultOptions()
	buf := new(bytes.Buffer)
	opts.Output = buf

	r := New(opts)

	_, ok := r.(*deltaRenderer)
	if !ok {
		t.Error("expected deltaRenderer when delta is available")
	}
}

func TestIsDeltaAvailable(t *testing.T) {
	// Just test that it doesn't panic
	_ = IsDeltaAvailable()
}

func TestFallbackRenderer_RenderSummary(t *testing.T) {
	buf := new(bytes.Buffer)
	r := newFallbackRenderer(Options{Output: buf, ColorEnabled: false})

	summary := &provider.SummarizeResponse{
		Overview: "Test overview",
		KeyChanges: []string{
			"Change 1",
			"Change 2",
		},
		Concerns: []string{
			"Concern 1",
		},
		FileGroups: []provider.FileGroup{
			{
				Name:        "API",
				Description: "API changes",
				Files:       []string{"api.go"},
			},
		},
	}

	err := r.RenderSummary(summary)
	if err != nil {
		t.Fatalf("RenderSummary() failed: %v", err)
	}

	output := buf.String()

	// Check key elements are present
	if !containsString(output, "Change Summary") {
		t.Error("output should contain 'Change Summary'")
	}
	if !containsString(output, "Test overview") {
		t.Error("output should contain overview")
	}
	if !containsString(output, "Change 1") {
		t.Error("output should contain changes")
	}
	if !containsString(output, "Concern 1") {
		t.Error("output should contain concerns")
	}
	if !containsString(output, "API") {
		t.Error("output should contain file group name")
	}
}

func TestFallbackRenderer_RenderOrdering(t *testing.T) {
	buf := new(bytes.Buffer)
	r := newFallbackRenderer(Options{Output: buf, ColorEnabled: false})

	order := &provider.OrderResponse{
		Reasoning: "Files ordered by architectural flow",
		Files: []provider.OrderedFile{
			{Path: "main.go", Category: provider.CategoryEntryPoint, Description: "Entry point"},
			{Path: "service.go", Category: provider.CategoryBusinessLogic, Description: "Business logic"},
		},
	}

	err := r.RenderOrdering(order)
	if err != nil {
		t.Fatalf("RenderOrdering() failed: %v", err)
	}

	output := buf.String()

	if !containsString(output, "Review Order") {
		t.Error("output should contain 'Review Order'")
	}
	if !containsString(output, "main.go") {
		t.Error("output should contain file paths")
	}
	if !containsString(output, "Entry point") {
		t.Error("output should contain descriptions")
	}
}

func TestFallbackRenderer_RenderFileHeader(t *testing.T) {
	buf := new(bytes.Buffer)
	r := newFallbackRenderer(Options{Output: buf, ColorEnabled: false})

	file := &provider.OrderedFile{
		Path:        "internal/service.go",
		Category:    provider.CategoryBusinessLogic,
		Description: "Core service implementation",
	}

	err := r.RenderFileHeader(file, 3, 10)
	if err != nil {
		t.Fatalf("RenderFileHeader() failed: %v", err)
	}

	output := buf.String()

	if !containsString(output, "[3/10]") {
		t.Error("output should contain file number")
	}
	if !containsString(output, "internal/service.go") {
		t.Error("output should contain file path")
	}
}

func TestFallbackRenderer_RenderFileDiff(t *testing.T) {
	// Create a temporary git repo
	dir := t.TempDir()

	// Initialize git repo
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "Test User")

	// Create initial commit
	writeFile(t, dir, "test.go", "package main\n")
	runGit(t, dir, "add", "test.go")
	runGit(t, dir, "commit", "-m", "Initial commit")

	branch := getCurrentBranch(t, dir)

	// Create a feature branch with changes
	runGit(t, dir, "checkout", "-b", "feature")
	writeFile(t, dir, "test.go", "package main\n\nfunc main() {}\n")
	runGit(t, dir, "add", "test.go")
	runGit(t, dir, "commit", "-m", "Add main function")

	buf := new(bytes.Buffer)
	r := newFallbackRenderer(Options{Output: buf, ColorEnabled: false})

	err := r.RenderFileDiff(context.Background(), dir, branch, "test.go", 1, 1)
	if err != nil {
		t.Fatalf("RenderFileDiff() failed: %v", err)
	}

	output := buf.String()

	// Should contain diff markers
	if !containsString(output, "+func main()") {
		t.Error("output should contain added line")
	}
}

func TestGetCategoryIcon(t *testing.T) {
	tests := []struct {
		category string
		wantIcon string
	}{
		{provider.CategoryEntryPoint, "→"},
		{provider.CategoryBusinessLogic, "◆"},
		{provider.CategoryAdapter, "◇"},
		{provider.CategoryModel, "●"},
		{provider.CategoryConfig, "⚙"},
		{provider.CategoryTest, "✓"},
		{provider.CategoryDocs, "📄"},
		{provider.CategoryOther, "○"},
		{"unknown", "○"},
	}

	for _, tt := range tests {
		t.Run(tt.category, func(t *testing.T) {
			got := getCategoryIcon(tt.category)
			if got != tt.wantIcon {
				t.Errorf("getCategoryIcon(%q) = %q, want %q", tt.category, got, tt.wantIcon)
			}
		})
	}
}

func TestFallbackRenderer_RenderOrdering_WithGroups(t *testing.T) {
	buf := new(bytes.Buffer)
	r := newFallbackRenderer(Options{Output: buf, ColorEnabled: false})

	order := &provider.OrderResponse{
		Reasoning: "Files grouped by feature",
		Groups: []provider.OrderGroup{
			{Name: "User Auth", Description: "Authentication changes", Priority: 1},
			{Name: "API Layer", Description: "API endpoint updates", Priority: 2},
		},
		Files: []provider.OrderedFile{
			{Path: "auth/handler.go", Category: provider.CategoryEntryPoint, Description: "Auth handler", Group: "User Auth"},
			{Path: "auth/service.go", Category: provider.CategoryBusinessLogic, Description: "Auth service", Group: "User Auth"},
			{Path: "api/routes.go", Category: provider.CategoryRouting, Description: "API routes", Group: "API Layer"},
		},
	}

	err := r.RenderOrdering(order)
	if err != nil {
		t.Fatalf("RenderOrdering() failed: %v", err)
	}

	output := buf.String()

	// Check groups section
	if !containsString(output, "Groups") {
		t.Error("output should contain 'Groups' header")
	}
	if !containsString(output, "User Auth") {
		t.Error("output should contain group name 'User Auth'")
	}
	if !containsString(output, "API Layer") {
		t.Error("output should contain group name 'API Layer'")
	}
	if !containsString(output, "(2 files)") {
		t.Error("output should show file count for User Auth group")
	}
	if !containsString(output, "(1 files)") {
		t.Error("output should show file count for API Layer group")
	}

	// Check files show group context
	if !containsString(output, "[User Auth]") {
		t.Error("output should show group context for files")
	}
	if !containsString(output, "[API Layer]") {
		t.Error("output should show group context for API Layer files")
	}
}

func TestFallbackRenderer_RenderFileHeader_WithGroup(t *testing.T) {
	buf := new(bytes.Buffer)
	r := newFallbackRenderer(Options{Output: buf, ColorEnabled: false})

	file := &provider.OrderedFile{
		Path:        "internal/auth/handler.go",
		Category:    provider.CategoryEntryPoint,
		Description: "Authentication handler",
		Group:       "User Auth",
	}

	err := r.RenderFileHeader(file, 2, 5)
	if err != nil {
		t.Fatalf("RenderFileHeader() failed: %v", err)
	}

	output := buf.String()

	// Check group context is shown
	if !containsString(output, "[2/5]") {
		t.Error("output should contain file number")
	}
	if !containsString(output, "User Auth ->") {
		t.Error("output should contain group name with arrow")
	}
	if !containsString(output, "internal/auth/handler.go") {
		t.Error("output should contain file path")
	}
}

func TestFallbackRenderer_RenderFileHeader_WithoutGroup(t *testing.T) {
	buf := new(bytes.Buffer)
	r := newFallbackRenderer(Options{Output: buf, ColorEnabled: false})

	file := &provider.OrderedFile{
		Path:        "config.json",
		Category:    provider.CategoryConfig,
		Description: "Configuration file",
		// No Group set
	}

	err := r.RenderFileHeader(file, 1, 1)
	if err != nil {
		t.Fatalf("RenderFileHeader() failed: %v", err)
	}

	output := buf.String()

	// Check no group context for ungrouped files
	if containsString(output, "->") {
		t.Error("output should not contain arrow for ungrouped files")
	}
	if !containsString(output, "[1/1]") {
		t.Error("output should contain file number")
	}
	if !containsString(output, "config.json") {
		t.Error("output should contain file path")
	}
}

func TestCountFilesInGroup(t *testing.T) {
	files := []provider.OrderedFile{
		{Path: "a.go", Group: "Group A"},
		{Path: "b.go", Group: "Group A"},
		{Path: "c.go", Group: "Group B"},
		{Path: "d.go", Group: "Group A"},
		{Path: "e.go", Group: ""},
	}

	tests := []struct {
		groupName string
		want      int
	}{
		{"Group A", 3},
		{"Group B", 1},
		{"Group C", 0},
		{"", 1}, // Empty string matches ungrouped files
	}

	for _, tt := range tests {
		t.Run(tt.groupName, func(t *testing.T) {
			got := countFilesInGroup(files, tt.groupName)
			if got != tt.want {
				t.Errorf("countFilesInGroup(%q) = %d, want %d", tt.groupName, got, tt.want)
			}
		})
	}
}

// Helper functions

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %s\n%s", args, err, output)
	}
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := exec.Command("sh", "-c", "echo '"+content+"' > "+path).Run(); err != nil {
		t.Fatal(err)
	}
}

func getCurrentBranch(t *testing.T, dir string) string {
	t.Helper()
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	return trimSpace(string(output))
}

func trimSpace(s string) string {
	start := 0
	for start < len(s) && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	end := len(s)
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}

func containsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
