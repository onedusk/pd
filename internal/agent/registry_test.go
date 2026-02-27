package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/dusk-indust/decompose/internal/a2a"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --------------------------------------------------------------------------
// T-05.09  Registry tests
// --------------------------------------------------------------------------

func TestRegistry_SpawnEachRole(t *testing.T) {
	roles := []Role{RoleResearch, RoleSchema, RolePlanning, RoleTaskWriter}

	for _, role := range roles {
		role := role // capture
		t.Run(string(role), func(t *testing.T) {
			reg := NewRegistry()
			ag, err := reg.Spawn(role)
			require.NoError(t, err, "Spawn(%q) should succeed", role)
			require.NotNil(t, ag)

			// Verify the agent has a non-empty card name.
			card := ag.Card()
			assert.NotEmpty(t, card.Name, "agent card name should not be empty for role %q", role)
			assert.NotEmpty(t, card.Skills, "agent should have at least one skill for role %q", role)
		})
	}
}

func TestRegistry_SpawnUnknownRole(t *testing.T) {
	reg := NewRegistry()
	ag, err := reg.Spawn(Role("nonexistent"))
	require.Error(t, err, "Spawn with unknown role should return an error")
	assert.Nil(t, ag)
	assert.Contains(t, err.Error(), "nonexistent")
}

// findAvailablePort asks the OS for an unused TCP port.
func findAvailablePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	return port
}

func TestRegistry_SpawnAll(t *testing.T) {
	reg := NewRegistry()

	// Find an available base port to avoid collisions.
	basePort := findAvailablePort(t)
	// Release 3 more sequential ports by verifying they are free.
	// In practice, high port ranges rarely collide during tests.
	for i := 1; i <= 3; i++ {
		ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", basePort+i))
		if err != nil {
			// Port in use; shift the base port.
			basePort = findAvailablePort(t)
			i = 0 // restart check
			continue
		}
		ln.Close()
	}

	agents, err := reg.SpawnAll(context.Background(), basePort)
	require.NoError(t, err)
	assert.Len(t, agents, 4, "SpawnAll should create 4 agents")

	defer func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		reg.StopAll(stopCtx)
	}()

	// Wait for servers to be ready.
	time.Sleep(100 * time.Millisecond)

	// Verify each agent card is discoverable via HTTP.
	client := &http.Client{Timeout: 2 * time.Second}
	for i := 0; i < 4; i++ {
		url := fmt.Sprintf("http://127.0.0.1:%d/.well-known/agent-card.json", basePort+i)
		resp, err := client.Get(url)
		require.NoError(t, err, "GET %s should succeed", url)
		assert.Equal(t, http.StatusOK, resp.StatusCode, "agent card endpoint should return 200 for port %d", basePort+i)

		// Decode the agent card to verify it is valid JSON with a name.
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		require.NoError(t, err)

		var card a2a.AgentCard
		err = json.Unmarshal(body, &card)
		require.NoError(t, err, "agent card should be valid JSON")
		assert.NotEmpty(t, card.Name, "agent card name should be non-empty at port %d", basePort+i)
		assert.NotEmpty(t, card.Skills, "agent card should have skills at port %d", basePort+i)
	}

	// Verify the expected agent order (deterministic: research, schema, planning, task-writer).
	expectedNames := []string{"research-agent", "schema-agent", "planning-agent", "task-writer-agent"}
	for i, ag := range agents {
		assert.Equal(t, expectedNames[i], ag.Card().Name,
			"agent at index %d should be %s", i, expectedNames[i])
	}
}

func TestRegistry_StopAll(t *testing.T) {
	reg := NewRegistry()

	basePort := findAvailablePort(t)
	for i := 1; i <= 3; i++ {
		ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", basePort+i))
		if err != nil {
			basePort = findAvailablePort(t)
			i = 0
			continue
		}
		ln.Close()
	}

	_, err := reg.SpawnAll(context.Background(), basePort)
	require.NoError(t, err)

	// Wait for servers to be ready.
	time.Sleep(100 * time.Millisecond)

	// Verify at least one agent is responding before StopAll.
	client := &http.Client{Timeout: 2 * time.Second}
	url := fmt.Sprintf("http://127.0.0.1:%d/.well-known/agent-card.json", basePort)
	resp, err := client.Get(url)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// StopAll should succeed.
	stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = reg.StopAll(stopCtx)
	require.NoError(t, err, "StopAll should not return an error")

	// After StopAll, agents should no longer respond.
	// Use a short timeout so the test does not hang.
	shortClient := &http.Client{Timeout: 500 * time.Millisecond}
	_, err = shortClient.Get(url)
	assert.Error(t, err, "agent should not respond after StopAll")
}

func TestRegistry_SpawnAll_EmptyAfterStop(t *testing.T) {
	reg := NewRegistry()

	basePort := findAvailablePort(t)
	for i := 1; i <= 3; i++ {
		ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", basePort+i))
		if err != nil {
			basePort = findAvailablePort(t)
			i = 0
			continue
		}
		ln.Close()
	}

	_, err := reg.SpawnAll(context.Background(), basePort)
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = reg.StopAll(stopCtx)
	require.NoError(t, err)

	// After StopAll, a second SpawnAll on different ports should also work,
	// demonstrating that the registry clears its spawned list.
	basePort2 := findAvailablePort(t)
	for i := 1; i <= 3; i++ {
		ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", basePort2+i))
		if err != nil {
			basePort2 = findAvailablePort(t)
			i = 0
			continue
		}
		ln.Close()
	}

	agents2, err := reg.SpawnAll(context.Background(), basePort2)
	require.NoError(t, err)
	assert.Len(t, agents2, 4)

	defer func() {
		stopCtx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel2()
		reg.StopAll(stopCtx2)
	}()

	time.Sleep(100 * time.Millisecond)

	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/.well-known/agent-card.json", basePort2))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()
}
