package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
)

const maxFileSize = 100 * 1024 // 100KB

// ReadFile is a tool that reads file contents from disk.
type ReadFile struct{}

func (t ReadFile) Name() string        { return "read_file" }
func (t ReadFile) Description() string { return "Read the contents of a file at the given path." }

func (t ReadFile) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "The path to the file to read.",
			},
		},
		"required":             []string{"path"},
		"additionalProperties": false,
	}
}

func (t ReadFile) Execute(_ context.Context, params json.RawMessage) (string, error) {
	var p struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return "", fmt.Errorf("invalid parameters: %w", err)
	}
	if p.Path == "" {
		return "", fmt.Errorf("path is required")
	}

	info, err := os.Stat(p.Path)
	if err != nil {
		return "", fmt.Errorf("cannot stat file: %w", err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("%s is a directory, not a file", p.Path)
	}
	if info.Size() > maxFileSize {
		return "", fmt.Errorf("file is too large (%d bytes, limit is %d bytes)", info.Size(), maxFileSize)
	}

	data, err := os.ReadFile(p.Path)
	if err != nil {
		return "", fmt.Errorf("cannot read file: %w", err)
	}
	return string(data), nil
}
