# Stage 2: Implementation Skeletons — Agent-Parallel Decomposition

> Code-level starting points derived from the Design Pack (Stage 1).
> All code in this document compiles with Go 1.26.0. This is NOT pseudocode.
>
> Module path: `github.com/dusk-indust/decompose`

---

## Data Model Code

### File: `internal/a2a/types.go`

```go
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
	TaskStateUnspecified  TaskState = ""
	TaskStateSubmitted    TaskState = "submitted"
	TaskStateWorking      TaskState = "working"
	TaskStateCompleted    TaskState = "completed"
	TaskStateFailed       TaskState = "failed"
	TaskStateCanceled     TaskState = "canceled"
	TaskStateInputRequired TaskState = "input-required"
	TaskStateRejected     TaskState = "rejected"
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
	Name              string            `json:"name"`
	Description       string            `json:"description"`
	Version           string            `json:"version"`
	Interfaces        []AgentInterface  `json:"supportedInterfaces"`
	Provider          *AgentProvider    `json:"provider,omitempty"`
	DocumentationURL  string            `json:"documentationUrl,omitempty"`
	Capabilities      AgentCapabilities `json:"capabilities"`
	DefaultInputModes []string          `json:"defaultInputModes"`
	DefaultOutputModes []string         `json:"defaultOutputModes"`
	Skills            []AgentSkill      `json:"skills"`
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
	ContextID string         `json:"contextId"`
	Status    TaskStatus      `json:"status"`
	Metadata  json.RawMessage `json:"metadata,omitempty"`
}

// TaskArtifactUpdateEvent is sent when an artifact is produced or updated.
type TaskArtifactUpdateEvent struct {
	TaskID    string          `json:"taskId"`
	ContextID string         `json:"contextId"`
	Artifact  Artifact        `json:"artifact"`
	Append    bool            `json:"append"`
	LastChunk bool            `json:"lastChunk"`
	Metadata  json.RawMessage `json:"metadata,omitempty"`
}

// --- Request / Response Types ---

// SendMessageRequest initiates or continues a task.
type SendMessageRequest struct {
	Message       Message                `json:"message"`
	Configuration *SendMessageConfig     `json:"configuration,omitempty"`
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
```

### File: `internal/a2a/jsonrpc.go`

```go
package a2a

import "encoding/json"

// JSONRPCVersion is the JSON-RPC protocol version.
const JSONRPCVersion = "2.0"

// JSONRPCRequest is a JSON-RPC 2.0 request envelope.
type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// JSONRPCResponse is a JSON-RPC 2.0 response envelope.
type JSONRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *JSONRPCError   `json:"error,omitempty"`
}

// JSONRPCError is a JSON-RPC 2.0 error object.
type JSONRPCError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

// Standard JSON-RPC error codes.
const (
	ErrCodeParse          = -32700
	ErrCodeInvalidRequest = -32600
	ErrCodeMethodNotFound = -32601
	ErrCodeInvalidParams  = -32602
	ErrCodeInternal       = -32603

	// A2A-specific error codes.
	ErrCodeTaskNotFound    = -32001
	ErrCodeTaskNotCancelable = -32002
)

// A2A method names.
const (
	MethodSendMessage     = "message/send"
	MethodStreamMessage   = "message/stream"
	MethodGetTask         = "tasks/get"
	MethodListTasks       = "tasks/list"
	MethodCancelTask      = "tasks/cancel"
)
```

### File: `internal/graph/schema.go`

```go
package graph

// --- Enums ---

// NodeKind classifies nodes in the code intelligence graph.
type NodeKind string

const (
	NodeKindFile    NodeKind = "file"
	NodeKindSymbol  NodeKind = "symbol"
	NodeKindCluster NodeKind = "cluster"
)

// SymbolKind classifies symbols within the code graph.
type SymbolKind string

const (
	SymbolKindFunction  SymbolKind = "function"
	SymbolKindClass     SymbolKind = "class"
	SymbolKindType      SymbolKind = "type"
	SymbolKindEnum      SymbolKind = "enum"
	SymbolKindInterface SymbolKind = "interface"
	SymbolKindVariable  SymbolKind = "variable"
	SymbolKindMethod    SymbolKind = "method"
)

// EdgeKind classifies relationships between nodes.
type EdgeKind string

const (
	EdgeKindDefines    EdgeKind = "DEFINES"
	EdgeKindImports    EdgeKind = "IMPORTS"
	EdgeKindCalls      EdgeKind = "CALLS"
	EdgeKindInherits   EdgeKind = "INHERITS"
	EdgeKindImplements EdgeKind = "IMPLEMENTS"
	EdgeKindBelongs    EdgeKind = "BELONGS"
)

// Language identifies a programming language for parsing.
type Language string

const (
	LangGo         Language = "go"
	LangTypeScript Language = "typescript"
	LangPython     Language = "python"
	LangRust       Language = "rust"
)

// Tier1Languages are languages with full graph support (symbol extraction,
// call chains, dependency edges, cluster detection) tested in CI.
var Tier1Languages = []Language{LangGo, LangTypeScript, LangPython, LangRust}

// --- Models ---

// FileNode represents a source file in the code graph.
type FileNode struct {
	Path     string   `json:"path"`
	Language Language `json:"language"`
	LOC      int      `json:"loc"`
}

// SymbolNode represents a named symbol (function, class, type, etc.).
type SymbolNode struct {
	Name       string     `json:"name"`
	Kind       SymbolKind `json:"kind"`
	Exported   bool       `json:"exported"`
	FilePath   string     `json:"filePath"`
	StartLine  int        `json:"startLine"`
	EndLine    int        `json:"endLine"`
}

// ClusterNode represents a group of tightly connected files.
type ClusterNode struct {
	Name          string   `json:"name"`
	CohesionScore float64  `json:"cohesionScore"`
	Members       []string `json:"members"` // file paths
}

// Edge represents a relationship between two nodes.
type Edge struct {
	SourceID string   `json:"sourceId"`
	TargetID string   `json:"targetId"`
	Kind     EdgeKind `json:"kind"`
}

// GraphStats summarizes a code intelligence graph.
type GraphStats struct {
	FileCount    int `json:"fileCount"`
	SymbolCount  int `json:"symbolCount"`
	ClusterCount int `json:"clusterCount"`
	EdgeCount    int `json:"edgeCount"`
}

// DependencyChain is an ordered sequence of nodes forming a dependency path.
type DependencyChain struct {
	Nodes []string `json:"nodes"` // node IDs in order
	Depth int      `json:"depth"`
}

// ImpactResult describes the blast radius of changing a set of files.
type ImpactResult struct {
	DirectlyAffected  []string `json:"directlyAffected"`  // files that import changed files
	TransitivelyAffected []string `json:"transitivelyAffected"` // full downstream closure
	RiskScore         float64  `json:"riskScore"` // 0.0–1.0, based on fan-out
}
```

### File: `internal/graph/store.go`

```go
package graph

import (
	"context"
	"io"
)

// Store is the interface for the code intelligence graph backend.
// Implementations: KuzuStore (production), MemoryStore (testing).
// All graph DB access goes through this interface (ADR-006).
type Store interface {
	io.Closer

	// Schema setup — called once before any data is inserted.
	InitSchema(ctx context.Context) error

	// Write operations.
	AddFile(ctx context.Context, node FileNode) error
	AddSymbol(ctx context.Context, node SymbolNode) error
	AddCluster(ctx context.Context, node ClusterNode) error
	AddEdge(ctx context.Context, edge Edge) error

	// Read operations.
	GetFile(ctx context.Context, path string) (*FileNode, error)
	GetSymbol(ctx context.Context, filePath, name string) (*SymbolNode, error)
	QuerySymbols(ctx context.Context, query string, limit int) ([]SymbolNode, error)

	// Graph traversal.
	GetDependencies(ctx context.Context, nodeID string, direction Direction, maxDepth int) ([]DependencyChain, error)
	AssessImpact(ctx context.Context, changedFiles []string) (*ImpactResult, error)
	GetClusters(ctx context.Context) ([]ClusterNode, error)

	// Stats.
	Stats(ctx context.Context) (*GraphStats, error)
}

// Direction controls dependency traversal direction.
type Direction string

const (
	DirectionUpstream   Direction = "upstream"   // what does this depend on?
	DirectionDownstream Direction = "downstream" // what depends on this?
)
```

### File: `internal/graph/parser.go`

```go
package graph

import "context"

// ParseResult holds the extracted symbols and edges from a single file.
type ParseResult struct {
	File    FileNode     `json:"file"`
	Symbols []SymbolNode `json:"symbols"`
	Edges   []Edge       `json:"edges"` // DEFINES, IMPORTS, CALLS edges
}

// Parser extracts structural information from source files.
// Implementations: TreeSitterParser (production), StubParser (testing).
type Parser interface {
	// Parse extracts symbols and relationships from a single source file.
	// source is the file content. lang determines which grammar to use.
	Parse(ctx context.Context, path string, source []byte, lang Language) (*ParseResult, error)

	// SupportedLanguages returns the languages this parser can handle.
	SupportedLanguages() []Language

	// Close releases parser resources (Tree-sitter C memory).
	Close() error
}
```

### File: `internal/orchestrator/config.go`

```go
package orchestrator

// CapabilityLevel describes the detected runtime capabilities.
// Determines which execution mode the orchestrator uses.
type CapabilityLevel int

const (
	// CapBasic is the fallback: no Go binary features, current /decompose behavior.
	CapBasic CapabilityLevel = iota

	// CapMCPOnly has MCP tools but no A2A agents. Single agent with enhanced tools.
	CapMCPOnly

	// CapA2AMCP has A2A agents and MCP tools but no code intelligence graph.
	CapA2AMCP

	// CapFull has A2A + MCP + code intelligence. Full parallel pipeline.
	CapFull
)

func (c CapabilityLevel) String() string {
	switch c {
	case CapBasic:
		return "basic"
	case CapMCPOnly:
		return "mcp-only"
	case CapA2AMCP:
		return "a2a+mcp"
	case CapFull:
		return "full"
	default:
		return "unknown"
	}
}

// Config holds runtime configuration for a decomposition run.
type Config struct {
	// Name is the decomposition name (kebab-case).
	Name string

	// ProjectRoot is the absolute path to the target project.
	ProjectRoot string

	// OutputDir is the path to docs/decompose/<name>/.
	OutputDir string

	// Stage0Path is the path to the shared development standards file.
	// Empty if Stage 0 does not exist.
	Stage0Path string

	// Capability is the detected runtime capability level.
	Capability CapabilityLevel

	// AgentEndpoints lists discovered specialist agent URLs.
	// Empty when Capability < CapA2AMCP.
	AgentEndpoints []string

	// SingleAgent forces single-agent mode regardless of available capabilities.
	SingleAgent bool

	// Verbose enables agent-level progress output.
	Verbose bool
}
```

### File: `internal/orchestrator/orchestrator.go`

```go
package orchestrator

import "context"

// Stage identifies a pipeline stage (0–4).
type Stage int

const (
	StageDevelopmentStandards Stage = 0
	StageDesignPack           Stage = 1
	StageImplementationSkeletons Stage = 2
	StageTaskIndex            Stage = 3
	StageTaskSpecifications   Stage = 4
)

func (s Stage) String() string {
	names := [...]string{
		"development-standards",
		"design-pack",
		"implementation-skeletons",
		"task-index",
		"task-specifications",
	}
	if int(s) < len(names) {
		return names[s]
	}
	return "unknown"
}

// StageResult holds the output of a completed stage.
type StageResult struct {
	Stage     Stage
	FilePaths []string // output files written
	Sections  []Section
}

// Section is a named chunk of stage output produced by one agent.
type Section struct {
	Name    string // section identifier (e.g., "platform-baseline")
	Content string // markdown content
	Agent   string // which agent produced this section
}

// ProgressEvent is emitted to the user during pipeline execution.
type ProgressEvent struct {
	Stage   Stage
	Section string
	Status  ProgressStatus
	Message string
}

// ProgressStatus is the state of a section within a stage.
type ProgressStatus string

const (
	ProgressPending  ProgressStatus = "pending"
	ProgressWorking  ProgressStatus = "working"
	ProgressComplete ProgressStatus = "complete"
	ProgressFailed   ProgressStatus = "failed"
)

// Orchestrator coordinates the decomposition pipeline.
type Orchestrator interface {
	// RunStage executes a single pipeline stage.
	RunStage(ctx context.Context, stage Stage) (*StageResult, error)

	// RunPipeline executes stages from..to inclusive.
	RunPipeline(ctx context.Context, from, to Stage) ([]StageResult, error)

	// Progress returns a channel that emits progress events.
	Progress() <-chan ProgressEvent
}
```

### File: `internal/orchestrator/merge.go`

```go
package orchestrator

// MergeStrategy defines how parallel agent outputs are combined.
type MergeStrategy string

const (
	// MergeConcatenate joins sections in template order.
	MergeConcatenate MergeStrategy = "concatenate"
)

// MergePlan describes how to combine sections from parallel agents.
type MergePlan struct {
	Strategy     MergeStrategy
	SectionOrder []string // section names in template order
}

// CoherenceIssue is a contradiction found during post-merge validation.
type CoherenceIssue struct {
	SectionA    string // first conflicting section
	SectionB    string // second conflicting section
	Description string // what the contradiction is
}
```

### File: `internal/agent/agent.go`

```go
package agent

import (
	"context"

	"github.com/dusk-indust/decompose/internal/a2a"
)

// Agent is the interface that all specialist agents implement.
type Agent interface {
	// Card returns the agent's A2A Agent Card.
	Card() a2a.AgentCard

	// HandleTask processes an A2A task and returns the completed task.
	HandleTask(ctx context.Context, task a2a.Task, msg a2a.Message) (*a2a.Task, error)

	// Start launches the agent's HTTP server on the given address.
	Start(ctx context.Context, addr string) error

	// Stop gracefully shuts down the agent.
	Stop(ctx context.Context) error
}

// Role identifies a specialist agent type.
type Role string

const (
	RoleResearch  Role = "research"
	RoleSchema    Role = "schema"
	RolePlanning  Role = "planning"
	RoleTaskWriter Role = "task-writer"
)
```

---

## Interface Contracts

### File: `internal/mcptools/codeintel.go`

```go
package mcptools

import "github.com/dusk-indust/decompose/internal/graph"

// --- MCP Tool Input Types ---
// These structs define the JSON schema for each MCP tool's input.
// The MCP Go SDK auto-generates JSON schemas from struct tags.

// BuildGraphInput is the input for the build_graph MCP tool.
type BuildGraphInput struct {
	RepoPath   string   `json:"repoPath" jsonschema:"the absolute path to the repository to index"`
	Languages  []string `json:"languages,omitempty" jsonschema:"languages to index (default: tier-1). Values: go, typescript, python, rust"`
	ExcludeDirs []string `json:"excludeDirs,omitempty" jsonschema:"directories to exclude from indexing (e.g. vendor, node_modules)"`
}

// BuildGraphOutput is the result of the build_graph MCP tool.
type BuildGraphOutput struct {
	Stats graph.GraphStats `json:"stats"`
}

// QuerySymbolsInput is the input for the query_symbols MCP tool.
type QuerySymbolsInput struct {
	Query string `json:"query" jsonschema:"search query for symbol names (substring match)"`
	Kind  string `json:"kind,omitempty" jsonschema:"filter by symbol kind: function, class, type, enum, interface, variable, method"`
	Limit int    `json:"limit,omitempty" jsonschema:"maximum number of results (default: 20)"`
}

// QuerySymbolsOutput is the result of the query_symbols MCP tool.
type QuerySymbolsOutput struct {
	Symbols []graph.SymbolNode `json:"symbols"`
	Total   int                `json:"total"`
}

// GetDependenciesInput is the input for the get_dependencies MCP tool.
type GetDependenciesInput struct {
	NodeID    string `json:"nodeId" jsonschema:"file path or qualified symbol name"`
	Direction string `json:"direction,omitempty" jsonschema:"upstream (what it depends on) or downstream (what depends on it). Default: downstream"`
	MaxDepth  int    `json:"maxDepth,omitempty" jsonschema:"maximum traversal depth (default: 5)"`
}

// GetDependenciesOutput is the result of the get_dependencies MCP tool.
type GetDependenciesOutput struct {
	Chains []graph.DependencyChain `json:"chains"`
}

// AssessImpactInput is the input for the assess_impact MCP tool.
type AssessImpactInput struct {
	ChangedFiles []string `json:"changedFiles" jsonschema:"list of file paths that will be modified"`
}

// AssessImpactOutput is the result of the assess_impact MCP tool.
type AssessImpactOutput struct {
	Impact graph.ImpactResult `json:"impact"`
}

// GetClustersInput is the input for the get_clusters MCP tool.
type GetClustersInput struct {
	// No input required — returns all clusters.
}

// GetClustersOutput is the result of the get_clusters MCP tool.
type GetClustersOutput struct {
	Clusters []graph.ClusterNode `json:"clusters"`
}
```

### File: `internal/a2a/client.go`

```go
package a2a

import "context"

// Client is the interface for an A2A client that sends tasks to agents.
type Client interface {
	// SendMessage sends a message to an agent and returns the task.
	// For blocking mode, waits until the task reaches a terminal or interrupted state.
	SendMessage(ctx context.Context, endpoint string, req SendMessageRequest) (*Task, error)

	// GetTask retrieves a task by ID from a specific agent.
	GetTask(ctx context.Context, endpoint string, req GetTaskRequest) (*Task, error)

	// ListTasks queries tasks from a specific agent.
	ListTasks(ctx context.Context, endpoint string, req ListTasksRequest) (*ListTasksResponse, error)

	// CancelTask cancels a running task.
	CancelTask(ctx context.Context, endpoint string, req CancelTaskRequest) (*Task, error)

	// SubscribeToTask opens an SSE stream for task updates.
	SubscribeToTask(ctx context.Context, endpoint string, taskID string) (<-chan StreamEvent, error)

	// DiscoverAgent fetches the Agent Card from a well-known URI.
	DiscoverAgent(ctx context.Context, baseURL string) (*AgentCard, error)
}

// StreamEvent is a typed event received from an SSE subscription.
type StreamEvent struct {
	// Exactly one of these is set.
	Task           *Task                    `json:"task,omitempty"`
	Message        *Message                 `json:"message,omitempty"`
	StatusUpdate   *TaskStatusUpdateEvent   `json:"statusUpdate,omitempty"`
	ArtifactUpdate *TaskArtifactUpdateEvent `json:"artifactUpdate,omitempty"`

	// Err is set if the stream encountered an error.
	Err error `json:"-"`
}
```

### File: `internal/a2a/server.go`

```go
package a2a

import (
	"context"
	"net/http"
)

// Handler processes incoming A2A requests for a specialist agent.
type Handler interface {
	// HandleSendMessage processes an incoming message and returns a task.
	HandleSendMessage(ctx context.Context, req SendMessageRequest) (*Task, error)

	// HandleGetTask returns the current state of a task.
	HandleGetTask(ctx context.Context, req GetTaskRequest) (*Task, error)

	// HandleListTasks returns tasks matching the filter.
	HandleListTasks(ctx context.Context, req ListTasksRequest) (*ListTasksResponse, error)

	// HandleCancelTask cancels a running task.
	HandleCancelTask(ctx context.Context, req CancelTaskRequest) (*Task, error)
}

// Server is the HTTP server that exposes an A2A agent.
type Server struct {
	card    AgentCard
	handler Handler
	http    *http.Server
}

// NewServer creates an A2A server for the given agent.
func NewServer(card AgentCard, handler Handler) *Server {
	return &Server{
		card:    card,
		handler: handler,
	}
}
```

### File: `internal/mcptools/decompose_server.go`

```go
package mcptools

// --- MCP Tool Types for the decompose server mode (--serve-mcp) ---
// These tools are exposed when the binary runs as an MCP server for Claude Code.
// They allow the /decompose skill to call structured tools instead of shelling out.

// RunStageInput is the input for the run_stage MCP tool.
type RunStageInput struct {
	Name        string `json:"name" jsonschema:"decomposition name (kebab-case)"`
	Stage       int    `json:"stage" jsonschema:"pipeline stage to run (0-4)"`
	ProjectRoot string `json:"projectRoot,omitempty" jsonschema:"path to the target project (default: cwd)"`
}

// RunStageOutput is the result of the run_stage MCP tool.
type RunStageOutput struct {
	FilesWritten []string `json:"filesWritten"`
	Stage        int      `json:"stage"`
	Status       string   `json:"status"` // "completed" or "failed"
	Message      string   `json:"message,omitempty"`
}

// GetStatusInput is the input for the get_status MCP tool.
type GetStatusInput struct {
	Name string `json:"name" jsonschema:"decomposition name"`
}

// GetStatusOutput is the result of the get_status MCP tool.
type GetStatusOutput struct {
	Name             string       `json:"name"`
	CompletedStages  []int        `json:"completedStages"`
	NextStage        int          `json:"nextStage"`
	CapabilityLevel  string       `json:"capabilityLevel"`
}

// ListDecompositionsInput is the input for the list_decompositions MCP tool.
type ListDecompositionsInput struct {
	ProjectRoot string `json:"projectRoot,omitempty" jsonschema:"path to the project (default: cwd)"`
}

// ListDecompositionsOutput is the result of the list_decompositions MCP tool.
type ListDecompositionsOutput struct {
	Decompositions []DecompositionSummary `json:"decompositions"`
	HasStage0      bool                   `json:"hasStage0"`
}

// DecompositionSummary is a brief overview of one decomposition.
type DecompositionSummary struct {
	Name            string `json:"name"`
	CompletedStages []int  `json:"completedStages"`
	NextStage       int    `json:"nextStage"`
}
```

### File: `internal/orchestrator/detector.go`

```go
package orchestrator

import "context"

// Detector probes the local environment to determine available capabilities.
type Detector interface {
	// Detect probes for A2A agents, MCP tools, and code intelligence,
	// and returns the highest available capability level.
	Detect(ctx context.Context) (CapabilityLevel, []string, error)
}
```

### File: `cmd/decompose/main.go`

```go
package main

import (
	"fmt"
	"os"
)

// CLI flags parsed from command line.
type cliFlags struct {
	ProjectRoot string
	OutputDir   string
	Agents      string
	SingleAgent bool
	Verbose     bool
	ServeMCP    bool
	Version     bool
}

// version is set by goreleaser at build time.
var version = "dev"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	// Argument parsing, config construction, and orchestrator invocation
	// will be implemented in M1.
	_ = args
	return nil
}
```

---

## Documentation Artifacts

### Data Model Reference

#### Task (A2A)

**Key:** `id` (UUID)

**Fields:** `id`, `context_id`, `status`, `artifacts`, `history`, `metadata`

**Relationships:** `artifacts` → `Artifact` (one-to-many, cascade), `history` → `Message` (one-to-many, cascade)

#### Message (A2A)

**Key:** `message_id` (UUID)

**Fields:** `message_id`, `context_id`, `task_id`, `role`, `parts`, `metadata`, `extensions`, `reference_task_ids`

**Relationships:** `parts` → `Part` (one-to-many, cascade), `reference_task_ids` → `Task` (many-to-many, none)

#### Artifact (A2A)

**Key:** `artifact_id` (UUID, unique within task)

**Fields:** `artifact_id`, `name`, `description`, `parts`, `metadata`, `extensions`

**Relationships:** `parts` → `Part` (one-to-many, cascade)

#### AgentCard (A2A)

**Key:** `name` (string, unique)

**Fields:** `name`, `description`, `version`, `interfaces`, `provider`, `capabilities`, `default_input_modes`, `default_output_modes`, `skills`

#### FileNode (Graph)

**Key:** `path` (string, unique)

**Fields:** `path`, `language`, `loc`

#### SymbolNode (Graph)

**Key:** Composite (`file_path` + `name`)

**Fields:** `name`, `kind`, `exported`, `file_path`, `start_line`, `end_line`

#### ClusterNode (Graph)

**Key:** `name` (string, unique)

**Fields:** `name`, `cohesion_score`, `members`

#### Edge (Graph)

**Fields:** `source_id`, `target_id`, `kind`

**Relationships:** `source` → `FileNode|SymbolNode` (many-to-one), `target` → `FileNode|SymbolNode` (many-to-one)

#### Config (Orchestrator)

**Key:** `name` (string, unique per run)

**Fields:** `name`, `project_root`, `output_dir`, `stage_0_path`, `capability`, `agent_endpoints`, `single_agent`, `verbose`

---

### API / Interface Reference

#### `Store.InitSchema`

**Input:** none

**Output:** error

**Semantics:** Creates node tables (File, Symbol, Cluster) and relationship tables (DEFINES, IMPORTS, CALLS, INHERITS, IMPLEMENTS, BELONGS) in the graph database. Idempotent — safe to call multiple times.

#### `Store.AddFile`

**Input:** `node` (FileNode, required)

**Output:** error

**Semantics:** Inserts a file node. Fails if path already exists.

#### `Store.AddSymbol`

**Input:** `node` (SymbolNode, required)

**Output:** error

**Semantics:** Inserts a symbol node linked to its file. Fails if the file node doesn't exist.

#### `Store.AddEdge`

**Input:** `edge` (Edge, required)

**Output:** error

**Semantics:** Creates a typed edge between two existing nodes. Edge kind determines which relationship table is used.

#### `Store.GetDependencies`

**Input:** `nodeID` (string, required), `direction` (Direction, required), `maxDepth` (int, required)

**Output:** `[]DependencyChain`, error

**Semantics:** Traverses the graph from the given node, following edges in the specified direction up to maxDepth. Returns all paths found. For `upstream`: follows IMPORTS/CALLS edges backward. For `downstream`: follows IMPORTS/CALLS edges forward.

#### `Store.AssessImpact`

**Input:** `changedFiles` ([]string, required)

**Output:** `*ImpactResult`, error

**Semantics:** Computes the blast radius of modifying the given files. DirectlyAffected = files that import any changed file. TransitivelyAffected = full downstream closure. RiskScore = normalized fan-out (0.0 = no dependents, 1.0 = everything depends on these files).

#### `Store.GetClusters`

**Input:** none

**Output:** `[]ClusterNode`, error

**Semantics:** Returns all functional clusters. Clusters are groups of files with high internal cohesion (many mutual edges) and low external coupling.

#### `Parser.Parse`

**Input:** `path` (string, required), `source` ([]byte, required), `lang` (Language, required)

**Output:** `*ParseResult`, error

**Semantics:** Parses a single source file using Tree-sitter. Extracts all symbols (functions, classes, types, etc.), import edges (file-to-file), and call edges (symbol-to-symbol). Returns a ParseResult containing the file node, symbols, and edges. The caller is responsible for inserting these into the Store.

#### `A2A Client.SendMessage`

**Input:** `endpoint` (string, required), `req` (SendMessageRequest, required)

**Output:** `*Task`, error

**Semantics:** Sends a message to an A2A agent. Creates a new task or continues an existing one (based on context_id). In blocking mode, waits for the task to reach a terminal or interrupted state before returning. Returns the task in its final state.

#### `A2A Client.DiscoverAgent`

**Input:** `baseURL` (string, required)

**Output:** `*AgentCard`, error

**Semantics:** Fetches `{baseURL}/.well-known/agent-card.json`. Returns the parsed Agent Card. Returns error if the endpoint is unreachable or the response is invalid.

#### `Orchestrator.RunStage`

**Input:** `stage` (Stage, required)

**Output:** `*StageResult`, error

**Semantics:** Executes one pipeline stage. Depending on capability level: fans out to specialist agents (CapA2AMCP/CapFull), uses MCP tools directly (CapMCPOnly), or delegates to single-agent logic (CapBasic). Emits ProgressEvents on the Progress channel. Returns the output files written and sections produced.

#### MCP Tool: `build_graph`

**Input:** `repoPath` (string, required), `languages` ([]string, optional), `excludeDirs` ([]string, optional)

**Output:** `stats` (GraphStats)

**Semantics:** Parses all source files in the repo matching the specified languages using Tree-sitter. Builds the full knowledge graph (files, symbols, edges, clusters). Returns summary statistics.

#### MCP Tool: `query_symbols`

**Input:** `query` (string, required), `kind` (string, optional), `limit` (int, optional)

**Output:** `symbols` ([]SymbolNode), `total` (int)

**Semantics:** Searches the graph for symbols whose name contains the query string (case-insensitive substring match). Optionally filtered by symbol kind. Returns up to `limit` results (default 20) and the total count.

#### MCP Tool: `get_dependencies`

**Input:** `nodeId` (string, required), `direction` (string, optional), `maxDepth` (int, optional)

**Output:** `chains` ([]DependencyChain)

**Semantics:** Returns dependency chains from the given node. Direction defaults to "downstream" (what depends on this). MaxDepth defaults to 5.

#### MCP Tool: `assess_impact`

**Input:** `changedFiles` ([]string, required)

**Output:** `impact` (ImpactResult)

**Semantics:** Computes blast radius for a set of file changes. Returns directly affected files, transitively affected files, and a risk score.

#### MCP Tool: `get_clusters`

**Input:** none

**Output:** `clusters` ([]ClusterNode)

**Semantics:** Returns all functional clusters identified in the codebase.

---

### Example Payloads

**Example: A2A SendMessage (orchestrator → Research Agent)**

Request:
```json
{
  "message": {
    "messageId": "msg-001",
    "contextId": "auth-system",
    "role": "user",
    "parts": [
      {
        "text": "Research the current platform versions and API surfaces for: Go 1.26, MCP Go SDK, A2A protocol.",
        "mediaType": "text/plain"
      }
    ]
  },
  "configuration": {
    "blocking": true
  }
}
```

Response:
```json
{
  "id": "task-abc-123",
  "contextId": "auth-system",
  "status": {
    "state": "completed",
    "timestamp": "2026-02-26T18:30:00Z"
  },
  "artifacts": [
    {
      "artifactId": "art-001",
      "name": "platform-baseline",
      "parts": [
        {
          "text": "## Target Platform & Tooling Baseline\n\n| Component | Version | ...",
          "mediaType": "text/markdown"
        }
      ],
      "metadata": {
        "tokenCount": 1250
      }
    }
  ]
}
```

**Example: MCP build_graph**

Request:
```json
{
  "repoPath": "/Users/dev/my-project",
  "languages": ["go", "typescript"],
  "excludeDirs": ["vendor", "node_modules", ".git"]
}
```

Response:
```json
{
  "stats": {
    "fileCount": 142,
    "symbolCount": 1893,
    "clusterCount": 12,
    "edgeCount": 4721
  }
}
```

**Example: MCP assess_impact**

Request:
```json
{
  "changedFiles": [
    "internal/auth/token.go",
    "internal/auth/middleware.go"
  ]
}
```

Response:
```json
{
  "impact": {
    "directlyAffected": [
      "internal/api/handler.go",
      "internal/api/routes.go",
      "cmd/server/main.go"
    ],
    "transitivelyAffected": [
      "internal/api/handler.go",
      "internal/api/routes.go",
      "internal/api/middleware_chain.go",
      "cmd/server/main.go",
      "cmd/worker/main.go"
    ],
    "riskScore": 0.34
  }
}
```

**Example: A2A Agent Card (Research Agent)**

```json
{
  "name": "research-agent",
  "description": "Platform investigation and verification specialist for progressive decomposition.",
  "version": "0.1.0",
  "supportedInterfaces": [
    {
      "url": "http://localhost:9101",
      "protocolBinding": "JSONRPC",
      "protocolVersion": "0.4"
    }
  ],
  "capabilities": {
    "streaming": true,
    "pushNotifications": false
  },
  "defaultInputModes": ["text/plain"],
  "defaultOutputModes": ["text/markdown"],
  "skills": [
    {
      "id": "research-platform",
      "name": "Research Platform",
      "description": "Investigate a platform, framework, or SDK — current version, API surface, known limitations.",
      "tags": ["research", "platform", "versions"]
    },
    {
      "id": "verify-versions",
      "name": "Verify Versions",
      "description": "Cross-check version numbers against official sources.",
      "tags": ["research", "verification"]
    },
    {
      "id": "explore-codebase",
      "name": "Explore Codebase",
      "description": "Understand existing project structure, patterns, and conventions.",
      "tags": ["research", "codebase", "exploration"]
    }
  ]
}
```

---

## Before Moving On

Verify before proceeding to Stage 3:

- [x] Every entity from Stage 1's data model has corresponding code
- [x] All enums are defined with cases matching the design pack (TaskState, Role, NodeKind, SymbolKind, EdgeKind, CapabilityLevel, Direction, Stage, ProgressStatus)
- [x] Relationships and delete rules match the schema specification
- [x] Helper functions for common operations are included (TextPart, DataPart, IsTerminal, String methods)
- [x] Code compiles in Go 1.26.0 (all imports resolve, all types are concrete)
- [x] Interface contracts cover every API operation listed in Stage 1 (Store, Parser, Client, Handler, Orchestrator, Detector, 5 MCP tools)
- [x] Documentation artifacts accurately reflect the code
- [x] Non-obvious decisions have inline comments (ADR-006 reference on Store, thread safety on Parser, tier-1 languages)
