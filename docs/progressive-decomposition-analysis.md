# Progressive Decomposition -- Analysis

## Part 1: Logic Gaps

### 1.1 The Stage 2 Justification Has a Scope Mismatch

The process guide argues that Stage 2 (Implementation Skeletons) catches ambiguities that prose can't -- a field can't be simultaneously required and nullable in a type system. This is true and well-argued. But the scope of what Stage 2 actually produces is narrower than the scope of what Stage 1 specifies.

Stage 1 covers architecture, UI/UX layout, features, integration points, security plans, and testing strategy. Stage 2 only materializes the data model and interface contracts. That means the architecture layer, the concurrency model, the navigation structure, the security plan, and the testing strategy all pass through to Stage 3/4 as prose -- the exact medium that Stage 2 is supposed to distrust.

This isn't necessarily wrong (materializing architecture as code pre-implementation is a different kind of exercise), but the argument as written implies Stage 2 is a general-purpose ambiguity filter, when it actually only filters data model and API contract ambiguities. The non-data-model sections of Stage 1 get no such verification.

**Potential fix:** Either narrow the claim ("Stage 2 catches data model ambiguities that prose descriptions hide") or expand Stage 2's scope to include architecture skeleton code (e.g., dependency injection containers, route registrations, middleware chains) that would catch architectural contradictions.

### 1.2 Feedback Loops Are Described But Not Procedurally Bounded

The process guide lists four feedback loops (Stage 2 -> Stage 1, Stage 3 -> Stage 4, Stage 4 -> Stage 2, Stage 4 -> Stage 1) and says they are "expected and healthy." But there is no guidance on when to stop iterating vs. when to accept a known imperfection and move forward.

The "when to iterate vs. when to ship" section gives three conditions for shipping (clear acceptance criteria, skeleton code compiles, no dependency cycles) but these are all binary checks on final state. They don't address the common real-world scenario: you're in feedback loop iteration 3, you keep finding new ambiguities each time you fix the previous batch, and you need a heuristic for "good enough."

The FAQ addresses this obliquely ("Do NOT iterate on cosmetic improvements") but there's a gap between "cosmetic" and "the schema has a genuine ambiguity that I could fix now or defer to implementation." Experienced engineers have intuition for this; less experienced ones (and AI agents) do not.

**Potential fix:** Add a convergence heuristic. Something like: "If a feedback loop iteration reveals only additive changes (new fields, new methods) rather than structural changes (changed relationships, changed cardinality, changed architectural layers), the design is converging and you should ship."

### 1.3 Stage 3 Milestone Dependency Graph Underspecifies Intra-Milestone Dependencies

Stage 3 draws dependencies between milestones, and Stage 4 draws dependencies between tasks within and across milestones. But Stage 3 only specifies milestone-level ordering. The directory tree in Stage 3 shows which milestones touch which files, but doesn't capture *why* the dependency exists -- which specific output of M1 does M2 need?

This matters for the agent-parallel-design.md document, which proposes cross-stage parallelism ("Stage 4 milestones can start as soon as their section of Stage 3 is written"). To actually parallelize, you need to know the specific artifact dependencies, not just the milestone ordering.

In the Arkived example, M3 (Calendar Tab) and M4 (Search Tab) both depend on M2 (Today/Day Screens + CRUD), but they depend on different parts of M2. M3 needs the date navigation infrastructure; M4 needs the SwiftDataStore search method. If those parts of M2 were completed at different times, M3 and M4 could start at different times. Stage 3 as currently specified can't express this.

**Potential fix:** Add an optional "dependency rationale" column to the milestone dependency graph that names the specific artifact (file, type, method) that creates the dependency. This would also directly feed the agent-parallel system's task assignment logic.

### 1.4 No Explicit Handling of Cross-Cutting Concerns

The methodology organizes work into milestones that are "independently testable" and ideally deployable. But cross-cutting concerns (logging, error handling, analytics, accessibility, localization) don't fit neatly into milestones because they touch every file.

The Arkived example sidesteps this by being a greenfield iOS project where these concerns are either handled by the platform (accessibility) or deferred (analytics, localization). But for a production project, these are real: "add error handling to every API call" is not a single milestone and not a single task -- it's a concern that modifies the acceptance criteria of every other task.

**Potential fix:** Add guidance for cross-cutting concerns in the Stage 3 section. Options include: (a) a dedicated early milestone that establishes the patterns (error handling utilities, logging infrastructure), with later tasks inheriting those patterns, (b) a "cross-cutting standards" appendix to Stage 0 that becomes implicit acceptance criteria for all tasks, or (c) explicit acknowledgment that cross-cutting concerns are handled per-task and should be reflected in Stage 4 acceptance criteria.

### 1.5 The "Once Per Project" Frequency Claim Is Optimistic

The pipeline table says Stages 1-4 are "Once per project." The "Living Specs" section then describes how to handle requirement changes mid-project, with detailed change propagation rules. These two statements are in tension.

In practice, for any project longer than a few weeks, Stages 1-4 become living documents that are modified multiple times. The methodology does address this (the change propagation rules are well-thought-out), but the pipeline table gives a misleading first impression. A reader could look at the table, think "I do this once and then code," and miss the living-specs section entirely.

**Potential fix:** Change the "Frequency" column to something like "Initial pass: once per project. Updates: as requirements change (see Living Specs)."

### 1.6 The Agent-Parallel Design Has an Unaddressed Merge Coherence Problem

The agent-parallel-design.md says merge strategy is "section-based concatenation" and "no conflict is possible because sections are assigned before fan-out." This is true at the structural level (sections don't overlap), but it doesn't address semantic coherence.

If Research Agent A investigates the platform and discovers a framework limitation, and Research Agent B investigates integrations without knowing about that limitation, their outputs will concatenate cleanly but may be semantically contradictory. Agent B might recommend an integration approach that's impossible given Agent A's findings.

The Resolved Questions section says "no LLM merge pass needed," but this seems premature. A coherence validation pass -- even if it's just checking that all sections are consistent with each other -- would catch this class of error.

**Potential fix:** Add a lightweight coherence check after merge: the orchestrator (or a dedicated validation step) reads the concatenated output and flags potential contradictions. This doesn't need to be a full LLM re-write, just a scan for inconsistencies.

### 1.7 Stage 4 Task Size Guidance Lacks Calibration for AI Agents

Tasks are sized at "15 minutes to 2 hours" of human work. The methodology explicitly targets AI-assisted development, but never recalibrates task granularity for AI agents.

An AI agent working through a task list has different characteristics than a human: it can produce boilerplate code faster, but it's worse at tasks requiring judgment calls, debugging runtime behavior, or integrating with unfamiliar APIs. A task that takes a human 15 minutes (like "delete Item.swift") might take an AI agent 30 seconds. A task that takes a human 2 hours (like "implement hybrid search ranking") might take an AI agent much longer due to iteration loops.

More importantly, AI agents have context windows. A task specification that fits comfortably in a human's working memory (read the outline, implement, verify) might exceed what an AI can hold alongside the relevant skeleton code and design context.

**Potential fix:** Add a note in the "Adapting The Methodology / For AI-Assisted Development" section about task granularity recalibration. AI agents benefit from more granular tasks for complex logic and coarser tasks for boilerplate. Consider a "context budget" column in task specs that estimates how much design context the implementer needs loaded.

---

## Part 2: Factual Inaccuracies

### 2.1 SDD "Recognized as One of 2025's Key Engineering Practices" -- Overstated

SDD appears in the Thoughtworks Technology Radar Volume 32 (April 2025), but in the **Assess** ring, not Adopt or Trial. "Assess" means "worth exploring to understand how it will affect your enterprise." Calling it "one of 2025's key engineering practices" is a stretch -- "recognized as an emerging technique" would be more accurate.

**Location:** process-guide.md, line 13; README.md, line 5-6.

### 2.2 "No Standard PDR Format Exists in the Literature" -- Inaccurate

The README (line 85) and process guide (line 174) claim that "No standard PDR format exists in the literature." There are published PDR specifications, including a structured "Process Decision Record" format by Valentin Lefebvre (Medium, 2024) with fields for Identifier, Context, Decision, Alternatives, Rationale, Success Criteria, Next Steps, and References. There is also a Product Decision Record practice documented at dinker.in targeting enterprise product managers.

These are not as widely adopted as ADRs, and the claim that PDRs are *less established* than ADRs is fair. But "no standard format exists" is factually incorrect.

**Suggested fix:** Change to something like "No widely adopted standard for product decision records exists, though some practitioners have proposed formats. This methodology provides a structured PDR template inspired by the ADR format."

### 2.3 MCP Go SDK Star Count -- Minor Inaccuracy

The agent-parallel-design.md (line 376) claims "3.9k stars" for the MCP Go SDK. The actual count as of research is approximately 3.5k. This is a moving target and not a major issue, but worth correcting or removing the specific number since it will go stale.

### 2.4 A2A Version Precision

The agent-parallel-design.md references A2A v0.3.0 as the version being designed against. The A2A specification.md in the local /A2A folder also shows version 0.3.0, which is consistent. However, the claim that it was "donated to Linux Foundation" is slightly imprecise -- A2A was *created* by Google and the project was *launched* under the Linux Foundation governance (June 2025), not donated in the traditional sense of transferring an existing mature project.

This is a minor wording issue, not a factual error.

---

## Part 3: Mathematical Formalization Assessment

The core question: can the progressive decomposition pipeline be formalized as a mathematical algorithm or formula?

### 3.1 What the Pipeline Actually Does (Abstractly)

Strip away the domain-specific language and the pipeline performs the following operations:

1. **Stage 0:** Define a constraint set C (development standards) that applies to all subsequent outputs.
2. **Stage 1:** Given an informal specification I and constraint set C, produce a structured specification S that includes entities E, relationships R, architectural decisions D, and an ordered milestone list M.
3. **Stage 2:** Given S, produce a formal type system T = (types, fields, constraints, relationships) such that T is internally consistent (compiles) and is a faithful materialization of E and R from S.
4. **Stage 3:** Given S, T, and M, produce a directed acyclic graph G = (V, E_dep) where V = milestones, E_dep = dependency edges, plus a file mapping F: files -> milestones.
5. **Stage 4:** Given G and F, for each milestone m in V, produce a set of tasks {t_1, ..., t_k} where each task specifies a file, an action, dependencies (a subset of previously produced tasks), an implementation outline, and acceptance criteria.

### 3.2 What Is Formalizable

Several components of this pipeline have clean mathematical representations:

**3.2.1 The Dependency Graph (Stage 3) Is a DAG**

The milestone dependency graph is a directed acyclic graph. This is well-studied. The pipeline's "critical path" is the longest path in the DAG, computable in O(V + E) via topological sort. The "parallelizable work" corresponds to the DAG's width (the maximum antichain, computable via Dilworth's theorem or Mirsky's theorem).

Formally: Let G = (M, D) where M = {m_1, ..., m_n} are milestones and D is a set of directed edges (m_i, m_j) meaning m_i must complete before m_j can start.

- Critical path length: max over all paths from source to sink of the sum of task durations
- Maximum parallelism: width of the DAG = max |A| where A is an antichain (set of pairwise incomparable elements under the partial order induced by D)

**3.2.2 The Task Dependency Graph (Stage 4) Is Also a DAG**

Extending the same structure: let G' = (T, D') where T = all tasks across all milestones, D' = task-level dependencies. Same algorithms apply. This is essentially a more granular version of the milestone DAG.

**3.2.3 The Stage Sequence Is a Monotone Refinement**

Each stage takes the output of the previous stage and produces a strictly more concrete representation. This maps to the concept of a **refinement chain** in formal methods:

    I (informal) >= S (structured) >= T (typed) >= G (graph) >= Tasks (executable)

where >= means "refines" (is more concrete than). This is analogous to stepwise refinement (Dijkstra/Wirth), where an abstract program is progressively detailed until it becomes executable.

**3.2.4 The Feedback Loops Are Fixed-Point Iteration**

The feedback loops (Stage 2 reveals Stage 1 ambiguities, Stage 4 reveals Stage 2 gaps) can be modeled as a fixed-point computation. Define a function f that maps a pipeline state (S, T, G, Tasks) to a refined state by checking consistency and propagating corrections. The pipeline converges when f(state) = state (no more corrections needed).

This maps to Kleene's fixed-point theorem: if f is monotone (each iteration only adds specificity, never removes it) and the domain is a complete lattice (there's a most-specific and least-specific state), then f has a least fixed point reachable by iterating from the bottom.

In practice, the pipeline's convergence is not formally guaranteed (a design change could cascade indefinitely in theory), but the "convergence heuristic" mentioned in the Logic Gaps section (only additive changes = converging) is an informal version of checking monotonicity.

### 3.3 What Is NOT Cleanly Formalizable

**3.3.1 The Specification Step (Stage 1) Is Not Algorithmic**

Going from an informal idea I to a structured specification S requires creative judgment, domain knowledge, and research. This is not a computable function -- it's a design activity. No algorithm takes "I want a journal app" and produces a correct data model. (LLMs approximate this, but they're doing pattern matching on training data, not computing a function.)

**3.3.2 The Skeleton Writing (Stage 2) Requires Interpretation**

Translating a prose schema into compilable code requires interpreting ambiguities. "Optional start time" could mean nullable, default-valued, or absent. The *whole point* of Stage 2 is to force these interpretations to be made explicitly. But the interpretation itself is a human judgment, not a computation.

**3.3.3 Task Decomposition Granularity Is Subjective**

"15 minutes to 2 hours" is a heuristic, not a formal constraint. Two reasonable people will decompose the same milestone into different task sets. The methodology acknowledges this ("split larger tasks, merge trivial ones") but there's no function from milestone to optimal task set.

### 3.4 A Candidate Formalization

Given the above, the pipeline can be partially formalized as a **refinement calculus with DAG scheduling**. Here's a sketch:

**Definition (Progressive Decomposition).** A progressive decomposition of an informal specification I under constraints C is a tuple (S, T, G, Tasks) where:

1. S is a structured specification satisfying C
2. T is a type system such that:
   - Every entity in S.entities has a corresponding type in T
   - T compiles (is internally consistent)
   - T is faithful to S (every field, relationship, and constraint in S has a corresponding element in T)
3. G = (M, D) is a DAG where:
   - M is a partition of the work implied by S into milestones
   - D encodes ordering constraints
   - G is acyclic
   - Every feature in S maps to at least one milestone in M
4. Tasks is a function M -> P(Task) where each task t has:
   - t.file: a specific file path
   - t.action in {CREATE, MODIFY, DELETE}
   - t.deps: a subset of tasks from earlier milestones or earlier in the same milestone
   - t.acceptance: a set of binary predicates

**Properties:**

- **Completeness:** Every feature in S is covered by at least one task in Tasks(m) for some m.
- **Consistency:** The task dependency graph (union of all t.deps across all milestones) is acyclic.
- **Refinement monotonicity:** |S| <= |T| <= |G| <= |Tasks| in information content (each stage is strictly more specific).
- **Schedulability:** The DAG G admits a valid topological ordering. The critical path through G determines the minimum sequential execution time.

**The formal part** is the DAG structure, completeness checking, consistency checking, and scheduling. **The informal part** is producing S, T, and the task decomposition -- these require judgment.

### 3.5 Relationship to Existing Formalisms

The closest existing formalisms are:

1. **Stepwise Refinement (Dijkstra, 1972; Wirth, 1971):** The idea that a program is developed by progressively refining an abstract specification into concrete code. Progressive Decomposition applies this to the *pre-coding* phase, refining a project idea into an executable task list rather than refining a program into executable code.

2. **Work Breakdown Structure (WBS) in Project Management:** WBS decomposes a project into deliverables and work packages. Stage 3 is essentially a WBS. The WBS standard (PMI Practice Standard for Work Breakdown Structures, 3rd edition) formalizes the decomposition rules. Progressive Decomposition adds the data-model-first requirement and the type-system verification step, which WBS does not include.

3. **PERT/CPM (Program Evaluation and Review Technique / Critical Path Method):** The milestone dependency graph and critical path analysis in Stage 3 map directly to PERT/CPM. The mathematical framework for computing critical paths, slack times, and optimal schedules is well-established (1950s, developed by the US Navy and DuPont respectively).

4. **Formal Specification Languages (Z, VDM, B Method):** These share the "refine an abstract spec into concrete code" philosophy but operate at a much more rigorous level (mathematical proofs of refinement correctness). Progressive Decomposition is closer in spirit to these than to Agile methodologies, but operates at a pragmatic rather than mathematical level of rigor.

5. **Lattice-based Fixed-Point Computation (Tarski, 1955; Kleene):** The feedback loops map to fixed-point iteration over a lattice of specification states. This is the same mathematical structure used in abstract interpretation (Cousot & Cousot, 1977) and dataflow analysis.

### 3.6 Transduction Algebra

A natural question is whether the pipeline can be modeled using transduction algebra -- the algebraic study of structure-to-structure transformations. Each stage takes structured input and produces structured output, which is what a transducer does, and the pipeline is a composition of transductions. There are several relevant branches of the theory.

**Where the mapping holds:**

*Graph transductions (Stages 3-4).* The transformation from milestone DAG to task dependency graph is a graph-to-graph transduction. Courcelle's MSO (Monadic Second-Order) transductions provide a framework for reasoning about such transformations. A key result: MSO transductions preserve decidability of graph properties on structures of bounded treewidth. A milestone DAG for a 5-12 milestone project has very bounded treewidth. This means if you can check a property (acyclicity, feature coverage completeness) on the coarse milestone graph, the transduction to the fine-grained task graph preserves that decidability. That is a real, useful result -- it tells you that verification at the milestone level carries through to the task level, not just by convention but by mathematical guarantee.

*Composition closure.* If each stage is modeled as a transduction T_i, the pipeline is the composition T_0 ; T_1 ; T_2 ; T_3 ; T_4. Rational transductions are closed under composition, meaning the composite function inherits tractability properties from the individual stages. In practical terms: if each stage individually preserves consistency (no contradictions introduced), the pipeline as a whole preserves consistency. This provides a formal justification for why staged decomposition is structurally safer than jumping from idea to tasks in a single step -- you can verify intermediate representations, and the verified properties survive composition.

*Krohn-Rhodes decomposition (suggestive parallel).* The Krohn-Rhodes theorem states that any finite-state transducer can be decomposed into a cascade of prime components (simple groups and reset automata). There is a structural echo: progressive decomposition claims that a complex design-to-tasks transformation can be decomposed into a cascade of simpler, typed transformations. But this parallel is suggestive rather than rigorous -- the pipeline stages are not finite-state, and the Krohn-Rhodes machinery does not directly apply.

**Where the mapping breaks down:**

*Stages 1 and 2 are not transductions in any algebraic sense.* A transduction is a function (or relation) between formal languages or structures. "Take an informal project idea and produce a structured specification" has no formal input language -- the input is natural language, sketches, conversations, and domain knowledge. You could model the output of Stage 1 formally (a spec as a tuple of entities, relationships, decisions), but the transformation itself is not mechanizable. Wrapping it in transduction notation would give a false sense of rigor without enabling any actual reasoning.

*Feedback loops break the cascade model.* Classical transducer cascades are feed-forward: the output of stage i feeds into stage i+1. The feedback loops (Stage 2 correcting Stage 1, Stage 4 updating Stage 2) make this a cyclic system. Streaming transducers with registers (Alur, 2010s) can model stateful, feedback-aware transformations, but the machinery is significantly heavier than the fixed-point iteration model described in Section 3.2.4, and it doesn't yield additional insights for this application. The fixed-point framing already captures the essential property (convergence under monotone refinement) without the overhead.

*The algebraic properties don't help with the hard problems.* The useful results from transduction algebra (composition closure, decidability preservation) apply to the algorithmic phases that are already well-served by simpler formalisms (DAG algorithms, graph theory). The creative phases, where formalization would actually add value if it were possible, resist transduction modeling entirely.

**Assessment:**

Transduction algebra provides two genuine contributions to the theoretical grounding: (1) composition closure as a formal justification for staged decomposition over single-step decomposition, and (2) Courcelle's decidability preservation for the graph transformation phases. These are worth citing. But modeling the full pipeline as a transduction algebra requires either treating the creative stages as black-box oracles (which discards the algebraic properties) or pretending they are formal transformations (which they are not). The stepwise refinement + DAG scheduling framing from Section 3.4 captures the same essential structure with less overhead, and the transduction results can be incorporated as supporting arguments within that framing rather than as an alternative formalism.

### 3.7 Recommendation

The pipeline should not be forced into a single formula -- it's a multi-phase process where some phases are algorithmic and others are creative. But the following components can and should be formalized:

1. **DAG scheduling** for milestones and tasks (this is PERT/CPM, already well-formalized).
2. **Completeness checking**: a coverage function that maps features to milestones to tasks and verifies no feature is orphaned.
3. **Consistency checking**: a dependency validator that confirms the task graph is acyclic and all referenced dependencies exist.
4. **Convergence detection** for feedback loops: a function that classifies changes as structural vs. additive and signals when the pipeline is converging.

These four could be implemented as concrete algorithms (and would be natural MCP tools for the Planning Agent in the agent-parallel system). The creative phases (Stages 1 and 2) resist formalization and should remain human/LLM-driven.

---

## Part 4: Summary of Recommendations

### Factual Corrections Needed

| Item | Current | Suggested |
|------|---------|-----------|
| SDD characterization | "one of 2025's key engineering practices" | "an emerging technique recognized in 2025" (Assess ring) |
| PDR novelty claim | "No standard PDR format exists in the literature" | "No widely adopted standard exists, though some practitioners have proposed formats" |
| MCP Go SDK stars | "3.9k stars" | Remove specific count or check at time of publication |

### Logic Gaps to Address

| Gap | Severity | Suggested Fix |
|-----|----------|---------------|
| Stage 2 scope vs. claim mismatch | Medium | Narrow the claim or expand Stage 2 scope |
| Feedback loop convergence | Medium | Add convergence heuristic |
| Milestone dependency granularity | Low-Medium | Add optional dependency rationale |
| Cross-cutting concerns | Medium | Add guidance in Stage 3 |
| "Once per project" frequency | Low | Update frequency column |
| Agent merge coherence | Medium (for agent-parallel doc) | Add coherence validation step |
| AI task size calibration | Low-Medium | Add recalibration note for AI workflows |

### Formalization Verdict

The pipeline is partially formalizable. The DAG scheduling, completeness checking, consistency validation, and convergence detection map cleanly to existing mathematical frameworks (PERT/CPM, lattice theory, graph algorithms). Transduction algebra contributes composition closure and decidability preservation results for the graph transformation phases. The creative phases (specification and skeleton writing) do not and should not be forced into formulas. The closest existing formalism is **stepwise refinement with DAG-scheduled work breakdown**, drawing on Dijkstra/Wirth for the refinement philosophy, PERT/CPM for the scheduling mathematics, and Courcelle's MSO transductions for the graph transformation guarantees.

---

## Part 5: Next Steps -- Applying Findings to the Project

The following is an ordered list of concrete changes to the project files, grouped by effort level. Each item names the file(s) to modify and describes the change.

### 5.1 Quick Fixes (factual corrections, wording adjustments)

These can be done in a single editing pass with no structural changes.

**1. Fix SDD characterization.**
Files: `README.md` (line 5-6), `process-guide.md` (line 13), `skill/decompose/references/process-guide.md` (same section).
Change: Replace "recognized by Thoughtworks' Technology Radar as one of 2025's key engineering practices" with "recognized as an emerging technique in Thoughtworks' Technology Radar (2025, Assess ring)." The Assess ring placement is accurate and still credible -- no need to oversell.

**2. Fix PDR novelty claim.**
Files: `README.md` (line 85), `process-guide.md` (line 174), `skill/decompose/references/process-guide.md`.
Change: Replace "No standard PDR format exists in the literature" with "No widely adopted standard for product decision records exists, though some practitioners have proposed formats (notably Valentin Lefebvre's Process Decision Record specification and enterprise-focused Product Decision Record templates). This methodology provides a structured PDR format inspired by the ADR pattern."

**3. Fix MCP Go SDK star count.**
File: `docs/internal/agent-parallel-design.md` (line 376).
Change: Either remove the specific star count ("well-starred, active repository") or replace with "3.5k+ stars" and accept it will need periodic updating.

**4. Fix A2A "donated" wording.**
File: `docs/internal/agent-parallel-design.md` (line 29).
Change: Replace "donated to Linux Foundation" with "launched under Linux Foundation governance (June 2025)."

**5. Update pipeline frequency column.**
Files: `README.md` (pipeline table), `process-guide.md` (if the table is duplicated there).
Change: Replace "Once per project" with "Once per project (initial pass); updated as requirements change" for Stages 1-4. Add "(see Living Specs)" as a parenthetical or footnote.

### 5.2 Medium Effort (logic gap fixes requiring new prose)

These require writing new content but don't change the structure of the methodology.

**6. Narrow the Stage 2 ambiguity-catching claim.**
Files: `process-guide.md` (the "Why Code Before Tasks" section, starting line 262), `README.md` (line 63).
Change: The current argument says "prose is ambiguous in ways that code is not" as a general claim, then gives data-model examples. Add a scoping sentence: "Stage 2 specifically targets data model and interface contract ambiguities -- the classes of ambiguity where type systems provide the clearest disambiguation. Architectural ambiguities (concurrency model, layer boundaries, dependency directions) are partially addressed by the architectural pattern choice in Stage 1, but are not formally verified until implementation." This is honest and avoids the scope mismatch without undermining the argument.

**7. Add feedback loop convergence heuristic.**
File: `process-guide.md` (the "When To Iterate vs. When To Ship" section, line 467-479).
Change: Add a new subsection or paragraph after the existing iterate/ship criteria:

"Convergence heuristic: if a feedback loop iteration reveals only additive changes (new fields, new helper methods, additional acceptance criteria) rather than structural changes (altered relationships, changed cardinality, reordered milestones, removed entities), the design is converging. Ship when two consecutive iterations produce only additive changes. If structural changes persist after three iterations, the design likely has a foundational ambiguity that should be resolved by going back to assumptions (Stage 1, Section 1) rather than continuing to iterate at the current level."

**8. Add cross-cutting concerns guidance.**
File: `process-guide.md` (Stage 3 section, after "Milestone Design Rules" around line 378).
Change: Add a new subsection:

"Cross-Cutting Concerns. Some concerns (error handling, logging, analytics, accessibility, localization) span all milestones rather than fitting into one. Handle these by establishing patterns in an early milestone: M1 should include the error handling utilities, logging infrastructure, or accessibility conventions that later milestones inherit. Reference these patterns in Stage 0's development standards so they become implicit acceptance criteria for every task. If a cross-cutting concern emerges mid-project, add it to Stage 0 and propagate to unfinished milestones using the change propagation rules."

**9. Add AI task size calibration note.**
File: `process-guide.md` (the "For AI-Assisted Development" section, starting line 509).
Change: Add a paragraph after the existing AI-adaptation content:

"Task granularity recalibration: AI agents have different performance profiles than human developers. They are faster at boilerplate and structural code, slower at tasks requiring runtime debugging or judgment calls about ambiguous requirements. They also have context windows -- a task specification that fits in a human's working memory may exceed what an AI can hold alongside relevant skeleton code and design context. When writing tasks for AI execution, consider splitting complex-logic tasks into smaller units and merging boilerplate tasks into larger batches. If a task outline exceeds roughly 20 lines or references more than 3 cross-file dependencies, it may benefit from splitting."

**10. Add coherence validation to agent-parallel design.**
File: `docs/internal/agent-parallel-design.md` (Resolved Questions section, item 5 on merge strategy, line 419).
Change: Amend the merge strategy resolution to add: "After section-based concatenation, the orchestrator runs a lightweight coherence check: it reads the merged output and flags potential contradictions between sections (e.g., a platform limitation discovered by one Research Agent instance that conflicts with an integration approach proposed by another). This does not require a full LLM re-generation pass -- a focused scan for cross-section consistency is sufficient. If contradictions are found, the orchestrator routes them back to the relevant agents as INPUT_REQUIRED tasks."

### 5.3 Larger Effort (structural additions)

These add new content to templates or introduce new concepts.

**11. Add optional dependency rationale to Stage 3.**
Files: `process-guide.md` (Stage 3 section), `templates/stage-3-task-index.md`.
Change: In the milestone dependency graph section, add an optional "dependency artifact" annotation. The ASCII diagram format could be extended:

```
M1 ──► M2 [via: SwiftDataStore, ArkivedModels]
       ├──► M3 [via: date navigation from TodayView]
       ├──► M4 [via: searchActivities method]
```

This is optional (not all users need this granularity), but it directly feeds the agent-parallel system's task fan-out logic and makes the dependency rationale explicit for human readers too. Update the template to show the optional format with a comment explaining when it's worth the extra effort.

**12. Add a "Formal Foundations" section to the process guide.**
File: `process-guide.md` (new section, perhaps after the FAQ or as an appendix).
Change: Add a concise section that grounds the methodology in existing formalisms. This is not required reading for practitioners, but it strengthens the intellectual foundation and is useful for anyone evaluating the methodology academically or comparing it to formal methods. Content:

- The stage sequence is a refinement chain (Dijkstra/Wirth stepwise refinement).
- The milestone and task dependency structures are DAGs, schedulable via PERT/CPM.
- The feedback loops are fixed-point iterations (Kleene/Tarski).
- The graph transformations (Stages 3 to 4) are MSO transductions (Courcelle), inheriting decidability of consistency checks from the coarser structure.
- Staged composition inherits consistency preservation from composition closure of rational transductions.
- The creative phases (Stages 1, 2) are outside the formalizable boundary and should remain judgment-driven.

This section should be short (half a page) and reference-heavy rather than proof-heavy.

### 5.4 Recommended Order of Execution

For efficiency, group the work:

1. Items 1-5 (quick fixes) -- single editing pass across README.md, process-guide.md, agent-parallel-design.md, and the skill copy of the process guide.
2. Items 6-9 (medium effort, process-guide.md changes) -- one focused session on the process guide, since all four changes touch that file.
3. Item 10 (agent-parallel-design.md coherence fix) -- independent, can be done anytime.
4. Items 11-12 (structural additions) -- these benefit from being done together since the formal foundations section can reference the dependency rationale format as an example of where the formalism has practical utility.
