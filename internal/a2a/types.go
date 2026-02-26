package a2a

import (
	"encoding/json"
	"time"
)

// --- Enums ---

// TaskState represents the lifecycle state of an A2A task.
// Maps to a2a.proto TaskState enum.
type TaskState string

const (
	TaskStateUnspecified   TaskState = ""
	TaskStateSubmitted     TaskState = "submitted"
	TaskStateWorking       TaskState = "working"
	TaskStateCompleted     TaskState = "completed"
	TaskStateFailed        TaskState = "failed"
	TaskStateCanceled      TaskState = "canceled"
	TaskStateInputRequired TaskState = "input-required"
	TaskStateRejected      TaskState = "rejected"
	TaskStateAuthRequired  TaskState = "auth-required"
)

// IsTerminal returns true if the task state is a final state.
func (s TaskState) IsTerminal() bool {
	switch s {
	case TaskStateCompleted, TaskStateFailed, TaskStateCanceled, TaskStateRejected:
		return true
	}
	return false
}

// Role identifies the sender of a message.
type Role string

const (
	RoleUser  Role = "user"
	RoleAgent Role = "agent"
)

// --- Core Types ---

// Task is the primary unit of work in A2A.
type Task struct {
	ID        string          `json:"id"`
	ContextID string          `json:"contextId"`
	Status    TaskStatus      `json:"status"`
	Artifacts []Artifact      `json:"artifacts,omitempty"`
	History   []Message       `json:"history,omitempty"`
	Metadata  json.RawMessage `json:"metadata,omitempty"`
}

// TaskStatus tracks the current state and when it changed.
type TaskStatus struct {
	State     TaskState `json:"state"`
	Message   *Message  `json:"message,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// Message is a unit of communication between client and agent.
type Message struct {
	MessageID        string          `json:"messageId"`
	ContextID        string          `json:"contextId,omitempty"`
	TaskID           string          `json:"taskId,omitempty"`
	Role             Role            `json:"role"`
	Parts            []Part          `json:"parts"`
	Metadata         json.RawMessage `json:"metadata,omitempty"`
	Extensions       []string        `json:"extensions,omitempty"`
	ReferenceTaskIDs []string        `json:"referenceTaskIds,omitempty"`
}

// Part carries content within a message or artifact.
// Exactly one of Text, Raw, URL, or Data must be set.
type Part struct {
	Text      string          `json:"text,omitempty"`
	Raw       []byte          `json:"raw,omitempty"`
	URL       string          `json:"url,omitempty"`
	Data      json.RawMessage `json:"data,omitempty"`
	Metadata  json.RawMessage `json:"metadata,omitempty"`
	Filename  string          `json:"filename,omitempty"`
	MediaType string          `json:"mediaType,omitempty"`
}

// TextPart creates a Part with text content.
func TextPart(text string) Part {
	return Part{Text: text, MediaType: "text/plain"}
}

// DataPart creates a Part with structured JSON data.
func DataPart(v any) (Part, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return Part{}, err
	}
	return Part{Data: data, MediaType: "application/json"}, nil
}

// Artifact is an output produced by an agent for a task.
type Artifact struct {
	ArtifactID  string          `json:"artifactId"`
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parts       []Part          `json:"parts"`
	Metadata    json.RawMessage `json:"metadata,omitempty"`
	Extensions  []string        `json:"extensions,omitempty"`
}

// --- Agent Card Types ---

// AgentCard is the self-describing manifest for an A2A agent.
type AgentCard struct {
	Name               string            `json:"name"`
	Description        string            `json:"description"`
	Version            string            `json:"version"`
	Interfaces         []AgentInterface  `json:"supportedInterfaces"`
	Provider           *AgentProvider    `json:"provider,omitempty"`
	DocumentationURL   string            `json:"documentationUrl,omitempty"`
	Capabilities       AgentCapabilities `json:"capabilities"`
	DefaultInputModes  []string          `json:"defaultInputModes"`
	DefaultOutputModes []string          `json:"defaultOutputModes"`
	Skills             []AgentSkill      `json:"skills"`
}

// AgentInterface declares a protocol binding endpoint.
type AgentInterface struct {
	URL             string `json:"url"`
	ProtocolBinding string `json:"protocolBinding"`
	ProtocolVersion string `json:"protocolVersion"`
}

// AgentProvider identifies the service provider.
type AgentProvider struct {
	Organization string `json:"organization"`
	URL          string `json:"url,omitempty"`
}

// AgentCapabilities declares which optional A2A features the agent supports.
type AgentCapabilities struct {
	Streaming         bool `json:"streaming"`
	PushNotifications bool `json:"pushNotifications"`
}

// AgentSkill declares a distinct capability of an agent.
type AgentSkill struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
	Examples    []string `json:"examples,omitempty"`
	InputModes  []string `json:"inputModes,omitempty"`
	OutputModes []string `json:"outputModes,omitempty"`
}

// --- Streaming Types ---

// TaskStatusUpdateEvent is sent when a task's status changes.
type TaskStatusUpdateEvent struct {
	TaskID    string          `json:"taskId"`
	ContextID string          `json:"contextId"`
	Status    TaskStatus      `json:"status"`
	Metadata  json.RawMessage `json:"metadata,omitempty"`
}

// TaskArtifactUpdateEvent is sent when an artifact is produced or updated.
type TaskArtifactUpdateEvent struct {
	TaskID    string          `json:"taskId"`
	ContextID string          `json:"contextId"`
	Artifact  Artifact        `json:"artifact"`
	Append    bool            `json:"append"`
	LastChunk bool            `json:"lastChunk"`
	Metadata  json.RawMessage `json:"metadata,omitempty"`
}

// --- Request / Response Types ---

// SendMessageRequest initiates or continues a task.
type SendMessageRequest struct {
	Message       Message            `json:"message"`
	Configuration *SendMessageConfig `json:"configuration,omitempty"`
}

// SendMessageConfig controls message handling behavior.
type SendMessageConfig struct {
	AcceptedOutputModes []string `json:"acceptedOutputModes,omitempty"`
	HistoryLength       *int     `json:"historyLength,omitempty"`
	Blocking            bool     `json:"blocking"`
}

// GetTaskRequest retrieves a task by ID.
type GetTaskRequest struct {
	ID            string `json:"id"`
	HistoryLength *int   `json:"historyLength,omitempty"`
}

// ListTasksRequest queries tasks with filtering and pagination.
type ListTasksRequest struct {
	ContextID            string `json:"contextId,omitempty"`
	Status               string `json:"status,omitempty"`
	StatusTimestampAfter string `json:"statusTimestampAfter,omitempty"`
	PageSize             int    `json:"pageSize,omitempty"`
	PageToken            string `json:"pageToken,omitempty"`
	HistoryLength        *int   `json:"historyLength,omitempty"`
	IncludeArtifacts     bool   `json:"includeArtifacts,omitempty"`
}

// ListTasksResponse is the paginated response for ListTasks.
type ListTasksResponse struct {
	Tasks         []Task `json:"tasks"`
	TotalSize     int    `json:"totalSize"`
	NextPageToken string `json:"nextPageToken,omitempty"`
}

// CancelTaskRequest cancels a running task.
type CancelTaskRequest struct {
	ID string `json:"id"`
}
