package agent

import (
	"context"
	"testing"

	"github.com/dusk-indust/decompose/internal/a2a"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// schemaMsg builds a user Message with the given text parts.
func schemaMsg(text string) a2a.Message {
	return a2a.Message{
		Role:  a2a.RoleUser,
		Parts: []a2a.Part{a2a.TextPart(text)},
	}
}

// schemaTask returns a fresh Task suitable for schema tests.
func schemaTask() a2a.Task {
	return a2a.Task{ID: a2a.NewTaskID(), ContextID: "test"}
}

func TestSchemaAgent_TranslateSchema(t *testing.T) {
	agent := NewSchemaAgent()

	msg := schemaMsg("translate-schema\nEntity User with fields name (string), age (int)")
	result, err := agent.HandleTask(context.Background(), schemaTask(), msg)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, a2a.TaskStateCompleted, result.Status.State)
	require.NotEmpty(t, result.Artifacts)
	require.NotEmpty(t, result.Artifacts[0].Parts)

	text := result.Artifacts[0].Parts[0].Text
	assert.Contains(t, text, "type User struct")
	assert.Contains(t, text, "Name")
	assert.Contains(t, text, "string")
	assert.Contains(t, text, "Age")
	assert.Contains(t, text, "int")
}

func TestSchemaAgent_TranslateSchemaBraceStyle(t *testing.T) {
	agent := NewSchemaAgent()

	msg := schemaMsg("translate-schema\ntype Product { title: string, price: float }")
	result, err := agent.HandleTask(context.Background(), schemaTask(), msg)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, a2a.TaskStateCompleted, result.Status.State)
	require.NotEmpty(t, result.Artifacts)
	require.NotEmpty(t, result.Artifacts[0].Parts)

	text := result.Artifacts[0].Parts[0].Text
	assert.Contains(t, text, "type Product struct")
	assert.Contains(t, text, "Title")
	assert.Contains(t, text, "string")
	assert.Contains(t, text, "Price")
	assert.Contains(t, text, "float64")
}

func TestSchemaAgent_WriteContracts(t *testing.T) {
	agent := NewSchemaAgent()

	msg := schemaMsg("write-contracts\nPOST /users takes UserInput returns UserOutput")
	result, err := agent.HandleTask(context.Background(), schemaTask(), msg)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, a2a.TaskStateCompleted, result.Status.State)
	require.NotEmpty(t, result.Artifacts)
	require.NotEmpty(t, result.Artifacts[0].Parts)

	text := result.Artifacts[0].Parts[0].Text
	assert.Contains(t, text, "Request")
	assert.Contains(t, text, "Response")
	assert.Contains(t, text, "UserInput")
	assert.Contains(t, text, "UserOutput")
}

func TestSchemaAgent_ValidateTypesFallback(t *testing.T) {
	agent := NewSchemaAgent()

	// Send a validate-types message without any Go code to trigger the
	// fallback path that notes MCP tools are unavailable.
	msg := schemaMsg("validate-types\nplease check these definitions")
	result, err := agent.HandleTask(context.Background(), schemaTask(), msg)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, a2a.TaskStateCompleted, result.Status.State)
	require.NotEmpty(t, result.Artifacts)
	require.NotEmpty(t, result.Artifacts[0].Parts)

	text := result.Artifacts[0].Parts[0].Text
	assert.Contains(t, text, "MCP tools")
	assert.Contains(t, text, "not yet available")
}

func TestSchemaAgent_AgentCard(t *testing.T) {
	agent := NewSchemaAgent()
	card := agent.Card()

	assert.Equal(t, "schema-agent", card.Name)
	assert.NotEmpty(t, card.Description)

	// Verify the three expected skills are present.
	require.Len(t, card.Skills, 3)

	skillIDs := make(map[string]bool)
	for _, s := range card.Skills {
		skillIDs[s.ID] = true
	}
	assert.True(t, skillIDs["translate-schema"], "missing skill translate-schema")
	assert.True(t, skillIDs["validate-types"], "missing skill validate-types")
	assert.True(t, skillIDs["write-contracts"], "missing skill write-contracts")

	// Verify default modes are set.
	assert.NotEmpty(t, card.DefaultInputModes)
	assert.NotEmpty(t, card.DefaultOutputModes)
}

func TestSchemaAgent_UnknownSkill(t *testing.T) {
	agent := NewSchemaAgent()

	msg := schemaMsg("do something completely unrelated with no keywords")
	result, err := agent.HandleTask(context.Background(), schemaTask(), msg)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown skill")
	require.NotNil(t, result)
	assert.Equal(t, a2a.TaskStateFailed, result.Status.State)
}
