# Stage 4: Task Specifications — Milestone 2: Code Intelligence Layer

> Core graph engine: Tree-sitter parsing with per-language extractors for tier-1 languages, KuzuDB-backed graph store behind the `Store` interface, in-memory store for testing, and clustering algorithm.
>
> Fulfills: ADR-003 (temporary graph), ADR-006 (KuzuDB behind interface), ADR-007 (official Tree-sitter bindings)

---

- [ ] **T-02.01 — Implement Tree-sitter parser core**
  - **File:** `internal/graph/treesitter.go` (CREATE)
  - **Depends on:** T-01.07
  - **Outline:**
    - Define `TreeSitterParser` struct implementing `Parser` interface
    - Constructor `NewTreeSitterParser() *TreeSitterParser` — initializes a language registry mapping `Language` → Tree-sitter language pointer
    - `Parse(ctx, path, source, lang)` — creates a `tree_sitter.Parser`, sets language, calls `parser.Parse(source, nil)`, walks the tree via `TreeCursor`, delegates symbol extraction to per-language extractors (T-02.02–T-02.05)
    - `SupportedLanguages()` — returns languages present in the registry
    - `Close()` — releases any held Tree-sitter resources
    - Import grammars: `tree-sitter-go`, `tree-sitter-typescript`, `tree-sitter-python`, `tree-sitter-rust`
    - One parser per `Parse()` call — parsers are NOT thread-safe
    - All `tree_sitter.Parser`, `Tree`, `TreeCursor` must have `defer X.Close()`
    - Define internal `extractor` interface: `Extract(root *tree_sitter.Node, source []byte, filePath string) ([]SymbolNode, []Edge)`
  - **Acceptance:** `NewTreeSitterParser()` returns non-nil. `SupportedLanguages()` returns all 4 tier-1 languages. `Parse` on an empty file returns a `ParseResult` with zero symbols and zero edges (no panic). `Close()` does not error.

---

- [ ] **T-02.02 — Implement Go symbol extractor**
  - **File:** `internal/graph/treesitter_go.go` (CREATE)
  - **Depends on:** T-02.01
  - **Outline:**
    - Define `goExtractor` implementing the `extractor` interface
    - Use Tree-sitter queries to extract:
      - Functions: `(function_declaration name: (identifier) @name)` → `SymbolKindFunction`
      - Methods: `(method_declaration name: (field_identifier) @name)` → `SymbolKindMethod`
      - Types: `(type_declaration (type_spec name: (type_identifier) @name))` → `SymbolKindType`
      - Interfaces: type specs where the body is `interface_type` → `SymbolKindInterface`
      - Structs: type specs where the body is `struct_type` → `SymbolKindType`
    - Extract import edges: `(import_spec path: (interpreted_string_literal) @path)` → `EdgeKindImports` (file-to-file)
    - Detect exported symbols: name starts with uppercase letter
    - Extract call edges: `(call_expression function: (identifier) @callee)` → `EdgeKindCalls` (best-effort, local scope only)
    - Set `StartLine`/`EndLine` from node positions
  - **Acceptance:** Given `testdata/fixtures/go_project/`, extractor finds all exported functions, types, and interfaces. Import edges point to correct package paths. Exported flag is correct for uppercase vs lowercase names.

---

- [ ] **T-02.03 — Implement TypeScript symbol extractor**
  - **File:** `internal/graph/treesitter_ts.go` (CREATE)
  - **Depends on:** T-02.01
  - **Outline:**
    - Define `tsExtractor` implementing the `extractor` interface
    - Extract: `function_declaration`, `class_declaration`, `interface_declaration`, `type_alias_declaration`, `enum_declaration`, `arrow_function` (named via variable_declarator)
    - Extract import edges from `import_statement` → `EdgeKindImports`
    - Detect exports: `export_statement` wrapping a declaration, or `export default`
    - Extract call edges from `call_expression`
  - **Acceptance:** Given `testdata/fixtures/ts_project/`, extractor finds classes, interfaces, type aliases, and enums. Import paths are resolved. Export detection works for named exports and default exports.

---

- [ ] **T-02.04 — Implement Python symbol extractor**
  - **File:** `internal/graph/treesitter_py.go` (CREATE)
  - **Depends on:** T-02.01
  - **Outline:**
    - Define `pyExtractor` implementing the `extractor` interface
    - Extract: `function_definition`, `class_definition`
    - Extract import edges from `import_statement` and `import_from_statement` → `EdgeKindImports`
    - All top-level symbols are considered exported (Python has no export keyword); symbols starting with `_` are marked not exported
    - Extract call edges from `call` nodes
    - Handle decorated functions/classes (`decorated_definition`)
  - **Acceptance:** Given `testdata/fixtures/py_project/`, extractor finds functions and classes. Imports (both `import x` and `from x import y`) produce correct edges. `_private` functions are marked not exported.

---

- [ ] **T-02.05 — Implement Rust symbol extractor**
  - **File:** `internal/graph/treesitter_rs.go` (CREATE)
  - **Depends on:** T-02.01
  - **Outline:**
    - Define `rsExtractor` implementing the `extractor` interface
    - Extract: `function_item`, `struct_item`, `enum_item`, `trait_item`, `impl_item` (methods within), `type_item`
    - Extract import edges from `use_declaration` → `EdgeKindImports`
    - Detect `pub` visibility qualifier for export status
    - Extract call edges from `call_expression`
    - Handle `impl Trait for Type` → `EdgeKindImplements`
  - **Acceptance:** Given `testdata/fixtures/rs_project/`, extractor finds functions, structs, enums, traits. `pub` items are marked exported. `use` statements produce import edges. `impl` blocks produce IMPLEMENTS edges.

---

- [ ] **T-02.06 — Implement KuzuDB graph store**
  - **File:** `internal/graph/kuzustore.go` (CREATE)
  - **Depends on:** T-01.07
  - **Outline:**
    - Define `KuzuStore` struct implementing `Store` interface
    - Constructor `NewKuzuStore() (*KuzuStore, error)` — opens in-memory KuzuDB via `kuzu.OpenInMemoryDatabase(kuzu.DefaultSystemConfig())`
    - `InitSchema(ctx)` — executes Cypher DDL:
      - `CREATE NODE TABLE File(path STRING PRIMARY KEY, language STRING, loc INT64)`
      - `CREATE NODE TABLE Symbol(id STRING PRIMARY KEY, name STRING, kind STRING, exported BOOLEAN, file_path STRING, start_line INT64, end_line INT64)`
      - `CREATE NODE TABLE Cluster(name STRING PRIMARY KEY, cohesion_score DOUBLE, members STRING[])` (members as string list)
      - `CREATE REL TABLE DEFINES(FROM File TO Symbol)`
      - `CREATE REL TABLE IMPORTS(FROM File TO File)`
      - `CREATE REL TABLE CALLS(FROM Symbol TO Symbol)`
      - `CREATE REL TABLE INHERITS(FROM Symbol TO Symbol)`
      - `CREATE REL TABLE IMPLEMENTS(FROM Symbol TO Symbol)`
      - `CREATE REL TABLE BELONGS(FROM File TO Cluster)`
    - Write operations use prepared statements for safety
    - `GetDependencies` uses recursive Cypher traversal: `MATCH (a)-[*1..{maxDepth}]->(b)` with direction-aware pattern
    - `AssessImpact` computes downstream closure from changed files via `IMPORTS` edges, calculates risk score as `len(transitivelyAffected) / totalFileCount`
    - `Close()` closes connection and database
    - Add `go.mod` dependency on `github.com/kuzudb/go-kuzu@v0.11.3`
  - **Acceptance:** `NewKuzuStore()` returns non-nil store. `InitSchema` creates all tables without error. Write→Read round-trip preserves data. `Close()` does not error. `go test -race` passes (no concurrent access to single connection).

---

- [ ] **T-02.07 — Implement in-memory graph store for testing**
  - **File:** `internal/graph/memstore.go` (CREATE)
  - **Depends on:** T-01.07
  - **Outline:**
    - Define `MemStore` struct implementing `Store` interface using Go maps:
      - `files map[string]FileNode`
      - `symbols map[string]SymbolNode` (key: `filePath:name`)
      - `edges []Edge`
      - `clusters []ClusterNode`
    - `InitSchema` is a no-op (in-memory, no DDL needed)
    - `GetDependencies` uses BFS/DFS on the edge list
    - `AssessImpact` computes downstream closure via iterative edge following
    - Thread-safe via `sync.RWMutex`
    - Used by tests that need a Store without CGO/KuzuDB dependency
  - **Acceptance:** All `Store` interface methods are implemented. Add/Get round-trips preserve data. `GetDependencies` returns correct chains for a known graph. Passes `go test -race`.

---

- [ ] **T-02.08 — Implement clustering algorithm**
  - **File:** `internal/graph/cluster.go` (CREATE)
  - **Depends on:** T-01.07
  - **Outline:**
    - Define `ComputeClusters(store Store) ([]ClusterNode, error)`
    - Algorithm: connected-component analysis on the file-to-file graph (IMPORTS edges), with cohesion scoring
    - Cohesion score: `internal_edges / (internal_edges + external_edges)` per cluster
    - Each connected component becomes a cluster; name derived from common path prefix
    - After computing, call `store.AddCluster()` for each cluster and `store.AddEdge()` for BELONGS edges
    - Minimum cluster size: 2 files (singletons are not clustered)
  - **Acceptance:** Given a graph with 3 disconnected groups of files, `ComputeClusters` produces 3 clusters. Cohesion scores are between 0.0 and 1.0. Singleton files are not in any cluster.

---

- [ ] **T-02.09 — Create test fixtures for tier-1 languages**
  - **File:** `testdata/fixtures/go_project/` (CREATE), `testdata/fixtures/ts_project/` (CREATE), `testdata/fixtures/py_project/` (CREATE), `testdata/fixtures/rs_project/` (CREATE)
  - **Depends on:** None
  - **Outline:**
    - Each fixture is a minimal but realistic project (3–5 source files) with:
      - At least 2 functions, 1 type/class, 1 interface/trait
      - Import relationships between files
      - At least 1 call chain spanning 2 files
      - Both exported and unexported symbols
    - Go fixture: `main.go`, `model.go`, `service.go` with package imports
    - TypeScript fixture: `index.ts`, `types.ts`, `service.ts` with ES module imports
    - Python fixture: `__init__.py`, `models.py`, `service.py` with relative imports
    - Rust fixture: `main.rs`, `model.rs`, `service.rs` with `mod`/`use`
  - **Acceptance:** Each fixture directory contains 3+ source files. Files are syntactically valid in their language. Import relationships exist between files.

---

- [ ] **T-02.10 — Write Tree-sitter parser tests**
  - **File:** `internal/graph/treesitter_test.go` (CREATE)
  - **Depends on:** T-02.01, T-02.02, T-02.03, T-02.04, T-02.05, T-02.09
  - **Outline:**
    - Table-driven tests using testify, one subtest per language
    - For each tier-1 language:
      - Parse each fixture file
      - Assert correct symbol count, names, kinds, export status
      - Assert correct import edges (source file → target file)
      - Assert at least one call edge exists
      - Assert `StartLine`/`EndLine` are non-zero and `StartLine <= EndLine`
    - Test error handling: parse with wrong language, parse invalid syntax (should not panic, may return partial results)
    - Test `SupportedLanguages()` returns all 4
  - **Acceptance:** `go test ./internal/graph/ -run TestTreeSitter` passes. Each language subtest validates symbols, imports, and calls. No panics on malformed input.

---

- [ ] **T-02.11 — Write KuzuDB store tests**
  - **File:** `internal/graph/kuzustore_test.go` (CREATE)
  - **Depends on:** T-02.06
  - **Outline:**
    - Use testify suites (`suite.Suite`) for setup/teardown (create fresh DB per test)
    - Test `InitSchema` — runs twice without error (idempotent)
    - Test `AddFile` + `GetFile` round-trip
    - Test `AddSymbol` + `GetSymbol` round-trip
    - Test `QuerySymbols` — substring search, kind filter, limit
    - Test `AddEdge` + `GetDependencies` — insert a chain A→B→C, query downstream from A, expect [A,B,C]
    - Test `AssessImpact` — insert a diamond dependency, verify direct and transitive sets
    - Test `GetClusters` after `ComputeClusters` has run
    - Test `Stats` returns correct counts
    - Build tag: `//go:build cgo` (skip when CGO is disabled)
  - **Acceptance:** `go test ./internal/graph/ -run TestKuzuStore` passes (with CGO enabled). All Store interface methods are tested. Race detector passes.

---

- [ ] **T-02.12 — Write clustering tests**
  - **File:** `internal/graph/cluster_test.go` (CREATE)
  - **Depends on:** T-02.07, T-02.08
  - **Outline:**
    - Use `MemStore` (no CGO dependency)
    - Test: 3 files with no edges → 0 clusters (all singletons)
    - Test: 3 files with A↔B imports → 1 cluster (A,B), C is singleton
    - Test: 6 files in two groups of 3 → 2 clusters
    - Test: cohesion score calculation — group with all-internal edges gets 1.0, group with many external edges gets lower score
    - Test: cluster names derived from common path prefixes
  - **Acceptance:** `go test ./internal/graph/ -run TestClustering` passes. Cluster count, membership, and cohesion scores match expected values.
