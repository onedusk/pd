package orchestrator

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
)

// MilestoneStatus tracks the state of a milestone during implementation.
type MilestoneStatus string

const (
	MilestonePending   MilestoneStatus = "pending"
	MilestoneRunning   MilestoneStatus = "running"
	MilestoneCompleted MilestoneStatus = "completed"
	MilestoneFailed    MilestoneStatus = "failed"
)

// MilestoneNode represents a milestone in the dependency graph.
type MilestoneNode struct {
	ID           string          // e.g., "M1", "M2"
	Name         string          // human-readable name
	DependsOn    []string        // milestone IDs this depends on
	TaskSpecPath string          // path to tasks_mNN.md
	Status       MilestoneStatus // current execution state
}

// Scheduler manages milestone execution ordering based on a dependency DAG.
// It is safe for concurrent use.
type Scheduler struct {
	mu         sync.Mutex
	milestones map[string]*MilestoneNode
	order      []string // insertion order for determinism
}

// NewScheduler builds a scheduler from milestone data. It validates that
// there are no cycles and no missing dependencies.
func NewScheduler(milestones []MilestoneNode) (*Scheduler, error) {
	s := &Scheduler{
		milestones: make(map[string]*MilestoneNode, len(milestones)),
	}
	for i := range milestones {
		m := &milestones[i]
		if m.Status == "" {
			m.Status = MilestonePending
		}
		s.milestones[m.ID] = m
		s.order = append(s.order, m.ID)
	}

	if err := s.validate(); err != nil {
		return nil, err
	}
	return s, nil
}

// Ready returns milestones whose dependencies are all completed and that
// are in the pending state.
func (s *Scheduler) Ready() []*MilestoneNode {
	s.mu.Lock()
	defer s.mu.Unlock()

	var ready []*MilestoneNode
	for _, id := range s.order {
		m := s.milestones[id]
		if m.Status != MilestonePending {
			continue
		}
		if s.depsCompleted(m) {
			ready = append(ready, m)
		}
	}
	return ready
}

// MarkRunning transitions a milestone to running state.
func (s *Scheduler) MarkRunning(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	m, ok := s.milestones[id]
	if !ok {
		return fmt.Errorf("scheduler: unknown milestone %q", id)
	}
	if m.Status != MilestonePending {
		return fmt.Errorf("scheduler: milestone %q is %s, expected pending", id, m.Status)
	}
	m.Status = MilestoneRunning
	return nil
}

// MarkCompleted transitions a milestone to completed state and returns
// any milestones that are now ready to run (all their deps are satisfied).
func (s *Scheduler) MarkCompleted(id string) ([]*MilestoneNode, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	m, ok := s.milestones[id]
	if !ok {
		return nil, fmt.Errorf("scheduler: unknown milestone %q", id)
	}
	if m.Status != MilestoneRunning {
		return nil, fmt.Errorf("scheduler: milestone %q is %s, expected running", id, m.Status)
	}
	m.Status = MilestoneCompleted

	// Find milestones newly unblocked by this completion. A milestone is
	// "newly ready" only if it depends on the just-completed milestone and
	// all its other deps are also completed.
	var ready []*MilestoneNode
	for _, oid := range s.order {
		om := s.milestones[oid]
		if om.Status != MilestonePending {
			continue
		}
		if !containsString(om.DependsOn, id) {
			continue // this milestone doesn't depend on the one we just completed
		}
		if s.depsCompleted(om) {
			ready = append(ready, om)
		}
	}
	return ready, nil
}

// MarkFailed transitions a milestone to failed state.
func (s *Scheduler) MarkFailed(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	m, ok := s.milestones[id]
	if !ok {
		return fmt.Errorf("scheduler: unknown milestone %q", id)
	}
	m.Status = MilestoneFailed
	return nil
}

// AllCompleted returns true if every milestone is in the completed state.
func (s *Scheduler) AllCompleted() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, m := range s.milestones {
		if m.Status != MilestoneCompleted {
			return false
		}
	}
	return true
}

// Milestones returns a snapshot of all milestones in insertion order.
func (s *Scheduler) Milestones() []MilestoneNode {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]MilestoneNode, 0, len(s.order))
	for _, id := range s.order {
		out = append(out, *s.milestones[id])
	}
	return out
}

// depsCompleted checks whether all dependencies of m are completed.
// Must be called with mu held.
func (s *Scheduler) depsCompleted(m *MilestoneNode) bool {
	for _, depID := range m.DependsOn {
		dep, ok := s.milestones[depID]
		if !ok || dep.Status != MilestoneCompleted {
			return false
		}
	}
	return true
}

// validate checks for missing dependencies and cycles.
func (s *Scheduler) validate() error {
	// Check for missing dependencies.
	for _, m := range s.milestones {
		for _, depID := range m.DependsOn {
			if _, ok := s.milestones[depID]; !ok {
				return fmt.Errorf("scheduler: milestone %q depends on unknown milestone %q", m.ID, depID)
			}
		}
	}

	// Cycle detection using Kahn's algorithm.
	inDegree := make(map[string]int, len(s.milestones))

	// Recompute in-degree properly: for each edge dep -> m, increment m's in-degree.
	for id := range s.milestones {
		inDegree[id] = 0
	}
	for _, m := range s.milestones {
		for range m.DependsOn {
			inDegree[m.ID]++
		}
	}

	var queue []string
	for id, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, id)
		}
	}

	visited := 0
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		visited++

		// For each milestone that depends on current, decrement in-degree.
		for _, m := range s.milestones {
			for _, depID := range m.DependsOn {
				if depID == current {
					inDegree[m.ID]--
					if inDegree[m.ID] == 0 {
						queue = append(queue, m.ID)
					}
				}
			}
		}
	}

	if visited != len(s.milestones) {
		return fmt.Errorf("scheduler: dependency cycle detected (%d of %d milestones reachable)", visited, len(s.milestones))
	}

	return nil
}

// ---------------------------------------------------------------------------
// Stage 3 milestone parsing
// ---------------------------------------------------------------------------

// milestoneDefRe matches milestone headings like "## M1: Project Scaffolding"
// or "### Milestone 1: Foundation".
var milestoneDefRe = regexp.MustCompile(`(?im)^#+\s+(M(\d+)|Milestone\s+(\d+))\s*[:\-–—]\s*(.+)$`)

// milestoneDependsRe matches dependency arrows like "M1 → M2", "M1 -> M2, M3",
// or "M3 + M4 (parallel) -> M5". Every milestone on the left is a source.
var milestoneDependsRe = regexp.MustCompile(`((?:M\d+)(?:[ \t]*[,+&][ \t]*M\d+)*)(?:[ \t]*\([^)\n]*\))?[ \t]*(?:→|->|—>|-->)[ \t]*((?:M\d+)(?:[ \t]*[,+&][ \t]*M\d+)*)`)

// milestoneBoxArrowRe matches a box-drawing arrow between milestones on one
// line, like "M1 ──► M2", "M1 ──┬──► M2", or "M1 + M2 ──► M3".
var milestoneBoxArrowRe = regexp.MustCompile(`((?:M\d+)(?:[ \t]*[,+&][ \t]*M\d+)*)(?:[ \t]*\([^)\n]*\))?[ \t]*[─┬┴┼]+[►▶][ \t]*(M\d+)`)

// milestoneLabelRe matches the milestone a line is about, like "M2: Name" or
// "## Milestone 2: Name".
var milestoneLabelRe = regexp.MustCompile(`\b(?:M|[Mm]ilestone\s+)(\d+)\s*:`)

// milestoneDependsOnRe matches a "Depends on: M1 (types) + M2" line. The
// milestones it names are dependencies of the last labelled milestone. The
// colon is required, so prose like "M4 depends on M3" is not read.
var milestoneDependsOnRe = regexp.MustCompile(`(?i)\bdepends\s+on\s*:(.*)$`)

// milestoneTableRowRe matches Progress table rows like
// "| M1 | Name | [tasks_m01.md](tasks_m01.md) | | 0 |".
var milestoneTableRowRe = regexp.MustCompile(`(?m)^[ \t]*\|[ \t]*M(\d+)[ \t]*\|[ \t]*([^|\n]+?)[ \t]*\|`)

// milestoneRefRe extracts individual milestone references like "M1", "M2".
var milestoneRefRe = regexp.MustCompile(`M(\d+)`)

// ErrNoMilestoneDependencies is returned by ParseMilestones when a plan has
// two or more milestones and no dependency arrow between any of them.
var ErrNoMilestoneDependencies = errors.New(`no milestone dependencies found in Stage 3 content: add arrows such as "M1 -> M2" or mermaid lines such as "M1 --> M2"`)

// ParseMilestones extracts milestone IDs, names, and dependency relationships
// from Stage 3 markdown content. Milestones come from headings, or from the
// Progress table rows when there are no headings. When it returns
// ErrNoMilestoneDependencies, the milestones are returned as well.
func ParseMilestones(stage3Content string) ([]MilestoneNode, error) {
	// Extract milestone definitions as (number, name) pairs.
	var defs [][2]string
	for _, match := range milestoneDefRe.FindAllStringSubmatch(stage3Content, -1) {
		num := match[2]
		if num == "" {
			num = match[3]
		}
		defs = append(defs, [2]string{num, match[4]})
	}
	if len(defs) == 0 {
		for _, match := range milestoneTableRowRe.FindAllStringSubmatch(stage3Content, -1) {
			defs = append(defs, [2]string{match[1], match[2]})
		}
	}
	if len(defs) == 0 {
		return nil, fmt.Errorf("no milestone definitions found in Stage 3 content")
	}

	milestones := make(map[string]*MilestoneNode)
	var order []string

	for _, def := range defs {
		id := "M" + def[0]
		name := strings.TrimSpace(def[1])

		if _, exists := milestones[id]; !exists {
			milestones[id] = &MilestoneNode{
				ID:   id,
				Name: name,
			}
			order = append(order, id)
		}
	}

	// addDep records that targetID depends on sourceID. Targets that are not
	// defined, and self references, are ignored.
	hasDeps := false
	addDep := func(sourceID, targetID string) {
		target, ok := milestones[targetID]
		if !ok || sourceID == targetID {
			return
		}
		if !containsString(target.DependsOn, sourceID) {
			target.DependsOn = append(target.DependsOn, sourceID)
		}
		hasDeps = true
	}

	// Extract dependencies one line at a time: from arrows, and from
	// "Depends on:" lines under a labelled milestone.
	current := ""
	for _, line := range strings.Split(stage3Content, "\n") {
		label := line
		dependsOn := milestoneDependsOnRe.FindStringSubmatchIndex(line)
		if dependsOn != nil {
			label = line[:dependsOn[0]]
		}
		if m := milestoneLabelRe.FindAllStringSubmatch(label, -1); m != nil {
			current = "M" + m[len(m)-1][1]
		}
		if dependsOn != nil && current != "" {
			for _, ref := range milestoneRefRe.FindAllStringSubmatch(line[dependsOn[2]:dependsOn[3]], -1) {
				addDep("M"+ref[1], current)
			}
		}

		for _, re := range []*regexp.Regexp{milestoneDependsRe, milestoneBoxArrowRe} {
			rest := line
			for {
				loc := re.FindStringSubmatchIndex(rest)
				if loc == nil {
					break
				}
				// The arrow "M1, M2 → M3" means M3 depends on M1 and M2.
				sourceRefs := milestoneRefRe.FindAllStringSubmatch(rest[loc[2]:loc[3]], -1)
				targetRefs := milestoneRefRe.FindAllStringSubmatch(rest[loc[4]:loc[5]], -1)
				for _, src := range sourceRefs {
					for _, ref := range targetRefs {
						addDep("M"+src[1], "M"+ref[1])
					}
				}

				// Resume at the target so that it can be the source of the
				// next link in a chain like "M1 -> M2 -> M3".
				rest = rest[loc[4]:]
			}
		}
	}

	// Build result slice in order.
	var result []MilestoneNode
	for _, id := range order {
		result = append(result, *milestones[id])
	}
	if len(result) > 1 && !hasDeps {
		return result, ErrNoMilestoneDependencies
	}
	return result, nil
}

func containsString(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}
