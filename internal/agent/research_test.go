package agent

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/dusk-indust/decompose/internal/a2a"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// T-05.02 — explore-codebase
// ---------------------------------------------------------------------------

func TestResearchAgent_ExploreCodebase(t *testing.T) {
	agent := NewResearchAgent()

	absPath, err := filepath.Abs("../../testdata/fixtures/go_project")
	require.NoError(t, err)

	msg := a2a.Message{
		Role:  a2a.RoleUser,
		Parts: []a2a.Part{a2a.TextPart("explore-codebase\n" + absPath)},
	}

	task := a2a.Task{ID: a2a.NewTaskID(), ContextID: "test"}
	result, err := agent.HandleTask(context.Background(), task, msg)
	require.NoError(t, err)
	assert.Equal(t, a2a.TaskStateCompleted, result.Status.State)
	require.NotEmpty(t, result.Artifacts)

	text := result.Artifacts[0].Parts[0].Text

	// Artifact must contain a file listing (directory tree).
	assert.Contains(t, text, "main.go")
	assert.Contains(t, text, "model.go")
	assert.Contains(t, text, "service.go")

	// Artifact must mention the detected language (Go or .go extension).
	goMentioned := false
	if assert.ObjectsAreEqual(true, containsAny(text, "Go", ".go")) {
		goMentioned = true
	}
	assert.True(t, goMentioned, "artifact should mention Go language or .go extension")
}

// containsAny returns true if s contains at least one of the given substrings.
func containsAny(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if len(sub) > 0 {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// T-05.02 — research-platform
// ---------------------------------------------------------------------------

func TestResearchAgent_ResearchPlatform_NoGoMod(t *testing.T) {
	// testdata/fixtures/go_project has no go.mod — agent should still produce
	// output in fallback mode rather than returning an error.
	agent := NewResearchAgent()

	absPath, err := filepath.Abs("../../testdata/fixtures/go_project")
	require.NoError(t, err)

	msg := a2a.Message{
		Role:  a2a.RoleUser,
		Parts: []a2a.Part{a2a.TextPart("research-platform\n" + absPath)},
	}

	task := a2a.Task{ID: a2a.NewTaskID(), ContextID: "test"}
	result, err := agent.HandleTask(context.Background(), task, msg)
	require.NoError(t, err)
	assert.Equal(t, a2a.TaskStateCompleted, result.Status.State)
	require.NotEmpty(t, result.Artifacts)

	text := result.Artifacts[0].Parts[0].Text
	// Even without go.mod, the agent should produce some output (the fallback
	// message indicating no recognized config files were found).
	assert.NotEmpty(t, text)
	assert.Contains(t, text, "Platform")
}

func TestResearchAgent_ResearchPlatform_WithGoMod(t *testing.T) {
	// The repo root has a go.mod — assert the artifact mentions the module path.
	agent := NewResearchAgent()

	absPath, err := filepath.Abs("../../")
	require.NoError(t, err)

	msg := a2a.Message{
		Role:  a2a.RoleUser,
		Parts: []a2a.Part{a2a.TextPart("research-platform\n" + absPath)},
	}

	task := a2a.Task{ID: a2a.NewTaskID(), ContextID: "test"}
	result, err := agent.HandleTask(context.Background(), task, msg)
	require.NoError(t, err)
	assert.Equal(t, a2a.TaskStateCompleted, result.Status.State)
	require.NotEmpty(t, result.Artifacts)

	text := result.Artifacts[0].Parts[0].Text
	assert.Contains(t, text, "github.com/dusk-indust/decompose")
}

// ---------------------------------------------------------------------------
// T-05.02 — Agent Card
// ---------------------------------------------------------------------------

func TestResearchAgent_Card(t *testing.T) {
	agent := NewResearchAgent()
	card := agent.Card()

	t.Run("name", func(t *testing.T) {
		assert.Equal(t, "research-agent", card.Name)
	})

	t.Run("version", func(t *testing.T) {
		assert.NotEmpty(t, card.Version)
	})

	t.Run("skills count", func(t *testing.T) {
		require.Len(t, card.Skills, 3)
	})

	t.Run("skill IDs", func(t *testing.T) {
		ids := make(map[string]bool)
		for _, s := range card.Skills {
			ids[s.ID] = true
		}
		assert.True(t, ids["explore-codebase"], "missing skill explore-codebase")
		assert.True(t, ids["research-platform"], "missing skill research-platform")
		assert.True(t, ids["verify-versions"], "missing skill verify-versions")
	})

	t.Run("input modes", func(t *testing.T) {
		require.NotEmpty(t, card.DefaultInputModes)
		assert.Contains(t, card.DefaultInputModes, "text/plain")
	})

	t.Run("output modes", func(t *testing.T) {
		require.NotEmpty(t, card.DefaultOutputModes)
		assert.Contains(t, card.DefaultOutputModes, "text/markdown")
	})
}

// ---------------------------------------------------------------------------
// T-05.02 — Unknown skill
// ---------------------------------------------------------------------------

func TestResearchAgent_UnknownSkill(t *testing.T) {
	agent := NewResearchAgent()

	msg := a2a.Message{
		Role:  a2a.RoleUser,
		Parts: []a2a.Part{a2a.TextPart("do-something-unknown")},
	}

	task := a2a.Task{ID: a2a.NewTaskID(), ContextID: "test"}
	result, err := agent.HandleTask(context.Background(), task, msg)

	// HandleTask returns an error and the task in FAILED state.
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown skill")

	require.NotNil(t, result)
	assert.Equal(t, a2a.TaskStateFailed, result.Status.State)

	// The failed task's status message should contain the error text.
	require.NotNil(t, result.Status.Message)
	require.NotEmpty(t, result.Status.Message.Parts)
	assert.Contains(t, result.Status.Message.Parts[0].Text, "unknown skill")
}

// ---------------------------------------------------------------------------
// T-05.02 — verify-versions (fallback mode)
// ---------------------------------------------------------------------------

func TestResearchAgent_VerifyVersions_FallbackMode(t *testing.T) {
	agent := NewResearchAgent()

	msg := a2a.Message{
		Role:  a2a.RoleUser,
		Parts: []a2a.Part{a2a.TextPart("verify-versions")},
	}

	task := a2a.Task{ID: a2a.NewTaskID(), ContextID: "test"}
	result, err := agent.HandleTask(context.Background(), task, msg)
	require.NoError(t, err)
	assert.Equal(t, a2a.TaskStateCompleted, result.Status.State)
	require.NotEmpty(t, result.Artifacts)

	text := result.Artifacts[0].Parts[0].Text
	assert.Contains(t, text, "fallback")
}
