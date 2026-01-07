package cli

import (
	"os"
	"strings"
	"testing"

	"github.com/mwistrand/graft/internal/git"
	"github.com/mwistrand/graft/internal/provider"
)

func TestBuildFileList_WithAIOrder(t *testing.T) {
	files := []git.FileDiff{
		{Path: "handler.go", Status: git.StatusModified},
		{Path: "service.go", Status: git.StatusAdded},
	}

	aiOrder := &provider.OrderResponse{
		Files: []provider.OrderedFile{
			{Path: "service.go", Category: provider.CategoryBusinessLogic, Priority: 1, Description: "Core logic"},
			{Path: "handler.go", Category: provider.CategoryEntryPoint, Priority: 2, Description: "HTTP handler"},
		},
	}

	result := buildFileList(files, aiOrder)

	if len(result) != 2 {
		t.Fatalf("expected 2 files, got %d", len(result))
	}

	// Should use AI ordering directly
	if result[0].Path != "service.go" {
		t.Errorf("expected first file 'service.go', got %q", result[0].Path)
	}
	if result[0].Description != "Core logic" {
		t.Errorf("expected description 'Core logic', got %q", result[0].Description)
	}
}

func TestBuildFileList_WithoutAIOrder(t *testing.T) {
	files := []git.FileDiff{
		{Path: "handler.go", Status: git.StatusModified},
		{Path: "service.go", Status: git.StatusAdded},
	}

	result := buildFileList(files, nil)

	if len(result) != 2 {
		t.Fatalf("expected 2 files, got %d", len(result))
	}

	// Should maintain original order with 1-indexed priorities
	if result[0].Path != "handler.go" {
		t.Errorf("expected first file 'handler.go', got %q", result[0].Path)
	}
	if result[0].Priority != 1 {
		t.Errorf("expected priority 1, got %d", result[0].Priority)
	}
	if result[0].Description != "Modified" {
		t.Errorf("expected description 'Modified', got %q", result[0].Description)
	}

	if result[1].Path != "service.go" {
		t.Errorf("expected second file 'service.go', got %q", result[1].Path)
	}
	if result[1].Priority != 2 {
		t.Errorf("expected priority 2, got %d", result[1].Priority)
	}
	if result[1].Description != "New file" {
		t.Errorf("expected description 'New file', got %q", result[1].Description)
	}
}

func TestBuildFileList_EmptyAIOrder(t *testing.T) {
	files := []git.FileDiff{
		{Path: "main.go", Status: git.StatusModified},
	}

	// AI order exists but has no files - should fall back to default
	aiOrder := &provider.OrderResponse{
		Files: []provider.OrderedFile{},
	}

	result := buildFileList(files, aiOrder)

	if len(result) != 1 {
		t.Fatalf("expected 1 file, got %d", len(result))
	}

	if result[0].Path != "main.go" {
		t.Errorf("expected file 'main.go', got %q", result[0].Path)
	}
}

func TestBuildFileList_CategorizesFallback(t *testing.T) {
	files := []git.FileDiff{
		{Path: "cmd/main.go", Status: git.StatusModified},
		{Path: "internal/service.go", Status: git.StatusAdded},
		{Path: "service_test.go", Status: git.StatusModified},
	}

	result := buildFileList(files, nil)

	if result[0].Category != provider.CategoryEntryPoint {
		t.Errorf("expected category %q, got %q", provider.CategoryEntryPoint, result[0].Category)
	}
	if result[1].Category != provider.CategoryBusinessLogic {
		t.Errorf("expected category %q, got %q", provider.CategoryBusinessLogic, result[1].Category)
	}
	if result[2].Category != provider.CategoryTest {
		t.Errorf("expected category %q, got %q", provider.CategoryTest, result[2].Category)
	}
}

func TestBuildGroupedFileList(t *testing.T) {
	files := []provider.OrderedFile{
		{Path: "auth/handler.go", Priority: 1, Group: "User Auth"},
		{Path: "auth/service.go", Priority: 2, Group: "User Auth"},
		{Path: "api/routes.go", Priority: 1, Group: "API Layer"},
		{Path: "api/handler.go", Priority: 2, Group: "API Layer"},
		{Path: "config.json", Priority: 1, Group: ""},
	}

	selectedGroups := []provider.OrderGroup{
		{Name: "API Layer", Priority: 1},
		{Name: "User Auth", Priority: 2},
	}

	result := buildGroupedFileList(files, selectedGroups)

	// Should have all files that belong to selected groups plus ungrouped
	if len(result) != 5 {
		t.Errorf("expected 5 files, got %d", len(result))
	}

	// First should be API Layer files (selected first)
	if result[0].Group != "API Layer" {
		t.Errorf("expected first file to be from 'API Layer', got %q", result[0].Group)
	}
	if result[0].Path != "api/routes.go" {
		t.Errorf("expected first file to be 'api/routes.go', got %q", result[0].Path)
	}

	// Second API Layer file
	if result[1].Group != "API Layer" {
		t.Errorf("expected second file to be from 'API Layer', got %q", result[1].Group)
	}

	// Then User Auth files
	if result[2].Group != "User Auth" {
		t.Errorf("expected third file to be from 'User Auth', got %q", result[2].Group)
	}
	if result[2].Path != "auth/handler.go" {
		t.Errorf("expected third file to be 'auth/handler.go', got %q", result[2].Path)
	}

	// Ungrouped files at the end
	if result[4].Group != "" {
		t.Errorf("expected last file to be ungrouped, got group %q", result[4].Group)
	}
}

func TestBuildGroupedFileList_FiltersBySelectedGroups(t *testing.T) {
	files := []provider.OrderedFile{
		{Path: "auth/handler.go", Priority: 1, Group: "User Auth"},
		{Path: "api/routes.go", Priority: 1, Group: "API Layer"},
		{Path: "db/model.go", Priority: 1, Group: "Database"},
	}

	// Only select API Layer
	selectedGroups := []provider.OrderGroup{
		{Name: "API Layer", Priority: 1},
	}

	result := buildGroupedFileList(files, selectedGroups)

	// Should only have API Layer files
	if len(result) != 1 {
		t.Errorf("expected 1 file, got %d", len(result))
	}
	if result[0].Group != "API Layer" {
		t.Errorf("expected file from 'API Layer', got %q", result[0].Group)
	}
}

func TestBuildGroupedFileList_PreservesPriorityWithinGroup(t *testing.T) {
	files := []provider.OrderedFile{
		{Path: "file3.go", Priority: 3, Group: "Group A"},
		{Path: "file1.go", Priority: 1, Group: "Group A"},
		{Path: "file2.go", Priority: 2, Group: "Group A"},
	}

	selectedGroups := []provider.OrderGroup{
		{Name: "Group A", Priority: 1},
	}

	result := buildGroupedFileList(files, selectedGroups)

	if len(result) != 3 {
		t.Fatalf("expected 3 files, got %d", len(result))
	}

	// Files should be ordered by priority within the group
	if result[0].Path != "file1.go" {
		t.Errorf("expected first file 'file1.go', got %q", result[0].Path)
	}
	if result[1].Path != "file2.go" {
		t.Errorf("expected second file 'file2.go', got %q", result[1].Path)
	}
	if result[2].Path != "file3.go" {
		t.Errorf("expected third file 'file3.go', got %q", result[2].Path)
	}
}

func TestBuildGroupedFileList_EmptySelectedGroups(t *testing.T) {
	files := []provider.OrderedFile{
		{Path: "a.go", Priority: 1, Group: "Group A"},
		{Path: "b.go", Priority: 1, Group: ""},
	}

	// No groups selected - should only return ungrouped files
	selectedGroups := []provider.OrderGroup{}

	result := buildGroupedFileList(files, selectedGroups)

	// Should only have ungrouped files
	if len(result) != 1 {
		t.Errorf("expected 1 file (ungrouped), got %d", len(result))
	}
	if result[0].Path != "b.go" {
		t.Errorf("expected ungrouped file 'b.go', got %q", result[0].Path)
	}
}

func TestBuildGroupedFileList_UngroupedFilesAtEnd(t *testing.T) {
	files := []provider.OrderedFile{
		{Path: "ungrouped.go", Priority: 1, Group: ""},
		{Path: "grouped.go", Priority: 1, Group: "Group A"},
	}

	selectedGroups := []provider.OrderGroup{
		{Name: "Group A", Priority: 1},
	}

	result := buildGroupedFileList(files, selectedGroups)

	if len(result) != 2 {
		t.Fatalf("expected 2 files, got %d", len(result))
	}

	// Grouped file should come first
	if result[0].Path != "grouped.go" {
		t.Errorf("expected first file 'grouped.go', got %q", result[0].Path)
	}
	// Ungrouped at end
	if result[1].Path != "ungrouped.go" {
		t.Errorf("expected last file 'ungrouped.go', got %q", result[1].Path)
	}
}

func TestCategorizeFile(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"main_test.go", provider.CategoryTest},
		{"service_test.go", provider.CategoryTest},
		{"test_helper.go", provider.CategoryTest},
		{"cmd/main.go", provider.CategoryEntryPoint},
		{"main.go", provider.CategoryEntryPoint},
		{"internal/service.go", provider.CategoryBusinessLogic},
		{"pkg/utils.go", provider.CategoryBusinessLogic},
		{"adapter/db.go", provider.CategoryAdapter},
		{"repository/user.go", provider.CategoryAdapter},
		{"client/http.go", provider.CategoryAdapter},
		{"model/user.go", provider.CategoryModel},
		{"entity/order.go", provider.CategoryModel},
		{"types/common.go", provider.CategoryModel},
		{"config.json", provider.CategoryConfig},
		{"settings.yaml", provider.CategoryConfig},
		{"app.toml", provider.CategoryConfig},
		{"README.md", provider.CategoryDocs},
		{"doc/guide.md", provider.CategoryDocs},
		{"docs/api.md", provider.CategoryDocs},
		{"random.go", provider.CategoryOther},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := categorizeFile(tt.path)
			if got != tt.expected {
				t.Errorf("categorizeFile(%q) = %q, want %q", tt.path, got, tt.expected)
			}
		})
	}
}

func TestDescribeStatus(t *testing.T) {
	tests := []struct {
		status   string
		expected string
	}{
		{"added", "New file"},
		{"deleted", "Deleted"},
		{"renamed", "Renamed"},
		{"modified", "Modified"},
		{"unknown", "Modified"},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			got := describeStatus(tt.status)
			if got != tt.expected {
				t.Errorf("describeStatus(%q) = %q, want %q", tt.status, got, tt.expected)
			}
		})
	}
}

func TestContainsAny(t *testing.T) {
	tests := []struct {
		s       string
		substrs []string
		want    bool
	}{
		{"main_test.go", []string{"_test.go", "_test."}, true},
		{"main.go", []string{"_test.go", "_test."}, false},
		{"cmd/main.go", []string{"cmd/", "main.go"}, true},
		{"something.go", []string{"foo", "bar"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.s, func(t *testing.T) {
			got := containsAny(tt.s, tt.substrs...)
			if got != tt.want {
				t.Errorf("containsAny(%q, %v) = %v, want %v", tt.s, tt.substrs, got, tt.want)
			}
		})
	}
}

func TestLoadReviewPrompt(t *testing.T) {
	t.Run("override file exists", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create .graft directory and override file
		graftDir := tmpDir + "/.graft"
		if err := os.MkdirAll(graftDir, 0755); err != nil {
			t.Fatal(err)
		}
		overridePath := graftDir + "/code-reviewer.md"
		expectedContent := "You are a custom code review expert."
		if err := os.WriteFile(overridePath, []byte(expectedContent), 0644); err != nil {
			t.Fatal(err)
		}

		content, err := loadReviewPrompt(tmpDir)
		if err != nil {
			t.Fatalf("loadReviewPrompt() failed: %v", err)
		}
		if content != expectedContent {
			t.Errorf("content = %q, want %q", content, expectedContent)
		}
	})

	t.Run("no override uses embedded default", func(t *testing.T) {
		tmpDir := t.TempDir()

		content, err := loadReviewPrompt(tmpDir)
		if err != nil {
			t.Fatalf("loadReviewPrompt() failed: %v", err)
		}
		// Should return the embedded default prompt
		if content == "" {
			t.Error("content should not be empty when using default")
		}
		if !strings.Contains(content, "code reviewer") {
			t.Error("content should contain 'code reviewer' from default prompt")
		}
	})
}

func TestOutputAIReview_ToFile(t *testing.T) {
	tmpDir := t.TempDir()
	outputPath := tmpDir + "/review.md"
	content := "# Code Review\n\nThis is a test review."

	err := outputAIReview(content, outputPath)
	if err != nil {
		t.Fatalf("outputAIReview() failed: %v", err)
	}

	// Verify file was written
	written, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}
	if string(written) != content {
		t.Errorf("written content = %q, want %q", string(written), content)
	}
}

func TestOutputAIReview_ToConsole(t *testing.T) {
	// Just verify it doesn't error - console output is hard to test
	content := "# Code Review\n\nThis is a test review."
	err := outputAIReview(content, "")
	if err != nil {
		t.Fatalf("outputAIReview() failed: %v", err)
	}
}

func TestOutputAIReview_EmptyContent(t *testing.T) {
	tmpDir := t.TempDir()
	outputPath := tmpDir + "/review.md"

	err := outputAIReview("", outputPath)
	if err == nil {
		t.Fatal("expected error for empty content")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Errorf("error should mention empty content: %v", err)
	}
}

func TestOutputAIReview_CreatesParentDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	// Create a nested path where the parent directory doesn't exist
	outputPath := tmpDir + "/nested/subdir/review.md"
	content := "# Code Review\n\nThis is a test review."

	err := outputAIReview(content, outputPath)
	if err != nil {
		t.Fatalf("outputAIReview() failed: %v", err)
	}

	// Verify file was written
	written, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}
	if string(written) != content {
		t.Errorf("written content = %q, want %q", string(written), content)
	}
}
