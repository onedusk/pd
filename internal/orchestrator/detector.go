package orchestrator

import "context"

// Detector probes the local environment to determine available capabilities.
type Detector interface {
	// Detect probes for A2A agents, MCP tools, and code intelligence,
	// and returns the highest available capability level.
	Detect(ctx context.Context) (CapabilityLevel, []string, error)
}
