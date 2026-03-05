package review

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/onedusk/pd/internal/a2a"
)

// InterpretConfig holds inputs for submitting a review-interpret A2A task.
type InterpretConfig struct {
	DecompName string   // decomposition name
	DecompDir  string   // docs/decompose/<name>/
	Endpoints  []string // A2A agent endpoint URLs to try
}

// InterpretResult holds the result of an A2A review-interpret task.
type InterpretResult struct {
	TaskID    string // A2A task ID
	Endpoint  string // agent endpoint that accepted the task
	Completed bool   // true if task reached terminal state
}

// SubmitInterpretTask discovers an A2A agent with the review-interpret skill
// and submits the interpretive triage task. Returns the task ID and endpoint
// on success. If no agent is reachable, returns an error.
func SubmitInterpretTask(ctx context.Context, client a2a.Client, cfg InterpretConfig) (*InterpretResult, error) {
	// Try each configured endpoint for agent discovery.
	var lastErr error
	for _, endpoint := range cfg.Endpoints {
		card, err := client.DiscoverAgent(ctx, endpoint)
		if err != nil {
			lastErr = err
			continue
		}

		// Check if agent has review-interpret skill.
		if !hasSkill(card, "review-interpret") {
			lastErr = fmt.Errorf("agent at %s does not have review-interpret skill", endpoint)
			continue
		}

		// Build and submit the task.
		task, err := submitTask(ctx, client, endpoint, cfg)
		if err != nil {
			lastErr = err
			continue
		}

		return &InterpretResult{
			TaskID:   task.ID,
			Endpoint: endpoint,
		}, nil
	}

	if lastErr != nil {
		return nil, fmt.Errorf("no review-interpret agent available: %w", lastErr)
	}
	return nil, fmt.Errorf("no agent endpoints configured")
}

// PollInterpretTask polls an A2A task until it reaches a terminal state or
// the context is cancelled. Returns the final task state.
func PollInterpretTask(ctx context.Context, client a2a.Client, endpoint, taskID string, interval time.Duration) (*a2a.Task, error) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			task, err := client.GetTask(ctx, endpoint, a2a.GetTaskRequest{ID: taskID})
			if err != nil {
				return nil, fmt.Errorf("poll task %s: %w", taskID, err)
			}
			if task.Status.State.IsTerminal() {
				return task, nil
			}
		}
	}
}

// hasSkill checks if an agent card declares a specific skill.
func hasSkill(card *a2a.AgentCard, skillID string) bool {
	for _, s := range card.Skills {
		if s.ID == skillID {
			return true
		}
	}
	return false
}

// submitTask creates the A2A message with artifacts and sends it.
func submitTask(ctx context.Context, client a2a.Client, endpoint string, cfg InterpretConfig) (*a2a.Task, error) {
	// Read artifact contents.
	findingsPath := filepath.Join(cfg.DecompDir, "review-findings.md")
	findings, err := os.ReadFile(findingsPath)
	if err != nil {
		return nil, fmt.Errorf("read review-findings.md: %w", err)
	}

	instructions := buildInterpretInstructions(cfg.DecompName)

	// Build message parts: instructions + findings content.
	parts := []a2a.Part{
		a2a.TextPart(instructions),
		{
			Text:      string(findings),
			MediaType: "text/markdown",
			Filename:  "review-findings.md",
		},
	}

	// Attach context files as additional parts.
	contextFiles := []struct {
		pattern  string
		filename string
	}{
		{filepath.Join(cfg.DecompDir, "stage-3-task-index.md"), "stage-3-task-index.md"},
		{filepath.Join(cfg.DecompDir, "stage-1-design-pack.md"), "stage-1-design-pack.md"},
	}
	for _, cf := range contextFiles {
		data, err := os.ReadFile(cf.pattern)
		if err != nil {
			continue // context files are best-effort
		}
		parts = append(parts, a2a.Part{
			Text:      string(data),
			MediaType: "text/markdown",
			Filename:  cf.filename,
		})
	}

	// Attach Stage 4 task spec files.
	taskFiles, _ := filepath.Glob(filepath.Join(cfg.DecompDir, "tasks_m*.md"))
	for _, tf := range taskFiles {
		data, err := os.ReadFile(tf)
		if err != nil {
			continue
		}
		parts = append(parts, a2a.Part{
			Text:      string(data),
			MediaType: "text/markdown",
			Filename:  filepath.Base(tf),
		})
	}

	msgID := fmt.Sprintf("review-interpret-%s-%d", cfg.DecompName, time.Now().Unix())

	req := a2a.SendMessageRequest{
		Message: a2a.Message{
			MessageID: msgID,
			ContextID: cfg.DecompName,
			Role:      a2a.RoleUser,
			Parts:     parts,
		},
		Configuration: &a2a.SendMessageConfig{
			AcceptedOutputModes: []string{"text/markdown"},
			Blocking:            false,
		},
	}

	return client.SendMessage(ctx, endpoint, req)
}

// buildInterpretInstructions returns the instructions text for the interpretive triage task.
func buildInterpretInstructions(name string) string {
	return fmt.Sprintf(`You are a review agent for progressive decomposition. You have been given
mechanical review findings produced by the run_review tool. Your job is
the interpretive pass -- work that requires reading code and understanding
intent, not just pattern matching.

READ FIRST:
- review-findings.md (the mechanical findings)
- stage-3-task-index.md (the milestone plan)
- All tasks_m*.md files (the task specs)
- stage-1-design-pack.md (the original design)

YOUR TASKS:

1. TRIAGE FALSE POSITIVES (especially from Check 5)
   For each OMISSION finding from Check 5, read the unlisted file and the
   planned files it relates to. Determine whether the planned changes will
   actually break the unlisted file. If not, reclassify as OK.

2. CATCH SEMANTIC CONFLICTS (Check 4 gaps)
   Read task outlines for every multi-milestone file and assess whether the
   changes are compatible. Add new findings with classification MISMATCH or
   OMISSION as appropriate.

3. VERIFY DEPENDENCY DIRECTION (Check 3 validation)
   For each OMISSION from Check 3, read the dependent file and the MODIFY
   target. Assess whether the planned change is backward-compatible. If so,
   reclassify as OK. If not, confirm the OMISSION and suggest what task
   should be added.

4. WRITE RECOMMENDED PLAN UPDATES
   Replace the <!-- INTERPRETIVE PASS NEEDED --> stub with actionable
   recommendations grouped by milestone.

5. UPDATE THE SUMMARY TABLE
   Recalculate summary table counts after triage.

OUTPUT:
Write the updated review-findings.md for decomposition %q.
Do NOT modify any Stage 3 or Stage 4 files.`, name)
}
