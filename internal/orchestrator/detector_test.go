package orchestrator

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/dusk-indust/decompose/internal/a2a"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mockAgentCardHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/agent-card.json", func(w http.ResponseWriter, r *http.Request) {
		card := a2a.AgentCard{
			Name:        "test-agent",
			Description: "test agent for detector tests",
			Version:     "dev",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(card)
	})
	return mux
}

func serverPort(t *testing.T, ts *httptest.Server) int {
	t.Helper()
	u, err := url.Parse(ts.URL)
	require.NoError(t, err)
	port, err := strconv.Atoi(u.Port())
	require.NoError(t, err)
	return port
}

func TestDetector_NoServices(t *testing.T) {
	client := a2a.NewHTTPClient(a2a.WithTimeout(200 * time.Millisecond))
	d := NewDefaultDetector(client, false)
	// Use a narrow port range that's very unlikely to have anything listening.
	d.portRange = [2]int{19100, 19101}
	d.probeTimeout = 200 * time.Millisecond

	ctx := context.Background()
	level, agents, err := d.Detect(ctx)
	require.NoError(t, err)
	// MCP tools are always available → CapMCPOnly.
	assert.Equal(t, CapMCPOnly, level)
	assert.Empty(t, agents)
}

func TestDetector_SingleAgentOverride(t *testing.T) {
	client := a2a.NewHTTPClient()
	d := NewDefaultDetector(client, true)

	ctx := context.Background()
	level, agents, err := d.Detect(ctx)
	require.NoError(t, err)
	assert.Equal(t, CapBasic, level)
	assert.Nil(t, agents)
}

func TestDetector_MockAgentDiscovered(t *testing.T) {
	ts := httptest.NewServer(mockAgentCardHandler())
	defer ts.Close()

	port := serverPort(t, ts)

	client := a2a.NewHTTPClient()
	d := NewDefaultDetector(client, false)
	d.portRange = [2]int{port, port}
	d.probeTimeout = 2 * time.Second

	ctx := context.Background()
	level, agents, err := d.Detect(ctx)
	require.NoError(t, err)
	assert.Equal(t, CapA2AMCP, level)
	require.Len(t, agents, 1)
	assert.Contains(t, agents[0], strconv.Itoa(port))
}

func TestDetector_AgentTimeout(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
	}))
	defer ts.Close()

	port := serverPort(t, ts)

	client := a2a.NewHTTPClient(a2a.WithTimeout(200 * time.Millisecond))
	d := NewDefaultDetector(client, false)
	d.portRange = [2]int{port, port}
	d.probeTimeout = 200 * time.Millisecond

	start := time.Now()
	ctx := context.Background()
	level, agents, err := d.Detect(ctx)
	elapsed := time.Since(start)

	require.NoError(t, err)
	// Agent should timeout → not discovered. hasMCP=true → CapMCPOnly.
	assert.Equal(t, CapMCPOnly, level)
	assert.Empty(t, agents)
	// Should not hang for 5 seconds.
	assert.Less(t, elapsed, 3*time.Second)
}

func TestDetector_MixedReachability(t *testing.T) {
	// One valid agent.
	ts1 := httptest.NewServer(mockAgentCardHandler())
	defer ts1.Close()
	port1 := serverPort(t, ts1)

	// One that returns 404 (no agent card).
	ts2 := httptest.NewServer(http.NotFoundHandler())
	defer ts2.Close()
	port2 := serverPort(t, ts2)

	client := a2a.NewHTTPClient()
	d := NewDefaultDetector(client, false)
	d.portRange = [2]int{min(port1, port2), max(port1, port2)}
	d.probeTimeout = 2 * time.Second

	ctx := context.Background()
	level, agents, err := d.Detect(ctx)
	require.NoError(t, err)
	assert.Equal(t, CapA2AMCP, level)
	// Only the valid agent should be in the list.
	assert.Len(t, agents, 1)
	assert.Contains(t, agents[0], strconv.Itoa(port1))
}
