package analysis

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAnalyzer_DetectGo(t *testing.T) {
	dir := t.TempDir()

	// Create go.mod
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create cmd directory structure
	cmdDir := filepath.Join(dir, "cmd", "app")
	if err := os.MkdirAll(cmdDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cmdDir, "main.go"), []byte("package main"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create internal directory
	internalDir := filepath.Join(dir, "internal", "service")
	if err := os.MkdirAll(internalDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(internalDir, "service.go"), []byte("package service"), 0644); err != nil {
		t.Fatal(err)
	}

	analyzer := NewAnalyzer(dir)
	result, err := analyzer.Analyze()
	if err != nil {
		t.Fatalf("Analyze() failed: %v", err)
	}

	if result.Type != ProjectTypeBackend {
		t.Errorf("Type = %q, want %q", result.Type, ProjectTypeBackend)
	}

	if !contains(result.Languages, "Go") {
		t.Errorf("Languages = %v, should contain 'Go'", result.Languages)
	}
}

func TestAnalyzer_DetectReact(t *testing.T) {
	dir := t.TempDir()

	// Create package.json with React
	packageJSON := `{
		"dependencies": {
			"react": "^18.0.0",
			"react-dom": "^18.0.0"
		}
	}`
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(packageJSON), 0644); err != nil {
		t.Fatal(err)
	}

	// Create tsconfig.json
	if err := os.WriteFile(filepath.Join(dir, "tsconfig.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create src/components directory
	componentsDir := filepath.Join(dir, "src", "components")
	if err := os.MkdirAll(componentsDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(componentsDir, "App.tsx"), []byte("export const App = () => {}"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create src/pages directory
	pagesDir := filepath.Join(dir, "src", "pages")
	if err := os.MkdirAll(pagesDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pagesDir, "Home.tsx"), []byte("export const Home = () => {}"), 0644); err != nil {
		t.Fatal(err)
	}

	analyzer := NewAnalyzer(dir)
	result, err := analyzer.Analyze()
	if err != nil {
		t.Fatalf("Analyze() failed: %v", err)
	}

	if result.Type != ProjectTypeFrontend {
		t.Errorf("Type = %q, want %q", result.Type, ProjectTypeFrontend)
	}

	if !contains(result.Languages, "TypeScript") {
		t.Errorf("Languages = %v, should contain 'TypeScript'", result.Languages)
	}

	if !contains(result.Frameworks, "React") {
		t.Errorf("Frameworks = %v, should contain 'React'", result.Frameworks)
	}
}

func TestAnalyzer_DetectFullstack(t *testing.T) {
	dir := t.TempDir()

	// Backend: go.mod and cmd/
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test"), 0644); err != nil {
		t.Fatal(err)
	}
	cmdDir := filepath.Join(dir, "cmd", "api")
	if err := os.MkdirAll(cmdDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cmdDir, "main.go"), []byte("package main"), 0644); err != nil {
		t.Fatal(err)
	}

	// Frontend: package.json with Vue
	packageJSON := `{"dependencies": {"vue": "^3.0.0"}}`
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(packageJSON), 0644); err != nil {
		t.Fatal(err)
	}

	// Frontend structure
	componentsDir := filepath.Join(dir, "src", "components")
	if err := os.MkdirAll(componentsDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(componentsDir, "App.vue"), []byte("<template></template>"), 0644); err != nil {
		t.Fatal(err)
	}

	analyzer := NewAnalyzer(dir)
	result, err := analyzer.Analyze()
	if err != nil {
		t.Fatalf("Analyze() failed: %v", err)
	}

	if result.Type != ProjectTypeFullstack {
		t.Errorf("Type = %q, want %q", result.Type, ProjectTypeFullstack)
	}
}

func TestAnalyzer_DetectAngular(t *testing.T) {
	dir := t.TempDir()

	packageJSON := `{"dependencies": {"@angular/core": "^17.0.0"}}`
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(packageJSON), 0644); err != nil {
		t.Fatal(err)
	}

	analyzer := NewAnalyzer(dir)
	result, err := analyzer.Analyze()
	if err != nil {
		t.Fatalf("Analyze() failed: %v", err)
	}

	if !contains(result.Frameworks, "Angular") {
		t.Errorf("Frameworks = %v, should contain 'Angular'", result.Frameworks)
	}
}

func TestAnalyzer_DetectExpress(t *testing.T) {
	dir := t.TempDir()

	packageJSON := `{"dependencies": {"express": "^4.18.0"}}`
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(packageJSON), 0644); err != nil {
		t.Fatal(err)
	}

	// Create controllers directory (backend signal)
	controllersDir := filepath.Join(dir, "controllers")
	if err := os.MkdirAll(controllersDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(controllersDir, "users.js"), []byte("module.exports = {}"), 0644); err != nil {
		t.Fatal(err)
	}

	analyzer := NewAnalyzer(dir)
	result, err := analyzer.Analyze()
	if err != nil {
		t.Fatalf("Analyze() failed: %v", err)
	}

	if result.Type != ProjectTypeBackend {
		t.Errorf("Type = %q, want %q", result.Type, ProjectTypeBackend)
	}

	if !contains(result.Frameworks, "Express") {
		t.Errorf("Frameworks = %v, should contain 'Express'", result.Frameworks)
	}
}

func TestAnalysis_FormatContext(t *testing.T) {
	analysis := &Analysis{
		Type:       ProjectTypeBackend,
		Languages:  []string{"Go"},
		Frameworks: []string{},
		Directories: []DirectorySummary{
			{Path: "cmd/app", FileCount: 1, Description: "Application entry points"},
			{Path: "internal/service", FileCount: 5, Description: "Business logic"},
		},
	}

	context := analysis.FormatContext()

	if context == "" {
		t.Error("FormatContext() returned empty string")
	}

	if !containsString(context, "backend") {
		t.Error("FormatContext() should contain project type 'backend'")
	}

	if !containsString(context, "Go") {
		t.Error("FormatContext() should contain language 'Go'")
	}

	if !containsString(context, "cmd/app") {
		t.Error("FormatContext() should contain directory 'cmd/app'")
	}
}

func TestDescribeDirectory(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"src/components", "UI components"},
		{"src/pages", "Route pages"},
		{"cmd", "Application entry points"},
		{"internal/service", "Business logic"},
		{"src/utils", "Utility functions"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := describeDirectory(tt.path)
			if got != tt.want {
				t.Errorf("describeDirectory(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestIsIgnoredDir(t *testing.T) {
	ignored := []string{"node_modules", "vendor", "dist", ".git", "__pycache__"}
	notIgnored := []string{"src", "cmd", "internal", "components"}

	for _, dir := range ignored {
		if !isIgnoredDir(dir) {
			t.Errorf("isIgnoredDir(%q) = false, want true", dir)
		}
	}

	for _, dir := range notIgnored {
		if isIgnoredDir(dir) {
			t.Errorf("isIgnoredDir(%q) = true, want false", dir)
		}
	}
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
