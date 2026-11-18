package review

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/onedusk/pd/internal/graph"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDirectionSemantics is the GATE test for Check 3. It validates that
// DirectionUpstream returns dependents (files that import the target),
// not dependencies (files the target imports).
//
// This must pass before Check 3 (check_deps.go) can be trusted.
//
// The memstore.go implementation:
//   - DirectionDownstream: matches SourceID == id, returns TargetID (outgoing edges = dependencies)
//   - DirectionUpstream: matches TargetID == id, returns SourceID (incoming edges = dependents)
//
// For an IMPORTS edge Source=A, Target=B meaning "A imports B":
//   - DirectionDownstream from A returns B (what A imports)
//   - DirectionUpstream from B returns A (who imports B)
func TestDirectionSemantics(t *testing.T) {
	ctx := context.Background()
	store := graph.NewMemStore()

	// Set up: A imports B (A depends on B).
	// Edge: SourceID="A", TargetID="B" meaning "A imports B".
	require.NoError(t, store.AddFile(ctx, graph.FileNode{Path: "A", Language: graph.LangGo}))
	require.NoError(t, store.AddFile(ctx, graph.FileNode{Path: "B", Language: graph.LangGo}))
	require.NoError(t, store.AddEdge(ctx, graph.Edge{
		SourceID: "A",
		TargetID: "B",
		Kind:     graph.EdgeKindImports,
	}))

	// DirectionDownstream from A should return B (what A imports = A's dependencies).
	downFromA, err := store.GetDependencies(ctx, "A", graph.DirectionDownstream, 1)
	require.NoError(t, err)
	require.Len(t, downFromA, 1, "DirectionDownstream from A should find B")
	assert.Equal(t, "B", downFromA[0].Nodes[len(downFromA[0].Nodes)-1],
		"DirectionDownstream from A should return B (A's dependency)")

	// DirectionUpstream from B should return A (who imports B = B's dependents).
	upFromB, err := store.GetDependencies(ctx, "B", graph.DirectionUpstream, 1)
	require.NoError(t, err)
	require.Len(t, upFromB, 1, "DirectionUpstream from B should find A")
	assert.Equal(t, "A", upFromB[0].Nodes[len(upFromB[0].Nodes)-1],
		"DirectionUpstream from B should return A (B's dependent)")

	// DirectionDownstream from B should return nothing (B imports nothing).
	downFromB, err := store.GetDependencies(ctx, "B", graph.DirectionDownstream, 1)
	require.NoError(t, err)
	assert.Empty(t, downFromB, "DirectionDownstream from B should be empty (B imports nothing)")

	// DirectionUpstream from A should return nothing (nobody imports A).
	upFromA, err := store.GetDependencies(ctx, "A", graph.DirectionUpstream, 1)
	require.NoError(t, err)
	assert.Empty(t, upFromA, "DirectionUpstream from A should be empty (nobody imports A)")
}

func TestParseDirectoryTree(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected []FileEntry
		wantErr  bool
	}{
		{
			name: "tree-drawing characters",
			content: `## Target Directory Tree

` + "```" + `
project/
├── go.mod                    CREATE (M1)
├── cmd/
│   └── main.go               CREATE (M1), MODIFY (M6)
└── internal/
    └── foo.go                 MODIFY (M2)
` + "```",
			expected: []FileEntry{
				{Path: "go.mod", Actions: map[string]string{"M1": "CREATE"}, Milestones: []string{"M1"}},
				{Path: "cmd/main.go", Actions: map[string]string{"M1": "CREATE", "M6": "MODIFY"}, Milestones: []string{"M1", "M6"}},
				{Path: "internal/foo.go", Actions: map[string]string{"M2": "MODIFY"}, Milestones: []string{"M2"}},
			},
		},
		{
			name: "plain indentation",
			content: `## Target Directory Tree

` + "```" + `
src/
  models/
    user.go                    CREATE (M1)
  api/
    handler.go                 MODIFY (M3)
` + "```",
			expected: []FileEntry{
				{Path: "models/user.go", Actions: map[string]string{"M1": "CREATE"}, Milestones: []string{"M1"}},
				{Path: "api/handler.go", Actions: map[string]string{"M3": "MODIFY"}, Milestones: []string{"M3"}},
			},
		},
		{
			name: "delete action",
			content: `## File Tree

` + "```" + `
old_file.go                    DELETE (M4)
` + "```",
			expected: []FileEntry{
				{Path: "old_file.go", Actions: map[string]string{"M4": "DELETE"}, Milestones: []string{"M4"}},
			},
		},
		{
			name: "extensionless files without actions",
			content: `## Target Directory Tree

` + "```" + `
project/
├── Makefile
├── LICENSE                   CREATE (M1)
├── scripts
│   └── build.sh              CREATE (M2)
└── ...
` + "```",
			expected: []FileEntry{
				{Path: "Makefile", Actions: map[string]string{}},
				{Path: "LICENSE", Actions: map[string]string{"M1": "CREATE"}, Milestones: []string{"M1"}},
				{Path: "scripts/build.sh", Actions: map[string]string{"M2": "CREATE"}, Milestones: []string{"M2"}},
			},
		},
		{
			name:    "no directory tree section",
			content: "# Stage 3\n\nSome content without a directory tree.",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entries, err := ParseDirectoryTree(tt.content)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Len(t, entries, len(tt.expected))
			for i, exp := range tt.expected {
				assert.Equal(t, exp.Path, entries[i].Path, "path mismatch at index %d", i)
				assert.Equal(t, exp.Actions, entries[i].Actions, "actions mismatch for %s", exp.Path)
				assert.Equal(t, exp.Milestones, entries[i].Milestones, "milestones mismatch for %s", exp.Path)
			}
		})
	}
}

func TestParseTaskSpecs(t *testing.T) {
	content := `# Stage 4: Task Specifications — Milestone 1: Foundation

- [ ] **T-01.01 — Initialize project**
  - **File:** ` + "`go.mod`" + ` (CREATE)
  - **Depends on:** None
  - **Outline:**
    - Run ` + "`go mod init`" + `
    - Add ` + "`testify`" + ` dependency
  - **Acceptance:** Project compiles.

- [ ] **T-01.02 — Add core types**
  - **File:** ` + "`internal/types.go`" + ` (CREATE)
  - **Depends on:** T-01.01
  - **Outline:**
    - Define ` + "`TaskState`" + ` enum with ` + "`IsTerminal`" + ` method
    - Define ` + "`AgentCard`" + ` struct
  - **Acceptance:** Types compile and are importable.

- [ ] **T-01.03 — Update handler**
  - **File:** ` + "`internal/handler.go`" + ` (MODIFY)
  - **Depends on:** T-01.01, T-01.02
  - **Outline:**
    - Add ` + "`HandleRequest`" + ` method to ` + "`ServerHandler`" + `
    - Use ` + "`TaskState`" + ` from types package
  - **Acceptance:** Handler processes requests.
`

	taskFiles := map[string]string{"M1": content}
	tasks, err := ParseTaskSpecs(taskFiles)
	require.NoError(t, err)
	require.Len(t, tasks, 3)

	// Task 1.
	assert.Equal(t, "T-01.01", tasks[0].ID)
	assert.Equal(t, "M1", tasks[0].Milestone)
	assert.Equal(t, "go.mod", tasks[0].File)
	assert.Equal(t, "CREATE", tasks[0].Action)
	assert.Empty(t, tasks[0].DependsOn)

	// Task 2.
	assert.Equal(t, "T-01.02", tasks[1].ID)
	assert.Equal(t, "internal/types.go", tasks[1].File)
	assert.Equal(t, "CREATE", tasks[1].Action)
	assert.Equal(t, []string{"T-01.01"}, tasks[1].DependsOn)
	assert.Contains(t, tasks[1].SymbolRefs, "TaskState")
	assert.Contains(t, tasks[1].SymbolRefs, "IsTerminal")
	assert.Contains(t, tasks[1].SymbolRefs, "AgentCard")

	// Task 3.
	assert.Equal(t, "T-01.03", tasks[2].ID)
	assert.Equal(t, "MODIFY", tasks[2].Action)
	assert.Equal(t, []string{"T-01.01", "T-01.02"}, tasks[2].DependsOn)
	assert.Contains(t, tasks[2].SymbolRefs, "HandleRequest")
	assert.Contains(t, tasks[2].SymbolRefs, "ServerHandler")
	assert.Contains(t, tasks[2].SymbolRefs, "TaskState")
}

func TestRunReview_WarnsOnTaskFileWithNoTasks(t *testing.T) {
	root := t.TempDir()
	decompDir := filepath.Join(root, "docs", "decompose", "demo")
	require.NoError(t, os.MkdirAll(decompDir, 0o755))

	stage3 := "## Target Directory Tree\n\n```\nmain.go    CREATE (M1)\n```\n"
	good := "- [ ] **T-01.01 — Add main**\n  - **File:** `main.go` (CREATE)\n  - **Depends on:** None\n"
	drifted := "### T-02.01: Heading format the parser does not recognize\n- **File:** `util.go` (CREATE)\n"
	require.NoError(t, os.WriteFile(filepath.Join(decompDir, "stage-3-task-index.md"), []byte(stage3), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(decompDir, "tasks_m01.md"), []byte(good), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(decompDir, "tasks_m02.md"), []byte(drifted), 0o644))

	report, err := RunReview(context.Background(), ReviewConfig{
		ProjectRoot: root,
		DecompName:  "demo",
		DecompDir:   decompDir,
	})
	require.NoError(t, err)

	require.Len(t, report.Warnings, 1)
	assert.Contains(t, report.Warnings[0], "tasks_m02.md")
	assert.Contains(t, report.Markdown(), "tasks_m02.md")
}

func TestExtractSymbolRefs(t *testing.T) {
	outline := "Define `ParseConfig` struct. Use `HandleRequest` method. Apply PascalCase naming for AgentCard types."
	refs := ExtractSymbolRefs(outline)

	// Only backtick-quoted names are symbol references.
	assert.Equal(t, []string{"ParseConfig", "HandleRequest"}, refs)

	// CamelCase words in prose are not symbol references.
	outline2 := "Port the TypeScript client and open a GitHub issue. Before doing this, check that None of the values match."
	assert.Empty(t, ExtractSymbolRefs(outline2))
}

// limitedGraph is a GraphProvider whose QuerySymbols returns substring matches
// in insertion order and stops at limit, like a store with a row cap.
type limitedGraph struct {
	GraphProvider
	symbols []graph.SymbolNode
}

func (g *limitedGraph) Available() bool { return true }

func (g *limitedGraph) QuerySymbols(_ context.Context, query string, limit int) ([]graph.SymbolNode, error) {
	var out []graph.SymbolNode
	for _, sym := range g.symbols {
		if !strings.Contains(sym.Name, query) {
			continue
		}
		out = append(out, sym)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}

// TestCheckSymbols_ExactNameAmongManySubstringMatches covers a reference to
// exactly "Store" in a codebase with more than 20 symbols containing "Store".
func TestCheckSymbols_ExactNameAmongManySubstringMatches(t *testing.T) {
	g := &limitedGraph{}
	for i := 0; i < 25; i++ {
		g.symbols = append(g.symbols, graph.SymbolNode{
			Name: fmt.Sprintf("Store%02d", i), Kind: graph.SymbolKindType, FilePath: "other.go",
		})
	}
	g.symbols = append(g.symbols, graph.SymbolNode{Name: "Store", Kind: graph.SymbolKindType, FilePath: "store.go"})

	tasks := []TaskEntry{{
		ID: "T-01.01", Milestone: "M1", File: "store.go", Action: "MODIFY",
		SymbolRefs: []string{"Store"},
	}}

	findings := CheckSymbols(context.Background(), ReviewConfig{Graph: g}, tasks)
	assert.Empty(t, findings)
}

// TestCheckSymbols_RelocatedSymbolIsStable checks that a symbol found only in
// other files is reported against the same file on every run.
func TestCheckSymbols_RelocatedSymbolIsStable(t *testing.T) {
	g := &limitedGraph{symbols: []graph.SymbolNode{
		{Name: "Store", Kind: graph.SymbolKindType, FilePath: "z.go"},
		{Name: "Store", Kind: graph.SymbolKindType, FilePath: "a.go"},
	}}

	tasks := []TaskEntry{{
		ID: "T-01.01", Milestone: "M1", File: "store.go", Action: "MODIFY",
		SymbolRefs: []string{"Store"},
	}}

	findings := CheckSymbols(context.Background(), ReviewConfig{Graph: g}, tasks)
	require.Len(t, findings, 1)
	assert.Equal(t, ClassStale, findings[0].Classification)
	assert.Contains(t, findings[0].Description, "exists in `a.go`")
}

func TestCheckFileExistence(t *testing.T) {
	ctx := context.Background()

	// Create temp project with some files.
	tmpDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "internal"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "existing.go"), []byte("package main"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "internal", "handler.go"), []byte("package internal"), 0o644))

	cfg := ReviewConfig{ProjectRoot: tmpDir}

	entries := []FileEntry{
		// CREATE but file exists → MISMATCH.
		{Path: "existing.go", Actions: map[string]string{"M1": "CREATE"}, Milestones: []string{"M1"}},
		// MODIFY but file missing → MISMATCH.
		{Path: "missing.go", Actions: map[string]string{"M2": "MODIFY"}, Milestones: []string{"M2"}},
		// DELETE but file missing → STALE.
		{Path: "deleted.go", Actions: map[string]string{"M3": "DELETE"}, Milestones: []string{"M3"}},
		// MODIFY and file exists → OK.
		{Path: "internal/handler.go", Actions: map[string]string{"M1": "MODIFY"}, Milestones: []string{"M1"}},
		// CREATE and file does not exist → OK.
		{Path: "new_file.go", Actions: map[string]string{"M1": "CREATE"}, Milestones: []string{"M1"}},
	}

	findings := CheckFileExistence(ctx, cfg, entries)

	// Should have exactly 3 findings (the 3 problems above).
	require.Len(t, findings, 3)
	assert.Equal(t, ClassMismatch, findings[0].Classification)
	assert.Equal(t, "existing.go", findings[0].FilePath)
	assert.Equal(t, ClassMismatch, findings[1].Classification)
	assert.Equal(t, "missing.go", findings[1].FilePath)
	assert.Equal(t, ClassStale, findings[2].Classification)
	assert.Equal(t, "deleted.go", findings[2].FilePath)
}

// TestCheckFileExistence_CreateThenModify covers a file that one milestone
// creates and a later one modifies. Only the CREATE can be checked against the
// filesystem: the MODIFY runs after the file has been created.
func TestCheckFileExistence_CreateThenModify(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	cfg := ReviewConfig{ProjectRoot: tmpDir}

	entries := []FileEntry{
		{Path: "planned.go", Actions: map[string]string{"M1": "CREATE", "M4": "MODIFY"}, Milestones: []string{"M1", "M4"}},
	}

	// File not created yet: the plan is consistent.
	assert.Empty(t, CheckFileExistence(ctx, cfg, entries))

	// File already exists: only the CREATE is reported.
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "planned.go"), []byte("package main"), 0o644))
	findings := CheckFileExistence(ctx, cfg, entries)
	require.Len(t, findings, 1)
	assert.Equal(t, ClassMismatch, findings[0].Classification)
	assert.Equal(t, "M1", findings[0].Milestone)
}

// TestCheckSymbols_BareMethodNameWithSameNamedMethods indexes a file with
// three methods of the same name on different receivers. All three must be in
// the graph, and a task that names the bare method must still pass Check 2.
func TestCheckSymbols_BareMethodNameWithSameNamedMethods(t *testing.T) {
	ctx := context.Background()
	source := `package orchestrator

type CLI struct{}
type PR struct{}
type File struct{}

func (c *CLI) RequestReview() error { return nil }

func (p *PR) RequestReview() error { return nil }

func (f *File) RequestReview() error { return nil }
`
	result, err := graph.NewTreeSitterParser().Parse(ctx, "review.go", []byte(source), graph.LangGo)
	require.NoError(t, err)

	store := graph.NewMemStore()
	require.NoError(t, store.AddFile(ctx, result.File))
	for _, sym := range result.Symbols {
		require.NoError(t, store.AddSymbol(ctx, sym))
	}

	methods, err := store.QuerySymbols(ctx, "RequestReview", 10)
	require.NoError(t, err)
	assert.Len(t, methods, 3)

	tasks := []TaskEntry{{
		ID: "T-01.01", Milestone: "M1", File: "review.go", Action: "MODIFY",
		SymbolRefs: []string{"RequestReview"},
	}}
	findings := CheckSymbols(ctx, ReviewConfig{Graph: NewStoreGraphProvider(store)}, tasks)
	assert.Empty(t, findings)
}

func TestCheckCrossMilestoneConsistency(t *testing.T) {
	ctx := context.Background()
	cfg := ReviewConfig{ProjectRoot: t.TempDir()}

	// Stage 3 content with milestone definitions.
	stage3Content := `## Milestone Dependencies

M1 → M2 → M3

## M1: Foundation
## M2: Features
## M3: Polish
`

	entries := []FileEntry{
		// MODIFY in M1 before CREATE in M2 → MISMATCH.
		{
			Path:       "src/new_file.go",
			Actions:    map[string]string{"M1": "MODIFY", "M2": "CREATE"},
			Milestones: []string{"M1", "M2"},
		},
		// CREATE in M1, MODIFY in M3 → OK.
		{
			Path:       "src/ok_file.go",
			Actions:    map[string]string{"M1": "CREATE", "M3": "MODIFY"},
			Milestones: []string{"M1", "M3"},
		},
	}

	findings := CheckCrossMilestoneConsistency(ctx, cfg, entries, nil, stage3Content)

	// Should find MODIFY-before-CREATE issue.
	found := false
	for _, f := range findings {
		if f.FilePath == "src/new_file.go" && f.Classification == ClassMismatch {
			found = true
			break
		}
	}
	assert.True(t, found, "should detect MODIFY before CREATE for src/new_file.go")
}

// TestBuildMilestoneOrder_SkillTemplate uses the Progress table and mermaid
// block from the skill's Stage 3 template, which has no milestone headings.
func TestBuildMilestoneOrder_SkillTemplate(t *testing.T) {
	stage3Content := `## Progress

| # | Milestone | File | Tasks | Done |
|---|-----------|------|:-----:|:----:|
| M1 | [Name] | [tasks_m01.md](tasks_m01.md) | | 0 |
| M2 | [Name] | [tasks_m02.md](tasks_m02.md) | | 0 |
| M3 | [Name] | [tasks_m03.md](tasks_m03.md) | | 0 |

## Milestone Dependencies

` + "```mermaid" + `
graph LR
    M1 --> M2
    M2 --> M3
` + "```" + `
`

	assert.Equal(t, map[string]int{"M1": 0, "M2": 1, "M3": 2}, buildMilestoneOrder(stage3Content))
}

// TestBuildMilestoneOrder_NoDependencies checks that a plan with no parseable
// dependency arrows still gets the listed milestone order, so Check 4 does not
// compare milestones in a random order.
func TestBuildMilestoneOrder_NoDependencies(t *testing.T) {
	stage3Content := `## M1: Foundation
## M2: Features
## M3: Polish
`

	assert.Equal(t, map[string]int{"M1": 0, "M2": 1, "M3": 2}, buildMilestoneOrder(stage3Content))
}

func TestMilestoneFromFilename(t *testing.T) {
	assert.Equal(t, "M1", milestoneFromFilename("tasks_m01.md"))
	assert.Equal(t, "M12", milestoneFromFilename("tasks_m12.md"))
	assert.Equal(t, "M1", milestoneFromFilename("tasks_m1.md"))
}

func TestFindingString(t *testing.T) {
	f := ReviewFinding{
		ID:             "R-1.03",
		Classification: ClassMismatch,
		FilePath:       "internal/handler.go",
		Description:    "File already exists but plan specifies CREATE",
		Suggestion:     "Change action to MODIFY",
	}

	s := f.String()
	assert.Contains(t, s, "R-1.03")
	assert.Contains(t, s, "[MISMATCH]")
	assert.Contains(t, s, "`internal/handler.go`")
	assert.Contains(t, s, "File already exists")
	assert.Contains(t, s, "Change action to MODIFY")
}
