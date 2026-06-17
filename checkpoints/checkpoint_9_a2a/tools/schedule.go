package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"myclaw/scheduler"
)

// Schedule is a tool that lets the agent schedule tasks for later execution.
type Schedule struct {
	Scheduler *scheduler.Scheduler
}

func (t Schedule) Name() string { return "schedule" }
func (t Schedule) Description() string {
	return "Schedule a task for later execution. Use this for reminders or recurring tasks. You can also list currently scheduled tasks by setting action to 'list'."
}

func (t Schedule) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action": map[string]any{
				"type":        "string",
				"description": "Either 'add' to schedule a new task, or 'list' to show all scheduled tasks. Defaults to 'add'.",
				"enum":        []string{"add", "list"},
			},
			"description": map[string]any{
				"type":        "string",
				"description": "What the task should do (required for 'add').",
			},
			"delay": map[string]any{
				"type":        "string",
				"description": "How long from now to execute, e.g. '30s', '5m', '1h', '24h' (required for 'add').",
			},
			"recurring": map[string]any{
				"type":        "boolean",
				"description": "Whether this task should repeat at the given interval. Defaults to false.",
			},
		},
		"additionalProperties": false,
	}
}

func (t Schedule) Execute(_ context.Context, params json.RawMessage) (string, error) {
	var p struct {
		Action      string `json:"action"`
		Description string `json:"description"`
		Delay       string `json:"delay"`
		Recurring   bool   `json:"recurring"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return "", fmt.Errorf("invalid parameters: %w", err)
	}

	if p.Action == "list" {
		tasks := t.Scheduler.List()
		if len(tasks) == 0 {
			return "No tasks currently scheduled.", nil
		}
		var sb strings.Builder
		sb.WriteString("Scheduled tasks:\n")
		for _, task := range tasks {
			recurring := ""
			if task.Recurring {
				recurring = fmt.Sprintf(" (recurring every %s)", task.Interval)
			}
			sb.WriteString(fmt.Sprintf("- [%s] %s - fires at %s%s\n",
				task.ID, task.Description,
				task.ExecuteAt.Format(time.RFC3339), recurring))
		}
		return sb.String(), nil
	}

	// Default action: add.
	if p.Description == "" {
		return "", fmt.Errorf("description is required")
	}
	if p.Delay == "" {
		return "", fmt.Errorf("delay is required")
	}

	delay, err := time.ParseDuration(p.Delay)
	if err != nil {
		return "", fmt.Errorf("invalid delay format: %w", err)
	}

	task, err := t.Scheduler.Add(p.Description, delay, p.Recurring)
	if err != nil {
		return "", fmt.Errorf("scheduling task: %w", err)
	}

	recurring := ""
	if task.Recurring {
		recurring = fmt.Sprintf(" (recurring every %s)", task.Interval)
	}
	return fmt.Sprintf("Scheduled task %q for %s%s.",
		task.Description,
		task.ExecuteAt.Format(time.RFC3339),
		recurring), nil
}
