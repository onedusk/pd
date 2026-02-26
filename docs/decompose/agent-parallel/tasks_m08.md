# Stage 4: Task Specifications — Milestone 8: Testing + Release

> End-to-end test suite, goreleaser configuration, CI setup, and documentation. This is the final milestone — all code is complete, this milestone validates and packages it.
>
> Fulfills: PDR-002 (single binary distribution)

---

- [ ] **T-08.01 — Create E2E pipeline test**
  - **File:** `internal/e2e/pipeline_test.go` (CREATE)
  - **Depends on:** T-06.10, T-07.05
  - **Outline:**
    - Build tag: `//go:build e2e` (run separately from unit tests)
    - Test the full pipeline against `testdata/fixtures/go_project/`:
      1. Create temp output directory
      2. Run Stage 1 with `CapBasic` (no external dependencies)
      3. Verify `stage-1-design-pack.md` exists and has all required sections
      4. Run Stage 2
      5. Verify `stage-2-implementation-skeletons.md` exists and has code blocks
      6. Run Stage 3
      7. Verify `stage-3-task-index.md` exists and has milestone table + dependency graph
      8. Run Stage 4
      9. Verify `tasks_m01.md` (at minimum) exists and has tasks in `T-MM.SS` format
    - Verify cross-stage references: Stage 3 references entities from Stage 2, Stage 4 task IDs match Stage 3 counts
    - Timeout: 2 minutes for the full pipeline
    - Clean up temp directory after test
  - **Acceptance:** `go test -tags e2e ./internal/e2e/ -run TestPipeline` passes. All 4 stage output files are created. Files are non-empty and contain stage-specific content markers.

---

- [ ] **T-08.02 — Create golden path tests**
  - **File:** `internal/e2e/golden_test.go` (CREATE)
  - **Depends on:** T-08.01
  - **Outline:**
    - Build tag: `//go:build e2e`
    - Run the full pipeline against `testdata/fixtures/go_project/` with `CapBasic`
    - Compare output files against golden files in `testdata/golden/`
    - Use `testify.Equal` for exact match, OR use a structural comparison that ignores timestamps and UUIDs
    - If golden files don't exist (first run), write them and skip comparison (`-update` flag pattern)
    - Define `TestUpdateGolden` that regenerates golden files when run with `-update`
    - Golden files serve as regression tests — any change in output format is detected
  - **Acceptance:** `go test -tags e2e ./internal/e2e/ -run TestGolden` passes when output matches golden files. `go test -tags e2e -run TestUpdateGolden ./internal/e2e/` regenerates golden files without error.

---

- [ ] **T-08.03 — Create golden test fixture files**
  - **File:** `testdata/golden/stage1_output.md` (CREATE), `testdata/golden/stage2_output.md` (CREATE), `testdata/golden/stage3_output.md` (CREATE), `testdata/golden/stage4_output.md` (CREATE)
  - **Depends on:** T-08.02
  - **Outline:**
    - Run the pipeline once in `CapBasic` mode against `testdata/fixtures/go_project/`
    - Capture the output of each stage
    - Save as golden files
    - Scrub any non-deterministic content (timestamps → placeholder, UUIDs → placeholder)
    - These files are committed to the repo and used by T-08.02
  - **Acceptance:** All 4 golden files exist. They are valid markdown. They match the output format defined by the stage templates.

---

- [ ] **T-08.04 — Configure goreleaser**
  - **File:** `.goreleaser.yml` (CREATE)
  - **Depends on:** T-01.02
  - **Outline:**
    - Binary name: `decompose`
    - Build: `cmd/decompose`
    - GOOS: `darwin`, `linux`, `windows`
    - GOARCH: `amd64`, `arm64`
    - CGO: enabled (required for Tree-sitter and KuzuDB)
    - Note: CGO cross-compilation requires C toolchain per target. For initial release, build only for the host platform. Cross-platform builds require CI with per-platform runners.
    - Ldflags: `-s -w -X main.version={{.Version}}`
    - Archives: tar.gz for linux/darwin, zip for windows
    - Changelog: auto-generated from git commits
    - Release: GitHub release with binaries attached
    - Snapshot: `--snapshot` flag for local testing without tag
  - **Acceptance:** `goreleaser build --snapshot --clean` produces a binary in `dist/`. Binary reports correct version via `--version`. Binary runs and exits cleanly.

---

- [ ] **T-08.05 — Create Makefile**
  - **File:** `Makefile` (CREATE)
  - **Depends on:** T-01.01
  - **Outline:**
    - Targets:
      - `build`: `go build -o bin/decompose ./cmd/decompose`
      - `test`: `go test -race -count=1 ./...`
      - `test-e2e`: `go test -tags e2e -race ./internal/e2e/`
      - `lint`: `golangci-lint run ./...`
      - `vet`: `go vet ./...`
      - `clean`: `rm -rf bin/ dist/`
      - `release-snapshot`: `goreleaser build --snapshot --clean`
      - `update-golden`: `go test -tags e2e -run TestUpdateGolden ./internal/e2e/ -update`
    - Default target: `build`
    - `.PHONY` declarations for all targets
    - Include CGO_ENABLED=1 in build and test commands
  - **Acceptance:** `make` produces `bin/decompose`. `make test` runs all unit tests. `make lint` runs linter (or prints install instructions if not installed). `make clean` removes build artifacts.

---

- [ ] **T-08.06 — Write README**
  - **File:** `README.md` (CREATE)
  - **Depends on:** T-08.04, T-08.05
  - **Outline:**
    - Project name and one-line description (from PRD goal)
    - Installation: `go install`, binary download from releases, homebrew (if applicable)
    - Prerequisites: Go 1.26+, C toolchain (for CGO), KuzuDB shared library
    - Quick start: `decompose myproject 1` through `decompose myproject 4`
    - CLI usage: flags from Stage 1 CLI interface section
    - Capability levels: table showing what's available at each level
    - Architecture overview: component diagram from Stage 1 (simplified)
    - Development: `make build`, `make test`, `make lint`
    - License: PolyForm Shield 1.0.0
    - Link to progressive-decomposition methodology repo
  - **Acceptance:** README has installation, quick start, CLI reference, and development sections. No broken links. No placeholder text.

---

- [ ] **T-08.07 — Add CI workflow**
  - **File:** `.github/workflows/ci.yml` (CREATE)
  - **Depends on:** T-08.05
  - **Outline:**
    - Trigger: push to `main`, pull requests
    - Matrix: Go 1.26.x on ubuntu-latest and macos-latest (need both for CGO testing)
    - Steps:
      1. Checkout
      2. Setup Go
      3. Install C toolchain (apt-get install build-essential on ubuntu)
      4. Install KuzuDB shared library
      5. `make vet`
      6. `make lint` (install golangci-lint first)
      7. `make test` (with race detector)
      8. `make build` (verify binary builds)
    - E2E tests: separate job, only on main branch (slower)
    - Cache: Go module cache and build cache
  - **Acceptance:** CI workflow file is valid YAML. Includes both ubuntu and macos runners. All make targets are invoked. E2E tests run on main branch pushes.

---

- [ ] **T-08.08 — Add x/sync dependency and final go mod tidy**
  - **File:** `go.mod` (MODIFY), `go.sum` (MODIFY)
  - **Depends on:** T-06.03
  - **Outline:**
    - `go get golang.org/x/sync` (for errgroup used in fan-out)
    - Run `go mod tidy` to clean up all dependencies accumulated across milestones
    - Verify: `go build ./...` succeeds
    - Verify: `go vet ./...` passes
    - Verify: no unused dependencies in `go.mod`
  - **Acceptance:** `go mod tidy` makes no changes (already clean). `go build ./...` succeeds. `go vet ./...` passes. All imports resolve.
