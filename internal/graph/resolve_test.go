package graph

import (
	"testing"
)

// --- TypeScript: relative imports ---

func TestResolveTS_Relative(t *testing.T) {
	r := NewResolver("/tmp/fake", []string{
		"src/index.ts",
		"src/service.ts",
		"src/types.ts",
	})

	tests := []struct {
		name       string
		importPath string
		sourceFile string
		want       string
		wantOK     bool
	}{
		{"dot-slash exact", "./service", "src/index.ts", "src/service.ts", true},
		{"dot-slash with extension probe", "./types", "src/index.ts", "src/types.ts", true},
		{"not found", "./nonexistent", "src/index.ts", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			edge := Edge{SourceID: tt.sourceFile, TargetID: tt.importPath, Kind: EdgeKindImports}
			got, ok := r.ResolveEdge(edge, LangTypeScript)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if ok && got.TargetID != tt.want {
				t.Errorf("TargetID = %q, want %q", got.TargetID, tt.want)
			}
		})
	}
}

func TestResolveTS_RelativeParent(t *testing.T) {
	r := NewResolver("/tmp/fake", []string{
		"src/types.ts",
		"src/sub/handler.ts",
	})

	edge := Edge{SourceID: "src/sub/handler.ts", TargetID: "../types", Kind: EdgeKindImports}
	got, ok := r.ResolveEdge(edge, LangTypeScript)
	if !ok {
		t.Fatal("expected resolution to succeed")
	}
	if got.TargetID != "src/types.ts" {
		t.Errorf("TargetID = %q, want %q", got.TargetID, "src/types.ts")
	}
}

func TestResolveTS_IndexFile(t *testing.T) {
	r := NewResolver("/tmp/fake", []string{
		"src/app.ts",
		"src/components/index.ts",
	})

	edge := Edge{SourceID: "src/app.ts", TargetID: "./components", Kind: EdgeKindImports}
	got, ok := r.ResolveEdge(edge, LangTypeScript)
	if !ok {
		t.Fatal("expected resolution to succeed")
	}
	if got.TargetID != "src/components/index.ts" {
		t.Errorf("TargetID = %q, want %q", got.TargetID, "src/components/index.ts")
	}
}

// --- TypeScript: workspace resolution ---

func TestResolveTS_WorkspaceDefault(t *testing.T) {
	fixtureRoot := "../../testdata/fixtures/ts_monorepo"

	knownFiles := []string{
		"packages/logger/src/index.ts",
		"packages/db/src/index.ts",
		"packages/db/src/queries.ts",
		"src/app.ts",
		"src/utils.ts",
	}

	r := NewResolver(fixtureRoot, knownFiles)

	edge := Edge{SourceID: "src/app.ts", TargetID: "@test/logger", Kind: EdgeKindImports}
	got, ok := r.ResolveEdge(edge, LangTypeScript)
	if !ok {
		t.Fatalf("expected @test/logger to resolve; workspaces found: %d", len(r.tsWorkspaces))
	}
	if got.TargetID != "packages/logger/src/index.ts" {
		t.Errorf("TargetID = %q, want %q", got.TargetID, "packages/logger/src/index.ts")
	}
}

func TestResolveTS_WorkspaceSubpath(t *testing.T) {
	fixtureRoot := "../../testdata/fixtures/ts_monorepo"

	knownFiles := []string{
		"packages/logger/src/index.ts",
		"packages/db/src/index.ts",
		"packages/db/src/queries.ts",
		"src/app.ts",
		"src/utils.ts",
	}

	r := NewResolver(fixtureRoot, knownFiles)

	edge := Edge{SourceID: "src/app.ts", TargetID: "@test/db/queries", Kind: EdgeKindImports}
	got, ok := r.ResolveEdge(edge, LangTypeScript)
	if !ok {
		t.Fatal("expected @test/db/queries to resolve")
	}
	if got.TargetID != "packages/db/src/queries.ts" {
		t.Errorf("TargetID = %q, want %q", got.TargetID, "packages/db/src/queries.ts")
	}
}

func TestResolveTS_ExternalPackage(t *testing.T) {
	r := NewResolver("/tmp/fake", []string{"src/app.ts"})

	edge := Edge{SourceID: "src/app.ts", TargetID: "lodash", Kind: EdgeKindImports}
	_, ok := r.ResolveEdge(edge, LangTypeScript)
	if ok {
		t.Fatal("expected external package to be unresolvable")
	}
}

// --- Go resolution ---

func TestResolveGo_LocalModule(t *testing.T) {
	r := NewResolver("/tmp/fake", []string{
		"internal/graph/schema.go",
		"internal/graph/store.go",
		"cmd/main.go",
	})
	r.goModPath = "github.com/example/project"

	edge := Edge{
		SourceID: "cmd/main.go",
		TargetID: "github.com/example/project/internal/graph",
		Kind:     EdgeKindImports,
	}
	got, ok := r.ResolveEdge(edge, LangGo)
	if !ok {
		t.Fatal("expected local module import to resolve")
	}
	if got.TargetID != "internal/graph/schema.go" {
		t.Errorf("TargetID = %q, want %q", got.TargetID, "internal/graph/schema.go")
	}
}

func TestResolveGo_StdLib(t *testing.T) {
	r := NewResolver("/tmp/fake", []string{"main.go"})
	r.goModPath = "github.com/example/project"

	edge := Edge{SourceID: "main.go", TargetID: "fmt", Kind: EdgeKindImports}
	_, ok := r.ResolveEdge(edge, LangGo)
	if ok {
		t.Fatal("expected stdlib import to be unresolvable")
	}
}

func TestResolveGo_External(t *testing.T) {
	r := NewResolver("/tmp/fake", []string{"main.go"})
	r.goModPath = "github.com/example/project"

	edge := Edge{SourceID: "main.go", TargetID: "github.com/other/lib", Kind: EdgeKindImports}
	_, ok := r.ResolveEdge(edge, LangGo)
	if ok {
		t.Fatal("expected external module import to be unresolvable")
	}
}

// --- Python resolution ---

func TestResolvePython_Relative(t *testing.T) {
	r := NewResolver("/tmp/fake", []string{
		"pkg/service.py",
		"pkg/models.py",
	})

	edge := Edge{SourceID: "pkg/service.py", TargetID: ".models", Kind: EdgeKindImports}
	got, ok := r.ResolveEdge(edge, LangPython)
	if !ok {
		t.Fatal("expected .models to resolve")
	}
	if got.TargetID != "pkg/models.py" {
		t.Errorf("TargetID = %q, want %q", got.TargetID, "pkg/models.py")
	}
}

func TestResolvePython_ParentRelative(t *testing.T) {
	r := NewResolver("/tmp/fake", []string{
		"pkg/sub/handler.py",
		"pkg/models.py",
	})

	edge := Edge{SourceID: "pkg/sub/handler.py", TargetID: "..models", Kind: EdgeKindImports}
	got, ok := r.ResolveEdge(edge, LangPython)
	if !ok {
		t.Fatal("expected ..models to resolve")
	}
	if got.TargetID != "pkg/models.py" {
		t.Errorf("TargetID = %q, want %q", got.TargetID, "pkg/models.py")
	}
}

func TestResolvePython_External(t *testing.T) {
	r := NewResolver("/tmp/fake", []string{"main.py"})

	edge := Edge{SourceID: "main.py", TargetID: "numpy", Kind: EdgeKindImports}
	_, ok := r.ResolveEdge(edge, LangPython)
	if ok {
		t.Fatal("expected external package to be unresolvable")
	}
}

// --- Rust resolution ---

func TestResolveRust_Crate(t *testing.T) {
	r := NewResolver("/tmp/fake", []string{
		"src/model.rs",
		"src/service.rs",
	})

	edge := Edge{
		SourceID: "src/service.rs",
		TargetID: "crate::model::{Repository, User}",
		Kind:     EdgeKindImports,
	}
	got, ok := r.ResolveEdge(edge, LangRust)
	if !ok {
		t.Fatal("expected crate::model to resolve")
	}
	if got.TargetID != "src/model.rs" {
		t.Errorf("TargetID = %q, want %q", got.TargetID, "src/model.rs")
	}
}

func TestResolveRust_CrateModDir(t *testing.T) {
	r := NewResolver("/tmp/fake", []string{
		"src/handlers/mod.rs",
		"src/main.rs",
	})

	edge := Edge{
		SourceID: "src/main.rs",
		TargetID: "crate::handlers",
		Kind:     EdgeKindImports,
	}
	got, ok := r.ResolveEdge(edge, LangRust)
	if !ok {
		t.Fatal("expected crate::handlers to resolve to mod.rs")
	}
	if got.TargetID != "src/handlers/mod.rs" {
		t.Errorf("TargetID = %q, want %q", got.TargetID, "src/handlers/mod.rs")
	}
}

func TestResolveRust_ExternalCrate(t *testing.T) {
	r := NewResolver("/tmp/fake", []string{"src/main.rs"})

	edge := Edge{SourceID: "src/main.rs", TargetID: "std::collections::HashMap", Kind: EdgeKindImports}
	_, ok := r.ResolveEdge(edge, LangRust)
	if ok {
		t.Fatal("expected external crate to be unresolvable")
	}
}

// --- ResolveAll ---

func TestResolveAll_PassthroughNonImports(t *testing.T) {
	r := NewResolver("/tmp/fake", []string{"src/index.ts"})

	edges := []Edge{
		{SourceID: "src/index.ts", TargetID: "src/index.ts::main", Kind: EdgeKindDefines},
		{SourceID: "main::init", TargetID: "main::run", Kind: EdgeKindCalls},
	}

	got := r.ResolveAll(edges, LangTypeScript)
	if len(got) != 2 {
		t.Fatalf("expected 2 edges, got %d", len(got))
	}
	if got[0].Kind != EdgeKindDefines || got[1].Kind != EdgeKindCalls {
		t.Error("non-IMPORTS edges should pass through unchanged")
	}
}

func TestResolveAll_DropsUnresolvable(t *testing.T) {
	r := NewResolver("/tmp/fake", []string{
		"src/index.ts",
		"src/service.ts",
	})

	edges := []Edge{
		{SourceID: "src/index.ts", TargetID: "./service", Kind: EdgeKindImports},
		{SourceID: "src/index.ts", TargetID: "lodash", Kind: EdgeKindImports},
		{SourceID: "src/index.ts", TargetID: "src/index.ts::main", Kind: EdgeKindDefines},
	}

	got := r.ResolveAll(edges, LangTypeScript)
	if len(got) != 2 {
		t.Fatalf("expected 2 edges (1 resolved import + 1 defines), got %d", len(got))
	}
	if got[0].TargetID != "src/service.ts" {
		t.Errorf("first edge TargetID = %q, want %q", got[0].TargetID, "src/service.ts")
	}
	if got[1].Kind != EdgeKindDefines {
		t.Error("second edge should be DEFINES passthrough")
	}
}

func TestResolver_NoPackageJSON(t *testing.T) {
	// Should not panic when no package.json or go.mod exists.
	r := NewResolver("/tmp/nonexistent-dir-12345", []string{
		"src/app.ts",
		"src/utils.ts",
	})

	if len(r.tsWorkspaces) != 0 {
		t.Errorf("expected no workspaces, got %d", len(r.tsWorkspaces))
	}
	if r.goModPath != "" {
		t.Errorf("expected empty goModPath, got %q", r.goModPath)
	}

	// Relative imports should still work.
	edge := Edge{SourceID: "src/app.ts", TargetID: "./utils", Kind: EdgeKindImports}
	got, ok := r.ResolveEdge(edge, LangTypeScript)
	if !ok {
		t.Fatal("expected relative import to resolve even without package.json")
	}
	if got.TargetID != "src/utils.ts" {
		t.Errorf("TargetID = %q, want %q", got.TargetID, "src/utils.ts")
	}
}
