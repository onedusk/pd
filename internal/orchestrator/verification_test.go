package orchestrator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Stage 0 rules
// ---------------------------------------------------------------------------

func TestStage0Rules_AllSectionsPresent(t *testing.T) {
	content := `# Development Standards

## Code Change Checklist
1. Plan  2. Implement  3. Test

## Changeset Format
Semantic versioning with categories.

## Escalation Guidance
Four severity levels.

## Testing Guidance
Priority order for tests.`

	report := RunLocalVerification(StageDevelopmentStandards, content, nil)
	assert.True(t, report.Passed, "should pass when all 4 sections are present")
	assert.Empty(t, report.Findings)
}

func TestStage0Rules_MissingSections(t *testing.T) {
	content := `# Development Standards

## Code Change Checklist
1. Plan  2. Implement

## Testing Guidance
Write tests first.`

	report := RunLocalVerification(StageDevelopmentStandards, content, nil)
	assert.False(t, report.Passed, "should fail when sections are missing")

	var criticalIDs []string
	for _, f := range report.Findings {
		if f.Severity == SeverityCritical {
			criticalIDs = append(criticalIDs, f.ID)
		}
	}
	assert.Contains(t, criticalIDs, "V-0.02", "should flag missing Changeset Format")
	assert.Contains(t, criticalIDs, "V-0.03", "should flag missing Escalation Guidance")
}

// ---------------------------------------------------------------------------
// Stage 1 rules
// ---------------------------------------------------------------------------

func TestStage1Rules_CompleteDesignPack(t *testing.T) {
	content := `# Design Pack

## Assumptions and Constraints
- Local-only for v1
- Go implementation
- No cloud infra

## Platform and Tooling Baseline
Go 1.22, PostgreSQL 16

## Data Model
User has-many Posts (one-to-many)

## Architecture
Microservices with REST

## Features
- Authentication
- Authorization

## Integration Points
External OAuth provider

## Security and Privacy
JWT-based auth

## ADR-001 — Use Go
Persistence choice.

## ADR-002 — PostgreSQL
Framework choice.

## ADR-003 — REST
Data storage.

## PDR-001 — Simple Auth
Mental models.

## PDR-002 — Minimal UI
Friction choice.`

	report := RunLocalVerification(StageDesignPack, content, nil)
	assert.True(t, report.Passed, "should pass with complete design pack")
}

func TestStage1Rules_MissingADRs(t *testing.T) {
	content := `# Design Pack

## Assumptions and Constraints
- One assumption

## Platform and Tooling Baseline
Go 1.22

## Data Model
User entity with one-to-many relationships

## Architecture
Monolith

## Features
- Login

## Integration Points
None

## Security
Basic auth

## ADR-001 — Use Go
Only one ADR.`

	report := RunLocalVerification(StageDesignPack, content, nil)

	var found *VerificationFinding
	for _, f := range report.Findings {
		if f.Category == "methodology" && f.Section == "adrs" {
			found = &f
			break
		}
	}
	require.NotNil(t, found, "should flag insufficient ADRs")
	assert.Equal(t, SeverityCritical, found.Severity)
	assert.Contains(t, found.Description, "1 ADR")
}

func TestStage1Rules_MissingSections(t *testing.T) {
	content := `# Design Pack

## Architecture
Something about architecture.

## ADR-001 — A
## ADR-002 — B
## ADR-003 — C
## PDR-001 — A
## PDR-002 — B`

	report := RunLocalVerification(StageDesignPack, content, nil)
	assert.False(t, report.Passed, "should fail with missing required sections")

	categories := make(map[string]int)
	for _, f := range report.Findings {
		categories[f.Category]++
	}
	assert.Greater(t, categories["completeness"], 0, "should have completeness findings")
}

// ---------------------------------------------------------------------------
// Stage 2 rules
// ---------------------------------------------------------------------------

func TestStage2Rules_WithCodeBlocks(t *testing.T) {
	content := "# Implementation Skeletons\n\n```go\ntype User struct {\n\tID   int    `json:\"id\"`\n\tName string `json:\"name\"`\n}\n```\n"

	report := RunLocalVerification(StageImplementationSkeletons, content, nil)
	assert.True(t, report.Passed, "should pass with code blocks and JSON tags")
}

func TestStage2Rules_NoCodeBlocks(t *testing.T) {
	content := `# Implementation Skeletons

The User type should have an ID and Name field.
It should serialize to JSON.`

	report := RunLocalVerification(StageImplementationSkeletons, content, nil)
	assert.False(t, report.Passed, "should fail without code blocks")
	require.NotEmpty(t, report.Findings)
	assert.Equal(t, "V-2.01", report.Findings[0].ID)
}

func TestStage2Rules_NoSerializationMentioned(t *testing.T) {
	content := "# Skeletons\n\n```go\ntype User struct {\n\tID   int\n\tName string\n}\n```\n"

	report := RunLocalVerification(StageImplementationSkeletons, content, nil)
	// Should pass (no critical) but have a warning about serialization.
	assert.True(t, report.Passed)
	var hasSerWarning bool
	for _, f := range report.Findings {
		if f.ID == "V-2.02" {
			hasSerWarning = true
		}
	}
	assert.True(t, hasSerWarning, "should warn about missing serialization format")
}

// ---------------------------------------------------------------------------
// Stage 3 rules
// ---------------------------------------------------------------------------

func TestStage3Rules_Complete(t *testing.T) {
	content := `# Task Index

## Milestone Dependency Graph
M1 → M2 → M3

Critical path: M1 → M2 → M3

## Target Directory Tree
src/
  main.go (CREATE M1)

## M1: Foundation
## M2: Core
## M3: Integration`

	report := RunLocalVerification(StageTaskIndex, content, nil)
	assert.True(t, report.Passed)
}

func TestStage3Rules_MissingMilestones(t *testing.T) {
	content := `# Task Index

Some text without any milestone definitions.`

	report := RunLocalVerification(StageTaskIndex, content, nil)
	assert.False(t, report.Passed)
	assert.True(t, report.HasCritical())
}

func TestStage3Rules_NoCriticalPath(t *testing.T) {
	content := `# Task Index

## Milestone Dependency Graph
M1 → M2

## Target Directory Tree
src/main.go

## M1: Foundation
## M2: Core`

	report := RunLocalVerification(StageTaskIndex, content, nil)
	// Should have a warning about missing critical path.
	var hasCritPathWarning bool
	for _, f := range report.Findings {
		if f.ID == "V-3.03" {
			hasCritPathWarning = true
		}
	}
	assert.True(t, hasCritPathWarning, "should warn about missing critical path")
}

// ---------------------------------------------------------------------------
// Stage 4 rules
// ---------------------------------------------------------------------------

func TestStage4Rules_Complete(t *testing.T) {
	content := `# Task Specifications — M1

## T-01.01 — Initialize module

**File:** go.mod (CREATE)
**Depends on:** None

### Outline
Run go mod init.

### Acceptance
- go build ./... succeeds
- go mod tidy makes no changes

## T-01.02 — Add config

**File:** config.go (CREATE)
**Depends on:** T-01.01

### Outline
Create config struct.

### Acceptance
- Config struct compiles
- Tests pass`

	report := RunLocalVerification(StageTaskSpecifications, content, nil)
	assert.True(t, report.Passed)
}

func TestStage4Rules_NoTaskIDs(t *testing.T) {
	content := `# Task Specifications

## Task 1 — Do something
Write some code.

## Task 2 — Do another thing
Write more code.`

	report := RunLocalVerification(StageTaskSpecifications, content, nil)
	assert.False(t, report.Passed)
	assert.Equal(t, "V-4.01", report.Findings[0].ID)
}

func TestStage4Rules_MissingAcceptanceCriteria(t *testing.T) {
	content := `# Task Specifications

## T-01.01 — Initialize module
Run go mod init.

## T-01.02 — Add config
Create config struct.

## T-01.03 — Add handler
Write handler logic.`

	report := RunLocalVerification(StageTaskSpecifications, content, nil)
	var found *VerificationFinding
	for _, f := range report.Findings {
		if f.ID == "V-4.02" {
			found = &f
			break
		}
	}
	require.NotNil(t, found, "should flag missing acceptance criteria")
	assert.Equal(t, SeverityCritical, found.Severity)
}

// ---------------------------------------------------------------------------
// Cross-stage rules
// ---------------------------------------------------------------------------

func TestCrossStage2_EntitiesFromStage1(t *testing.T) {
	stage1 := StageResult{
		Stage: StageDesignPack,
		Sections: []Section{
			{Name: "data-model", Content: "Entities: UserAccount, OrderItem, PaymentMethod. UserAccount has-many OrderItem."},
		},
	}
	stage2Content := "```go\ntype UserAccount struct {}\ntype OrderItem struct {}\n```"

	report := RunLocalVerification(StageImplementationSkeletons, stage2Content, []StageResult{stage1})
	// PaymentMethod is missing — should warn.
	var found *VerificationFinding
	for _, f := range report.Findings {
		if f.ID == "V-X2.01" {
			found = &f
			break
		}
	}
	require.NotNil(t, found, "should flag missing entity")
	assert.Contains(t, found.Description, "PaymentMethod")
}

func TestCrossStage2_AllEntitiesPresent(t *testing.T) {
	stage1 := StageResult{
		Stage: StageDesignPack,
		Sections: []Section{
			{Name: "data-model", Content: "Entities: UserAccount, OrderItem. UserAccount has-many OrderItem."},
		},
	}
	stage2Content := "```go\ntype UserAccount struct {}\ntype OrderItem struct {}\n```"

	report := RunLocalVerification(StageImplementationSkeletons, stage2Content, []StageResult{stage1})
	for _, f := range report.Findings {
		assert.NotEqual(t, "V-X2.01", f.ID, "should not flag cross-stage entity issues when all are present")
	}
}

// ---------------------------------------------------------------------------
// Report formatting
// ---------------------------------------------------------------------------

func TestVerificationReport_Markdown(t *testing.T) {
	report := &VerificationReport{
		Stage:  StageDesignPack,
		Passed: false,
		Findings: []VerificationFinding{
			{
				ID:          "V-1.01",
				Severity:    SeverityCritical,
				Category:    "completeness",
				Section:     "design-pack",
				Description: "Missing assumptions section",
				Suggestion:  "Add an assumptions section.",
			},
			{
				ID:          "V-1.10",
				Severity:    SeverityWarning,
				Category:    "methodology",
				Section:     "pdrs",
				Description: "Only 1 PDR found",
				Suggestion:  "Add more PDRs.",
			},
		},
		Summary: "Verification found issues.",
	}

	md := report.Markdown()
	assert.Contains(t, md, "FAILED")
	assert.Contains(t, md, "1 critical")
	assert.Contains(t, md, "1 warnings")
	assert.Contains(t, md, "V-1.01")
	assert.Contains(t, md, "V-1.10")
	assert.Contains(t, md, "Critical Findings")
	assert.Contains(t, md, "Warnings")
}

func TestVerificationReport_HasCritical(t *testing.T) {
	tests := []struct {
		name     string
		findings []VerificationFinding
		want     bool
	}{
		{
			name: "with critical",
			findings: []VerificationFinding{
				{Severity: SeverityWarning},
				{Severity: SeverityCritical},
			},
			want: true,
		},
		{
			name: "without critical",
			findings: []VerificationFinding{
				{Severity: SeverityWarning},
				{Severity: SeverityInfo},
			},
			want: false,
		},
		{
			name:     "empty",
			findings: nil,
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &VerificationReport{Findings: tt.findings}
			assert.Equal(t, tt.want, r.HasCritical())
		})
	}
}
