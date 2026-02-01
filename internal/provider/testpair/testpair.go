// Package testpair provides utilities for matching test files with their
// implementation files and reordering file lists to show them together.
package testpair

import (
	"path/filepath"
	"strings"

	"github.com/mwistrand/graft/internal/provider"
)

// testPattern defines how test files relate to implementation files.
type testPattern struct {
	testSuffix     string // suffix indicating a test file (e.g., "_test.go")
	implSuffix     string // corresponding implementation suffix (e.g., ".go")
	isPrefix       bool   // true for prefix patterns (e.g., "test_" for Python)
	prefixToRemove string // prefix to remove when finding the implementation
}

var patterns = []testPattern{
	// Go: foo_test.go -> foo.go
	{testSuffix: "_test.go", implSuffix: ".go"},

	// JavaScript/TypeScript: foo.test.ts -> foo.ts, foo.spec.ts -> foo.ts
	{testSuffix: ".test.ts", implSuffix: ".ts"},
	{testSuffix: ".spec.ts", implSuffix: ".ts"},
	{testSuffix: ".test.tsx", implSuffix: ".tsx"},
	{testSuffix: ".spec.tsx", implSuffix: ".tsx"},
	{testSuffix: ".test.js", implSuffix: ".js"},
	{testSuffix: ".spec.js", implSuffix: ".js"},
	{testSuffix: ".test.jsx", implSuffix: ".jsx"},
	{testSuffix: ".spec.jsx", implSuffix: ".jsx"},
	{testSuffix: ".test.mjs", implSuffix: ".mjs"},
	{testSuffix: ".spec.mjs", implSuffix: ".mjs"},
	{testSuffix: ".test.cjs", implSuffix: ".cjs"},
	{testSuffix: ".spec.cjs", implSuffix: ".cjs"},

	// Python: foo_test.py -> foo.py, test_foo.py -> foo.py
	{testSuffix: "_test.py", implSuffix: ".py"},
	{isPrefix: true, prefixToRemove: "test_", implSuffix: ".py"},

	// Ruby: foo_spec.rb -> foo.rb, foo_test.rb -> foo.rb
	{testSuffix: "_spec.rb", implSuffix: ".rb"},
	{testSuffix: "_test.rb", implSuffix: ".rb"},

	// Java: FooTest.java -> Foo.java
	{testSuffix: "Test.java", implSuffix: ".java"},
	{testSuffix: "Tests.java", implSuffix: ".java"},

	// C#: FooTests.cs -> Foo.cs, FooTest.cs -> Foo.cs
	{testSuffix: "Tests.cs", implSuffix: ".cs"},
	{testSuffix: "Test.cs", implSuffix: ".cs"},

	// Rust: foo_test.rs -> foo.rs
	{testSuffix: "_test.rs", implSuffix: ".rs"},
}

// IsTestFile returns true if the path appears to be a test file.
func IsTestFile(path string) bool {
	base := filepath.Base(path)

	for _, p := range patterns {
		if p.isPrefix {
			if strings.HasPrefix(base, p.prefixToRemove) && strings.HasSuffix(base, p.implSuffix) {
				return true
			}
		} else {
			if strings.HasSuffix(path, p.testSuffix) {
				return true
			}
		}
	}
	return false
}

// FindImplementation returns the likely implementation file path for a test file.
// Returns empty string if the path is not a test file or no pattern matches.
func FindImplementation(testPath string) string {
	dir := filepath.Dir(testPath)
	base := filepath.Base(testPath)

	for _, p := range patterns {
		if p.isPrefix {
			if strings.HasPrefix(base, p.prefixToRemove) && strings.HasSuffix(base, p.implSuffix) {
				implBase := strings.TrimPrefix(base, p.prefixToRemove)
				return filepath.Join(dir, implBase)
			}
		} else {
			if strings.HasSuffix(base, p.testSuffix) {
				implBase := strings.TrimSuffix(base, p.testSuffix) + p.implSuffix
				return filepath.Join(dir, implBase)
			}
		}
	}
	return ""
}

// FindTest returns the likely test file path for an implementation file.
// Returns empty string if the file is already a test file or no pattern matches.
// Note: This returns the first matching pattern (e.g., _test.go for Go files).
func FindTest(implPath string) string {
	// Don't return a test path for files that are already tests
	if IsTestFile(implPath) {
		return ""
	}

	dir := filepath.Dir(implPath)
	base := filepath.Base(implPath)

	for _, p := range patterns {
		if p.isPrefix {
			// For prefix patterns (like test_foo.py), check if this is the right file type
			if strings.HasSuffix(base, p.implSuffix) && !strings.HasPrefix(base, p.prefixToRemove) {
				testBase := p.prefixToRemove + base
				return filepath.Join(dir, testBase)
			}
		} else {
			if strings.HasSuffix(base, p.implSuffix) && !strings.HasSuffix(base, p.testSuffix) {
				testBase := strings.TrimSuffix(base, p.implSuffix) + p.testSuffix
				return filepath.Join(dir, testBase)
			}
		}
	}
	return ""
}

// PairFiles reorders files to place test files alongside their implementation files.
// If testsFirst is true, test files appear before their implementation.
// If testsFirst is false, test files appear after their implementation.
// Files without a pair remain in their original relative order.
func PairFiles(files []provider.OrderedFile, testsFirst bool) []provider.OrderedFile {
	if len(files) == 0 {
		return files
	}

	// Build a map of all file paths for quick lookup
	fileByPath := make(map[string]provider.OrderedFile)
	for _, f := range files {
		fileByPath[f.Path] = f
	}

	// First pass: identify all implementation -> test pairs
	// Only pair if BOTH files are in the list
	implToTest := make(map[string]string) // impl path -> test path
	testToImpl := make(map[string]string) // test path -> impl path

	for _, f := range files {
		if IsTestFile(f.Path) {
			implPath := FindImplementation(f.Path)
			if implPath != "" {
				if _, exists := fileByPath[implPath]; exists {
					implToTest[implPath] = f.Path
					testToImpl[f.Path] = implPath
				}
			}
		}
	}

	// Second pass: build result
	// When we see an implementation with a paired test, add both together
	// When we see a paired test, skip it (it was added with its implementation)
	added := make(map[string]bool)
	result := make([]provider.OrderedFile, 0, len(files))

	for _, f := range files {
		if added[f.Path] {
			continue
		}

		if testPath, hasPair := implToTest[f.Path]; hasPair {
			// This implementation has a paired test
			testFile := fileByPath[testPath]
			if testsFirst {
				result = append(result, testFile, f)
			} else {
				result = append(result, f, testFile)
			}
			added[f.Path] = true
			added[testPath] = true
		} else if _, isPairedTest := testToImpl[f.Path]; isPairedTest {
			// This is a paired test - skip, it will be added with its implementation
			continue
		} else {
			// Unpaired file - add as-is
			result = append(result, f)
			added[f.Path] = true
		}
	}

	return result
}
