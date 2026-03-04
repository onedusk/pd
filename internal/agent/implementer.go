package agent

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/onedusk/pd/internal/a2a"
	"github.com/onedusk/pd/internal/orchestrator"
)

// ImplementerAgent wraps Claude Code CLI sessions as an A2A-compatible agent.
// Each invocation spawns a separate `claude` process with focused context for
// one milestone's task specification.
type ImplementerAgent struct {
	projectRoot string
	outputDir   string // decomposition output dir (docs/decompose/<name>/)
}

// NewImplementerAgent creates an ImplementerAgent for the given project.
func NewImplementerAgent(projectRoot, outputDir string) *ImplementerAgent {
	return &ImplementerAgent{
		projectRoot: projectRoot,
		outputDir:   outputDir,
	}
}

// ImplementMilestone executes a Claude Code session to implement all tasks
// for a single milestone. It composes a focused prompt and spawns `claude`
// as a subprocess.
func (ia *ImplementerAgent) ImplementMilestone(ctx context.Context, milestone *orchestrator.MilestoneNode) ([]orchestrator.ImplementationArtifact, error) {
	// Read the task spec file.
	taskSpec, err := os.ReadFile(milestone.TaskSpecPath)
	if err != nil {
		return nil, fmt.Errorf("read task spec %s: %w", milestone.TaskSpecPath, err)
	}

	// Compose the prompt.
	prompt := ia.composePrompt(string(taskSpec))

	// Spawn Claude Code session.
	cmd := exec.CommandContext(ctx, "claude",
		"--print",
		"--output-format", "stream-json",
		"--max-turns", "50",
		"-p", prompt,
	)
	cmd.Dir = ia.projectRoot

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("claude session for %s failed: %w\nstderr: %s", milestone.ID, err, stderr.String())
	}

	// Collect changed files via git diff.
	artifacts, err := ia.collectChangedFiles(ctx)
	if err != nil {
		// Non-fatal — we still have the Claude output.
		artifacts = nil
	}

	return artifacts, nil
}

// composePrompt builds the focused context for a Claude Code session.
// Each session receives only what it needs — no other milestone context.
func (ia *ImplementerAgent) composePrompt(taskSpec string) string {
	var sb strings.Builder

	// Stage 0 — development standards.
	stage0Content := ia.readStageFile(0)
	if stage0Content != "" {
		sb.WriteString("## Development Standards\n\n")
		sb.WriteString(stage0Content)
		sb.WriteString("\n\n---\n\n")
	}

	// Stage 2 — relevant skeleton types.
	stage2Content := ia.readStageFile(2)
	if stage2Content != "" {
		sb.WriteString("## Type Definitions (from Stage 2 Skeletons)\n\n")
		sb.WriteString(stage2Content)
		sb.WriteString("\n\n---\n\n")
	}

	// The task spec itself.
	sb.WriteString("## Task Specification\n\n")
	sb.WriteString(taskSpec)
	sb.WriteString("\n\n---\n\n")

	// Directive.
	sb.WriteString("Implement ALL tasks in the specification above, in dependency order. ")
	sb.WriteString("For each task:\n")
	sb.WriteString("1. Follow the implementation outline exactly\n")
	sb.WriteString("2. Write the code for the specified file and action (CREATE/MODIFY/DELETE)\n")
	sb.WriteString("3. Verify against the acceptance criteria\n")
	sb.WriteString("4. Move to the next task\n\n")
	sb.WriteString("When all tasks are complete, provide a summary of what was implemented and any issues encountered.")

	return sb.String()
}

// readStageFile reads a stage output file by number. Returns empty string if
// the file doesn't exist.
func (ia *ImplementerAgent) readStageFile(stageNum int) string {
	slug := orchestrator.Stage(stageNum).String()
	// Stage 0 is at the parent level.
	var path string
	if stageNum == 0 {
		path = filepath.Join(filepath.Dir(ia.outputDir), fmt.Sprintf("stage-0-%s.md", slug))
	} else {
		path = filepath.Join(ia.outputDir, fmt.Sprintf("stage-%d-%s.md", stageNum, slug))
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

// collectChangedFiles runs git diff to find files modified by the Claude session.
func (ia *ImplementerAgent) collectChangedFiles(ctx context.Context) ([]orchestrator.ImplementationArtifact, error) {
	// Get list of changed files.
	cmd := exec.CommandContext(ctx, "git", "diff", "--name-status", "HEAD")
	cmd.Dir = ia.projectRoot
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git diff: %w", err)
	}

	var artifacts []orchestrator.ImplementationArtifact
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		var action string
		switch parts[0] {
		case "A":
			action = "CREATE"
		case "M":
			action = "MODIFY"
		case "D":
			action = "DELETE"
		default:
			action = parts[0]
		}

		filePath := parts[len(parts)-1]

		// Get diff for this file.
		diffCmd := exec.CommandContext(ctx, "git", "diff", "HEAD", "--", filePath)
		diffCmd.Dir = ia.projectRoot
		diff, _ := diffCmd.Output()

		artifacts = append(artifacts, orchestrator.ImplementationArtifact{
			Path:       filePath,
			Action:     action,
			DiffOrBody: string(diff),
		})
	}

	// Also check untracked files.
	untrackedCmd := exec.CommandContext(ctx, "git", "ls-files", "--others", "--exclude-standard")
	untrackedCmd.Dir = ia.projectRoot
	untrackedOutput, _ := untrackedCmd.Output()

	for _, line := range strings.Split(strings.TrimSpace(string(untrackedOutput)), "\n") {
		if line == "" {
			continue
		}
		fullPath := filepath.Join(ia.projectRoot, line)
		content, _ := os.ReadFile(fullPath)
		artifacts = append(artifacts, orchestrator.ImplementationArtifact{
			Path:       line,
			Action:     "CREATE",
			DiffOrBody: string(content),
		})
	}

	return artifacts, nil
}

// buildA2AArtifacts converts implementation artifacts to A2A artifacts
// for protocol compatibility.
func buildA2AArtifacts(milestone *orchestrator.MilestoneNode, implArtifacts []orchestrator.ImplementationArtifact) []a2a.Artifact {
	var parts []a2a.Part
	for _, ia := range implArtifacts {
		parts = append(parts, a2a.TextPart(fmt.Sprintf("[%s] %s", ia.Action, ia.Path)))
	}

	return []a2a.Artifact{
		{
			ArtifactID:  a2a.NewTaskID(),
			Name:        fmt.Sprintf("implementation-%s", milestone.ID),
			Description: fmt.Sprintf("Implementation results for %s: %s", milestone.ID, milestone.Name),
			Parts:       parts,
		},
	}
}
