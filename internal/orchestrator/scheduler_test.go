package orchestrator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScheduler_LinearChain(t *testing.T) {
	milestones := []MilestoneNode{
		{ID: "M1", Name: "Foundation"},
		{ID: "M2", Name: "Core", DependsOn: []string{"M1"}},
		{ID: "M3", Name: "Integration", DependsOn: []string{"M2"}},
	}

	s, err := NewScheduler(milestones)
	require.NoError(t, err)

	// Only M1 should be ready initially.
	ready := s.Ready()
	require.Len(t, ready, 1)
	assert.Equal(t, "M1", ready[0].ID)

	// Start and complete M1.
	require.NoError(t, s.MarkRunning("M1"))
	newlyReady, err := s.MarkCompleted("M1")
	require.NoError(t, err)
	require.Len(t, newlyReady, 1)
	assert.Equal(t, "M2", newlyReady[0].ID)

	// Start and complete M2.
	require.NoError(t, s.MarkRunning("M2"))
	newlyReady, err = s.MarkCompleted("M2")
	require.NoError(t, err)
	require.Len(t, newlyReady, 1)
	assert.Equal(t, "M3", newlyReady[0].ID)

	// Complete M3.
	require.NoError(t, s.MarkRunning("M3"))
	newlyReady, err = s.MarkCompleted("M3")
	require.NoError(t, err)
	assert.Empty(t, newlyReady)
	assert.True(t, s.AllCompleted())
}

func TestScheduler_Diamond(t *testing.T) {
	// M1 -> M2, M1 -> M3, M2+M3 -> M4
	milestones := []MilestoneNode{
		{ID: "M1", Name: "Foundation"},
		{ID: "M2", Name: "Left", DependsOn: []string{"M1"}},
		{ID: "M3", Name: "Right", DependsOn: []string{"M1"}},
		{ID: "M4", Name: "Merge", DependsOn: []string{"M2", "M3"}},
	}

	s, err := NewScheduler(milestones)
	require.NoError(t, err)

	// Only M1 ready initially.
	ready := s.Ready()
	require.Len(t, ready, 1)
	assert.Equal(t, "M1", ready[0].ID)

	// Complete M1 -> M2 and M3 become ready.
	require.NoError(t, s.MarkRunning("M1"))
	newlyReady, err := s.MarkCompleted("M1")
	require.NoError(t, err)
	require.Len(t, newlyReady, 2)
	readyIDs := []string{newlyReady[0].ID, newlyReady[1].ID}
	assert.ElementsMatch(t, []string{"M2", "M3"}, readyIDs)

	// Complete M2 -> M4 not yet ready (M3 still pending).
	require.NoError(t, s.MarkRunning("M2"))
	newlyReady, err = s.MarkCompleted("M2")
	require.NoError(t, err)
	assert.Empty(t, newlyReady, "M4 should not be ready until M3 completes")

	// Complete M3 -> M4 becomes ready.
	require.NoError(t, s.MarkRunning("M3"))
	newlyReady, err = s.MarkCompleted("M3")
	require.NoError(t, err)
	require.Len(t, newlyReady, 1)
	assert.Equal(t, "M4", newlyReady[0].ID)
}

func TestScheduler_NoDeps_AllReady(t *testing.T) {
	milestones := []MilestoneNode{
		{ID: "M1", Name: "A"},
		{ID: "M2", Name: "B"},
		{ID: "M3", Name: "C"},
	}

	s, err := NewScheduler(milestones)
	require.NoError(t, err)

	ready := s.Ready()
	assert.Len(t, ready, 3, "all milestones should be ready when there are no deps")
}

func TestScheduler_CycleDetection(t *testing.T) {
	milestones := []MilestoneNode{
		{ID: "M1", Name: "A", DependsOn: []string{"M2"}},
		{ID: "M2", Name: "B", DependsOn: []string{"M1"}},
	}

	_, err := NewScheduler(milestones)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cycle")
}

func TestScheduler_MissingDep(t *testing.T) {
	milestones := []MilestoneNode{
		{ID: "M1", Name: "A", DependsOn: []string{"M99"}},
	}

	_, err := NewScheduler(milestones)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "M99")
}

func TestScheduler_MarkFailed(t *testing.T) {
	milestones := []MilestoneNode{
		{ID: "M1", Name: "A"},
		{ID: "M2", Name: "B", DependsOn: []string{"M1"}},
	}

	s, err := NewScheduler(milestones)
	require.NoError(t, err)

	require.NoError(t, s.MarkRunning("M1"))
	require.NoError(t, s.MarkFailed("M1"))

	// M2 should not be ready since M1 is failed, not completed.
	ready := s.Ready()
	assert.Empty(t, ready)
	assert.False(t, s.AllCompleted())
}

func TestScheduler_MarkRunning_InvalidState(t *testing.T) {
	milestones := []MilestoneNode{
		{ID: "M1", Name: "A"},
	}

	s, err := NewScheduler(milestones)
	require.NoError(t, err)

	require.NoError(t, s.MarkRunning("M1"))
	err = s.MarkRunning("M1") // already running
	require.Error(t, err)
	assert.Contains(t, err.Error(), "running")
}

// ---------------------------------------------------------------------------
// ParseMilestones tests
// ---------------------------------------------------------------------------

func TestParseMilestones_Basic(t *testing.T) {
	stage3 := `# Task Index

## M1: Foundation
Tasks for foundation.

## M2: Core Logic
Tasks for core.

## M3: Integration
Tasks for integration.

### Milestone Dependency Graph
M1 → M2
M1 → M3
M2 → M3`

	milestones, err := ParseMilestones(stage3)
	require.NoError(t, err)
	require.Len(t, milestones, 3)

	assert.Equal(t, "M1", milestones[0].ID)
	assert.Equal(t, "Foundation", milestones[0].Name)
	assert.Empty(t, milestones[0].DependsOn)

	assert.Equal(t, "M2", milestones[1].ID)
	assert.Equal(t, []string{"M1"}, milestones[1].DependsOn)

	assert.Equal(t, "M3", milestones[2].ID)
	assert.ElementsMatch(t, []string{"M1", "M2"}, milestones[2].DependsOn)
}

func TestParseMilestones_ParallelPaths(t *testing.T) {
	stage3 := `# Task Index

## M1: Foundation
## M2: Path A
## M3: Path B
## M4: Merge

M1 → M2
M1 → M3
M2 → M4
M3 → M4`

	milestones, err := ParseMilestones(stage3)
	require.NoError(t, err)
	require.Len(t, milestones, 4)

	assert.Empty(t, milestones[0].DependsOn)                         // M1
	assert.Equal(t, []string{"M1"}, milestones[1].DependsOn)         // M2
	assert.Equal(t, []string{"M1"}, milestones[2].DependsOn)         // M3
	assert.ElementsMatch(t, []string{"M2", "M3"}, milestones[3].DependsOn) // M4
}

func TestParseMilestones_NoMilestones(t *testing.T) {
	_, err := ParseMilestones("Some random content without milestones.")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no milestone definitions")
}
