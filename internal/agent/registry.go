package agent

import (
	"context"
	"fmt"
	"sync"
)

// AgentFactory is a constructor that creates an Agent.
type AgentFactory func() Agent

// Registry maps agent roles to their factory constructors and manages
// the lifecycle of spawned agents.
type Registry struct {
	mu        sync.Mutex
	factories map[Role]AgentFactory
	spawned   []Agent
}

// NewRegistry creates a Registry pre-registered with all specialist agents.
func NewRegistry() *Registry {
	r := &Registry{
		factories: make(map[Role]AgentFactory),
	}
	r.factories[RoleResearch] = func() Agent { return NewResearchAgent() }
	r.factories[RoleSchema] = func() Agent { return NewSchemaAgent() }
	r.factories[RolePlanning] = func() Agent { return NewPlanningAgent() }
	r.factories[RoleTaskWriter] = func() Agent { return NewTaskWriterAgent() }
	return r
}

// Spawn creates a single agent by role using the registered factory.
func (r *Registry) Spawn(role Role) (Agent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	factory, ok := r.factories[role]
	if !ok {
		return nil, fmt.Errorf("no factory registered for role %q", role)
	}
	ag := factory()
	r.spawned = append(r.spawned, ag)
	return ag, nil
}

// SpawnAll creates all registered agents, assigns sequential ports starting
// from basePort, and starts each agent's HTTP server.
func (r *Registry) SpawnAll(ctx context.Context, basePort int) ([]Agent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Use a deterministic order for port assignment.
	roles := []Role{RoleResearch, RoleSchema, RolePlanning, RoleTaskWriter}

	var agents []Agent
	for i, role := range roles {
		factory, ok := r.factories[role]
		if !ok {
			// Stop any agents that were already started.
			for j := len(agents) - 1; j >= 0; j-- {
				_ = agents[j].Stop(ctx)
			}
			return nil, fmt.Errorf("no factory registered for role %q", role)
		}

		ag := factory()
		addr := fmt.Sprintf("127.0.0.1:%d", basePort+i)
		if err := ag.Start(ctx, addr); err != nil {
			// Stop any agents that were already started.
			for j := len(agents) - 1; j >= 0; j-- {
				_ = agents[j].Stop(ctx)
			}
			return nil, fmt.Errorf("start agent %q on %s: %w", role, addr, err)
		}

		agents = append(agents, ag)
	}

	r.spawned = append(r.spawned, agents...)
	return agents, nil
}

// StopAll gracefully stops all spawned agents in reverse order.
func (r *Registry) StopAll(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	var firstErr error
	for i := len(r.spawned) - 1; i >= 0; i-- {
		if err := r.spawned[i].Stop(ctx); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	r.spawned = nil
	return firstErr
}

// --- Specialist agent types ---
//
// Implementations are in their respective files:
// research.go, schema.go, planning.go, taskwriter.go
