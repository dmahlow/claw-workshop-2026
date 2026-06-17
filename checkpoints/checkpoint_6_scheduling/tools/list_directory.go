package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// ListDirectory is a tool that lists the contents of a directory.
type ListDirectory struct{}

func (t ListDirectory) Name() string        { return "list_directory" }
func (t ListDirectory) Description() string { return "List files and directories at the given path." }

func (t ListDirectory) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "The directory path to list. Defaults to the current directory.",
			},
		},
		"additionalProperties": false,
	}
}

func (t ListDirectory) Execute(_ context.Context, params json.RawMessage) (string, error) {
	var p struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return "", fmt.Errorf("invalid parameters: %w", err)
	}
	if p.Path == "" {
		p.Path = "."
	}

	entries, err := os.ReadDir(p.Path)
	if err != nil {
		return "", fmt.Errorf("cannot read directory: %w", err)
	}

	var sb strings.Builder
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() {
			name += "/"
		}
		sb.WriteString(name)
		sb.WriteString("\n")
	}
	return sb.String(), nil
}
