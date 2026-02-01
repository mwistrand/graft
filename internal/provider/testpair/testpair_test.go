package testpair

import (
	"testing"

	"github.com/mwistrand/graft/internal/provider"
)

func TestIsTestFile(t *testing.T) {
	tests := []struct {
		path   string
		isTest bool
	}{
		// Go
		{"foo_test.go", true},
		{"internal/cli/review_test.go", true},
		{"foo.go", false},

		// JavaScript/TypeScript
		{"foo.test.ts", true},
		{"foo.spec.ts", true},
		{"foo.test.tsx", true},
		{"foo.spec.tsx", true},
		{"foo.test.js", true},
		{"foo.spec.js", true},
		{"foo.ts", false},
		{"foo.js", false},

		// Python
		{"test_foo.py", true},
		{"foo_test.py", true},
		{"foo.py", false},

		// Ruby
		{"foo_spec.rb", true},
		{"foo_test.rb", true},
		{"foo.rb", false},

		// Java
		{"FooTest.java", true},
		{"FooTests.java", true},
		{"Foo.java", false},

		// C#
		{"FooTest.cs", true},
		{"FooTests.cs", true},
		{"Foo.cs", false},

		// Rust
		{"foo_test.rs", true},
		{"foo.rs", false},

		// Edge cases
		{"test.go", false},        // not _test.go
		{"testing.go", false},     // not a test file
		{"my_test_helper.go", false}, // not a test file (different pattern)
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := IsTestFile(tt.path)
			if got != tt.isTest {
				t.Errorf("IsTestFile(%q) = %v, want %v", tt.path, got, tt.isTest)
			}
		})
	}
}

func TestFindImplementation(t *testing.T) {
	tests := []struct {
		testPath string
		implPath string
	}{
		// Go
		{"foo_test.go", "foo.go"},
		{"internal/cli/review_test.go", "internal/cli/review.go"},

		// JavaScript/TypeScript
		{"foo.test.ts", "foo.ts"},
		{"foo.spec.ts", "foo.ts"},
		{"src/components/Button.test.tsx", "src/components/Button.tsx"},

		// Python
		{"test_foo.py", "foo.py"},
		{"foo_test.py", "foo.py"},
		{"tests/test_utils.py", "tests/utils.py"},

		// Ruby
		{"foo_spec.rb", "foo.rb"},
		{"foo_test.rb", "foo.rb"},

		// Java
		{"FooTest.java", "Foo.java"},
		{"FooTests.java", "Foo.java"},

		// C#
		{"FooTest.cs", "Foo.cs"},
		{"FooTests.cs", "Foo.cs"},

		// Rust
		{"foo_test.rs", "foo.rs"},

		// Non-test files return empty
		{"foo.go", ""},
		{"foo.ts", ""},
	}

	for _, tt := range tests {
		t.Run(tt.testPath, func(t *testing.T) {
			got := FindImplementation(tt.testPath)
			if got != tt.implPath {
				t.Errorf("FindImplementation(%q) = %q, want %q", tt.testPath, got, tt.implPath)
			}
		})
	}
}

func TestFindTest(t *testing.T) {
	tests := []struct {
		implPath string
		testPath string
	}{
		// Go
		{"foo.go", "foo_test.go"},
		{"internal/cli/review.go", "internal/cli/review_test.go"},

		// JavaScript/TypeScript (returns first matching pattern)
		{"foo.ts", "foo.test.ts"},
		{"foo.tsx", "foo.test.tsx"},
		{"foo.js", "foo.test.js"},

		// Python (returns first matching pattern - suffix style)
		{"foo.py", "foo_test.py"},

		// Ruby
		{"foo.rb", "foo_spec.rb"},

		// Java
		{"Foo.java", "FooTest.java"},

		// C#
		{"Foo.cs", "FooTests.cs"},

		// Rust
		{"foo.rs", "foo_test.rs"},

		// Test files should not match themselves
		{"foo_test.go", ""},
		{"foo.test.ts", ""},

		// Unsupported file types return empty
		{"foo.txt", ""},
		{"data.json", ""},
	}

	for _, tt := range tests {
		t.Run(tt.implPath, func(t *testing.T) {
			got := FindTest(tt.implPath)
			if got != tt.testPath {
				t.Errorf("FindTest(%q) = %q, want %q", tt.implPath, got, tt.testPath)
			}
		})
	}
}

func TestPairFiles(t *testing.T) {
	t.Run("pairs implementation with test (tests after)", func(t *testing.T) {
		files := []provider.OrderedFile{
			{Path: "foo.go", Priority: 1},
			{Path: "foo_test.go", Priority: 2},
			{Path: "bar.go", Priority: 3},
		}

		result := PairFiles(files, false)

		if len(result) != 3 {
			t.Fatalf("expected 3 files, got %d", len(result))
		}
		if result[0].Path != "foo.go" {
			t.Errorf("expected foo.go first, got %s", result[0].Path)
		}
		if result[1].Path != "foo_test.go" {
			t.Errorf("expected foo_test.go second, got %s", result[1].Path)
		}
		if result[2].Path != "bar.go" {
			t.Errorf("expected bar.go third, got %s", result[2].Path)
		}
	})

	t.Run("pairs implementation with test (tests first)", func(t *testing.T) {
		files := []provider.OrderedFile{
			{Path: "foo.go", Priority: 1},
			{Path: "foo_test.go", Priority: 2},
			{Path: "bar.go", Priority: 3},
		}

		result := PairFiles(files, true)

		if len(result) != 3 {
			t.Fatalf("expected 3 files, got %d", len(result))
		}
		if result[0].Path != "foo_test.go" {
			t.Errorf("expected foo_test.go first, got %s", result[0].Path)
		}
		if result[1].Path != "foo.go" {
			t.Errorf("expected foo.go second, got %s", result[1].Path)
		}
		if result[2].Path != "bar.go" {
			t.Errorf("expected bar.go third, got %s", result[2].Path)
		}
	})

	t.Run("handles test appearing before implementation in input", func(t *testing.T) {
		files := []provider.OrderedFile{
			{Path: "foo_test.go", Priority: 1},
			{Path: "bar.go", Priority: 2},
			{Path: "foo.go", Priority: 3},
		}

		result := PairFiles(files, false)

		// Implementation should come first with its test, then bar
		if len(result) != 3 {
			t.Fatalf("expected 3 files, got %d", len(result))
		}
		// The implementation should be with its test
		if result[0].Path != "bar.go" {
			t.Errorf("expected bar.go first, got %s", result[0].Path)
		}
		if result[1].Path != "foo.go" {
			t.Errorf("expected foo.go second, got %s", result[1].Path)
		}
		if result[2].Path != "foo_test.go" {
			t.Errorf("expected foo_test.go third, got %s", result[2].Path)
		}
	})

	t.Run("unpaired test stays in order", func(t *testing.T) {
		files := []provider.OrderedFile{
			{Path: "foo.go", Priority: 1},
			{Path: "bar_test.go", Priority: 2}, // No bar.go in list
			{Path: "baz.go", Priority: 3},
		}

		result := PairFiles(files, false)

		if len(result) != 3 {
			t.Fatalf("expected 3 files, got %d", len(result))
		}
		if result[0].Path != "foo.go" {
			t.Errorf("expected foo.go first, got %s", result[0].Path)
		}
		if result[1].Path != "bar_test.go" {
			t.Errorf("expected bar_test.go second, got %s", result[1].Path)
		}
		if result[2].Path != "baz.go" {
			t.Errorf("expected baz.go third, got %s", result[2].Path)
		}
	})

	t.Run("unpaired implementation stays in order", func(t *testing.T) {
		files := []provider.OrderedFile{
			{Path: "foo.go", Priority: 1}, // No foo_test.go in list
			{Path: "bar.go", Priority: 2},
		}

		result := PairFiles(files, false)

		if len(result) != 2 {
			t.Fatalf("expected 2 files, got %d", len(result))
		}
		if result[0].Path != "foo.go" {
			t.Errorf("expected foo.go first, got %s", result[0].Path)
		}
		if result[1].Path != "bar.go" {
			t.Errorf("expected bar.go second, got %s", result[1].Path)
		}
	})

	t.Run("multiple pairs", func(t *testing.T) {
		files := []provider.OrderedFile{
			{Path: "a.go", Priority: 1},
			{Path: "b.go", Priority: 2},
			{Path: "a_test.go", Priority: 3},
			{Path: "b_test.go", Priority: 4},
		}

		result := PairFiles(files, false)

		if len(result) != 4 {
			t.Fatalf("expected 4 files, got %d", len(result))
		}
		// a.go followed by a_test.go, then b.go followed by b_test.go
		if result[0].Path != "a.go" {
			t.Errorf("expected a.go first, got %s", result[0].Path)
		}
		if result[1].Path != "a_test.go" {
			t.Errorf("expected a_test.go second, got %s", result[1].Path)
		}
		if result[2].Path != "b.go" {
			t.Errorf("expected b.go third, got %s", result[2].Path)
		}
		if result[3].Path != "b_test.go" {
			t.Errorf("expected b_test.go fourth, got %s", result[3].Path)
		}
	})

	t.Run("empty input", func(t *testing.T) {
		result := PairFiles(nil, false)
		if len(result) != 0 {
			t.Errorf("expected empty result, got %d files", len(result))
		}
	})

	t.Run("cross-language files", func(t *testing.T) {
		files := []provider.OrderedFile{
			{Path: "api.ts", Priority: 1},
			{Path: "api.test.ts", Priority: 2},
			{Path: "utils.py", Priority: 3},
			{Path: "test_utils.py", Priority: 4},
		}

		result := PairFiles(files, false)

		if len(result) != 4 {
			t.Fatalf("expected 4 files, got %d", len(result))
		}
		if result[0].Path != "api.ts" {
			t.Errorf("expected api.ts first, got %s", result[0].Path)
		}
		if result[1].Path != "api.test.ts" {
			t.Errorf("expected api.test.ts second, got %s", result[1].Path)
		}
		if result[2].Path != "utils.py" {
			t.Errorf("expected utils.py third, got %s", result[2].Path)
		}
		if result[3].Path != "test_utils.py" {
			t.Errorf("expected test_utils.py fourth, got %s", result[3].Path)
		}
	})
}
