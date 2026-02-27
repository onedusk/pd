//go:build cgo

package agent

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dusk-indust/decompose/internal/a2a"
	"github.com/dusk-indust/decompose/internal/graph"
	"github.com/dusk-indust/decompose/internal/mcptools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// designPackText is a sample design pack with multiple sections used for
// milestone planning tests.
const designPackText = `plan-milestones

## Authentication Layer
Implement JWT-based auth with refresh tokens and role-based access control.

## Data Access Layer
Build repository pattern over PostgreSQL with connection pooling and migrations.

## API Gateway
Create HTTP router with middleware chain, rate limiting, and request validation.

## Event Bus
Set up async messaging with dead-letter queue and retry policies.
`

func TestPlanningAgent_BuildCodeGraph_WithMCP(t *testing.T) {
	store := graph.NewMemStore()
	parser := graph.NewTreeSitterParser()
	svc := mcptools.NewCodeIntelService(store, parser)
	agent := NewPlanningAgent(WithCodeIntelService(svc))

	absPath, err := filepath.Abs("../../testdata/fixtures/go_project")
	require.NoError(t, err)

	msg := a2a.Message{
		Role:  a2a.RoleUser,
		Parts: []a2a.Part{a2a.TextPart("build-code-graph\n" + absPath)},
	}

	task := a2a.Task{ID: a2a.NewTaskID(), ContextID: "test-build-graph"}
	result, err := agent.HandleTask(context.Background(), task, msg)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, a2a.TaskStateCompleted, result.Status.State)

	// The agent must produce at least one artifact with graph statistics.
	require.NotEmpty(t, result.Artifacts, "expected at least one artifact")

	art := result.Artifacts[0]
	assert.Equal(t, "graph-stats", art.Name)
	require.NotEmpty(t, art.Parts, "artifact must have at least one part")

	text := art.Parts[0].Text
	assert.True(t,
		strings.Contains(text, "Files") || strings.Contains(text, "files"),
		"artifact text should mention files: %s", text)
	assert.True(t,
		strings.Contains(text, "Symbols") || strings.Contains(text, "symbols"),
		"artifact text should mention symbols: %s", text)
}

func TestPlanningAgent_PlanMilestones(t *testing.T) {
	// plan-milestones works without MCP tools, so no CodeIntelService needed.
	agent := NewPlanningAgent()

	msg := a2a.Message{
		Role:  a2a.RoleUser,
		Parts: []a2a.Part{a2a.TextPart(designPackText)},
	}

	task := a2a.Task{ID: a2a.NewTaskID(), ContextID: "test-milestones"}
	result, err := agent.HandleTask(context.Background(), task, msg)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, a2a.TaskStateCompleted, result.Status.State)

	require.NotEmpty(t, result.Artifacts, "expected at least one artifact")

	art := result.Artifacts[0]
	assert.Equal(t, "milestone-plan", art.Name)
	require.NotEmpty(t, art.Parts)

	text := art.Parts[0].Text

	// The output must contain a milestone list table.
	assert.Contains(t, text, "Milestones", "output should contain milestones header")
	assert.Contains(t, text, "M1", "output should contain milestone M1")
	assert.Contains(t, text, "M2", "output should contain milestone M2")

	// The output must contain a dependency graph section.
	assert.Contains(t, text, "Dependency Graph", "output should contain dependency graph section")

	// Milestone IDs should appear in dependency arrow notation.
	assert.True(t,
		strings.Contains(text, "M1") && strings.Contains(text, "M2"),
		"dependency graph should reference milestone IDs: %s", text)
}

func TestPlanningAgent_FallbackMode_NoMCP(t *testing.T) {
	// Create agent without CodeIntelService — MCP-dependent skills should fail,
	// but plan-milestones should still work.
	agent := NewPlanningAgent()
	ctx := context.Background()

	t.Run("build-code-graph returns error without MCP", func(t *testing.T) {
		msg := a2a.Message{
			Role:  a2a.RoleUser,
			Parts: []a2a.Part{a2a.TextPart("build-code-graph\n/some/repo/path")},
		}
		task := a2a.Task{ID: a2a.NewTaskID(), ContextID: "test-fallback-graph"}
		result, err := agent.HandleTask(ctx, task, msg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "CodeIntelService")
		require.NotNil(t, result)
		assert.Equal(t, a2a.TaskStateFailed, result.Status.State)
	})

	t.Run("analyze-dependencies returns error without MCP", func(t *testing.T) {
		msg := a2a.Message{
			Role:  a2a.RoleUser,
			Parts: []a2a.Part{a2a.TextPart("analyze-dependencies\nservice.go downstream")},
		}
		task := a2a.Task{ID: a2a.NewTaskID(), ContextID: "test-fallback-deps"}
		result, err := agent.HandleTask(ctx, task, msg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "CodeIntelService")
		require.NotNil(t, result)
		assert.Equal(t, a2a.TaskStateFailed, result.Status.State)
	})

	t.Run("assess-impact returns error without MCP", func(t *testing.T) {
		msg := a2a.Message{
			Role:  a2a.RoleUser,
			Parts: []a2a.Part{a2a.TextPart("assess-impact\nmodel.go service.go")},
		}
		task := a2a.Task{ID: a2a.NewTaskID(), ContextID: "test-fallback-impact"}
		result, err := agent.HandleTask(ctx, task, msg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "CodeIntelService")
		require.NotNil(t, result)
		assert.Equal(t, a2a.TaskStateFailed, result.Status.State)
	})

	t.Run("plan-milestones still works without MCP", func(t *testing.T) {
		msg := a2a.Message{
			Role:  a2a.RoleUser,
			Parts: []a2a.Part{a2a.TextPart(designPackText)},
		}
		task := a2a.Task{ID: a2a.NewTaskID(), ContextID: "test-fallback-milestones"}
		result, err := agent.HandleTask(ctx, task, msg)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, a2a.TaskStateCompleted, result.Status.State)
		require.NotEmpty(t, result.Artifacts)
		assert.Equal(t, "milestone-plan", result.Artifacts[0].Name)
	})
}

func TestPlanningAgent_AgentCard(t *testing.T) {
	agent := NewPlanningAgent()
	card := agent.Card()

	t.Run("name", func(t *testing.T) {
		assert.Equal(t, "planning-agent", card.Name)
	})

	t.Run("skills", func(t *testing.T) {
		require.Len(t, card.Skills, 4, "planning agent should expose 4 skills")

		skillIDs := make([]string, len(card.Skills))
		for i, s := range card.Skills {
			skillIDs[i] = s.ID
		}
		assert.Contains(t, skillIDs, "build-code-graph")
		assert.Contains(t, skillIDs, "analyze-dependencies")
		assert.Contains(t, skillIDs, "assess-impact")
		assert.Contains(t, skillIDs, "plan-milestones")
	})

	t.Run("input and output modes", func(t *testing.T) {
		assert.Contains(t, card.DefaultInputModes, "text/plain")
		assert.Contains(t, card.DefaultInputModes, "application/json")
		assert.Contains(t, card.DefaultOutputModes, "text/markdown")
		assert.Contains(t, card.DefaultOutputModes, "application/json")
	})
}
