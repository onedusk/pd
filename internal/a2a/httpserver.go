package a2a

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// Start creates an HTTP server, registers routes, and begins serving.
// It returns immediately after starting the server in a background goroutine.
func (s *Server) Start(ctx context.Context, addr string) error {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /.well-known/agent-card.json", s.handleAgentCard)
	mux.HandleFunc("POST /", s.handleJSONRPC)

	s.http = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	go s.http.ListenAndServe()

	return nil
}

// Stop gracefully shuts down the HTTP server.
func (s *Server) Stop(ctx context.Context) error {
	return s.http.Shutdown(ctx)
}

// handleAgentCard serves the agent card as JSON at the well-known endpoint.
func (s *Server) handleAgentCard(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(s.card); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// handleJSONRPC processes incoming JSON-RPC 2.0 requests and dispatches them
// to the appropriate handler method.
func (s *Server) handleJSONRPC(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req JSONRPCRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONRPCError(w, nil, ErrCodeParse, "Parse error: "+err.Error())
		return
	}

	ctx := r.Context()

	switch req.Method {
	case MethodSendMessage:
		s.dispatchSendMessage(ctx, w, &req)
	case MethodGetTask:
		s.dispatchGetTask(ctx, w, &req)
	case MethodListTasks:
		s.dispatchListTasks(ctx, w, &req)
	case MethodCancelTask:
		s.dispatchCancelTask(ctx, w, &req)
	default:
		writeJSONRPCError(w, req.ID, ErrCodeMethodNotFound, fmt.Sprintf("Method not found: %s", req.Method))
	}
}

// dispatchSendMessage unmarshals params and calls HandleSendMessage.
func (s *Server) dispatchSendMessage(ctx context.Context, w http.ResponseWriter, req *JSONRPCRequest) {
	var params SendMessageRequest
	if err := json.Unmarshal(req.Params, &params); err != nil {
		writeJSONRPCError(w, req.ID, ErrCodeInvalidParams, "Invalid params: "+err.Error())
		return
	}

	result, err := s.handler.HandleSendMessage(ctx, params)
	if err != nil {
		writeJSONRPCError(w, req.ID, ErrCodeInternal, err.Error())
		return
	}

	writeJSONRPCResult(w, req.ID, result)
}

// dispatchGetTask unmarshals params and calls HandleGetTask.
func (s *Server) dispatchGetTask(ctx context.Context, w http.ResponseWriter, req *JSONRPCRequest) {
	var params GetTaskRequest
	if err := json.Unmarshal(req.Params, &params); err != nil {
		writeJSONRPCError(w, req.ID, ErrCodeInvalidParams, "Invalid params: "+err.Error())
		return
	}

	result, err := s.handler.HandleGetTask(ctx, params)
	if err != nil {
		writeJSONRPCError(w, req.ID, ErrCodeInternal, err.Error())
		return
	}

	writeJSONRPCResult(w, req.ID, result)
}

// dispatchListTasks unmarshals params and calls HandleListTasks.
func (s *Server) dispatchListTasks(ctx context.Context, w http.ResponseWriter, req *JSONRPCRequest) {
	var params ListTasksRequest
	if err := json.Unmarshal(req.Params, &params); err != nil {
		writeJSONRPCError(w, req.ID, ErrCodeInvalidParams, "Invalid params: "+err.Error())
		return
	}

	result, err := s.handler.HandleListTasks(ctx, params)
	if err != nil {
		writeJSONRPCError(w, req.ID, ErrCodeInternal, err.Error())
		return
	}

	writeJSONRPCResult(w, req.ID, result)
}

// dispatchCancelTask unmarshals params and calls HandleCancelTask.
func (s *Server) dispatchCancelTask(ctx context.Context, w http.ResponseWriter, req *JSONRPCRequest) {
	var params CancelTaskRequest
	if err := json.Unmarshal(req.Params, &params); err != nil {
		writeJSONRPCError(w, req.ID, ErrCodeInvalidParams, "Invalid params: "+err.Error())
		return
	}

	result, err := s.handler.HandleCancelTask(ctx, params)
	if err != nil {
		writeJSONRPCError(w, req.ID, ErrCodeInternal, err.Error())
		return
	}

	writeJSONRPCResult(w, req.ID, result)
}

// writeJSONRPCResult writes a successful JSON-RPC response.
func writeJSONRPCResult(w http.ResponseWriter, id any, result any) {
	data, err := json.Marshal(result)
	if err != nil {
		writeJSONRPCError(w, id, ErrCodeInternal, "Failed to marshal result: "+err.Error())
		return
	}

	resp := JSONRPCResponse{
		JSONRPC: JSONRPCVersion,
		ID:      id,
		Result:  data,
	}

	json.NewEncoder(w).Encode(resp)
}

// writeJSONRPCError writes a JSON-RPC error response.
func writeJSONRPCError(w http.ResponseWriter, id any, code int, message string) {
	resp := JSONRPCResponse{
		JSONRPC: JSONRPCVersion,
		ID:      id,
		Error: &JSONRPCError{
			Code:    code,
			Message: message,
		},
	}

	json.NewEncoder(w).Encode(resp)
}
