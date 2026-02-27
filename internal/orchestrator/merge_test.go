package orchestrator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMerge_InOrder(t *testing.T) {
	plan := MergePlan{
		Strategy:     MergeConcatenate,
		SectionOrder: []string{"alpha", "beta", "gamma"},
	}
	m := NewMerger(plan)

	sections := []Section{
		{Name: "alpha", Content: "AAA", Agent: "agent-1"},
		{Name: "beta", Content: "BBB", Agent: "agent-2"},
		{Name: "gamma", Content: "CCC", Agent: "agent-3"},
	}

	got, err := m.Merge(sections)
	require.NoError(t, err)
	assert.Equal(t, "AAA\n\n---\n\nBBB\n\n---\n\nCCC", got)
}

func TestMerge_OutOfOrder_Reordered(t *testing.T) {
	plan := MergePlan{
		Strategy:     MergeConcatenate,
		SectionOrder: []string{"alpha", "beta", "gamma"},
	}
	m := NewMerger(plan)

	// Provide sections in reverse order.
	sections := []Section{
		{Name: "gamma", Content: "CCC", Agent: "agent-3"},
		{Name: "alpha", Content: "AAA", Agent: "agent-1"},
		{Name: "beta", Content: "BBB", Agent: "agent-2"},
	}

	got, err := m.Merge(sections)
	require.NoError(t, err)
	assert.Equal(t, "AAA\n\n---\n\nBBB\n\n---\n\nCCC", got,
		"sections should be reordered to match the plan's SectionOrder")
}

func TestMerge_MissingSection_Error(t *testing.T) {
	plan := MergePlan{
		Strategy:     MergeConcatenate,
		SectionOrder: []string{"alpha", "beta", "gamma"},
	}
	m := NewMerger(plan)

	sections := []Section{
		{Name: "alpha", Content: "AAA", Agent: "agent-1"},
		{Name: "gamma", Content: "CCC", Agent: "agent-3"},
	}

	_, err := m.Merge(sections)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "beta",
		"error should list the missing section name")
}

func TestMerge_DuplicateSectionName_Error(t *testing.T) {
	plan := MergePlan{
		Strategy:     MergeConcatenate,
		SectionOrder: []string{"alpha", "beta"},
	}
	m := NewMerger(plan)

	sections := []Section{
		{Name: "alpha", Content: "AAA", Agent: "agent-1"},
		{Name: "beta", Content: "BBB", Agent: "agent-2"},
		{Name: "alpha", Content: "AAA-dup", Agent: "agent-3"},
	}

	_, err := m.Merge(sections)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate")
	assert.Contains(t, err.Error(), "alpha")
}

func TestMerge_ExtraSection_AppendedAtEnd(t *testing.T) {
	plan := MergePlan{
		Strategy:     MergeConcatenate,
		SectionOrder: []string{"alpha", "beta"},
	}
	m := NewMerger(plan)

	sections := []Section{
		{Name: "alpha", Content: "AAA", Agent: "agent-1"},
		{Name: "beta", Content: "BBB", Agent: "agent-2"},
		{Name: "extra", Content: "EEE", Agent: "agent-3"},
	}

	got, err := m.Merge(sections)
	require.NoError(t, err)
	assert.Equal(t, "AAA\n\n---\n\nBBB\n\n---\n\nEEE", got,
		"extra section not in the plan should be appended at the end")
}

func TestMerge_EmptySectionContent_Included(t *testing.T) {
	plan := MergePlan{
		Strategy:     MergeConcatenate,
		SectionOrder: []string{"alpha", "beta", "gamma"},
	}
	m := NewMerger(plan)

	sections := []Section{
		{Name: "alpha", Content: "AAA", Agent: "agent-1"},
		{Name: "beta", Content: "", Agent: "agent-2"},
		{Name: "gamma", Content: "CCC", Agent: "agent-3"},
	}

	got, err := m.Merge(sections)
	require.NoError(t, err)
	assert.Equal(t, "AAA\n\n---\n\n\n\n---\n\nCCC", got,
		"empty section content should still appear between separators")
}
