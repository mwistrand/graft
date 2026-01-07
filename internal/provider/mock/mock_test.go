package mock

import (
	"context"
	"errors"
	"testing"

	"github.com/mwistrand/graft/internal/git"
	"github.com/mwistrand/graft/internal/provider"
)

func TestProvider_Name(t *testing.T) {
	p := New()
	if p.Name() != "mock" {
		t.Errorf("Name() = %q, want %q", p.Name(), "mock")
	}
}

func TestProvider_ReviewChanges_Default(t *testing.T) {
	p := New()

	result, err := p.ReviewChanges(context.Background(), &provider.ReviewRequest{
		Files:    []git.FileDiff{{Path: "main.go", Status: git.StatusModified}},
		FullDiff: "+line1\n-line2",
	})

	if err != nil {
		t.Fatalf("ReviewChanges() failed: %v", err)
	}

	if result.Content == "" {
		t.Error("Content should not be empty")
	}
	if len(p.ReviewCalls) != 1 {
		t.Errorf("expected 1 review call, got %d", len(p.ReviewCalls))
	}
}

func TestProvider_ReviewChanges_CustomFunc(t *testing.T) {
	p := New()
	p.ReviewFunc = func(ctx context.Context, req *provider.ReviewRequest) (*provider.ReviewResponse, error) {
		return &provider.ReviewResponse{
			Content: "Custom review for " + req.Files[0].Path,
		}, nil
	}

	result, err := p.ReviewChanges(context.Background(), &provider.ReviewRequest{
		Files: []git.FileDiff{{Path: "custom.go"}},
	})

	if err != nil {
		t.Fatalf("ReviewChanges() failed: %v", err)
	}

	expected := "Custom review for custom.go"
	if result.Content != expected {
		t.Errorf("Content = %q, want %q", result.Content, expected)
	}
}

func TestProvider_ReviewChanges_Error(t *testing.T) {
	p := New()
	expectedErr := errors.New("review error")
	p.ReviewFunc = func(ctx context.Context, req *provider.ReviewRequest) (*provider.ReviewResponse, error) {
		return nil, expectedErr
	}

	_, err := p.ReviewChanges(context.Background(), &provider.ReviewRequest{
		Files: []git.FileDiff{{Path: "test.go"}},
	})

	if err != expectedErr {
		t.Errorf("err = %v, want %v", err, expectedErr)
	}
}

func TestProvider_Reset(t *testing.T) {
	p := New()

	// Make some calls
	p.SummarizeChanges(context.Background(), &provider.SummarizeRequest{
		Files: []git.FileDiff{{Path: "test.go"}},
	})
	p.OrderFiles(context.Background(), &provider.OrderRequest{
		Files: []git.FileDiff{{Path: "test.go"}},
	})
	p.ReviewChanges(context.Background(), &provider.ReviewRequest{
		Files: []git.FileDiff{{Path: "test.go"}},
	})

	if len(p.SummarizeCalls) != 1 || len(p.OrderCalls) != 1 || len(p.ReviewCalls) != 1 {
		t.Error("expected calls to be tracked")
	}

	p.Reset()

	if len(p.SummarizeCalls) != 0 || len(p.OrderCalls) != 0 || len(p.ReviewCalls) != 0 {
		t.Error("Reset() should clear all call records")
	}
}
