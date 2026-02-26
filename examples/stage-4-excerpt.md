# Stage 4 Example: Task Specifications

> Excerpted from a real iOS project (Arkived — a local-first journal and activity archive).
> 6 representative tasks selected from 57 total across 9 milestones, showing the range
> of task types: foundational creation, deletion, service creation, wiring, cross-milestone
> integration, and enhancement of existing code.

---

## From Milestone 1 — SwiftData Models + Repositories

> Foundation layer. Everything else depends on this.

### T-01.01 — Create ArkivedModels.swift (CREATE — large foundational task)

- [x] **T-01.01 — Create ArkivedModels.swift**
  - **File:** `arkived/Models/ArkivedModels.swift` (CREATE)
  - **Depends on:** None
  - **Outline:**
    - Copy model code from `docs/index_02.md` (the `ArkivedModels.swift` section), adapting the file path from `Sources/Arkived/Models/` to `arkived/Models/`
    - 5 enums (`String, Codable, CaseIterable`): `ActivityKind` (work, personal, health, admin, learning, social, travel, other), `ActivitySource` (manual, calendarImport, healthImport, locationImport, shortcut), `AttachmentType` (photo, audioNote, file, link), `ExternalProvider` (calendar, health, location, weather, photos), `EmbeddingModel` (nlSentenceEmbedding_v1, nlContextualEmbedding_v1)
    - 8 `@Model` classes:
      - `DayEntry` — `@Attribute(.unique) dayKey: String`, `date: Date`, `title: String?`, `note: String`, `mood: Int?`, `energy: Int?`, `createdAt/updatedAt`. Cascade-delete relationships: `activities`, `attachments`, `externalRefs`, `embeddings`
      - `Activity` — `@Attribute(.unique) id: UUID`, `startAt/endAt: Date?`, `kind: ActivityKind`, `text: String`, `source: ActivitySource`, `isPinned: Bool`, `locationLabel/lat/lon`, `createdAt/updatedAt`. Required `day: DayEntry`. Cascade-delete: `attachments`, `tagLinks`, `externalRefs`, `embeddings`
      - `Tag` — `@Attribute(.unique) name: String`, `colorHex: String?`, `createdAt`. Cascade-delete: `activityLinks`
      - `ActivityTag` — join model with `activity: Activity`, `tag: Tag`, `createdAt`
      - `Attachment` — `@Attribute(.unique) id: UUID`, `type: AttachmentType`, `caption/relativePath/externalURL`, `createdAt`. Polymorphic ownership: `day: DayEntry?` OR `activity: Activity?` (precondition: exactly one non-nil)
      - `ExternalRef` — `@Attribute(.unique) id: UUID`, `provider: ExternalProvider`, `externalId: String`, `fetchedAt`. Polymorphic ownership (same pattern)
      - `Embedding` — `@Attribute(.unique) id: UUID`, `model: EmbeddingModel`, `dims: Int`, `vector: Data`, `updatedAt`. Polymorphic ownership
      - `AssistantThread` + `AssistantMessage` — optional chat history; thread has cascade-delete to messages
    - `ArkivedDates` enum helper: `normalizedStartOfDay(for:calendar:) -> Date`, `dayKey(for:calendar:) -> String` ("YYYY-MM-DD")
  - **Acceptance:** Project compiles. All model types and enums are importable from the arkived target.

---

### T-01.02 — Delete Item.swift (DELETE — simplest task type)

- [x] **T-01.02 — Delete Item.swift**
  - **File:** `arkived/Item.swift` (DELETE)
  - **Depends on:** T-01.01
  - **Outline:**
    - Remove the Xcode template model file — it is fully replaced by `ArkivedModels.swift`
  - **Acceptance:** No `Item` type references remain. Project compiles.

---

### T-01.05 — Create SwiftDataStore.swift (CREATE — service with method-level detail)

- [x] **T-01.05 — Create SwiftDataStore.swift**
  - **File:** `arkived/Repositories/SwiftDataStore.swift` (CREATE)
  - **Depends on:** T-01.01
  - **Outline:**
    - Declare as `@ModelActor actor SwiftDataStore` (the `@ModelActor` macro auto-provides `modelContainer` and `modelContext`)
    - DayEntry methods:
      - `fetchDayEntry(for dayKey: String) -> DayEntry?` — predicate on `dayKey`
      - `fetchDayEntry(for date: Date) -> DayEntry?` — normalizes via `ArkivedDates`, then fetches by `dayKey`
      - `createDayEntry(date:title:note:mood:energy:) -> DayEntry` — normalizes date, inserts
    - Activity methods:
      - `fetchActivities(for dayKey: String, sortedBy: KeyPath?) -> [Activity]`
      - `createActivity(day:startAt:endAt:kind:text:source:isPinned:locationLabel:lat:lon:) -> Activity`
      - `deleteActivity(_ activity: Activity)`
    - Tag methods:
      - `fetchAllTags() -> [Tag]`
      - `fetchOrCreateTag(name:colorHex:) -> Tag` — finds by unique name or creates
      - `linkTag(_ tag: Tag, to activity: Activity)` — creates `ActivityTag` join
    - Search method:
      - `searchActivities(query:startDate:endDate:tags:kind:source:limit:) -> [Activity]` — builds `#Predicate` with `.localizedStandardContains` for text, date range filter, kind/source filter
    - `save()` — explicit `try modelContext.save()`
  - **Acceptance:** Can be instantiated with an in-memory `ModelContainer`. CRUD operations work in unit tests.

---

## From Milestone 2 — Today/Day Screens + CRUD

### T-02.14 — Wire MainTabView as app root (MODIFY — wiring task)

- [x] **T-02.14 — Wire MainTabView as app root**
  - **File:** `arkived/arkivedApp.swift` (MODIFY)
  - **Depends on:** T-02.08, T-02.09
  - **Outline:**
    - Change `ContentView()` to `MainTabView()` in the `WindowGroup` body
    - `ContentView.swift` becomes dead code (can delete or leave)
  - **Acceptance:** App launches to 4-tab interface. Today tab is default with working timeline.

---

## From Milestone 7 — EventKit Import

### T-07.02 — Create ImportCalendarUseCase.swift (CREATE — cross-milestone dependencies)

- [x] **T-07.02 — Create ImportCalendarUseCase.swift**
  - **File:** `arkived/UseCases/ImportCalendarUseCase.swift` (CREATE)
  - **Depends on:** T-07.01, T-02.01, T-01.05
  - **Outline:**
    - Takes `CalendarImporter`, `CreateActivityUseCase`, `SwiftDataStore`
    - `func importEvents(for date: Date) async throws -> Int`
      1. `calendarImporter.requestAccess()` (if not already granted)
      2. `calendarImporter.fetchEvents(for: date)`
      3. For each event:
         - Check `ExternalRef` with `provider: .calendar` and `externalId: event.eventIdentifier`
         - If exists → skip (already imported)
         - If not → create Activity via `CreateActivityUseCase` + create `ExternalRef` linking to the new activity
      4. Return count of newly imported activities
  - **Acceptance:** First import creates activities. Second import with same events creates zero duplicates.

---

## From Milestone 9 — Semantic Embeddings + Hybrid Ranking

### T-09.03 — Update SearchUseCase for hybrid ranking (MODIFY — enhancement of existing code)

- [x] **T-09.03 — Update SearchUseCase for hybrid ranking**
  - **File:** `arkived/UseCases/SearchUseCase.swift` (MODIFY)
  - **Depends on:** T-09.02, T-04.01
  - **Outline:**
    - When `EmbeddingService.isAvailable()`:
      1. Run keyword search (existing predicate-based, from M4)
      2. Run semantic search (via `SearchIndexService`)
      3. Merge results using reciprocal rank fusion (RRF) or a weighted score blend
      4. Deduplicate by activity ID
      5. Return merged, ranked results
    - When embeddings unavailable: fall back to keyword-only (existing behavior)
    - Consider: weight keyword matches higher when query is short/exact, semantic higher when query is natural language
  - **Acceptance:** Search returns improved results when embeddings exist. Falls back gracefully to keyword-only. No regression for existing M4 tests.
