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
	ErrCodeTaskNotFound      = -32001
	ErrCodeTaskNotCancelable = -32002
)

// A2A method names.
const (
	MethodSendMessage   = "message/send"
	MethodStreamMessage = "message/stream"
	MethodGetTask       = "tasks/get"
	MethodListTasks     = "tasks/list"
	MethodCancelTask    = "tasks/cancel"
)
