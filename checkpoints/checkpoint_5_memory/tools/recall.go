package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"myclaw/memory"
)

// Recall is a tool that retrieves information from persistent memory.
type Recall struct {
	Store *memory.Store
}

func (t Recall) Name() string { return "recall" }
func (t Recall) Description() string {
	return "Search or list persistent memories. If a query is provided, search for matching memories. If no query, list all stored memory keys."
}

func (t Recall) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "Optional search query. If empty, lists all memory keys.",
			},
		},
		"additionalProperties": false,
	}
}

func (t Recall) Execute(_ context.Context, params json.RawMessage) (string, error) {
	var p struct {
		Query string `json:"query"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return "", fmt.Errorf("invalid parameters: %w", err)
	}

	if p.Query == "" {
		// List all memory keys.
		keys, err := t.Store.List()
		if err != nil {
			return "", fmt.Errorf("listing memories: %w", err)
		}
		if len(keys) == 0 {
			return "No memories stored yet.", nil
		}
		return "Stored memories:\n- " + strings.Join(keys, "\n- "), nil
	}

	// Search memories.
	results, err := t.Store.Search(p.Query)
	if err != nil {
		return "", fmt.Errorf("searching memories: %w", err)
	}
	if len(results) == 0 {
		return fmt.Sprintf("No memories found matching %q.", p.Query), nil
	}
	return "Found memories:\n" + strings.Join(results, "\n"), nil
}
