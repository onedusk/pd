# Review Phase: Codebase-Plan Cross-Reference

**Output:** `docs/decompose/<name>/review-findings.md`
**Prerequisites:** Stages 1-4 must be complete

The review phase sits between Stage 4 completion and implementation start. If `run_review` is available as an MCP tool, use it. It runs 5 mechanical checks comparing the decomposition plan against the actual codebase:

1. **File existence** -- validates CREATE/MODIFY/DELETE actions against filesystem state
2. **Symbol verification** -- confirms that symbols referenced in task outlines exist in the expected files
3. **Dependency completeness** -- finds files that import MODIFY targets but aren't in the plan
4. **Cross-milestone consistency** -- detects conflicting actions and ordering issues across milestones
5. **Coverage gap scan** -- identifies files within scope that may be affected but aren't planned

## Workflow

1. Call `run_review` with the decomposition name (or run the checks manually if the tool is unavailable).
2. Review the findings. Findings are classified as:
   - **MISMATCH** -- must fix before implementing
   - **OMISSION** -- evaluate and decide whether to add to the plan or confirm as intentionally excluded
   - **STALE** -- update the plan to reflect current codebase state
3. Resolve any MISMATCH findings by updating Stage 3/4 files before implementing.

## Running Checks Manually (Without the Binary)

If `run_review` is not available, run the checks using native tools:

1. **File existence:** For each MODIFY/DELETE target in the task specs, verify the file exists using Glob. For each CREATE target, verify the file does NOT exist.
2. **Symbol verification:** For key symbols referenced in task outlines (types, functions, methods), use Grep to confirm they exist in the expected files.
3. **Dependency completeness:** For each MODIFY target, use Grep to find other files that import/reference it. Check whether those files are accounted for in the plan.
4. **Cross-milestone consistency:** Check that no two milestones CREATE the same file, and that MODIFY targets in later milestones aren't CREATE targets in earlier ones that haven't been built yet per the ordering.
5. **Coverage gap:** Scan the directories that the plan touches. Are there files in those directories that the plan doesn't mention but that might need changes?

## Done When

No MISMATCH findings remain. OMISSION findings have been triaged (confirmed or reclassified as OK).
