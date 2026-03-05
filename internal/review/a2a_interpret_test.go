package review

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/onedusk/pd/internal/a2a"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSubmitInterpretTask(t *testing.T) {
	// Set up a mock A2A server.
	var receivedReq a2a.JSONRPCRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/.well-known/agent-card.json" {
			card := a2a.AgentCard{
				Name:    "review-agent",
				Version: "1.0.0",
				Skills: []a2a.AgentSkill{
					{ID: "review-interpret", Name: "Review Interpret"},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(card)
			return
		}

		// Handle SendMessage.
		json.NewDecoder(r.Body).Decode(&receivedReq)

		task := a2a.Task{
			ID:        "task-123",
			ContextID: "test-decomp",
			Status: a2a.TaskStatus{
				State: a2a.TaskStateSubmitted,
			},
		}

		resp := a2a.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      receivedReq.ID,
		}
		result, _ := json.Marshal(task)
		resp.Result = result

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// Create temp decomposition dir with findings.
	tmpDir := t.TempDir()
	decompDir := filepath.Join(tmpDir, "docs", "decompose", "test-decomp")
	require.NoError(t, os.MkdirAll(decompDir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(decompDir, "review-findings.md"),
		[]byte("# Review Findings\n\nTest findings content."),
		0o644,
	))

	ctx := context.Background()
	client := a2a.NewHTTPClient()

	cfg := InterpretConfig{
		DecompName: "test-decomp",
		DecompDir:  decompDir,
		Endpoints:  []string{server.URL},
	}

	result, err := SubmitInterpretTask(ctx, client, cfg)
	require.NoError(t, err)
	assert.Equal(t, "task-123", result.TaskID)
	assert.Equal(t, server.URL, result.Endpoint)

	// Verify the request was a SendMessage.
	assert.Equal(t, "message/send", receivedReq.Method)
}

func TestSubmitInterpretTaskNoAgent(t *testing.T) {
	// Server without the review-interpret skill.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/.well-known/agent-card.json" {
			card := a2a.AgentCard{
				Name:    "other-agent",
				Version: "1.0.0",
				Skills: []a2a.AgentSkill{
					{ID: "other-skill", Name: "Other"},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(card)
			return
		}
	}))
	defer server.Close()

	ctx := context.Background()
	client := a2a.NewHTTPClient()

	cfg := InterpretConfig{
		DecompName: "test",
		DecompDir:  t.TempDir(),
		Endpoints:  []string{server.URL},
	}

	_, err := SubmitInterpretTask(ctx, client, cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no review-interpret agent available")
}

func TestSubmitInterpretTaskNoEndpoints(t *testing.T) {
	ctx := context.Background()
	client := a2a.NewHTTPClient()

	cfg := InterpretConfig{
		DecompName: "test",
		DecompDir:  t.TempDir(),
		Endpoints:  nil,
	}

	_, err := SubmitInterpretTask(ctx, client, cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no agent endpoints configured")
}

func TestHasSkill(t *testing.T) {
	card := &a2a.AgentCard{
		Skills: []a2a.AgentSkill{
			{ID: "review-interpret", Name: "Review Interpret"},
			{ID: "code-review", Name: "Code Review"},
		},
	}

	assert.True(t, hasSkill(card, "review-interpret"))
	assert.True(t, hasSkill(card, "code-review"))
	assert.False(t, hasSkill(card, "nonexistent"))
}
