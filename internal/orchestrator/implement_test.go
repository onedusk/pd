package orchestrator

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockImplementer returns a mock ImplementerFunc that tracks calls.
func mockImplementer(results map[string]error) (ImplementerFunc, *[]string) {
	var calls []string
	fn := func(_ context.Context, milestone *MilestoneNode) ([]ImplementationArtifact, error) {
		calls = append(calls, milestone.ID)
		if err, ok := results[milestone.ID]; ok && err != nil {
			return nil, err
		}
		return []ImplementationArtifact{
			{Path: fmt.Sprintf("src/%s.go", milestone.ID), Action: "CREATE"},
		}, nil
	}
	return fn, &calls
}

// autoApproveReview always approves.
type autoApproveReview struct{}

func (a *autoApproveReview) RequestReview(_ context.Context, _ *MilestoneNode, _ []ImplementationArtifact) (ReviewDecision, error) {
	return ReviewApproved, nil
}

func TestImplementPipeline_LinearChain(t *testing.T) {
	milestones := []MilestoneNode{
		{ID: "M1", Name: "Foundation"},
		{ID: "M2", Name: "Core", DependsOn: []string{"M1"}},
		{ID: "M3", Name: "Integration", DependsOn: []string{"M2"}},
	}

	scheduler, err := NewScheduler(milestones)
	require.NoError(t, err)

	impl, calls := mockImplementer(nil)

	pipeline := NewImplementPipeline(
		ImplementConfig{MaxConcurrent: 1},
		scheduler,
		&autoApproveReview{},
		impl,
	)

	err = pipeline.Run(context.Background())
	require.NoError(t, err)

	// All milestones should have been called in order.
	require.Len(t, *calls, 3)
	assert.Equal(t, "M1", (*calls)[0])
	assert.Equal(t, "M2", (*calls)[1])
	assert.Equal(t, "M3", (*calls)[2])
	assert.True(t, scheduler.AllCompleted())

	pipeline.Close()
}

func TestImplementPipeline_ParallelDiamond(t *testing.T) {
	milestones := []MilestoneNode{
		{ID: "M1", Name: "Foundation"},
		{ID: "M2", Name: "Left", DependsOn: []string{"M1"}},
		{ID: "M3", Name: "Right", DependsOn: []string{"M1"}},
		{ID: "M4", Name: "Merge", DependsOn: []string{"M2", "M3"}},
	}

	scheduler, err := NewScheduler(milestones)
	require.NoError(t, err)

	impl, calls := mockImplementer(nil)

	pipeline := NewImplementPipeline(
		ImplementConfig{MaxConcurrent: 3},
		scheduler,
		&autoApproveReview{},
		impl,
	)

	err = pipeline.Run(context.Background())
	require.NoError(t, err)

	// All milestones should have been called.
	require.Len(t, *calls, 4)
	// M1 must be first. M4 must be last.
	assert.Equal(t, "M1", (*calls)[0])
	assert.Equal(t, "M4", (*calls)[3])
	// M2 and M3 can be in either order.
	middle := []string{(*calls)[1], (*calls)[2]}
	assert.ElementsMatch(t, []string{"M2", "M3"}, middle)
	assert.True(t, scheduler.AllCompleted())

	pipeline.Close()
}

// rejectReview always rejects.
type rejectReview struct{}

func (r *rejectReview) RequestReview(_ context.Context, _ *MilestoneNode, _ []ImplementationArtifact) (ReviewDecision, error) {
	return ReviewRejected, nil
}

func TestImplementPipeline_ReviewReject(t *testing.T) {
	milestones := []MilestoneNode{
		{ID: "M1", Name: "Foundation"},
	}

	scheduler, err := NewScheduler(milestones)
	require.NoError(t, err)

	impl, _ := mockImplementer(nil)

	pipeline := NewImplementPipeline(
		ImplementConfig{MaxConcurrent: 1},
		scheduler,
		&rejectReview{},
		impl,
	)

	err = pipeline.Run(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rejected")

	pipeline.Close()
}

func TestImplementPipeline_ImplementerFails(t *testing.T) {
	milestones := []MilestoneNode{
		{ID: "M1", Name: "Foundation"},
	}

	scheduler, err := NewScheduler(milestones)
	require.NoError(t, err)

	impl, _ := mockImplementer(map[string]error{
		"M1": fmt.Errorf("claude process crashed"),
	})

	pipeline := NewImplementPipeline(
		ImplementConfig{MaxConcurrent: 1},
		scheduler,
		&autoApproveReview{},
		impl,
	)

	err = pipeline.Run(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "claude process crashed")

	pipeline.Close()
}

func TestFormatImplementationSummary(t *testing.T) {
	milestones := []MilestoneNode{
		{ID: "M1", Name: "Foundation", Status: MilestoneCompleted},
		{ID: "M2", Name: "Core", Status: MilestoneCompleted},
		{ID: "M3", Name: "Integration", Status: MilestoneFailed},
	}

	summary := FormatImplementationSummary(milestones)
	assert.Contains(t, summary, "M1")
	assert.Contains(t, summary, "completed")
	assert.Contains(t, summary, "FAILED")
	assert.Contains(t, summary, "2 completed, 1 failed")
}
