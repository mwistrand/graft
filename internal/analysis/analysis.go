// Package analysis provides repository structure analysis for smarter file ordering.
// It scans directory structure and config files to determine project type and framework.
package analysis

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ProjectType represents the detected project architecture.
type ProjectType string

const (
	ProjectTypeFrontend  ProjectType = "frontend"
	ProjectTypeBackend   ProjectType = "backend"
	ProjectTypeFullstack ProjectType = "fullstack"
	ProjectTypeUnknown   ProjectType = "unknown"
)

// Analysis contains the results of repository analysis.
type Analysis struct {
	// Type is the detected project type.
	Type ProjectType `json:"type"`

	// Languages lists detected programming languages.
	Languages []string `json:"languages"`

	// Frameworks lists detected frameworks.
	Frameworks []string `json:"frameworks"`

	// Directories summarizes the repository structure.
	Directories []DirectorySummary `json:"directories"`

	// AnalyzedAt is when the analysis was performed.
	AnalyzedAt time.Time `json:"analyzed_at"`

	// RepoRoot is the repository root path (not serialized).
	RepoRoot string `json:"-"`
}

// DirectorySummary contains metadata about a directory.
type DirectorySummary struct {
	// Path is relative to repository root.
	Path string `json:"path"`

	// FileCount is the number of files in this directory (non-recursive).
	FileCount int `json:"file_count"`

	// Description is a brief description of what this directory contains.
	Description string `json:"description,omitempty"`
}

// Analyzer scans repositories to build analysis context.
type Analyzer struct {
	repoRoot string
}

// NewAnalyzer creates an analyzer for the given repository root.
func NewAnalyzer(repoRoot string) *Analyzer {
	return &Analyzer{repoRoot: repoRoot}
}

// Analyze scans the repository and returns analysis results.
func (a *Analyzer) Analyze() (*Analysis, error) {
	analysis := &Analysis{
		Type:        ProjectTypeUnknown,
		Languages:   []string{},
		Frameworks:  []string{},
		Directories: []DirectorySummary{},
		AnalyzedAt:  time.Now(),
		RepoRoot:    a.repoRoot,
	}

	// Detect languages and frameworks from config files
	if err := a.detectFromConfigFiles(analysis); err != nil {
		return nil, fmt.Errorf("detecting from config files: %w", err)
	}

	// Scan directory structure
	if err := a.scanDirectories(analysis); err != nil {
		return nil, fmt.Errorf("scanning directories: %w", err)
	}

	// Determine project type based on collected info
	a.determineProjectType(analysis)

	return analysis, nil
}

// detectFromConfigFiles checks for common config files to detect languages/frameworks.
func (a *Analyzer) detectFromConfigFiles(analysis *Analysis) error {
	// Check for Go
	if _, err := os.Stat(filepath.Join(a.repoRoot, "go.mod")); err == nil {
		analysis.Languages = append(analysis.Languages, "Go")
	}

	// Check for Node.js/JavaScript/TypeScript
	packageJSONPath := filepath.Join(a.repoRoot, "package.json")
	if data, err := os.ReadFile(packageJSONPath); err == nil {
		analysis.Languages = append(analysis.Languages, "JavaScript")
		a.parsePackageJSON(data, analysis)
	}

	// Check for TypeScript
	if _, err := os.Stat(filepath.Join(a.repoRoot, "tsconfig.json")); err == nil {
		// Replace JavaScript with TypeScript if tsconfig exists
		for i, lang := range analysis.Languages {
			if lang == "JavaScript" {
				analysis.Languages[i] = "TypeScript"
				break
			}
		}
		if len(analysis.Languages) == 0 {
			analysis.Languages = append(analysis.Languages, "TypeScript")
		}
	}

	// Check for Python
	if _, err := os.Stat(filepath.Join(a.repoRoot, "pyproject.toml")); err == nil {
		analysis.Languages = append(analysis.Languages, "Python")
	} else if _, err := os.Stat(filepath.Join(a.repoRoot, "requirements.txt")); err == nil {
		analysis.Languages = append(analysis.Languages, "Python")
	}

	// Check for Rust
	if _, err := os.Stat(filepath.Join(a.repoRoot, "Cargo.toml")); err == nil {
		analysis.Languages = append(analysis.Languages, "Rust")
	}

	// Check for Java/Kotlin
	if _, err := os.Stat(filepath.Join(a.repoRoot, "pom.xml")); err == nil {
		analysis.Languages = append(analysis.Languages, "Java")
	} else if _, err := os.Stat(filepath.Join(a.repoRoot, "build.gradle")); err == nil {
		analysis.Languages = append(analysis.Languages, "Java")
	} else if _, err := os.Stat(filepath.Join(a.repoRoot, "build.gradle.kts")); err == nil {
		analysis.Languages = append(analysis.Languages, "Kotlin")
	}

	return nil
}

// parsePackageJSON extracts framework info from package.json.
func (a *Analyzer) parsePackageJSON(data []byte, analysis *Analysis) {
	var pkg struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return
	}

	// Merge dependencies
	allDeps := make(map[string]bool)
	for dep := range pkg.Dependencies {
		allDeps[dep] = true
	}
	for dep := range pkg.DevDependencies {
		allDeps[dep] = true
	}

	// Detect frameworks
	if allDeps["react"] {
		analysis.Frameworks = append(analysis.Frameworks, "React")
	}
	if allDeps["vue"] {
		analysis.Frameworks = append(analysis.Frameworks, "Vue")
	}
	if allDeps["@angular/core"] {
		analysis.Frameworks = append(analysis.Frameworks, "Angular")
	}
	if allDeps["svelte"] {
		analysis.Frameworks = append(analysis.Frameworks, "Svelte")
	}
	if allDeps["next"] {
		analysis.Frameworks = append(analysis.Frameworks, "Next.js")
	}
	if allDeps["nuxt"] {
		analysis.Frameworks = append(analysis.Frameworks, "Nuxt")
	}
	if allDeps["express"] {
		analysis.Frameworks = append(analysis.Frameworks, "Express")
	}
	if allDeps["fastify"] {
		analysis.Frameworks = append(analysis.Frameworks, "Fastify")
	}
	if allDeps["nest"] || allDeps["@nestjs/core"] {
		analysis.Frameworks = append(analysis.Frameworks, "NestJS")
	}
}

// scanDirectories walks the repository and summarizes directory structure.
func (a *Analyzer) scanDirectories(analysis *Analysis) error {
	dirCounts := make(map[string]int)

	err := filepath.Walk(a.repoRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		// Skip hidden directories and common non-source directories
		if info.IsDir() {
			name := info.Name()
			if strings.HasPrefix(name, ".") || isIgnoredDir(name) {
				return filepath.SkipDir
			}
			return nil
		}

		// Count files per directory
		relPath, err := filepath.Rel(a.repoRoot, filepath.Dir(path))
		if err != nil {
			return nil
		}
		if relPath == "." {
			relPath = "(root)"
		}
		dirCounts[relPath]++

		return nil
	})
	if err != nil {
		return err
	}

	// Convert to sorted slice, keeping only significant directories
	for dir, count := range dirCounts {
		if count >= 1 && dir != "(root)" {
			analysis.Directories = append(analysis.Directories, DirectorySummary{
				Path:        dir,
				FileCount:   count,
				Description: describeDirectory(dir),
			})
		}
	}

	// Sort by path for consistent output
	sort.Slice(analysis.Directories, func(i, j int) bool {
		return analysis.Directories[i].Path < analysis.Directories[j].Path
	})

	return nil
}

// isIgnoredDir returns true for directories that should be skipped.
func isIgnoredDir(name string) bool {
	ignored := map[string]bool{
		"node_modules": true,
		"vendor":       true,
		"dist":         true,
		"build":        true,
		"target":       true,
		"__pycache__":  true,
		".git":         true,
		".idea":        true,
		".vscode":      true,
		"coverage":     true,
	}
	return ignored[name]
}

// describeDirectory returns a brief description based on common directory names.
func describeDirectory(dir string) string {
	base := filepath.Base(dir)
	parent := filepath.Dir(dir)

	descriptions := map[string]string{
		// Frontend
		"components":  "UI components",
		"pages":       "Route pages",
		"views":       "View components",
		"routes":      "Route definitions",
		"router":      "Routing configuration",
		"store":       "State management",
		"stores":      "State stores",
		"hooks":       "React hooks",
		"composables": "Vue composables",
		"context":     "React context providers",
		"layouts":     "Layout components",
		"assets":      "Static assets",
		"styles":      "Stylesheets",
		"utils":       "Utility functions",
		"helpers":     "Helper functions",
		"lib":         "Library code",

		// Backend
		"cmd":         "Application entry points",
		"internal":    "Private application code",
		"pkg":         "Public library code",
		"api":         "API definitions",
		"handlers":    "HTTP handlers",
		"controllers": "Request controllers",
		"services":    "Business logic services",
		"service":     "Business logic",
		"models":      "Data models",
		"entities":    "Domain entities",
		"repository":  "Data access layer",
		"adapters":    "External service adapters",
		"middleware":  "HTTP middleware",
		"config":      "Configuration",

		// Shared
		"types":   "Type definitions",
		"tests":   "Test files",
		"test":    "Test files",
		"__tests__": "Test files",
		"docs":    "Documentation",
		"scripts": "Build/utility scripts",
	}

	if desc, ok := descriptions[base]; ok {
		return desc
	}

	// Check parent directory for context
	if parent != "." {
		parentBase := filepath.Base(parent)
		if parentBase == "src" || parentBase == "app" || parentBase == "internal" {
			return "" // Let the base name speak for itself
		}
	}

	return ""
}

// determineProjectType sets the project type based on detected info.
func (a *Analyzer) determineProjectType(analysis *Analysis) {
	hasFrontend := false
	hasBackend := false

	// Check frameworks
	frontendFrameworks := map[string]bool{
		"React": true, "Vue": true, "Angular": true, "Svelte": true,
		"Next.js": true, "Nuxt": true,
	}
	backendFrameworks := map[string]bool{
		"Express": true, "Fastify": true, "NestJS": true,
	}

	for _, fw := range analysis.Frameworks {
		if frontendFrameworks[fw] {
			hasFrontend = true
		}
		if backendFrameworks[fw] {
			hasBackend = true
		}
	}

	// Check directory structure
	for _, dir := range analysis.Directories {
		base := filepath.Base(dir.Path)
		switch base {
		case "components", "pages", "views", "hooks", "store", "stores":
			hasFrontend = true
		case "cmd", "internal", "handlers", "controllers", "repository":
			hasBackend = true
		}
	}

	// Check languages
	for _, lang := range analysis.Languages {
		if lang == "Go" || lang == "Rust" || lang == "Java" || lang == "Kotlin" {
			hasBackend = true
		}
	}

	// Determine type
	if hasFrontend && hasBackend {
		analysis.Type = ProjectTypeFullstack
	} else if hasFrontend {
		analysis.Type = ProjectTypeFrontend
	} else if hasBackend {
		analysis.Type = ProjectTypeBackend
	} else {
		analysis.Type = ProjectTypeUnknown
	}
}

// FormatContext returns a formatted string for inclusion in AI prompts.
func (a *Analysis) FormatContext() string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("- Type: %s\n", a.Type))

	if len(a.Languages) > 0 {
		b.WriteString(fmt.Sprintf("- Languages: %s\n", strings.Join(a.Languages, ", ")))
	}

	if len(a.Frameworks) > 0 {
		b.WriteString(fmt.Sprintf("- Frameworks: %s\n", strings.Join(a.Frameworks, ", ")))
	}

	if len(a.Directories) > 0 {
		b.WriteString("- Structure:\n")
		for _, dir := range a.Directories {
			if dir.Description != "" {
				b.WriteString(fmt.Sprintf("  - %s/ (%d files) - %s\n", dir.Path, dir.FileCount, dir.Description))
			} else {
				b.WriteString(fmt.Sprintf("  - %s/ (%d files)\n", dir.Path, dir.FileCount))
			}
		}
	}

	return b.String()
}
