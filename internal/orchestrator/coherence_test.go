package orchestrator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckCoherence_MatchingVersions_NoIssues(t *testing.T) {
	sections := []Section{
		{Name: "architecture", Content: "We use React 18.2 for the frontend.", Agent: "agent-1"},
		{Name: "features", Content: "The UI is built with React 18.2.", Agent: "agent-2"},
	}

	issues, err := CheckCoherence(sections)
	require.NoError(t, err)
	assert.Empty(t, issues, "matching versions across sections should produce no issues")
}

func TestCheckCoherence_ConflictingVersions_OneIssue(t *testing.T) {
	sections := []Section{
		{Name: "architecture", Content: "We use React 18.2 for the frontend.", Agent: "agent-1"},
		{Name: "features", Content: "The UI requires React 19.0 features.", Agent: "agent-2"},
	}

	issues, err := CheckCoherence(sections)
	require.NoError(t, err)
	require.Len(t, issues, 1)
	assert.Equal(t, "architecture", issues[0].SectionA)
	assert.Equal(t, "features", issues[0].SectionB)
	assert.Contains(t, issues[0].Description, "react",
		"description should mention the conflicting dependency")
}

func TestCheckCoherence_VersionInsideCodeBlock_NotFlagged(t *testing.T) {
	sections := []Section{
		{Name: "architecture", Content: "We use React 18.2 for the frontend.", Agent: "agent-1"},
		{Name: "features", Content: "Example:\n```\nnpm install React 19.0\n```\nSome other text.", Agent: "agent-2"},
	}

	issues, err := CheckCoherence(sections)
	require.NoError(t, err)
	assert.Empty(t, issues,
		"version numbers inside code blocks should not be flagged as conflicts")
}

func TestCheckCoherence_NoVersionNumbers_NoIssues(t *testing.T) {
	sections := []Section{
		{Name: "architecture", Content: "We use a microservices approach.", Agent: "agent-1"},
		{Name: "features", Content: "The system supports real-time updates.", Agent: "agent-2"},
	}

	issues, err := CheckCoherence(sections)
	require.NoError(t, err)
	assert.Empty(t, issues, "sections without version numbers should produce no issues")
}

func TestCheckCoherence_SameTechSameVersion_DifferentSections(t *testing.T) {
	sections := []Section{
		{Name: "architecture", Content: "We target Go 1.22.3 for backend services.", Agent: "agent-1"},
		{Name: "integrations", Content: "All services compile with Go 1.22.3.", Agent: "agent-2"},
		{Name: "testing", Content: "CI runs Go 1.22.3 across all modules.", Agent: "agent-3"},
	}

	issues, err := CheckCoherence(sections)
	require.NoError(t, err)
	assert.Empty(t, issues,
		"same technology with the same version across different sections should produce no issues")
}
