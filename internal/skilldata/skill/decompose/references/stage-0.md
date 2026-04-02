# Stage 0: Development Standards

**Output:** `docs/decompose/stage-0-development-standards.md`
**Prerequisites:** None
**Template:** `assets/templates/stage-0-development-standards.md`

## Workflow

1. Ask the user about their team's existing norms: code review process, testing expectations, change management, escalation rules.
2. If an `AGENTS.md`, `CLAUDE.md`, or `.cursorrules` file exists in the project root, read it -- it likely contains conventions to incorporate.
3. Fill in the template with the user's norms. Keep it under 2 pages.
4. This stage is optional for solo developers. If the user says "skip," note that Stage 0 was skipped and proceed to ask which decomposition to start.

## Review Checkpoint

After writing, verify that the file covers:
- Code change checklist
- Changeset format
- Escalation guidance
- Testing guidance

If any section is missing or trivially filled ("TBD"), ask the user for input before marking complete.

## Done When

The file exists and covers code change checklist, changeset format, escalation guidance, and testing guidance. The user has confirmed or explicitly skipped.
