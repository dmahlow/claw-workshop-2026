package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"myclaw/a2a"
	"myclaw/peers"
)

// DiscoverPeer is a tool that discovers a peer agent by fetching its Agent Card.
type DiscoverPeer struct {
	Registry *peers.Registry
}

func (t DiscoverPeer) Name() string { return "discover_peer" }
func (t DiscoverPeer) Description() string {
	return "Discover a peer agent by URL. Fetches their Agent Card and adds them to the peer registry. Returns the peer's name, description, and skills."
}

func (t DiscoverPeer) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"url": map[string]any{
				"type":        "string",
				"description": "The base URL of the peer agent (e.g., http://192.168.1.42:8080).",
			},
		},
		"required":             []string{"url"},
		"additionalProperties": false,
	}
}

func (t DiscoverPeer) Execute(ctx context.Context, params json.RawMessage) (string, error) {
	var p struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return "", fmt.Errorf("invalid parameters: %w", err)
	}
	if p.URL == "" {
		return "", fmt.Errorf("url is required")
	}

	card, err := a2a.Discover(ctx, p.URL)
	if err != nil {
		return "", fmt.Errorf("discovering peer at %s: %w", p.URL, err)
	}

	t.Registry.Add(card)

	// Format the result.
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Discovered peer: %s\n", card.Name))
	sb.WriteString(fmt.Sprintf("Description: %s\n", card.Description))
	sb.WriteString(fmt.Sprintf("URL: %s\n", card.URL))
	if len(card.Skills) > 0 {
		sb.WriteString("Skills:\n")
		for _, skill := range card.Skills {
			sb.WriteString(fmt.Sprintf("  - %s: %s\n", skill.Name, skill.Description))
		}
	}

	return sb.String(), nil
}
