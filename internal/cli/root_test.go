package cli

import (
	"bytes"
	"testing"
)

func TestRootCommand(t *testing.T) {
	// Test that root command runs without error
	cmd := rootCmd
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("root command failed: %v", err)
	}

	output := buf.String()
	if output == "" {
		t.Error("expected help output, got empty string")
	}
}

func TestVersionCommand(t *testing.T) {
	SetVersionInfo("1.0.0", "abc123", "2024-01-01")

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"version"})

	// Also set output on the version subcommand
	versionCmd.SetOut(buf)

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("version command failed: %v", err)
	}

	output := buf.String()
	if output == "" {
		t.Error("expected version output, got empty string")
	}

	// Verify version info appears in output
	if !bytes.Contains([]byte(output), []byte("1.0.0")) {
		t.Errorf("expected output to contain version '1.0.0', got: %s", output)
	}
}

func TestSetVersionInfo(t *testing.T) {
	SetVersionInfo("2.0.0", "def456", "2024-06-01")

	if Version != "2.0.0" {
		t.Errorf("Version = %q, want %q", Version, "2.0.0")
	}
	if Commit != "def456" {
		t.Errorf("Commit = %q, want %q", Commit, "def456")
	}
	if Date != "2024-06-01" {
		t.Errorf("Date = %q, want %q", Date, "2024-06-01")
	}
}

func TestIsVerbose(t *testing.T) {
	// Default should be false
	verbose = false
	if IsVerbose() {
		t.Error("expected IsVerbose() to be false by default")
	}

	verbose = true
	if !IsVerbose() {
		t.Error("expected IsVerbose() to be true when verbose is set")
	}

	// Reset
	verbose = false
}
