# Stage 1: Design Pack — [Project Name]

> Research-based specification. Fill every required section before moving to Stage 2.
> Sections marked *(required)* must be completed. Sections marked *(if applicable)* can be
> skipped with a brief note explaining why.
>
> Delete the HTML comments as you fill in each section.

---

## Assumptions & Constraints *(required)*

<!-- List 3–5 fundamental assumptions. These are the ground truth you are designing against.
     Examples:
     - "Single user, on one device."
     - "Local-only persistence — no backend, no cloud sync."
     - "Budget: $0 infrastructure cost."
     - "Must work offline." -->

*

---

## Target Platform & Tooling Baseline *(required)*

<!-- Specific versions. Not "use React" but "React 19.1". Include links to docs you read.
     This section forces you to research before designing. -->

| Component | Version | Reference |
|-----------|---------|-----------|
| Language | | [link] |
| Framework / SDK | | [link] |
| Runtime / Platform | | [link] |
| Build tool | | [link] |
| Key dependency 1 | | [link] |
| Key dependency 2 | | [link] |

---

## Data Model / Schema *(required)*

<!-- Every entity, every field, every relationship.
     For each entity: name, key/unique fields, all fields with types and nullability,
     relationships with cardinality and delete rules. -->

### Entity: [Name]

**Purpose:** [one sentence]
**Key:** `fieldName` ([type], unique)

**Fields:**

| Field | Type | Nullable | Purpose |
|-------|------|:--------:|---------|
| | | | |
| | | | |

**Relationships:**

| Relationship | Target | Cardinality | Delete Rule |
|-------------|--------|-------------|-------------|
| | | | |

<!-- Repeat for each entity. -->

---

## Architecture *(required)*

### Component Diagram

<!-- ASCII diagram showing layers and their connections. -->

```
[Presentation Layer]
        |
[Domain / Business Logic Layer]
        |
[Data / Infrastructure Layer]
```

### Architectural Pattern

<!-- Name it (MVVM, Clean Architecture, Hexagonal, etc.) and explain why. -->

### Concurrency / Threading Model

<!-- What runs where? Main thread? Background workers? Queues? Actor isolation? -->

---

## UI/UX Layout *(required for apps; skip for libraries/CLIs)*

### Navigation Model

<!-- Tabs? Stack? Drawer? Modal sheets? Describe the top-level navigation structure. -->

### Screen Inventory

<!-- List every screen/page with a one-sentence purpose. -->

| Screen | Purpose |
|--------|---------|
| | |
| | |

### Wireframes

<!-- ASCII wireframes for the 3–5 most important screens.
     Goal is structural layout, not visual design. -->

```
+----------------------------------+
| [Screen Name]                    |
|                                  |
| [Component / section layout]     |
|                                  |
+----------------------------------+
```

---

## Features *(required)*

### Core

<!-- The minimum set of features that make the product useful. -->

- [ ] [Feature]
- [ ] [Feature]

### Retrieval / Discovery

<!-- How users find and browse their data. -->

- [ ] [Feature]

### Quality of Life

<!-- Nice-to-have features that reduce friction. -->

- [ ] [Feature]

### Integrations *(if applicable)*

<!-- External system integrations. -->

- [ ] [Feature]

---

## Integration Points *(if applicable)*

<!-- Every external system you touch. For each: API surface, auth requirements, constraints. -->

### [Integration Name]

- **API surface:** [specific methods/endpoints you'll use]
- **Auth / permissions:** [what's required]
- **Constraints:** [known limitations, rate limits, availability]

<!-- Repeat for each integration. -->

---

## Security & Privacy Plan *(required)*

- **Data at rest:** [how stored data is protected]
- **Data in transit:** [encryption, HTTPS, etc.]
- **Permissions required:** [list all OS/platform permissions]
- **System exposure:** [what is/isn't indexed, logged, or shared with the OS]
- **Optional hardening:** [additional measures not required for v1]

---

## Architecture Decision Records *(required, minimum 3)*

### ADR-001 — [Title]

- **Status:** Accepted
- **Context:** [why this decision was needed]
- **Decision:** [what was decided]
- **Consequences:** [trade-offs — what this enables and prevents]

### ADR-002 — [Title]

- **Status:** Accepted
- **Context:**
- **Decision:**
- **Consequences:**

### ADR-003 — [Title]

- **Status:** Accepted
- **Context:**
- **Decision:**
- **Consequences:**

<!-- Add more as needed. Strong design packs have 5–10 ADRs. -->

---

## Product Decision Records *(required, minimum 2)*

### PDR-001 — [Title]

- **Status:** Accepted
- **Problem:** [what user/product problem prompted this]
- **Decision:** [what was decided]
- **Rationale:** [why this is right for users]

### PDR-002 — [Title]

- **Status:** Accepted
- **Problem:**
- **Decision:**
- **Rationale:**

<!-- Add more as needed. -->

---

## Condensed PRD *(required)*

**Goal:** [one sentence]

**Primary User Stories:**

1. [As a user, I can...]
2. [As a user, I can...]
3. [As a user, I can...]

**Non-Goals (this version):**

- [Explicitly not building this]
- [Explicitly not building this]

**Success Criteria:**

- [Measurable condition]
- [Measurable condition]

---

## Data Lifecycle & Retention *(required)*

- **Deletion behavior:** [cascade rules — what gets cleaned up when an entity is deleted]
- **Export format:** [what an export/backup contains]
- **Retention policy:** [how long data lives, archival rules]

---

## Testing Strategy *(required)*

### Unit Tests

- [Specific scenario to test]
- [Specific scenario to test]

### Integration Tests

- [Specific scenario to test]

### UI / E2E Tests *(if applicable)*

- [Specific scenario to test]

---

## Implementation Plan *(required)*

<!-- Ordered milestone list. Names and sequence only — details go in Stages 3–4. -->

1. [Milestone name — one-sentence description]
2. [Milestone name — one-sentence description]
3. [Milestone name — one-sentence description]

---

## Before Moving On

Verify before proceeding to Stage 2:

- [ ] Every assumption is written down
- [ ] Platform/tooling versions are specific and researched (not guessed)
- [ ] Data model covers every entity with all fields, types, and relationships
- [ ] Architecture pattern is named and justified
- [ ] At least 3 ADRs are written
- [ ] At least 2 PDRs are written
- [ ] Implementation plan has an ordered milestone list
- [ ] You can describe the project in one sentence (the PRD goal)
