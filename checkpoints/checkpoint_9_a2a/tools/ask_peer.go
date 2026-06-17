package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"myclaw/a2a"
	"myclaw/peers"
)

// AskPeer is a tool that sends a message to a discovered peer via A2A.
type AskPeer struct {
	Registry *peers.Registry
}

func (t AskPeer) Name() string { return "ask_peer" }
func (t AskPeer) Description() string {
	return "Send a message to a discovered peer agent and get their response. The peer must have been discovered first with discover_peer."
}

func (t AskPeer) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"peer_url": map[string]any{
				"type":        "string",
				"description": "The URL of the peer agent to message.",
			},
			"message": map[string]any{
				"type":        "string",
				"description": "The message to send to the peer.",
			},
		},
		"required":             []string{"peer_url", "message"},
		"additionalProperties": false,
	}
}

func (t AskPeer) Execute(ctx context.Context, params json.RawMessage) (string, error) {
	var p struct {
		PeerURL string `json:"peer_url"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return "", fmt.Errorf("invalid parameters: %w", err)
	}
	if p.PeerURL == "" {
		return "", fmt.Errorf("peer_url is required")
	}
	if p.Message == "" {
		return "", fmt.Errorf("message is required")
	}

	// Verify the peer is in our registry.
	if _, ok := t.Registry.Get(p.PeerURL); !ok {
		return "", fmt.Errorf("peer %s not found in registry - use discover_peer first", p.PeerURL)
	}

	response, err := a2a.SendMessage(ctx, p.PeerURL, p.Message)
	if err != nil {
		return "", fmt.Errorf("sending message to peer: %w", err)
	}

	return response, nil
}
