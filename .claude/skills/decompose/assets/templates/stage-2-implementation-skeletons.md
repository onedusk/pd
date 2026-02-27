# Stage 2: Implementation Skeletons — [Project Name]

> Code-level starting points derived from the Design Pack (Stage 1).
> All code in this document must compile/parse in the target language — this is NOT pseudocode.
>
> Delete the HTML comments as you fill in each section.

---

## Data Model Code

<!-- Translate every entity from Stage 1's data model into actual type definitions.
     Include: all fields with types, relationships, enums, initialization logic,
     validation constraints, and helper utilities.

     This code will be copy-pasted as the starting point for implementation.
     It must be complete enough to compile on its own. -->

### File: `[path/to/models/file]`

```[language]
// --- Enums ---

// [Define all enums referenced by the data model.
//  Include all cases. Make them serializable if needed.]


// --- Models / Entities ---

// [Define all model classes/structs/tables from Stage 1.
//  Include: all fields, relationships, initializers, unique constraints.]


// --- Helpers ---

// [Date formatters, ID generators, normalization functions,
//  or any utility the models depend on.]
```

<!-- Repeat for additional model files if your project organizes them separately. -->

---

## Interface Contracts *(if applicable)*

<!-- Typed request/response structures for any API surface your system exposes or consumes:
     tool-call schemas, REST endpoint DTOs, GraphQL types, RPC definitions,
     CLI argument types, webhook payloads, etc.

     Include serialization format decisions (JSON encoding strategy, date format, etc.). -->

### File: `[path/to/contracts/file]`

```[language]
// --- Request / Argument Types ---

// [One typed struct per operation.
//  Include all fields with types and nullability.
//  Group by category (read operations, write operations, etc.).]


// --- Response / Result Types ---

// [One typed struct per operation.
//  Include summary/preview types if full entities are too heavy for responses.]


// --- Serialization Configuration ---

// [Date format, JSON encoding settings, custom codable implementations,
//  or any configuration needed for correct serialization.]
```

---

## Documentation Artifacts

<!-- Markdown reference documents derived from the code above.
     These serve as human-readable summaries for onboarding and review.
     They should mirror the code — not duplicate it by hand, but summarize it. -->

### Data Model Reference

<!-- Entity-by-entity reference: key, fields, relationships.
     Keep it structured and scannable. -->

#### [Entity Name]

**Key:** `fieldName` (type)

**Fields:** `field1`, `field2`, `field3`, ...

**Relationships:** `relName` → `Target` (cardinality, delete rule)

<!-- Repeat for each entity. -->

### API / Interface Reference *(if applicable)*

<!-- Operation-by-operation reference: name, inputs, outputs, semantics. -->

#### [Operation Name]

**Input:** `field1` (type, required), `field2` (type, optional), ...

**Output:** `resultField1`, `resultField2`, ...

**Semantics:** [what this operation does, any side effects]

<!-- Repeat for each operation. -->

### Example Payloads *(if applicable)*

<!-- Concrete JSON/XML/protobuf examples for key operations.
     Include both request and response examples. -->

**Example: [Operation Name]**

Request:
```json
{
  "field1": "value",
  "field2": 42
}
```

Response:
```json
{
  "result": {
    "id": "abc-123",
    "status": "created"
  }
}
```

---

## Before Moving On

Verify before proceeding to Stage 3:

- [ ] Every entity from Stage 1's data model has corresponding code
- [ ] All enums are defined with cases matching the design pack
- [ ] Relationships and delete rules match the schema specification
- [ ] Helper functions for common operations are included
- [ ] Code compiles / parses without errors in the target language
- [ ] Interface contracts cover every API operation listed in Stage 1 (if applicable)
- [ ] Documentation artifacts accurately reflect the code
- [ ] Non-obvious decisions have inline comments explaining them
