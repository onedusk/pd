# Stage 2: Implementation Skeletons

**Output:** `docs/decompose/<name>/stage-2-implementation-skeletons.md`
**Prerequisites:** Stage 1 must exist
**Template:** `assets/templates/stage-2-implementation-skeletons.md`

## Before You Start

Before defining type names, verify they don't collide with existing exports in the codebase. If code intelligence is available, use `query_symbols` to check. Otherwise, use Grep/Glob to search for the names you plan to use.

## Workflow

1. Read Stage 1.
2. Translate the data model into compilable code in the target language. This is NOT pseudocode -- it must parse/compile.
3. Write interface contracts (request/response types) for any API surface described in Stage 1.
4. Write documentation artifacts: entity reference, operation reference, example payloads.
5. If ambiguities are found while writing code (e.g., a field's nullability is unclear, a relationship's delete rule is unspecified), go back and update Stage 1 before continuing.

## Review Checkpoint

After writing, verify skeletons against the codebase:
- Do the type names collide with existing exports?
- Do the interface signatures match the codebase's conventions (naming style, error handling patterns, parameter ordering)?
- Are there missing error types that the codebase's existing code would expect?

Fix any issues before proceeding to Stage 3.

## Done When

All entities from Stage 1 have corresponding type definitions. The code compiles/parses. The skeleton checklist passes.
