package orchestrator

import (
	"fmt"
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

// agentParallelStage3 holds lines copied from
// docs/decompose/agent-parallel/stage-3-task-index.md: the Progress table, the
// Milestone Dependencies section, and the milestone summary headings.
const agentParallelStage3 = `## Progress

| # | Milestone | File | Tasks | Done |
|---|-----------|------|:-----:|:----:|
| M1 | Project Scaffolding + A2A Types | [tasks_m01.md](tasks_m01.md) | 9 | 0 |
| M2 | Code Intelligence Layer | [tasks_m02.md](tasks_m02.md) | 12 | 0 |
| M3 | MCP Tool Servers | [tasks_m03.md](tasks_m03.md) | 6 | 0 |
| M4 | A2A Agent Framework | [tasks_m04.md](tasks_m04.md) | 9 | 0 |
| M5 | Specialist Agents | [tasks_m05.md](tasks_m05.md) | 9 | 0 |
| M6 | Orchestrator Pipeline | [tasks_m06.md](tasks_m06.md) | 10 | 0 |
| M7 | Graceful Degradation + Skill Integration | [tasks_m07.md](tasks_m07.md) | 10 | 0 |
| M8 | Testing + Release | [tasks_m08.md](tasks_m08.md) | 8 | 0 |
| | **Total** | | **73** | **0** |

---

## Milestone Dependencies

` + "```" + `
M1 ──┬──► M2 ──► M3 ──┐
     │                  ├──► M5 ──► M6 ──► M7 ──► M8
     └──► M4 ──────────┘
` + "```" + `

**Critical path:** M1 → M2 → M3 → M5 → M6 → M7 → M8

**Convergence point:** M5 requires both M3 (MCP tools for agents to call) and M4 (A2A framework for agents to serve).

### Expanded Dependency Detail

` + "```" + `
M1: Project Scaffolding + A2A Types
│   Produces: go.mod, A2A types, graph schema types, orchestrator config,
│             CLI shell. Foundation for everything.
│
├──► M2: Code Intelligence Layer
│    │   Depends on: M1 (graph schema types)
│    │   Produces: Tree-sitter parser, KuzuDB store, clustering
│    │
│    └──► M3: MCP Tool Servers
│         │   Depends on: M2 (graph Store + Parser to wrap)
│         │   Produces: MCP server exposing code intelligence as tools
│         │
│         └──────────┐
│                     ▼
├──► M4: A2A Agent Framework
│    │   Depends on: M1 (A2A types)
│    │   Produces: HTTP client/server, SSE streaming, agent base, task store
│    │
│    └──────────┐
│               ▼
│         M5: Specialist Agents
│         │   Depends on: M3 (MCP tools to call) + M4 (A2A framework to serve)
│         │   Produces: Research, Schema, Planning, Task Writer agents
│         │
│         └──► M6: Orchestrator Pipeline
│              │   Depends on: M5 (agents to orchestrate)
│              │   Produces: Stage routing, fan-out, merge, coherence, progress
│              │
│              └──► M7: Graceful Degradation + Skill Integration
│                   │   Depends on: M6 (full pipeline to degrade from)
│                   │   Produces: Capability detection, fallback paths, skill shim
│                   │
│                   └──► M8: Testing + Release
│                        Depends on: M7 (all code paths complete)
│                        Produces: E2E tests, goreleaser, CI, docs
` + "```" + `

## Milestone Summaries

### M1: Project Scaffolding + A2A Types (9 tasks)
### M2: Code Intelligence Layer (12 tasks)
### M3: MCP Tool Servers (6 tasks)
### M4: A2A Agent Framework (9 tasks)
### M5: Specialist Agents (9 tasks)
### M6: Orchestrator Pipeline (10 tasks)
### M7: Graceful Degradation + Skill Integration (10 tasks)
### M8: Testing + Release (8 tasks)
`

func TestParseMilestones_AgentParallelPlan(t *testing.T) {
	milestones, err := ParseMilestones(agentParallelStage3)
	require.NoError(t, err)
	require.Len(t, milestones, 8)

	deps := make(map[string][]string, len(milestones))
	for i, m := range milestones {
		assert.Equal(t, fmt.Sprintf("M%d", i+1), m.ID)
		deps[m.ID] = m.DependsOn
	}

	assert.Empty(t, deps["M1"])
	assert.Equal(t, []string{"M1"}, deps["M2"])
	assert.Equal(t, []string{"M2"}, deps["M3"])
	// M4's edges (M1 to M4, M4 to M5) come from the "Depends on:" lines; the
	// multi-line branch art that also shows them is not read.
	assert.Equal(t, []string{"M1"}, deps["M4"])
	assert.ElementsMatch(t, []string{"M3", "M4"}, deps["M5"])
	assert.Equal(t, []string{"M5"}, deps["M6"])
	assert.Equal(t, []string{"M6"}, deps["M7"])
	assert.Equal(t, []string{"M7"}, deps["M8"])
}

// TestParseMilestones_FanIn checks that every milestone on the left of an
// arrow becomes a dependency, as in "M3, M4 -> M5".
func TestParseMilestones_FanIn(t *testing.T) {
	cases := map[string]string{
		"comma":       "M1 -> M2\nM1 -> M3\nM2, M3 -> M4",
		"plus":        "M1 -> M2\nM1 -> M3\nM2 + M3 -> M4",
		"box arrow":   "M1 ──► M2\nM1 ──► M3\nM2 + M3 ──► M4",
		"parenthesis": "M1 → M2+M3 (parallel) → M4",
	}
	for name, arrows := range cases {
		t.Run(name, func(t *testing.T) {
			content := "## M1: A\n## M2: B\n## M3: C\n## M4: D\n\n" + arrows + "\n"
			milestones, err := ParseMilestones(content)
			require.NoError(t, err)
			require.Len(t, milestones, 4)
			assert.Equal(t, []string{"M1"}, milestones[1].DependsOn)
			assert.Equal(t, []string{"M1"}, milestones[2].DependsOn)
			assert.ElementsMatch(t, []string{"M2", "M3"}, milestones[3].DependsOn)
		})
	}
}

// TestParseMilestones_ExampleExcerpt uses the dependency lines of
// examples/stage-3-excerpt.md. Every milestone after M1 must wait on
// something, or it would be scheduled alongside M1.
func TestParseMilestones_ExampleExcerpt(t *testing.T) {
	content := "| M1 | A | tasks_m01.md | 1 | 1 |\n| M2 | B | tasks_m02.md | 1 | 1 |\n| M3 | C | tasks_m03.md | 1 | 1 |\n" +
		"| M4 | D | tasks_m04.md | 1 | 1 |\n| M5 | E | tasks_m05.md | 1 | 1 |\n| M6 | F | tasks_m06.md | 1 | 1 |\n" +
		"| M7 | G | tasks_m07.md | 1 | 1 |\n| M8 | H | tasks_m08.md | 1 | 1 |\n| M9 | I | tasks_m09.md | 1 | 1 |\n\n" +
		"M1 ──► M2 ──┬──► M3 ─────────┐\n" +
		"             ├──► M4 (parallel with M3) ──┐\n" +
		"                      M1 + M2 + M4 ──► M8 ──► M9\n\n" +
		"**Strict sequential path:** M1 → M2 → M3+M4 (parallel) → M5 → M6 → M7 → M8 → M9\n"
	milestones, err := ParseMilestones(content)
	require.NoError(t, err)
	require.Len(t, milestones, 9)

	deps := make(map[string][]string, len(milestones))
	for _, m := range milestones {
		deps[m.ID] = m.DependsOn
	}
	assert.Empty(t, deps["M1"])
	assert.Equal(t, []string{"M1"}, deps["M2"])
	assert.Equal(t, []string{"M2"}, deps["M3"])
	assert.Equal(t, []string{"M2"}, deps["M4"])
	assert.ElementsMatch(t, []string{"M3", "M4"}, deps["M5"])
	assert.Equal(t, []string{"M5"}, deps["M6"])
	assert.Equal(t, []string{"M6"}, deps["M7"])
	assert.ElementsMatch(t, []string{"M1", "M2", "M4", "M7"}, deps["M8"])
	assert.Equal(t, []string{"M8"}, deps["M9"])
}

// TestParseMilestones_DependsOnLines checks "Depends on:" lines under a
// milestone, and that such a line before any milestone is ignored.
func TestParseMilestones_DependsOnLines(t *testing.T) {
	// The prose line under M2 has no colon after "depends on", so it adds
	// nothing.
	content := "Depends on: M1\n\n## M1: A\n\n## M2: B\nDepends on: M1 (types)\nUnlike M2, M4 depends on M3.\n\n## M3: C\n- **Depends on:** M1 + M2, M3\n"
	milestones, err := ParseMilestones(content)
	require.NoError(t, err)
	require.Len(t, milestones, 3)
	assert.Empty(t, milestones[0].DependsOn)
	assert.Equal(t, []string{"M1"}, milestones[1].DependsOn)
	assert.Equal(t, []string{"M1", "M2"}, milestones[2].DependsOn)
}

func TestParseMilestones_ChainOnOneLine(t *testing.T) {
	stage3 := `## M1: Foundation
## M2: Core
## M3: Integration
## M4: Release

**Critical path:** M1 → M2 -> M3 --> M4`

	milestones, err := ParseMilestones(stage3)
	require.NoError(t, err)
	require.Len(t, milestones, 4)

	assert.Empty(t, milestones[0].DependsOn)
	assert.Equal(t, []string{"M1"}, milestones[1].DependsOn)
	assert.Equal(t, []string{"M2"}, milestones[2].DependsOn)
	assert.Equal(t, []string{"M3"}, milestones[3].DependsOn)
}

func TestParseMilestones_BoxDrawingArrows(t *testing.T) {
	stage3 := `## M1: Foundation
## M2: Path A
## M3: Path B
## M4: Merge
## M5: Release

M1 ──┬──► M2 ──► M3 ──┐
     │                  ├──► M5
     └──► M4 ──────────┘`

	milestones, err := ParseMilestones(stage3)
	require.NoError(t, err)
	require.Len(t, milestones, 5)

	assert.Empty(t, milestones[0].DependsOn)
	assert.Equal(t, []string{"M1"}, milestones[1].DependsOn)
	assert.Equal(t, []string{"M2"}, milestones[2].DependsOn)
	// Arrows that leave the line (M1 to M4, M3 and M4 to M5) are not read.
	assert.Empty(t, milestones[3].DependsOn)
	assert.Empty(t, milestones[4].DependsOn)
}

// TestParseMilestones_SkillTemplate uses the Progress table and mermaid block
// copied from the skill's Stage 3 template, which has no milestone headings.
func TestParseMilestones_SkillTemplate(t *testing.T) {
	stage3 := `## Progress

| # | Milestone | File | Tasks | Done |
|---|-----------|------|:-----:|:----:|
| M1 | [Name] | [tasks_m01.md](tasks_m01.md) | | 0 |
| M2 | [Name] | [tasks_m02.md](tasks_m02.md) | | 0 |
| M3 | [Name] | [tasks_m03.md](tasks_m03.md) | | 0 |
| | **Total** | | **0** | **0** |

## Milestone Dependencies

` + "```mermaid" + `
graph LR
    M1 --> M2
    M2 --> M3
    M2 --> M4
` + "```" + `
`

	milestones, err := ParseMilestones(stage3)
	require.NoError(t, err)
	require.Len(t, milestones, 3)

	assert.Equal(t, "M1", milestones[0].ID)
	assert.Equal(t, "[Name]", milestones[0].Name)
	assert.Empty(t, milestones[0].DependsOn)

	assert.Equal(t, "M2", milestones[1].ID)
	assert.Equal(t, []string{"M1"}, milestones[1].DependsOn)

	assert.Equal(t, "M3", milestones[2].ID)
	assert.Equal(t, []string{"M2"}, milestones[2].DependsOn)
}

func TestParseMilestones_NoDependencies(t *testing.T) {
	stage3 := `## M1: Foundation
## M2: Core
## M3: Integration`

	milestones, err := ParseMilestones(stage3)
	require.ErrorIs(t, err, ErrNoMilestoneDependencies)
	assert.Contains(t, err.Error(), "no milestone dependencies")
	// The milestones still come back so callers can list them.
	require.Len(t, milestones, 3)
	assert.Equal(t, "M1", milestones[0].ID)
}

func TestParseMilestones_SingleMilestoneNeedsNoDependencies(t *testing.T) {
	milestones, err := ParseMilestones("## M1: Foundation")
	require.NoError(t, err)
	require.Len(t, milestones, 1)
}
