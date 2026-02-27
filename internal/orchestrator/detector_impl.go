package orchestrator

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/dusk-indust/decompose/internal/a2a"
)

// Compile-time check.
var _ Detector = (*DefaultDetector)(nil)

// DefaultDetector probes the local environment for A2A agents, MCP tools,
// and code intelligence capabilities.
type DefaultDetector struct {
	client       a2a.Client
	singleAgent  bool
	probeTimeout time.Duration
	portRange    [2]int // [start, end] inclusive
}

// NewDefaultDetector creates a DefaultDetector. If singleAgent is true,
// Detect always returns CapBasic without probing.
func NewDefaultDetector(client a2a.Client, singleAgent bool) *DefaultDetector {
	return &DefaultDetector{
		client:       client,
		singleAgent:  singleAgent,
		probeTimeout: 500 * time.Millisecond,
		portRange:    [2]int{9100, 9110},
	}
}

// Detect probes for A2A agents, MCP tools, and code intelligence. It returns
// the highest detected capability level and any discovered agent endpoints.
func (d *DefaultDetector) Detect(ctx context.Context) (CapabilityLevel, []string, error) {
	if d.singleAgent {
		return CapBasic, nil, nil
	}

	// Probe for A2A agents in parallel.
	agents := d.probeAgents(ctx)

	// MCP tools are always available in the binary.
	hasMCP := true

	// Probe for code intelligence (CGO-dependent features).
	hasCodeIntel := d.probeCodeIntel()

	// Determine capability level.
	hasAgents := len(agents) > 0

	var level CapabilityLevel
	switch {
	case hasAgents && hasMCP && hasCodeIntel:
		level = CapFull
	case hasAgents && hasMCP:
		level = CapA2AMCP
	case hasMCP:
		level = CapMCPOnly
	default:
		level = CapBasic
	}

	log.Printf("detector: level=%s agents=%d mcp=%v codeIntel=%v", level, len(agents), hasMCP, hasCodeIntel)

	return level, agents, nil
}

// probeAgents concurrently probes the port range for A2A agent endpoints.
func (d *DefaultDetector) probeAgents(ctx context.Context) []string {
	var (
		mu       sync.Mutex
		agents   []string
		wg       sync.WaitGroup
	)

	for port := d.portRange[0]; port <= d.portRange[1]; port++ {
		endpoint := fmt.Sprintf("http://localhost:%d", port)
		wg.Add(1)
		go func(ep string) {
			defer wg.Done()
			if d.probeAgent(ctx, ep) {
				mu.Lock()
				agents = append(agents, ep)
				mu.Unlock()
			}
		}(endpoint)
	}

	wg.Wait()
	return agents
}

// probeAgent attempts to discover an A2A agent at the given endpoint.
// Returns true if the agent responds with a valid card within the timeout.
func (d *DefaultDetector) probeAgent(ctx context.Context, endpoint string) (ok bool) {
	// Catch panics from CGO or network issues.
	defer func() {
		if r := recover(); r != nil {
			log.Printf("detector: panic probing %s: %v", endpoint, r)
			ok = false
		}
	}()

	probeCtx, cancel := context.WithTimeout(ctx, d.probeTimeout)
	defer cancel()

	card, err := d.client.DiscoverAgent(probeCtx, endpoint)
	if err != nil {
		return false
	}

	return card != nil
}

// probeCodeIntel checks if CGO-dependent code intelligence features are
// available. This is a best-effort check that catches panics.
func (d *DefaultDetector) probeCodeIntel() (ok bool) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("detector: code intelligence probe panicked: %v", r)
			ok = false
		}
	}()

	// Code intelligence requires CGO for tree-sitter and KuzuDB.
	// The actual availability is determined by build tags. For now,
	// we return false and let the CGO build tag variant override this.
	return false
}
