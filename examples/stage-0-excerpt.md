# Stage 0 Example: Development Standards

> Excerpted from a real iOS project (Arkived — a local-first journal and activity archive).
> These standards were used across 9 milestones and 57 tasks.

---

## Code Change Checklist

### 1. Plan

- [ ] Define the scope of the change (what and why)
- [ ] Map all affected files and their relationships
- [ ] Identify dependency impacts (upstream and downstream)
- [ ] Note any required documentation updates
- [ ] Identify risks or edge cases
- [ ] Determine the change severity (see Escalation Guidance)

Self-check:

- Can I describe this change in one sentence?
- Have all affected files been mapped?
- Are there side effects outside the immediate scope?

### 2. Implement

- [ ] Execute the plan, one logical unit at a time
- [ ] Keep changes scoped to the plan (avoid drive-by fixes)

### 3. Test

- [ ] Write or update tests for the changed behavior
- [ ] Cover the expected path (does it do what it should?)
- [ ] Cover at least one failure path (does it fail how it should?)
- [ ] Test boundary conditions if applicable (empty inputs, nulls, limits)
- [ ] Run the full affected test suite, not just the new tests

Self-check:

- Do tests describe *behavior*, not implementation details?
- If I refactored the internals without changing behavior, would these tests still pass?
- Am I testing at the right level?

### 4. Changelog

- [ ] Write a changeset entry for the change
- [ ] Categorize the change appropriately
- [ ] Include migration steps if the change affects existing users or data
- [ ] If the change is user-facing, write it from the user's perspective

### 5. Report

- [ ] Write a summary of what was actually implemented
- [ ] Note any deviations from the plan and why
- [ ] Note any test gaps accepted and why

### 6. Review

- [ ] Compare the implementation against both the plan and the report
- [ ] Verify no files were missed
- [ ] Confirm all tests pass
- [ ] Check that documentation reflects the current state
- [ ] Confirm changeset entry is accurate and complete

### 7. Escalate (if applicable)

- [ ] Route the change for approval based on severity
- [ ] Obtain sign-off before merging

### 8. Iterate

- [ ] Address any gaps found in review or escalation
- [ ] Repeat steps 5–7 until the report, plan, and code all agree

---

## Changeset Format

**Version impact:** major | minor | patch

- **patch**: bug fixes, typo corrections, internal refactors with no behavior change. No user action required.
- **minor**: new features, non-breaking changes to existing behavior. Users may want to know but don't need to act.
- **major**: breaking changes, data migrations, removed features, or anything that requires user action.

**Categories:** added | changed | deprecated | removed | fixed | security

**Example:**

```
## [minor] added
Summary: Recurring invoice scheduling for service-based billing
Detail: Users can now set invoices to auto-generate on a weekly,
biweekly, or monthly cadence. Existing invoices are unaffected.
No migration required.
```

---

## Escalation Guidance

| Severity | Examples | Approval | Deploy |
|----------|----------|----------|--------|
| **Low** | Internal refactors, style changes, documentation updates, dependency patches | Self-review | Standard |
| **Medium** | New features, non-breaking changes, non-critical bug fixes, test additions | Self-review + fresh-eyes pass | Monitor after deploy |
| **High** | Breaking changes, data model changes, security fixes, auth/payments/user data | Formal review required | Staged rollout, rollback plan |
| **Critical** | Billing logic, production data migrations, security incident response, infra availability | All stakeholders sign off | Maintenance window, tested rollback |

**When in doubt, escalate up one level.**

---

## Testing Guidance

### Priority order

1. **Data integrity paths** — anything that writes, transforms, or deletes data
2. **Integration boundaries** — where your code talks to external systems
3. **Business logic** — rules and calculations that define correct behavior
4. **UI and presentation** — lowest priority for automated tests

### What makes a good test

- Tests behavior, not implementation
- One assertion per concept
- Readable as documentation
- Independent (no shared mutable state, no execution order dependency)

### Practical tradeoffs (solo developer)

- Perfect coverage is not the goal. Covering critical paths well beats covering everything superficially.
- If writing a test feels disproportionately hard, that's often a signal the code needs restructuring.
- Tests are most valuable when they let you change code with confidence.
