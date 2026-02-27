package agent

import (
	"context"
	"regexp"
	"strings"
	"testing"

	"github.com/dusk-indust/decompose/internal/a2a"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --------------------------------------------------------------------------
// T-05.08  TaskWriterAgent tests
// --------------------------------------------------------------------------

func TestTaskWriter_WriteTaskSpecs(t *testing.T) {
	agent := NewTaskWriterAgent()

	input := `write-task-specs
Milestone 2: Code Intelligence
1. internal/graph/parser.go (CREATE) - Tree-sitter parser interface
2. internal/graph/memstore.go (CREATE) - In-memory graph store. Depends on: parser.go
3. internal/graph/treesitter.go (CREATE) - Tree-sitter implementation. Depends on: parser.go`

	msg := a2a.Message{
		Role:  a2a.RoleUser,
		Parts: []a2a.Part{a2a.TextPart(input)},
	}
	task := a2a.Task{ID: a2a.NewTaskID(), ContextID: "test"}
	result, err := agent.HandleTask(context.Background(), task, msg)
	require.NoError(t, err)
	assert.Equal(t, a2a.TaskStateCompleted, result.Status.State)

	require.NotEmpty(t, result.Artifacts, "expected at least one artifact")
	require.NotEmpty(t, result.Artifacts[0].Parts, "expected at least one part in the artifact")

	text := result.Artifacts[0].Parts[0].Text

	// The output should reference milestone 02.
	assert.Contains(t, text, "T-02.")

	// There should be at least 3 task IDs in T-MM.SS format.
	taskIDPattern := regexp.MustCompile(`T-\d{2}\.\d{2}`)
	matches := taskIDPattern.FindAllString(text, -1)
	assert.GreaterOrEqual(t, len(matches), 3, "expected at least 3 task IDs, got %d", len(matches))

	// Verify all three file paths appear.
	assert.Contains(t, text, "internal/graph/parser.go")
	assert.Contains(t, text, "internal/graph/memstore.go")
	assert.Contains(t, text, "internal/graph/treesitter.go")

	// Verify action is CREATE for the generated tasks.
	assert.Contains(t, text, "CREATE")

	// Verify artifact metadata.
	assert.Equal(t, "tasks_m02", result.Artifacts[0].ArtifactID)
	assert.Equal(t, "tasks_m02.md", result.Artifacts[0].Name)
}

func TestTaskWriter_TaskOrdering(t *testing.T) {
	agent := NewTaskWriterAgent()

	// File B depends on file A, so task for A should come before B.
	input := `write-task-specs
Milestone 3: Service Layer
1. internal/service/repo.go (CREATE) - Repository interface
2. internal/service/handler.go (CREATE) - HTTP handler. Depends on: repo.go`

	msg := a2a.Message{
		Role:  a2a.RoleUser,
		Parts: []a2a.Part{a2a.TextPart(input)},
	}
	task := a2a.Task{ID: a2a.NewTaskID(), ContextID: "test-ordering"}
	result, err := agent.HandleTask(context.Background(), task, msg)
	require.NoError(t, err)
	assert.Equal(t, a2a.TaskStateCompleted, result.Status.State)

	text := result.Artifacts[0].Parts[0].Text

	// Task for repo.go (T-03.01) should appear before handler.go (T-03.02).
	posRepo := strings.Index(text, "T-03.01")
	posHandler := strings.Index(text, "T-03.02")

	require.NotEqual(t, -1, posRepo, "T-03.01 not found in output")
	require.NotEqual(t, -1, posHandler, "T-03.02 not found in output")
	assert.Less(t, posRepo, posHandler,
		"task for repo.go (T-03.01) should appear before handler.go (T-03.02)")

	// Also verify repo.go appears with T-03.01 and handler.go with T-03.02.
	// Split by T-03.02 section; the first half should contain repo.go.
	sections := strings.SplitN(text, "## T-03.02", 2)
	require.Len(t, sections, 2, "expected text to be split by T-03.02 heading")
	assert.Contains(t, sections[0], "repo.go", "repo.go should be in the first task section")
	assert.Contains(t, sections[1], "handler.go", "handler.go should be in the second task section")
}

func TestTaskWriter_ValidateDependencies_MissingReference(t *testing.T) {
	agent := NewTaskWriterAgent()

	// T-01.03 is referenced but no M1 task file defines it.
	input := `validate-dependencies

## T-02.01 — Graph parser
- Depends on: T-01.03

## T-02.02 — Graph store
- Depends on: T-02.01`

	msg := a2a.Message{
		Role:  a2a.RoleUser,
		Parts: []a2a.Part{a2a.TextPart(input)},
	}
	task := a2a.Task{ID: a2a.NewTaskID(), ContextID: "test-missing-dep"}
	result, err := agent.HandleTask(context.Background(), task, msg)
	require.NoError(t, err)
	assert.Equal(t, a2a.TaskStateCompleted, result.Status.State)

	require.NotEmpty(t, result.Artifacts)
	text := result.Artifacts[0].Parts[0].Text

	// The report should flag T-01.03 as missing/undefined.
	assert.Contains(t, text, "Missing References",
		"report should contain a missing references section")
	assert.Contains(t, text, "T-01.03",
		"report should mention the undefined task T-01.03")
	assert.Contains(t, text, "undefined",
		"report should state the reference is undefined")
}

func TestTaskWriter_ValidateDependencies_CircularDependency(t *testing.T) {
	agent := NewTaskWriterAgent()

	input := `validate-dependencies

## T-01.01 — Setup
- Depends on: T-01.02

## T-01.02 — Config
- Depends on: T-01.01`

	msg := a2a.Message{
		Role:  a2a.RoleUser,
		Parts: []a2a.Part{a2a.TextPart(input)},
	}
	task := a2a.Task{ID: a2a.NewTaskID(), ContextID: "test-circular"}
	result, err := agent.HandleTask(context.Background(), task, msg)
	require.NoError(t, err)
	assert.Equal(t, a2a.TaskStateCompleted, result.Status.State)

	require.NotEmpty(t, result.Artifacts)
	text := result.Artifacts[0].Parts[0].Text

	// The report should detect a circular dependency.
	assert.Contains(t, text, "Circular Dependencies",
		"report should contain a circular dependencies section")
	assert.Contains(t, text, "T-01.01",
		"report should mention T-01.01 as part of the cycle")
	assert.Contains(t, text, "T-01.02",
		"report should mention T-01.02 as part of the cycle")
}

func TestTaskWriter_AgentCard(t *testing.T) {
	agent := NewTaskWriterAgent()
	card := agent.Card()

	t.Run("name", func(t *testing.T) {
		assert.Equal(t, "task-writer-agent", card.Name)
	})

	t.Run("skills", func(t *testing.T) {
		require.Len(t, card.Skills, 2, "task-writer-agent should have exactly 2 skills")

		skillIDs := make(map[string]bool)
		for _, skill := range card.Skills {
			skillIDs[skill.ID] = true
		}
		assert.True(t, skillIDs["write-task-specs"],
			"agent should have the write-task-specs skill")
		assert.True(t, skillIDs["validate-dependencies"],
			"agent should have the validate-dependencies skill")
	})

	t.Run("input and output modes", func(t *testing.T) {
		assert.Contains(t, card.DefaultInputModes, "text/plain",
			"input modes should include text/plain")
		assert.Contains(t, card.DefaultInputModes, "text/markdown",
			"input modes should include text/markdown")
		assert.Contains(t, card.DefaultOutputModes, "text/markdown",
			"output modes should include text/markdown")
	})
}

func TestTaskWriter_UnknownSkill(t *testing.T) {
	agent := NewTaskWriterAgent()

	msg := a2a.Message{
		Role:  a2a.RoleUser,
		Parts: []a2a.Part{a2a.TextPart("do something unrecognized")},
	}
	task := a2a.Task{ID: a2a.NewTaskID(), ContextID: "test-unknown"}
	result, err := agent.HandleTask(context.Background(), task, msg)

	require.Error(t, err, "should return an error for unrecognized skill")
	assert.Contains(t, err.Error(), "unknown skill")
	require.NotNil(t, result)
	assert.Equal(t, a2a.TaskStateFailed, result.Status.State)
}
