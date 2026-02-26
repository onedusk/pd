# Stage 0: Development Standards

> Go project — single binary CLI tool with cgo dependencies (Tree-sitter, KuzuDB).
> Solo developer, AI-assisted workflow. These standards apply to all decompositions in this org.

---

## Code Change Checklist

### 1. Plan

- [ ] Define the scope of the change in one sentence
- [ ] Map affected files and their relationships
- [ ] Identify dependency impacts — especially across cgo boundaries (Tree-sitter bindings, KuzuDB)
- [ ] Note any required documentation updates
- [ ] Identify risks (breaking public API, data model changes, cgo build issues)
- [ ] Determine the change severity (see Escalation Guidance)

Self-check:
- Can I describe this change in one sentence?
- Are there side effects outside the immediate scope?
- Does this touch cgo code? If so, have I verified the C toolchain builds on my target platforms?

### 2. Implement

- [ ] Execute the plan, one logical unit at a time
- [ ] Keep changes scoped to the plan — no drive-by fixes
- [ ] Run `go vet ./...` before considering implementation complete

### 3. Test

- [ ] Write or update tests for the changed behavior
- [ ] Use testify assertions (`assert`, `require`) and table-driven tests
- [ ] Cover the expected path and at least one failure path
- [ ] Test boundary conditions where applicable (empty inputs, nil pointers, max sizes)
- [ ] Run `go test -race ./...` for the affected packages
- [ ] For cgo-dependent code: verify tests pass with cgo enabled and disabled (where applicable)

Self-check:
- Do tests describe behavior, not implementation details?
- Am I testing at the right level? (see Testing Guidance)

### 4. Changelog

- [ ] Write a changeset entry categorized by type (see Changeset Format)
- [ ] Include migration steps if the change affects existing users or data
- [ ] User-facing changes: write from the user's perspective

### 5. Report

- [ ] Summarize what was actually implemented
- [ ] Note deviations from the plan and why
- [ ] Note any test gaps accepted and why

### 6. Review

- [ ] Self-review: re-read the diff after a break (fresh eyes)
- [ ] Compare implementation against plan and report
- [ ] Verify no files were missed
- [ ] Confirm all tests pass (`go test -race ./...`)
- [ ] Check that documentation reflects current state
- [ ] For AI-generated code: verify cgo interactions, error handling, and goroutine safety

### 7. Escalate (if applicable)

- [ ] Route based on severity (see below)
- [ ] For High/Critical: document the decision before merging

### 8. Iterate

- [ ] Address gaps found in review
- [ ] Repeat steps 5–7 until report, plan, and code agree

---

## Changeset Format

**Version impact:** major | minor | patch

Versioning rules:
- **patch**: bug fixes, internal refactors with no behavior change, dependency updates
- **minor**: new features, new CLI flags, non-breaking changes to existing behavior
- **major**: breaking CLI changes, breaking changes to A2A agent cards or MCP tool interfaces, data model changes requiring migration

**Categories:**

- `added` — new functionality
- `changed` — modifications to existing functionality
- `deprecated` — soon-to-be-removed functionality
- `removed` — removed functionality
- `fixed` — bug fixes
- `security` — vulnerability patches

**Entry format:**

```
## [impact] category
Summary: [one-line description]
Detail: [optional — context, reasoning, migration notes]
```

**Release process:** Semver git tags (`v1.2.3`). Binaries via goreleaser. CHANGELOG.md updated before tagging.

---

## Escalation Guidance

| Severity | Examples | Approval | Deploy |
|----------|----------|----------|--------|
| **Low** | Internal refactors, docs, style, dependency patches | Self-review | Tag and release |
| **Medium** | New features, new CLI flags, new MCP tools, non-breaking agent card changes | Self-review + fresh-eyes pass | Tag and release, monitor |
| **High** | Breaking CLI changes, A2A protocol changes, data model changes, cgo binding updates | Document the decision (ADR), sleep on it | Test on all target platforms first |
| **Critical** | KuzuDB schema migrations, Tree-sitter grammar changes affecting tier-1 languages, security fixes | Write it up, get external feedback if possible | Full cross-platform build + test before release |

**Rule:** when in doubt, escalate up one level.

---

## Testing Guidance

### Priority order

1. **Data integrity paths** — graph database writes/reads (KuzuDB), file system operations, artifact merging. A bug here corrupts decomposition output.
2. **Integration boundaries** — A2A HTTP communication, MCP tool calls, Tree-sitter parsing. Assumptions break at protocol boundaries.
3. **Business logic** — milestone dependency analysis, task ordering, section merging, graceful degradation logic.
4. **CLI and presentation** — output formatting, progress reporting. Lowest priority for automation.

### Test levels

- **Unit tests** for pure logic: dependency graph analysis, merge strategies, argument parsing. Use testify assertions and table-driven tests.
- **Integration tests** for boundary code: A2A message round-trips, MCP tool call/response cycles, Tree-sitter parse → graph build pipeline. Use testify suites where setup/teardown is needed.
- **E2E tests** sparingly: full `/decompose` pipeline producing output files. Expensive, but critical for validating the complete flow.

Aim for: many unit tests, targeted integration tests at every protocol boundary, few E2E tests for the golden path.

### Go-specific conventions

- Test files: `*_test.go` in the same package (white-box) or `*_test` package (black-box, preferred for public APIs)
- Table-driven tests with `t.Run` subtests for all non-trivial functions
- Use `testify/assert` for soft assertions, `testify/require` for preconditions that should halt the test
- Use `testify/mock` for external dependencies (A2A clients, MCP servers)
- Race detector: always run CI with `-race` flag
- cgo tests: ensure CI has C toolchain available; skip cgo-dependent tests with build tags when cross-compiling

### Target CI pipeline (not yet configured)

```
golangci-lint run ./...
go vet ./...
go test -race -count=1 ./...
go build -o /dev/null ./cmd/...    # verify build
```

Lint config and CI setup will be added when the project has buildable code.

### Practical tradeoffs

- This is a CLI tool with cgo dependencies. Cross-compilation requires a C toolchain per target — test on macOS and Linux at minimum.
- Goroutine-heavy code (agent orchestration) must be tested with the race detector. No exceptions.
- Tree-sitter grammar coverage: tier-1 languages (Go, TypeScript, Python, Rust) get full test coverage. Others are best-effort.
