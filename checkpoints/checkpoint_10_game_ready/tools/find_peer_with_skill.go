package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"myclaw/peers"
)

// FindPeerWithSkill is a tool that searches the peer registry for agents with a matching skill.
type FindPeerWithSkill struct {
	Registry *peers.Registry
}

func (t FindPeerWithSkill) Name() string { return "find_peer_with_skill" }
func (t FindPeerWithSkill) Description() string {
	return "Search the peer registry for agents that have a specific skill or capability. Returns matching peers with their URLs."
}

func (t FindPeerWithSkill) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"skill": map[string]any{
				"type":        "string",
				"description": "The skill to search for (e.g., 'mage', 'rogue', 'file operations').",
			},
		},
		"required":             []string{"skill"},
		"additionalProperties": false,
	}
}

func (t FindPeerWithSkill) Execute(_ context.Context, params json.RawMessage) (string, error) {
	var p struct {
		Skill string `json:"skill"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return "", fmt.Errorf("invalid parameters: %w", err)
	}
	if p.Skill == "" {
		return "", fmt.Errorf("skill is required")
	}

	matches := t.Registry.FindBySkill(p.Skill)
	if len(matches) == 0 {
		return fmt.Sprintf("No peers found with skill matching %q.", p.Skill), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d peer(s) with skill matching %q:\n", len(matches), p.Skill))
	for _, card := range matches {
		sb.WriteString(fmt.Sprintf("  - %s (%s)\n", card.Name, card.URL))
		for _, skill := range card.Skills {
			sb.WriteString(fmt.Sprintf("      Skill: %s - %s\n", skill.Name, skill.Description))
		}
	}

	return sb.String(), nil
}
