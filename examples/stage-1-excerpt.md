# Stage 1 Example: Design Pack

> Excerpted from a real iOS project (Arkived — a local-first journal and activity archive).
> The full design pack was ~20 pages covering 6 core entities, 10 ADRs, 6 PDRs, and 9 milestones.
> Key sections are shown below to illustrate the pattern.

---

## Assumptions

- Single user, on one iPhone.
- Local-only persistence (no backend, no cloud sync).
- "Searchable on any day" means: fast date navigation + good full-text/semantic search across activity text + day notes.

---

## Target Platform & Tooling Baseline

| Component | Version | Reference |
|-----------|---------|-----------|
| iOS target | iOS/iPadOS 26.x | [Apple Support](https://support.apple.com/en-us/100100) |
| Xcode | Xcode 26.x | [Apple Developer](https://developer.apple.com/news/releases/) |
| Swift | Swift 6.2 (Xcode 26 toolchain) | [What's New in Swift](https://developer.apple.com/swift/whats-new/) |
| SwiftUI | iOS 26 SDK | [iOS 26 Release Notes](https://developer.apple.com/documentation/ios-ipados-release-notes/ios-ipados-26-release-notes) |
| On-device AI | Apple Foundation Models | [Apple Newsroom](https://www.apple.com/newsroom/2025/09/apples-foundation-models-framework-unlocks-new-intelligent-app-experiences/) |

---

## Data Model (2 of 6 entities shown)

### DayEntry

One row per calendar day (in user's timezone).

- `id: UUID`
- `date: Date` (normalized to local day start)
- `title: String?`
- `note: String` (long-form daily journal text)
- `mood: Int?` (optional scale 1–5)
- `energy: Int?` (optional scale 1–5)
- `createdAt`, `updatedAt`

Relationships:
- `activities: [Activity]` (1-to-many, cascade delete)

### Activity

The atomic "what I did" unit.

- `id: UUID`
- `day: DayEntry` (relationship)
- `startAt: Date?`, `endAt: Date?`
- `kind: ActivityKind` (enum: work/personal/health/admin/etc.)
- `text: String` (the canonical description)
- `source: ActivitySource` (manual, calendarImport, healthImport, shortcut, etc.)
- `isPinned: Bool`
- `createdAt`, `updatedAt`

Optional fields:
- `locationLabel: String?` (human label)
- `lat: Double?`, `lon: Double?` (only if user enables)

---

## Architecture

```
SwiftUI Views
  ├─ TodayView / DayDetailView / EditorSheets
  ├─ CalendarView
  ├─ SearchView
  └─ AssistantChatView
          |
ViewModels (@MainActor, Observable)
          |
Use Cases (Domain layer)
  ├─ CreateActivity / UpdateActivity / DeleteActivity
  ├─ UpsertDayEntry
  ├─ Search (keyword + semantic)
  ├─ ImportCalendar / ImportHealth / ImportLocation
  └─ SummarizeDay / WeeklyRecap
          |
Repositories / Services
  ├─ SwiftDataStore (ModelContainer/ModelContext)
  ├─ AttachmentStore (file I/O + thumbnails)
  ├─ SearchIndexService (optional FTS / tokenization)
  ├─ EmbeddingService (NaturalLanguage)
  ├─ AIService (FoundationModels)
  └─ Importers (EventKit, HealthKit, etc.)
```

**Pattern:** SwiftUI + MVVM with a thin Domain "Use Case" layer.
**Concurrency:** UI on `@MainActor`, persistence via a dedicated actor, imports/indexing via structured concurrency.

---

## Wireframe: Today (default landing)

```
┌──────────────────────────────────────┐
│ Feb 11, 2026                 [gear]  │
│ TODAY                                │
│ ┌─────────────── Quick Add ─────────┐│
│ │ [ + Activity ] [ Voice ] [ Note ] ││
│ └───────────────────────────────────┘│
│                                      │
│ Day Note (collapsed preview...)      │
│ "Short summary / intention..."       │
│ [Edit]                               │
│                                      │
│ Timeline                             │
│ 09:00–09:15  Standup        (Work)   │
│ 10:00–11:30  Build Arkived   (Work)  │
│ 12:10–12:40  Walk            (Health)│
│ ...                                  │
│                                      │
│ [ + ] floating add button            │
└──────────────────────────────────────┘
```

---

## ADRs (2 of 10 shown)

### ADR-001 — Local-only persistence (no backend)

- **Status:** Accepted
- **Context:** App is personal-use; privacy is primary.
- **Decision:** No networked persistence; all data stored on-device.
- **Consequences:** Simplifies compliance and ops; no cross-device sync; need export/backup path.

### ADR-010 — Explicit confirmation for destructive AI actions

- **Status:** Accepted
- **Context:** Tool-calling models can make mistakes.
- **Decision:** Deletions require explicit UI confirmation; updates show diffs.
- **Consequences:** Adds one tap; prevents catastrophic loss.

---

## PDRs (2 of 6 shown)

### PDR-002 — Minimize required fields for activity capture

- **Status:** Accepted
- **Problem:** Journals fail when logging is too heavy.
- **Decision:** Only `text` is required; time/kind/tags optional.
- **Rationale:** Lower friction → higher retention.

### PDR-006 — "Export/backup" is a v1 requirement

- **Status:** Accepted
- **Problem:** Local-only without a backup path is risky.
- **Decision:** Provide encrypted export to Files (ZIP containing JSON + attachments).
- **Rationale:** Prevents data loss without adding cloud sync.

---

## Condensed PRD

**Goal:** Let the user reconstruct any day accurately via: (1) timeline, (2) day note, (3) search.

**Primary user stories:**

1. Log an activity in <10 seconds.
2. At end of day, write a short note and see the timeline.
3. Search "what did I do with Alex last month?" and get correct results.
4. Ask the assistant the same question and get an answer + links to entries.

**Non-goals (v1):** multi-device sync, sharing/collaboration, social features, server-based AI.

---

## Implementation Plan

1. SwiftData models + repositories
2. Today/Day screens + CRUD
3. Calendar tab
4. Search tab (keyword)
5. Attachments (photo/link)
6. Export/backup
7. EventKit import
8. Assistant: Foundation Models + tool calling
9. Semantic embeddings + hybrid ranking
