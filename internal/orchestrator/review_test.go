package orchestrator

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCLIReviewStrategy_Approve(t *testing.T) {
	input := bytes.NewBufferString("a\n")
	output := &bytes.Buffer{}

	strategy := NewCLIReviewStrategy(input, output)
	milestone := &MilestoneNode{ID: "M1", Name: "Foundation"}
	artifacts := []ImplementationArtifact{
		{Path: "cmd/main.go", Action: "CREATE"},
	}

	decision, err := strategy.RequestReview(context.Background(), milestone, artifacts)
	require.NoError(t, err)
	assert.Equal(t, ReviewApproved, decision)
	assert.Contains(t, output.String(), "M1")
	assert.Contains(t, output.String(), "cmd/main.go")
}

func TestCLIReviewStrategy_Reject(t *testing.T) {
	input := bytes.NewBufferString("r\n")
	output := &bytes.Buffer{}

	strategy := NewCLIReviewStrategy(input, output)
	milestone := &MilestoneNode{ID: "M1", Name: "Foundation"}

	decision, err := strategy.RequestReview(context.Background(), milestone, nil)
	require.NoError(t, err)
	assert.Equal(t, ReviewRejected, decision)
}

func TestCLIReviewStrategy_ViewThenApprove(t *testing.T) {
	input := bytes.NewBufferString("v\na\n")
	output := &bytes.Buffer{}

	strategy := NewCLIReviewStrategy(input, output)
	milestone := &MilestoneNode{ID: "M2", Name: "Core"}
	artifacts := []ImplementationArtifact{
		{Path: "pkg/core.go", Action: "CREATE", DiffOrBody: "+package core\n+func Init() {}"},
	}

	decision, err := strategy.RequestReview(context.Background(), milestone, artifacts)
	require.NoError(t, err)
	assert.Equal(t, ReviewApproved, decision)
	assert.Contains(t, output.String(), "+package core")
}

func TestCLIReviewStrategy_InvalidThenApprove(t *testing.T) {
	input := bytes.NewBufferString("x\napprove\n")
	output := &bytes.Buffer{}

	strategy := NewCLIReviewStrategy(input, output)
	milestone := &MilestoneNode{ID: "M1", Name: "Foundation"}

	decision, err := strategy.RequestReview(context.Background(), milestone, nil)
	require.NoError(t, err)
	assert.Equal(t, ReviewApproved, decision)
	assert.Contains(t, output.String(), "Unknown option")
}

func TestFileReviewStrategy_Approve(t *testing.T) {
	dir := t.TempDir()
	strategy := &FileReviewStrategy{
		reportDir:    dir,
		pollInterval: 50 * time.Millisecond,
	}

	milestone := &MilestoneNode{ID: "M1", Name: "Foundation"}
	artifacts := []ImplementationArtifact{
		{Path: "cmd/main.go", Action: "CREATE"},
	}

	// Create the approval file after a brief delay.
	go func() {
		time.Sleep(150 * time.Millisecond)
		approvedPath := filepath.Join(dir, "milestone-m1-review.approved")
		_ = os.WriteFile(approvedPath, []byte("ok"), 0o644)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	decision, err := strategy.RequestReview(ctx, milestone, artifacts)
	require.NoError(t, err)
	assert.Equal(t, ReviewApproved, decision)

	// Verify the review report was written.
	reportPath := filepath.Join(dir, "milestone-m1-review.md")
	data, err := os.ReadFile(reportPath)
	require.NoError(t, err)
	assert.Contains(t, string(data), "M1")
	assert.Contains(t, string(data), "cmd/main.go")
}

func TestFileReviewStrategy_Reject(t *testing.T) {
	dir := t.TempDir()
	strategy := &FileReviewStrategy{
		reportDir:    dir,
		pollInterval: 50 * time.Millisecond,
	}

	milestone := &MilestoneNode{ID: "M2", Name: "Core"}

	go func() {
		time.Sleep(150 * time.Millisecond)
		rejectedPath := filepath.Join(dir, "milestone-m2-review.rejected")
		_ = os.WriteFile(rejectedPath, []byte("not good"), 0o644)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	decision, err := strategy.RequestReview(ctx, milestone, nil)
	require.NoError(t, err)
	assert.Equal(t, ReviewRejected, decision)
}

func TestFileReviewStrategy_ContextCanceled(t *testing.T) {
	dir := t.TempDir()
	strategy := &FileReviewStrategy{
		reportDir:    dir,
		pollInterval: 50 * time.Millisecond,
	}

	milestone := &MilestoneNode{ID: "M1", Name: "Foundation"}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	decision, err := strategy.RequestReview(ctx, milestone, nil)
	require.Error(t, err)
	assert.Equal(t, ReviewRejected, decision)
}
