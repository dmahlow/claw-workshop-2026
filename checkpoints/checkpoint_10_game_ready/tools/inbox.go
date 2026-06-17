package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"myclaw/a2a"
)

// InboxPoller polls the game server's relay inbox for incoming A2A messages,
// feeds them through the agent loop, and posts the responses back.
type InboxPoller struct {
	ExplorerID string
	Handler    func(text string) (string, error)
}

// StartInboxPoller launches a goroutine that polls the game server inbox
// every second and answers incoming A2A messages via the given handler.
// It exits when ctx is done.
func StartInboxPoller(ctx context.Context, explorerID string, handler func(string) (string, error)) {
	p := &InboxPoller{ExplorerID: explorerID, Handler: handler}
	go p.run(ctx)
}

func (p *InboxPoller) run(ctx context.Context) {
	slog.Info("inbox poller started", "explorer_id", p.ExplorerID)

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	consecutiveFailures := 0
	for {
		select {
		case <-ctx.Done():
			slog.Info("inbox poller stopped", "explorer_id", p.ExplorerID)
			return
		case <-ticker.C:
			if err := p.poll(ctx); err != nil {
				consecutiveFailures++
				// Log sparsely so a briefly unreachable server doesn't spam.
				if consecutiveFailures%30 == 1 {
					slog.Debug("inbox poll failed", "error", err, "consecutive_failures", consecutiveFailures)
				}
				continue
			}
			consecutiveFailures = 0
		}
	}
}

// poll fetches pending inbox messages and handles each one.
func (p *InboxPoller) poll(ctx context.Context) error {
	body, err := gameGet(ctx, "/api/inbox?explorer_id="+url.QueryEscape(p.ExplorerID))
	if err != nil {
		return err
	}

	var inbox struct {
		Messages []struct {
			MsgID string          `json:"msg_id"`
			Body  json.RawMessage `json:"body"`
		} `json:"messages"`
	}
	if err := json.Unmarshal([]byte(body), &inbox); err != nil {
		return fmt.Errorf("parsing inbox: %w", err)
	}

	for _, msg := range inbox.Messages {
		p.handleMessage(ctx, msg.MsgID, msg.Body)
	}
	return nil
}

// handleMessage answers a single relayed JSON-RPC request through the agent loop.
func (p *InboxPoller) handleMessage(ctx context.Context, msgID string, body json.RawMessage) {
	var req a2a.JSONRPCRequest
	if err := json.Unmarshal(body, &req); err != nil {
		slog.Debug("inbox message is not valid JSON-RPC", "msg_id", msgID, "error", err)
		return
	}

	text, err := extractInboxText(req)
	if err != nil {
		p.respond(ctx, msgID, inboxErrorResponse(req.ID, -32602, err.Error()))
		return
	}

	slog.Info("inbox message received", "msg_id", msgID, "text_length", len(text))

	// Blocking: this feeds the agent loop and returns the response text.
	response, err := p.Handler(text)
	if err != nil {
		p.respond(ctx, msgID, inboxErrorResponse(req.ID, -32000, "handler error: "+err.Error()))
		return
	}

	p.respond(ctx, msgID, a2a.JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: a2a.SendMessageResult{
			Message: a2a.Message{
				Role: "agent",
				Parts: []a2a.Part{
					{Type: "text", Text: response},
				},
			},
		},
	})
}

// respond posts a JSON-RPC response for the given message back to the relay.
func (p *InboxPoller) respond(ctx context.Context, msgID string, rpcResp a2a.JSONRPCResponse) {
	payload := map[string]interface{}{
		"msg_id": msgID,
		"body":   rpcResp,
	}
	if _, err := gamePost(ctx, "/api/inbox/respond", payload); err != nil {
		slog.Warn("failed to deliver inbox response", "msg_id", msgID, "error", err)
	}
}

// extractInboxText pulls the text content out of a message/send JSON-RPC
// request, using the same logic as the A2A server.
func extractInboxText(req a2a.JSONRPCRequest) (string, error) {
	paramsBytes, err := json.Marshal(req.Params)
	if err != nil {
		return "", fmt.Errorf("invalid params: %w", err)
	}

	var params a2a.SendMessageParams
	if err := json.Unmarshal(paramsBytes, &params); err != nil {
		return "", fmt.Errorf("invalid params: %w", err)
	}

	var textParts []string
	for _, part := range params.Message.Parts {
		if part.Type == "text" && part.Text != "" {
			textParts = append(textParts, part.Text)
		}
	}
	text := strings.Join(textParts, "\n")
	if text == "" {
		return "", fmt.Errorf("message contains no text parts")
	}
	return text, nil
}

// inboxErrorResponse builds a JSON-RPC error response.
func inboxErrorResponse(id interface{}, code int, message string) a2a.JSONRPCResponse {
	return a2a.JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &a2a.JSONRPCError{
			Code:    code,
			Message: message,
		},
	}
}
