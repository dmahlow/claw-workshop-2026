package a2a

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
)

// MessageHandler is called when an A2A message arrives.
// It receives the text content and returns the agent's response text.
type MessageHandler func(text string) (string, error)

// Server handles A2A protocol endpoints on an existing HTTP mux.
type Server struct {
	card    AgentCard
	handler MessageHandler
	mu      sync.RWMutex
}

// NewServer creates a new A2A server with the given agent card and message handler.
func NewServer(card AgentCard, handler MessageHandler) *Server {
	return &Server{
		card:    card,
		handler: handler,
	}
}

// RegisterRoutes adds the A2A endpoints to the provided HTTP mux.
// - /.well-known/agent-card.json serves the Agent Card
// - /a2a handles JSON-RPC 2.0 message requests
func (s *Server) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/.well-known/agent-card.json", s.handleAgentCard)
	mux.HandleFunc("/a2a", s.handleA2A)
}

func (s *Server) handleAgentCard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.mu.RLock()
	card := s.card
	s.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(card); err != nil {
		slog.Error("failed to encode agent card", "error", err)
	}
}

func (s *Server) handleA2A(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req JSONRPCRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONRPCError(w, nil, -32700, "parse error: "+err.Error())
		return
	}

	if req.JSONRPC != "2.0" {
		writeJSONRPCError(w, req.ID, -32600, "invalid request: jsonrpc must be 2.0")
		return
	}

	switch req.Method {
	case "message/send":
		s.handleMessageSend(w, req)
	default:
		writeJSONRPCError(w, req.ID, -32601, fmt.Sprintf("method not found: %s", req.Method))
	}
}

func (s *Server) handleMessageSend(w http.ResponseWriter, req JSONRPCRequest) {
	// Parse params.
	paramsBytes, err := json.Marshal(req.Params)
	if err != nil {
		writeJSONRPCError(w, req.ID, -32602, "invalid params: "+err.Error())
		return
	}

	var params SendMessageParams
	if err := json.Unmarshal(paramsBytes, &params); err != nil {
		writeJSONRPCError(w, req.ID, -32602, "invalid params: "+err.Error())
		return
	}

	// Extract text from message parts.
	var textParts []string
	for _, part := range params.Message.Parts {
		if part.Type == "text" && part.Text != "" {
			textParts = append(textParts, part.Text)
		}
	}
	text := strings.Join(textParts, "\n")
	if text == "" {
		writeJSONRPCError(w, req.ID, -32602, "message contains no text parts")
		return
	}

	slog.Info("a2a message received", "text_length", len(text))

	// Call the handler.
	response, err := s.handler(text)
	if err != nil {
		slog.Error("a2a handler error", "error", err)
		writeJSONRPCError(w, req.ID, -32000, "handler error: "+err.Error())
		return
	}

	// Build the response.
	result := SendMessageResult{
		Message: Message{
			Role: "agent",
			Parts: []Part{
				{Type: "text", Text: response},
			},
		},
	}

	writeJSONRPCResult(w, req.ID, result)
}

func writeJSONRPCResult(w http.ResponseWriter, id interface{}, result interface{}) {
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		slog.Error("failed to write JSON-RPC response", "error", err)
	}
}

func writeJSONRPCError(w http.ResponseWriter, id interface{}, code int, message string) {
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &JSONRPCError{
			Code:    code,
			Message: message,
		},
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK) // JSON-RPC errors still use 200
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		slog.Error("failed to write JSON-RPC error response", "error", err)
	}
}
