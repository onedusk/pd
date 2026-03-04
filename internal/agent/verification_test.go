package agent

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/onedusk/pd/internal/a2a"
	"github.com/onedusk/pd/internal/orchestrator"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVerificationAgent_Card(t *testing.T) {
	va := NewVerificationAgent()
	card := va.Card()
	assert.Equal(t, "verification-agent", card.Name)
	assert.Len(t, card.Skills, 2)
	assert.Equal(t, "verify-stage", card.Skills[0].ID)
	assert.Equal(t, "verify-cross-stage", card.Skills[1].ID)
}

func TestVerificationAgent_VerifyStage_Passes(t *testing.T) {
	va := NewVerificationAgent()
	ctx := context.Background()

	content := `# Development Standards

## Code Change Checklist
Steps here.

## Changeset Format
Format here.

## Escalation Guidance
Guidance here.

## Testing Guidance
Guidance here.`

	msgText := BuildVerificationMessage("verify-stage", orchestrator.StageDevelopmentStandards, content, nil)

	task := a2a.Task{
		ID: a2a.NewTaskID(),
	}
	msg := a2a.Message{
		Role:  a2a.RoleUser,
		Parts: []a2a.Part{a2a.TextPart(msgText)},
	}

	result, err := va.HandleTask(ctx, task, msg)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, a2a.TaskStateCompleted, result.Status.State)
	require.Len(t, result.Artifacts, 2)

	// First artifact is markdown.
	assert.Equal(t, "verification-report", result.Artifacts[0].Name)
	assert.Contains(t, result.Artifacts[0].Parts[0].Text, "PASSED")

	// Second artifact is JSON.
	assert.Equal(t, "verification-data", result.Artifacts[1].Name)
	var report orchestrator.VerificationReport
	err = json.Unmarshal([]byte(result.Artifacts[1].Parts[0].Text), &report)
	require.NoError(t, err)
	assert.True(t, report.Passed)
}

func TestVerificationAgent_VerifyStage_FailsMissingSections(t *testing.T) {
	va := NewVerificationAgent()
	ctx := context.Background()

	content := `# Development Standards

## Code Change Checklist
Steps here.`

	msgText := BuildVerificationMessage("verify-stage", orchestrator.StageDevelopmentStandards, content, nil)

	task := a2a.Task{
		ID: a2a.NewTaskID(),
	}
	msg := a2a.Message{
		Role:  a2a.RoleUser,
		Parts: []a2a.Part{a2a.TextPart(msgText)},
	}

	result, err := va.HandleTask(ctx, task, msg)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, a2a.TaskStateCompleted, result.Status.State)

	// Parse JSON report.
	var report orchestrator.VerificationReport
	err = json.Unmarshal([]byte(result.Artifacts[1].Parts[0].Text), &report)
	require.NoError(t, err)
	assert.False(t, report.Passed)
	assert.True(t, report.HasCritical())
}

func TestVerificationAgent_VerifyCrossStage(t *testing.T) {
	va := NewVerificationAgent()
	ctx := context.Background()

	stage1Content := "Data model: UserAccount, OrderItem entities with one-to-many relationships."
	stage2Content := "```go\ntype UserAccount struct {}\n```"

	priorStages := []orchestrator.StageResult{
		{
			Stage: orchestrator.StageDesignPack,
			Sections: []orchestrator.Section{
				{Name: "design-pack", Content: stage1Content},
			},
		},
	}

	msgText := BuildVerificationMessage("verify-cross-stage", orchestrator.StageImplementationSkeletons, stage2Content, priorStages)

	task := a2a.Task{
		ID: a2a.NewTaskID(),
	}
	msg := a2a.Message{
		Role:  a2a.RoleUser,
		Parts: []a2a.Part{a2a.TextPart(msgText)},
	}

	result, err := va.HandleTask(ctx, task, msg)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, a2a.TaskStateCompleted, result.Status.State)
	require.Len(t, result.Artifacts, 2)
}

func TestVerificationAgent_UnknownSkill(t *testing.T) {
	va := NewVerificationAgent()
	ctx := context.Background()

	task := a2a.Task{
		ID: a2a.NewTaskID(),
	}
	msg := a2a.Message{
		Role:  a2a.RoleUser,
		Parts: []a2a.Part{a2a.TextPart("do something unknown")},
	}

	result, err := va.HandleTask(ctx, task, msg)
	require.Error(t, err)
	require.NotNil(t, result)
	assert.Equal(t, a2a.TaskStateFailed, result.Status.State)
}

func TestBuildVerificationMessage_RoundTrip(t *testing.T) {
	stage := orchestrator.StageImplementationSkeletons
	content := "```go\ntype Foo struct{}\n```"
	priorStages := []orchestrator.StageResult{
		{
			Stage: orchestrator.StageDesignPack,
			Sections: []orchestrator.Section{
				{Name: "design-pack", Content: "Stage 1 content here."},
			},
		},
	}

	msgText := BuildVerificationMessage("verify-stage", stage, content, priorStages)

	parsedStage, parsedContent, parsedPrior, err := parseVerificationInput(msgText)
	require.NoError(t, err)
	assert.Equal(t, stage, parsedStage)
	assert.Equal(t, content, parsedContent)
	require.Len(t, parsedPrior, 1)
	assert.Equal(t, orchestrator.StageDesignPack, parsedPrior[0].Stage)
	assert.Contains(t, parsedPrior[0].Sections[0].Content, "Stage 1 content here.")
}

func TestRegistryIncludesVerification(t *testing.T) {
	reg := NewRegistry()
	agent, err := reg.Spawn(RoleVerification)
	require.NoError(t, err)
	assert.Equal(t, "verification-agent", agent.Card().Name)
}
