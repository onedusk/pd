# Stage 0: Development Standards

> Fill this in once for your team or organization. Reference it from every project.
> Delete the HTML comments as you fill in each section.

---

## Code Change Checklist

<!-- Steps every change should follow, in order.
     Adapt to your workflow — add or remove steps as needed. -->

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
- Am I testing at the right level? (see Testing Guidance)

### 4. Changelog

- [ ] Write a changeset entry for the change
- [ ] Categorize the change appropriately (see Changeset Format)
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

- [ ] Route the change for approval based on severity (see Escalation Guidance)
- [ ] Obtain sign-off before merging

### 8. Iterate

- [ ] Address any gaps found in review or escalation
- [ ] Repeat steps 5–7 until the report, plan, and code all agree

---

## Changeset Format

<!-- Define how changes are categorized and versioned. -->

**Version impact:** major | minor | patch

<!-- Versioning rules:
     - patch: bug fixes, typo corrections, internal refactors with no behavior change
     - minor: new features, non-breaking changes to existing behavior
     - major: breaking changes, data migrations, removed features, anything requiring user action -->

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
Summary: [one-line description of what changed]
Detail: [optional — additional context, reasoning, or migration notes]
```

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

<!-- Define severity levels appropriate for your team size and risk tolerance. -->

| Severity | Examples | Approval | Deploy |
|----------|----------|----------|--------|
| **Low** | Internal refactors, style changes, documentation, dependency patches with no behavior change | Self-review is sufficient | Standard process |
| **Medium** | New features, non-breaking changes, non-critical bug fixes, test additions | Self-review + a second pass after a break (fresh eyes) | Standard process, monitor after deploy |
| **High** | Breaking changes, data model changes, security fixes, anything touching auth/payments/user data | Formal review required | Staged rollout, active monitoring, rollback plan |
| **Critical** | Billing logic, production data migrations, security incident response, infrastructure affecting availability | All stakeholders sign off | Maintenance window, tested rollback, post-deploy verification |

**Rule: when in doubt, escalate up one level.** The cost of over-reviewing is minutes. The cost of under-reviewing changes to payments or user data can be significant and hard to reverse.

---

## Testing Guidance

### Priority order

1. **Data integrity paths** — anything that writes, transforms, or deletes data. A bug here is expensive. Test these first.
2. **Integration boundaries** — where your code talks to external systems (APIs, databases, payment processors). Assumptions break here most often.
3. **Business logic** — the rules and calculations that define correct behavior. Easiest to test and most valuable as documentation.
4. **UI and presentation** — lowest priority for automated tests. Visual correctness is hard to assert and changes frequently.

### Test levels

- **Unit tests** for pure logic and calculations. Fast, cheap, stable.
- **Integration tests** for code that crosses boundaries (DB queries, API calls, file I/O). More expensive but catches real-world failures.
- **End-to-end tests** sparingly, for critical user-facing workflows only. Slow, brittle, expensive to maintain.

Aim for: many unit tests, some integration tests, few E2E tests.

### What makes a good test

- **Tests behavior, not implementation.** A test that breaks when you rename a private function is a liability.
- **One assertion per concept.** A test that checks five things tells you something failed but not what.
- **Readable as documentation.** Someone unfamiliar with the code should understand what the system does.
- **Independent.** Tests should not depend on execution order or shared mutable state.

### Practical tradeoffs

<!-- Add notes specific to your context below -->

- Perfect coverage is not the goal. Covering critical paths well beats covering everything superficially.
- If writing a test feels disproportionately hard, that's often a signal the code needs restructuring — not that the test should be skipped.
- Tests are most valuable when they let you change code with confidence. If a test suite makes you afraid to refactor, it's working against you.
