package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/dusk-indust/decompose/internal/a2a"
	"github.com/dusk-indust/decompose/internal/mcptools"
)

// PlanningAgent is a specialist agent that builds code graphs, analyzes
// dependencies, assesses impact, and plans milestones. It embeds BaseAgent
// and optionally uses CodeIntelService for direct MCP tool access.
type PlanningAgent struct {
	*BaseAgent
	mcpSvc *mcptools.CodeIntelService
}

// PlanningOption configures a PlanningAgent during construction.
type PlanningOption func(*PlanningAgent)

// WithCodeIntelService injects an MCP CodeIntelService for direct graph operations.
func WithCodeIntelService(svc *mcptools.CodeIntelService) PlanningOption {
	return func(pa *PlanningAgent) {
		pa.mcpSvc = svc
	}
}

// NewPlanningAgent creates a PlanningAgent with the given options.
func NewPlanningAgent(opts ...PlanningOption) *PlanningAgent {
	pa := &PlanningAgent{}

	for _, opt := range opts {
		opt(pa)
	}

	card := a2a.AgentCard{
		Name:        "planning-agent",
		Description: "Builds code graphs, analyzes dependencies, assesses impact, and plans milestones",
		Version:     "dev",
		Skills: []a2a.AgentSkill{
			{
				ID:          "build-code-graph",
				Name:        "Build Code Graph",
				Description: "Index a repository and build the code intelligence graph",
				Tags:        []string{"graph", "indexing"},
			},
			{
				ID:          "analyze-dependencies",
				Name:        "Analyze Dependencies",
				Description: "Traverse the dependency graph upstream or downstream from a node",
				Tags:        []string{"graph", "dependencies"},
			},
			{
				ID:          "assess-impact",
				Name:        "Assess Impact",
				Description: "Compute the blast radius of modifying a set of files",
				Tags:        []string{"graph", "impact"},
			},
			{
				ID:          "plan-milestones",
				Name:        "Plan Milestones",
				Description: "Organize a design pack into ordered milestones with dependencies",
				Tags:        []string{"planning", "milestones"},
			},
		},
		DefaultInputModes:  []string{"text/plain", "application/json"},
		DefaultOutputModes: []string{"text/markdown", "application/json"},
	}

	pa.BaseAgent = NewBaseAgent(card, pa.processMessage)
	return pa
}

// processMessage routes incoming messages to the appropriate skill handler.
func (pa *PlanningAgent) processMessage(ctx context.Context, task *a2a.Task, msg a2a.Message) ([]a2a.Artifact, error) {
	text := planningExtractText(msg)
	skill := detectPlanningSkill(text)

	switch skill {
	case "build-code-graph":
		return pa.handleBuildCodeGraph(ctx, text)
	case "analyze-dependencies":
		return pa.handleAnalyzeDependencies(ctx, text)
	case "assess-impact":
		return pa.handleAssessImpact(ctx, text)
	case "plan-milestones":
		return pa.handlePlanMilestones(text)
	default:
		return nil, fmt.Errorf("unknown skill %q: supported skills are build-code-graph, analyze-dependencies, assess-impact, plan-milestones", skill)
	}
}

// handleBuildCodeGraph indexes a repository and returns graph statistics.
func (pa *PlanningAgent) handleBuildCodeGraph(ctx context.Context, text string) ([]a2a.Artifact, error) {
	if pa.mcpSvc == nil {
		return nil, fmt.Errorf("MCP CodeIntelService is required for build-code-graph; configure with WithCodeIntelService")
	}

	path := extractRepoPath(text)
	if path == "" {
		return nil, fmt.Errorf("could not extract repository path from message; include a path like /path/to/repo")
	}

	_, out, err := pa.mcpSvc.BuildGraph(ctx, nil, mcptools.BuildGraphInput{RepoPath: path})
	if err != nil {
		return nil, fmt.Errorf("build graph: %w", err)
	}

	md := fmt.Sprintf("## Code Graph Statistics\n\n"+
		"| Metric | Count |\n"+
		"|--------|-------|\n"+
		"| Files | %d |\n"+
		"| Symbols | %d |\n"+
		"| Clusters | %d |\n"+
		"| Edges | %d |\n",
		out.Stats.FileCount, out.Stats.SymbolCount, out.Stats.ClusterCount, out.Stats.EdgeCount)

	return []a2a.Artifact{
		{
			ArtifactID:  a2a.NewTaskID(),
			Name:        "graph-stats",
			Description: "Code intelligence graph statistics",
			Parts:       []a2a.Part{a2a.TextPart(md)},
		},
	}, nil
}

// handleAnalyzeDependencies traverses dependencies from a node.
func (pa *PlanningAgent) handleAnalyzeDependencies(ctx context.Context, text string) ([]a2a.Artifact, error) {
	if pa.mcpSvc == nil {
		return nil, fmt.Errorf("MCP CodeIntelService is required for analyze-dependencies; configure with WithCodeIntelService")
	}

	nodeID, direction := extractNodeAndDirection(text)
	if nodeID == "" {
		return nil, fmt.Errorf("could not extract node ID from message; include a file path or symbol name")
	}

	_, out, err := pa.mcpSvc.GetDependencies(ctx, nil, mcptools.GetDependenciesInput{
		NodeID:    nodeID,
		Direction: direction,
	})
	if err != nil {
		return nil, fmt.Errorf("get dependencies: %w", err)
	}

	var sb strings.Builder
	sb.WriteString("## Dependency Chains\n\n")
	if len(out.Chains) == 0 {
		sb.WriteString("No dependency chains found.\n")
	} else {
		sb.WriteString("| Chain | Depth |\n")
		sb.WriteString("|-------|-------|\n")
		for _, chain := range out.Chains {
			sb.WriteString(fmt.Sprintf("| %s | %d |\n", strings.Join(chain.Nodes, " -> "), chain.Depth))
		}
	}

	return []a2a.Artifact{
		{
			ArtifactID:  a2a.NewTaskID(),
			Name:        "dependency-chains",
			Description: "Dependency chain analysis",
			Parts:       []a2a.Part{a2a.TextPart(sb.String())},
		},
	}, nil
}

// handleAssessImpact computes the blast radius of file changes.
func (pa *PlanningAgent) handleAssessImpact(ctx context.Context, text string) ([]a2a.Artifact, error) {
	if pa.mcpSvc == nil {
		return nil, fmt.Errorf("MCP CodeIntelService is required for assess-impact; configure with WithCodeIntelService")
	}

	files := extractFilePaths(text)
	if len(files) == 0 {
		return nil, fmt.Errorf("could not extract file paths from message; include file paths to assess")
	}

	_, out, err := pa.mcpSvc.AssessImpact(ctx, nil, mcptools.AssessImpactInput{
		ChangedFiles: files,
	})
	if err != nil {
		return nil, fmt.Errorf("assess impact: %w", err)
	}

	var sb strings.Builder
	sb.WriteString("## Impact Assessment\n\n")
	sb.WriteString(fmt.Sprintf("**Risk Score:** %.2f\n\n", out.Impact.RiskScore))

	sb.WriteString("### Directly Affected\n\n")
	if len(out.Impact.DirectlyAffected) == 0 {
		sb.WriteString("None\n\n")
	} else {
		for _, f := range out.Impact.DirectlyAffected {
			sb.WriteString(fmt.Sprintf("- `%s`\n", f))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("### Transitively Affected\n\n")
	if len(out.Impact.TransitivelyAffected) == 0 {
		sb.WriteString("None\n")
	} else {
		for _, f := range out.Impact.TransitivelyAffected {
			sb.WriteString(fmt.Sprintf("- `%s`\n", f))
		}
	}

	return []a2a.Artifact{
		{
			ArtifactID:  a2a.NewTaskID(),
			Name:        "impact-assessment",
			Description: "Change impact analysis",
			Parts:       []a2a.Part{a2a.TextPart(sb.String())},
		},
	}, nil
}

// handlePlanMilestones organizes a design pack into ordered milestones.
// This skill works without MCP tools, using text heuristics.
func (pa *PlanningAgent) handlePlanMilestones(text string) ([]a2a.Artifact, error) {
	sections := splitSections(text)
	if len(sections) == 0 {
		return nil, fmt.Errorf("could not parse design pack; expected markdown with ## section headers")
	}

	milestones := buildMilestones(sections)

	var sb strings.Builder
	sb.WriteString("## Milestones\n\n")
	sb.WriteString("| ID | Name | Description | Depends On |\n")
	sb.WriteString("|----|------|-------------|------------|\n")
	for _, m := range milestones {
		deps := "—"
		if len(m.dependsOn) > 0 {
			deps = strings.Join(m.dependsOn, ", ")
		}
		sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n", m.id, m.name, m.description, deps))
	}

	sb.WriteString("\n## Dependency Graph\n\n")
	if len(milestones) > 0 {
		ids := make([]string, len(milestones))
		for i, m := range milestones {
			ids[i] = m.id
		}
		sb.WriteString(strings.Join(ids, " → "))
		sb.WriteString("\n")
	}

	return []a2a.Artifact{
		{
			ArtifactID:  a2a.NewTaskID(),
			Name:        "milestone-plan",
			Description: "Stage 3 milestone plan",
			Parts:       []a2a.Part{a2a.TextPart(sb.String())},
		},
	}, nil
}

// --- Helpers ---

// milestone represents a planned milestone derived from design pack sections.
type milestone struct {
	id          string
	name        string
	description string
	dependsOn   []string
}

// planningExtractText pulls the concatenated text content from a message's parts.
func planningExtractText(msg a2a.Message) string {
	var parts []string
	for _, p := range msg.Parts {
		if p.Text != "" {
			parts = append(parts, p.Text)
		}
	}
	return strings.Join(parts, "\n")
}

// detectPlanningSkill determines which planning skill is being requested from the message text.
// It looks for skill keywords in the text.
func detectPlanningSkill(text string) string {
	lower := strings.ToLower(text)

	// Check for explicit skill references first.
	skills := []struct {
		id       string
		keywords []string
	}{
		{"build-code-graph", []string{"build-code-graph", "build graph", "build code graph", "index repo"}},
		{"analyze-dependencies", []string{"analyze-dependencies", "analyze dependencies", "get dependencies", "dependency chain"}},
		{"assess-impact", []string{"assess-impact", "assess impact", "impact assessment", "blast radius"}},
		{"plan-milestones", []string{"plan-milestones", "plan milestones", "milestone", "design pack"}},
	}

	for _, s := range skills {
		for _, kw := range s.keywords {
			if strings.Contains(lower, kw) {
				return s.id
			}
		}
	}

	return ""
}

// extractRepoPath extracts a file system path from the message text.
// Looks for common path patterns.
func extractRepoPath(text string) string {
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		// Look for lines that look like absolute paths.
		if strings.HasPrefix(trimmed, "/") && !strings.HasPrefix(trimmed, "//") {
			// Take just the path portion (stop at whitespace or end).
			parts := strings.Fields(trimmed)
			if len(parts) > 0 {
				return parts[0]
			}
		}
	}
	// Fallback: look for path-like tokens anywhere.
	for _, token := range strings.Fields(text) {
		if strings.HasPrefix(token, "/") && strings.Contains(token, "/") && len(token) > 1 {
			return token
		}
	}
	return ""
}

// extractNodeAndDirection extracts a node ID and optional direction from message text.
func extractNodeAndDirection(text string) (nodeID, direction string) {
	direction = "downstream" // default

	lower := strings.ToLower(text)
	if strings.Contains(lower, "upstream") {
		direction = "upstream"
	}

	// Look for path-like or symbol-like tokens.
	for _, token := range strings.Fields(text) {
		clean := strings.Trim(token, "`\"',;:")
		if clean == "" {
			continue
		}
		// Skip common keywords.
		lowerClean := strings.ToLower(clean)
		if lowerClean == "upstream" || lowerClean == "downstream" || lowerClean == "analyze" ||
			lowerClean == "dependencies" || lowerClean == "analyze-dependencies" ||
			lowerClean == "get" || lowerClean == "dependency" || lowerClean == "chain" {
			continue
		}
		// Accept paths or qualified names.
		if strings.Contains(clean, "/") || strings.Contains(clean, ".") {
			nodeID = clean
			return nodeID, direction
		}
	}

	return nodeID, direction
}

// extractFilePaths extracts file paths from the message text.
func extractFilePaths(text string) []string {
	var paths []string
	seen := make(map[string]bool)

	for _, token := range strings.Fields(text) {
		clean := strings.Trim(token, "`\"',;:()[]")
		if clean == "" {
			continue
		}
		// Look for things that look like file paths.
		if (strings.Contains(clean, "/") || strings.Contains(clean, ".")) &&
			!strings.HasPrefix(clean, "http") &&
			strings.ContainsAny(clean, "/.") &&
			len(clean) > 2 {
			// Skip known keywords.
			lowerClean := strings.ToLower(clean)
			if lowerClean == "assess-impact" || lowerClean == "assess" || lowerClean == "impact" {
				continue
			}
			if !seen[clean] {
				seen[clean] = true
				paths = append(paths, clean)
			}
		}
	}

	return paths
}

// section represents a parsed markdown section from a design pack.
type section struct {
	heading string
	body    string
}

// splitSections splits markdown text into sections by ## headers.
func splitSections(text string) []section {
	var sections []section
	var current *section

	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "## ") {
			if current != nil {
				current.body = strings.TrimSpace(current.body)
				sections = append(sections, *current)
			}
			current = &section{
				heading: strings.TrimPrefix(trimmed, "## "),
			}
		} else if current != nil {
			current.body += line + "\n"
		}
	}

	if current != nil {
		current.body = strings.TrimSpace(current.body)
		sections = append(sections, *current)
	}

	return sections
}

// buildMilestones groups sections into milestones with sequential dependencies.
func buildMilestones(sections []section) []milestone {
	if len(sections) == 0 {
		return nil
	}

	// Simple heuristic: each section (or small group of related sections)
	// becomes a milestone. For now, group every 2 sections together, or
	// 1-to-1 if there are 4 or fewer sections.
	groupSize := 1
	if len(sections) > 4 {
		groupSize = 2
	}

	var milestones []milestone
	idx := 1

	for i := 0; i < len(sections); i += groupSize {
		end := i + groupSize
		if end > len(sections) {
			end = len(sections)
		}

		group := sections[i:end]
		names := make([]string, len(group))
		for j, s := range group {
			names[j] = s.heading
		}

		id := fmt.Sprintf("M%d", idx)
		name := strings.Join(names, " + ")
		desc := summarizeSections(group)

		var deps []string
		if idx > 1 {
			deps = []string{fmt.Sprintf("M%d", idx-1)}
		}

		milestones = append(milestones, milestone{
			id:          id,
			name:        name,
			description: desc,
			dependsOn:   deps,
		})
		idx++
	}

	return milestones
}

// summarizeSections produces a brief description from section content.
func summarizeSections(sections []section) string {
	var parts []string
	for _, s := range sections {
		// Take the first non-empty line of the body as the summary.
		for _, line := range strings.Split(s.body, "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed != "" {
				parts = append(parts, trimmed)
				break
			}
		}
	}
	summary := strings.Join(parts, "; ")
	// Truncate long descriptions for table readability.
	if len(summary) > 120 {
		summary = summary[:117] + "..."
	}
	return summary
}
