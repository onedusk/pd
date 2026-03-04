package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/onedusk/pd/internal/a2a"
	"github.com/onedusk/pd/internal/orchestrator"
)

// VerificationAgent is a specialist agent that reviews stage outputs for
// completeness, coherence, and methodology compliance. It operates with
// "fresh eyes" — receiving only the stage output content, not the producing
// agent's reasoning or context.
type VerificationAgent struct {
	*BaseAgent
}

// NewVerificationAgent creates a new VerificationAgent with its agent card and
// process function wired up.
func NewVerificationAgent() *VerificationAgent {
	va := &VerificationAgent{}

	card := a2a.AgentCard{
		Name:        "verification-agent",
		Description: "Reviews stage outputs for completeness, coherence, and methodology compliance",
		Version:     "dev",
		Skills: []a2a.AgentSkill{
			{
				ID:          "verify-stage",
				Name:        "Verify Stage",
				Description: "Verify a single stage output against methodology rules and produce a verification report",
				Tags:        []string{"verification", "quality", "methodology"},
			},
			{
				ID:          "verify-cross-stage",
				Name:        "Verify Cross-Stage",
				Description: "Verify cross-stage consistency between stage outputs",
				Tags:        []string{"verification", "coherence", "cross-stage"},
			},
		},
		DefaultInputModes:  []string{"text/plain"},
		DefaultOutputModes: []string{"text/markdown", "application/json"},
	}

	va.BaseAgent = NewBaseAgent(card, va.processMessage)
	return va
}

// processMessage is the ProcessFunc that dispatches to the appropriate skill.
func (va *VerificationAgent) processMessage(ctx context.Context, task *a2a.Task, msg a2a.Message) ([]a2a.Artifact, error) {
	text := extractText(msg)

	switch {
	case strings.Contains(text, "verify-cross-stage"):
		return va.verifyCrossStage(ctx, text)
	case strings.Contains(text, "verify-stage"):
		return va.verifyStage(ctx, text)
	default:
		return nil, fmt.Errorf("unknown skill: message does not contain a recognized skill ID (verify-stage, verify-cross-stage)")
	}
}

// verifyStage runs per-stage and cross-stage validation rules against the
// provided stage content.
func (va *VerificationAgent) verifyStage(_ context.Context, text string) ([]a2a.Artifact, error) {
	stage, content, priorStages, err := parseVerificationInput(text)
	if err != nil {
		return nil, fmt.Errorf("verify-stage: %w", err)
	}

	report := orchestrator.RunLocalVerification(stage, content, priorStages)
	return reportToArtifacts(report)
}

// verifyCrossStage runs only the cross-stage validation rules.
func (va *VerificationAgent) verifyCrossStage(_ context.Context, text string) ([]a2a.Artifact, error) {
	stage, content, priorStages, err := parseVerificationInput(text)
	if err != nil {
		return nil, fmt.Errorf("verify-cross-stage: %w", err)
	}

	rules := orchestrator.CrossStageRules(stage)
	var findings []orchestrator.VerificationFinding
	for _, rule := range rules {
		if f := rule.Check(content, priorStages); f != nil {
			findings = append(findings, *f)
		}
	}

	passed := true
	for _, f := range findings {
		if f.Severity == orchestrator.SeverityCritical {
			passed = false
			break
		}
	}

	report := &orchestrator.VerificationReport{
		Stage:    stage,
		Passed:   passed,
		Findings: findings,
		Summary:  fmt.Sprintf("Cross-stage verification for stage %d with %d findings.", int(stage), len(findings)),
	}
	return reportToArtifacts(report)
}

// reportToArtifacts converts a VerificationReport into two artifacts:
// a human-readable markdown report and a machine-readable JSON report.
func reportToArtifacts(report *orchestrator.VerificationReport) ([]a2a.Artifact, error) {
	jsonData, err := json.Marshal(report)
	if err != nil {
		return nil, fmt.Errorf("marshal verification report: %w", err)
	}

	mdArtifact := a2a.Artifact{
		ArtifactID:  a2a.NewTaskID(),
		Name:        "verification-report",
		Description: fmt.Sprintf("Verification report for stage %d (%s)", int(report.Stage), report.Stage),
		Parts:       []a2a.Part{a2a.TextPart(report.Markdown())},
	}

	jsonArtifact := a2a.Artifact{
		ArtifactID:  a2a.NewTaskID(),
		Name:        "verification-data",
		Description: "Machine-readable verification report",
		Parts: []a2a.Part{
			{Text: string(jsonData)},
		},
	}

	return []a2a.Artifact{mdArtifact, jsonArtifact}, nil
}

// verificationInput is the structured format for verification messages.
// The message text is parsed to extract the stage number, content, and
// any prior stage outputs.
//
// Expected format:
//
//	verify-stage
//	STAGE: <number>
//	---CONTENT---
//	<stage output content>
//	---PRIOR:<stage_num>---
//	<prior stage content>
//	---PRIOR:<stage_num>---
//	<prior stage content>

const (
	contentDelimiter = "---CONTENT---"
	priorPrefix      = "---PRIOR:"
	priorSuffix      = "---"
)

func parseVerificationInput(text string) (orchestrator.Stage, string, []orchestrator.StageResult, error) {
	// Find stage number.
	stageNum := -1
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "STAGE:") {
			numStr := strings.TrimSpace(strings.TrimPrefix(trimmed, "STAGE:"))
			n := 0
			for _, c := range numStr {
				if c >= '0' && c <= '9' {
					n = n*10 + int(c-'0')
				} else {
					break
				}
			}
			stageNum = n
			break
		}
	}
	if stageNum < 0 || stageNum > 4 {
		return 0, "", nil, fmt.Errorf("invalid or missing STAGE: header (expected 0-4, got %d)", stageNum)
	}
	stage := orchestrator.Stage(stageNum)

	// Extract content between ---CONTENT--- and first ---PRIOR: or end.
	contentStart := strings.Index(text, contentDelimiter)
	if contentStart < 0 {
		return stage, "", nil, fmt.Errorf("missing %s delimiter", contentDelimiter)
	}
	contentStart += len(contentDelimiter) + 1 // skip delimiter + newline

	// Find where content ends (next ---PRIOR: or end of text).
	contentEnd := len(text)
	priorStart := strings.Index(text[contentStart:], priorPrefix)
	if priorStart >= 0 {
		contentEnd = contentStart + priorStart
	}

	content := strings.TrimSpace(text[contentStart:contentEnd])

	// Extract prior stages.
	var priorStages []orchestrator.StageResult
	remaining := text[contentEnd:]
	for {
		idx := strings.Index(remaining, priorPrefix)
		if idx < 0 {
			break
		}
		remaining = remaining[idx+len(priorPrefix):]
		// Parse stage number from ---PRIOR:<num>---
		endIdx := strings.Index(remaining, priorSuffix)
		if endIdx < 0 {
			break
		}
		priorNumStr := remaining[:endIdx]
		remaining = remaining[endIdx+len(priorSuffix):]

		priorNum := 0
		for _, c := range priorNumStr {
			if c >= '0' && c <= '9' {
				priorNum = priorNum*10 + int(c-'0')
			}
		}

		// Content goes until next ---PRIOR: or end.
		nextPrior := strings.Index(remaining, priorPrefix)
		var priorContent string
		if nextPrior >= 0 {
			priorContent = strings.TrimSpace(remaining[:nextPrior])
			remaining = remaining[nextPrior:]
		} else {
			priorContent = strings.TrimSpace(remaining)
			remaining = ""
		}

		priorStages = append(priorStages, orchestrator.StageResult{
			Stage: orchestrator.Stage(priorNum),
			Sections: []orchestrator.Section{
				{
					Name:    orchestrator.Stage(priorNum).String(),
					Content: priorContent,
				},
			},
		})
	}

	return stage, content, priorStages, nil
}

// BuildVerificationMessage constructs the message text that the orchestrator
// sends to the VerificationAgent. This enforces "fresh eyes" isolation by
// including only stage output content, not producing-agent context.
func BuildVerificationMessage(skill string, stage orchestrator.Stage, content string, priorStages []orchestrator.StageResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\nSTAGE: %d\n", skill, int(stage))
	fmt.Fprintf(&b, "%s\n%s\n", contentDelimiter, content)

	for _, ps := range priorStages {
		for _, sec := range ps.Sections {
			fmt.Fprintf(&b, "%s%d%s\n%s\n", priorPrefix, int(ps.Stage), priorSuffix, sec.Content)
		}
	}

	return b.String()
}
