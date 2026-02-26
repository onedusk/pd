# Stage 3 Example: Task Index

> Complete task index from a real iOS project (Arkived — a local-first journal and activity archive).
> This index was derived from a 20-page design pack and a skeleton code file,
> and it drove all implementation work across 9 milestones and 57 tasks.

---

## Legend

- `[ ]` = Not started
- `[x]` = Complete
- **CREATE** = new file | **MODIFY** = edit existing | **DELETE** = remove
- Task IDs: `T-{milestone}.{sequence}` (e.g., T-01.03 = Milestone 1, task 3)

---

## Progress

| # | Milestone | File | Tasks | Done |
|---|-----------|------|:-----:|:----:|
| M1 | SwiftData Models + Repositories | tasks_m01.md | 7 | 7 |
| M2 | Today/Day Screens + CRUD | tasks_m02.md | 14 | 14 |
| M3 | Calendar Tab | tasks_m03.md | 4 | 4 |
| M4 | Search Tab (Keyword) | tasks_m04.md | 6 | 6 |
| M5 | Attachments (Photo/Link) | tasks_m05.md | 5 | 5 |
| M6 | Export/Backup + Settings | tasks_m06.md | 5 | 5 |
| M7 | EventKit Import | tasks_m07.md | 5 | 5 |
| M8 | Assistant (Foundation Models) | tasks_m08.md | 7 | 7 |
| M9 | Semantic Embeddings + Hybrid Ranking | tasks_m09.md | 4 | 4 |
| | **Total** | | **57** | **57** |

---

## Milestone Dependencies

```
M1 ──► M2 ──┬──► M3 ─────────────────────────────────────┐
             │                                             │
             ├──► M4 (parallel with M3) ──────────────┐    │
             │                                        │    │
             ├──► M5 (needs CRUD from M2) ──┐         │    │
             │                              │         │    │
             ├──► M6 (parallel with M3–M5)  │         │    │
             │                              │         │    │
             └──► M7 (needs use cases)      │         │    │
                                            ▼         ▼    ▼
                                     M1 + M2 + M4 ──► M8 ──► M9
```

**Strict sequential path:** M1 → M2 → M3+M4 (parallel) → M5 → M6 → M7 → M8 → M9

---

## Target Directory Tree

All files produced by the task list. `(MX)` = milestone that creates it.

```
arkived/
  arkivedApp.swift                          MODIFY (M1, M2)
  ContentView.swift                         MODIFY (M1) → dead code after M2
  Item.swift                                DELETE (M1)
  Info.plist                                MODIFY (M6, M7)
  arkived.entitlements                      existing
  Assets.xcassets/                          existing

  Models/
    ArkivedModels.swift                     CREATE (M1)

  Repositories/
    SwiftDataStore.swift                    CREATE (M1)
    AttachmentStore.swift                   CREATE (M1), MODIFY (M5)

  UseCases/
    CreateActivityUseCase.swift             CREATE (M2), MODIFY (M5)
    UpdateActivityUseCase.swift             CREATE (M2), MODIFY (M5)
    DeleteActivityUseCase.swift             CREATE (M2)
    UpsertDayEntryUseCase.swift             CREATE (M2)
    SearchUseCase.swift                     CREATE (M4), MODIFY (M9)
    ImportCalendarUseCase.swift             CREATE (M7)

  ViewModels/
    TodayViewModel.swift                    CREATE (M2)
    DayDetailViewModel.swift                CREATE (M2)
    ActivityEditorViewModel.swift           CREATE (M2)
    CalendarViewModel.swift                 CREATE (M3)
    SearchViewModel.swift                   CREATE (M4)
    AssistantViewModel.swift                CREATE (M8)

  Views/
    MainTabView.swift                       CREATE (M2), MODIFY (M3, M4, M8)
    Today/
      TodayView.swift                       CREATE (M2), MODIFY (M6)
      DayDetailView.swift                   CREATE (M2), MODIFY (M5)
      ActivityRowView.swift                 CREATE (M2), MODIFY (M5)
      ActivityEditorSheet.swift             CREATE (M2), MODIFY (M5)
      DayNoteEditorView.swift               CREATE (M2)
    Calendar/
      CalendarView.swift                    CREATE (M3)
      CalendarDayCell.swift                 CREATE (M3)
    Search/
      SearchView.swift                      CREATE (M4)
      SearchFilterRow.swift                 CREATE (M4)
      SearchResultRow.swift                 CREATE (M4)
    Assistant/
      AssistantChatView.swift               CREATE (M8)
      ToolActionPreviewRow.swift            CREATE (M8)
    Attachments/
      AttachmentPreviewView.swift           CREATE (M5)
    Settings/
      SettingsView.swift                    CREATE (M6), MODIFY (M7)

  Services/
    ExportService.swift                     CREATE (M6)
    NotificationService.swift               CREATE (M6)
    AIService.swift                         CREATE (M8)
    EmbeddingService.swift                  CREATE (M9)
    SearchIndexService.swift                CREATE (M9)

  Importers/
    CalendarImporter.swift                  CREATE (M7)

  Assistant/
    AssistantToolSchema.swift               CREATE (M8)
    ToolRouter.swift                        CREATE (M8)

arkivedTests/
  arkivedTests.swift                        existing placeholder
  DateNormalizationTests.swift              CREATE (M1)
  SearchTests.swift                         CREATE (M4)
  ImportDedupeTests.swift                   CREATE (M7)
  ToolCallValidationTests.swift             CREATE (M8)

arkivedUITests/
  arkivedUITests.swift                      existing placeholder
  arkivedUITestsLaunchTests.swift           existing placeholder
```

**Totals:** 36 files created, 15 modifications, 1 deletion
