package agent

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/dusk-indust/decompose/internal/a2a"
)

// TaskWriterAgent writes detailed task specifications and validates
// cross-milestone dependencies. It supports two skills:
//   - write-task-specs: generates a tasks_mNN.md file from a milestone description
//   - validate-dependencies: checks task dependency graphs for missing/circular refs
type TaskWriterAgent struct {
	*BaseAgent
}

// NewTaskWriterAgent creates a TaskWriterAgent with its agent card and process function.
func NewTaskWriterAgent() *TaskWriterAgent {
	tw := &TaskWriterAgent{}
	card := a2a.AgentCard{
		Name:        "task-writer-agent",
		Description: "Writes detailed task specifications and validates cross-milestone dependencies",
		Version:     "dev",
		Skills: []a2a.AgentSkill{
			{
				ID:          "write-task-specs",
				Name:        "Write Task Specs",
				Description: "Generates detailed task specifications from a milestone description",
				Tags:        []string{"task", "specification", "milestone"},
			},
			{
				ID:          "validate-dependencies",
				Name:        "Validate Dependencies",
				Description: "Validates cross-milestone task dependencies for missing or circular references",
				Tags:        []string{"task", "dependency", "validation"},
			},
		},
		DefaultInputModes:  []string{"text/plain", "text/markdown"},
		DefaultOutputModes: []string{"text/markdown"},
	}
	tw.BaseAgent = NewBaseAgent(card, tw.processMessage)
	return tw
}

// processMessage dispatches to the appropriate skill handler based on the
// message text content.
func (tw *TaskWriterAgent) processMessage(ctx context.Context, task *a2a.Task, msg a2a.Message) ([]a2a.Artifact, error) {
	text := extractText(msg)

	switch {
	case strings.Contains(strings.ToLower(text), "write-task-specs"):
		return tw.writeTaskSpecs(ctx, text)
	case strings.Contains(strings.ToLower(text), "validate-dependencies"):
		return tw.validateDependencies(ctx, text)
	default:
		return nil, fmt.Errorf("unknown skill: could not determine skill from message text")
	}
}

// writeTaskSpecs parses a milestone description and generates task
// specifications in T-MM.SS format.
func (tw *TaskWriterAgent) writeTaskSpecs(_ context.Context, text string) ([]a2a.Artifact, error) {
	milestoneNum := parseMilestoneNumber(text)

	// Split the input into blocks separated by blank lines or numbered items.
	blocks := splitIntoBlocks(text)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Tasks for Milestone %02d\n\n", milestoneNum))

	taskSeq := 1
	for _, block := range blocks {
		block = strings.TrimSpace(block)
		if block == "" {
			continue
		}

		filePath := extractFilePath(block)
		if filePath == "" {
			continue
		}

		taskID := fmt.Sprintf("T-%02d.%02d", milestoneNum, taskSeq)
		action := determineAction(block)
		deps := extractDependsOn(block)
		outline := extractOutline(block)
		acceptance := extractAcceptance(block)

		sb.WriteString(fmt.Sprintf("## %s\n\n", taskID))
		sb.WriteString(fmt.Sprintf("- **File**: `%s`\n", filePath))
		sb.WriteString(fmt.Sprintf("- **Action**: %s\n", action))
		if deps != "" {
			sb.WriteString(fmt.Sprintf("- **Depends on**: %s\n", deps))
		}
		sb.WriteString(fmt.Sprintf("\n### Implementation Outline\n\n%s\n", outline))
		sb.WriteString(fmt.Sprintf("\n### Acceptance Criteria\n\n%s\n\n---\n\n", acceptance))

		taskSeq++
	}

	if taskSeq == 1 {
		sb.WriteString("_No tasks could be extracted from the provided milestone description._\n")
	}

	artifact := a2a.Artifact{
		ArtifactID:  fmt.Sprintf("tasks_m%02d", milestoneNum),
		Name:        fmt.Sprintf("tasks_m%02d.md", milestoneNum),
		Description: fmt.Sprintf("Task specifications for milestone %02d", milestoneNum),
		Parts: []a2a.Part{
			{Text: sb.String(), MediaType: "text/markdown"},
		},
	}
	return []a2a.Artifact{artifact}, nil
}

// validateDependencies parses task specs from text, checks that all
// referenced task IDs exist, and detects circular dependencies using
// Kahn's algorithm for topological sorting.
func (tw *TaskWriterAgent) validateDependencies(_ context.Context, text string) ([]a2a.Artifact, error) {
	// Extract all defined task IDs (T-XX.YY at the start of headings or list items).
	definedPattern := regexp.MustCompile(`(?m)(?:^##?\s+|^- \*\*Task\*\*:\s*)(T-\d{2}\.\d{2})`)
	definedMatches := definedPattern.FindAllStringSubmatch(text, -1)

	defined := make(map[string]bool)
	for _, m := range definedMatches {
		defined[m[1]] = true
	}

	// Also pick up any T-XX.YY that appears alone on a heading line.
	headingPattern := regexp.MustCompile(`(?m)^##\s+(T-\d{2}\.\d{2})`)
	for _, m := range headingPattern.FindAllStringSubmatch(text, -1) {
		defined[m[1]] = true
	}

	// Extract dependency relationships: "Depends on: T-XX.YY, T-XX.YY"
	depLinePattern := regexp.MustCompile(`(?mi)(?:depends\s+on|blocked\s+by)[:\s]+(T-[\d.]+(?:\s*,\s*T-[\d.]+)*)`)
	taskRefPattern := regexp.MustCompile(`T-\d{2}\.\d{2}`)

	// Build adjacency and in-degree maps for topological sort.
	graph := make(map[string][]string)    // task -> tasks it depends on
	inDegree := make(map[string]int)      // how many deps each task has
	allNodes := make(map[string]bool)

	// Ensure all defined tasks are in the graph.
	for id := range defined {
		allNodes[id] = true
		if _, ok := inDegree[id]; !ok {
			inDegree[id] = 0
		}
	}

	// Parse each dependency line. We need to associate it with the nearest
	// preceding task ID heading.
	lines := strings.Split(text, "\n")
	currentTask := ""
	var missing []string

	for _, line := range lines {
		// Check if this line defines a task heading.
		if hm := headingPattern.FindStringSubmatch(line); len(hm) > 1 {
			currentTask = hm[1]
		}

		// Check if this line contains dependency declarations.
		if dm := depLinePattern.FindStringSubmatch(line); len(dm) > 1 {
			refs := taskRefPattern.FindAllString(dm[1], -1)
			for _, ref := range refs {
				allNodes[ref] = true
				if !defined[ref] {
					missing = append(missing, fmt.Sprintf("%s references undefined task %s", currentTask, ref))
				}
				if currentTask != "" {
					graph[currentTask] = append(graph[currentTask], ref)
					inDegree[currentTask]++ // currentTask depends on ref
					if _, ok := inDegree[ref]; !ok {
						inDegree[ref] = 0
					}
				}
			}
		}
	}

	// Kahn's algorithm: detect cycles.
	queue := make([]string, 0)
	for node := range allNodes {
		if inDegree[node] == 0 {
			queue = append(queue, node)
		}
	}

	sorted := 0
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		sorted++

		// Find nodes that depend on this node (reverse edges).
		for dependent, deps := range graph {
			for _, dep := range deps {
				if dep == node {
					inDegree[dependent]--
					if inDegree[dependent] == 0 {
						queue = append(queue, dependent)
					}
				}
			}
		}
	}

	var circular []string
	if sorted < len(allNodes) {
		// Find which nodes are involved in cycles.
		for node := range allNodes {
			if inDegree[node] > 0 {
				circular = append(circular, node)
			}
		}
	}

	// Build report.
	var sb strings.Builder
	sb.WriteString("# Dependency Validation Report\n\n")

	if len(missing) == 0 && len(circular) == 0 {
		sb.WriteString("All dependency references are valid. No circular dependencies detected.\n")
	}

	if len(missing) > 0 {
		sb.WriteString("## Missing References\n\n")
		for _, m := range missing {
			sb.WriteString(fmt.Sprintf("- %s\n", m))
		}
		sb.WriteString("\n")
	}

	if len(circular) > 0 {
		sb.WriteString("## Circular Dependencies\n\n")
		sb.WriteString("The following tasks are involved in dependency cycles:\n\n")
		for _, c := range circular {
			sb.WriteString(fmt.Sprintf("- %s\n", c))
		}
		sb.WriteString("\n")
	}

	sb.WriteString(fmt.Sprintf("\n**Defined tasks**: %d\n", len(defined)))
	sb.WriteString(fmt.Sprintf("**Total references checked**: %d\n", countRefs(graph)))

	artifact := a2a.Artifact{
		ArtifactID:  "dep-validation",
		Name:        "dependency-validation.md",
		Description: "Dependency validation report",
		Parts: []a2a.Part{
			{Text: sb.String(), MediaType: "text/markdown"},
		},
	}
	return []a2a.Artifact{artifact}, nil
}

// --- Helper functions ---

// parseMilestoneNumber extracts a milestone number from text like
// "Milestone 5" or "milestone 12". Defaults to 1 if not found.
func parseMilestoneNumber(text string) int {
	re := regexp.MustCompile(`(?i)milestone\s+(\d+)`)
	m := re.FindStringSubmatch(text)
	if len(m) < 2 {
		return 1
	}
	n := 0
	for _, c := range m[1] {
		n = n*10 + int(c-'0')
	}
	if n == 0 {
		return 1
	}
	return n
}

// splitIntoBlocks splits text into logical blocks by blank lines or
// numbered list items.
func splitIntoBlocks(text string) []string {
	// First, remove the skill directive line.
	lines := strings.Split(text, "\n")
	var filtered []string
	for _, line := range lines {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "write-task-specs") && !strings.Contains(lower, "/") {
			continue
		}
		filtered = append(filtered, line)
	}

	joined := strings.Join(filtered, "\n")

	// Split by blank lines.
	blocks := regexp.MustCompile(`\n\s*\n`).Split(joined, -1)

	// Further split numbered items within blocks.
	var result []string
	numbered := regexp.MustCompile(`(?m)^\d+[\.\)]\s+`)
	for _, block := range blocks {
		if numbered.MatchString(block) {
			items := numbered.Split(block, -1)
			for _, item := range items {
				item = strings.TrimSpace(item)
				if item != "" {
					result = append(result, item)
				}
			}
		} else {
			result = append(result, block)
		}
	}

	return result
}

// extractFilePath looks for file path patterns in text.
func extractFilePath(text string) string {
	// Match common file path patterns.
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?m)` + "`" + `([a-zA-Z0-9_/.-]+\.[a-zA-Z]+)` + "`"),
		regexp.MustCompile(`(?m)((?:[a-zA-Z0-9_-]+/)+[a-zA-Z0-9_.-]+\.[a-zA-Z]+)`),
	}
	for _, p := range patterns {
		if m := p.FindStringSubmatch(text); len(m) > 1 {
			return m[1]
		}
	}
	return ""
}

// determineAction returns CREATE or MODIFY based on keywords in the text.
func determineAction(text string) string {
	lower := strings.ToLower(text)
	createKeywords := []string{"create", "new file", "add new", "introduce", "define new"}
	for _, kw := range createKeywords {
		if strings.Contains(lower, kw) {
			return "CREATE"
		}
	}
	return "MODIFY"
}

// extractDependsOn finds "Depends on: T-XX.YY" patterns in text.
func extractDependsOn(text string) string {
	re := regexp.MustCompile(`(?i)depends?\s+on[:\s]+(T-[\d.]+(?:\s*,\s*T-[\d.]+)*)`)
	m := re.FindStringSubmatch(text)
	if len(m) > 1 {
		return m[1]
	}
	return ""
}

// extractOutline generates an implementation outline from the block text.
func extractOutline(text string) string {
	// Look for lines that describe implementation steps.
	lines := strings.Split(text, "\n")
	var outline []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Skip file paths and dependency declarations.
		lower := strings.ToLower(line)
		if strings.Contains(lower, "depends on") {
			continue
		}
		outline = append(outline, fmt.Sprintf("- %s", line))
	}
	if len(outline) == 0 {
		return "- Implementation details to be specified"
	}
	return strings.Join(outline, "\n")
}

// extractAcceptance generates acceptance criteria from the block text.
func extractAcceptance(text string) string {
	filePath := extractFilePath(text)
	action := determineAction(text)

	var criteria []string
	if action == "CREATE" {
		criteria = append(criteria, fmt.Sprintf("- [ ] File `%s` exists", filePath))
	}
	criteria = append(criteria, fmt.Sprintf("- [ ] `%s` compiles without errors", filePath))
	criteria = append(criteria, "- [ ] Unit tests pass")

	return strings.Join(criteria, "\n")
}

// countRefs counts the total number of dependency references in the graph.
func countRefs(graph map[string][]string) int {
	n := 0
	for _, deps := range graph {
		n += len(deps)
	}
	return n
}
