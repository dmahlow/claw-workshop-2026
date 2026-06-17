# Round 3: Doors + Ability + Broadcast

> Stuck for more than 2 minutes? Paste this to your coding agent.

## What's new

The `parseInteresting` function from Round 2 already detects locked doors.
When a door is found, the LLM is called with the look description and a
focused instruction. The LLM decides: use its ability (if role matches) or
broadcast the door's location.

This round adds the `use_ability` tool and ensures the LLM instructions
guide it toward door interaction.

## UseAbility tool

Add this to `tools/` (e.g. `tools/use_ability.go`). Uses the same
`gamePost()` helper from previous rounds.

```go
package tools

import (
	"context"
	"encoding/json"
	"fmt"
)

type UseAbility struct {
	ExplorerID *string
}

func (t UseAbility) Name() string        { return "use_ability" }
func (t UseAbility) Description() string {
	return "Use your role's special ability on a target (e.g., a door ID). Only works if your role matches the door's requirement and you are close enough."
}

func (t UseAbility) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"target": map[string]any{
				"type":        "string",
				"description": "The target to use your ability on (e.g., a door ID like 'door-3').",
			},
		},
		"required":             []string{"target"},
		"additionalProperties": false,
	}
}

func (t UseAbility) Execute(ctx context.Context, params json.RawMessage) (string, error) {
	var p struct {
		Target string `json:"target"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return "", fmt.Errorf("invalid parameters: %w", err)
	}
	explorerID := ""
	if t.ExplorerID != nil {
		explorerID = *t.ExplorerID
	}
	payload := map[string]string{
		"explorer_id": explorerID,
		"target":      p.Target,
	}
	return gamePost(ctx, "/api/ability", payload)
}
```

## How it fits the architecture

The explorer's tick loop runs every 3 seconds. When it sees `"locked door"`
in the look text, it sends an `agent.Message` to the LLM with:

```
[Explorer event] You found a locked door. If it matches your role, use your
ability. Otherwise broadcast its location.

Current view:
You are c-1 (role: mage) at position (3, 2).
...
  - south: locked door (door-5) - Mage door (1/3 mages present: c-1)
...
```

The LLM reads this, sees door-5 needs a mage, and calls `use_ability`
with target "door-5". Or if the role doesn't match, it calls `broadcast`
to tell peers.

**The LLM is only called when a door is found** - not every 3 seconds.

## API reference

**POST /api/ability**:
```json
{"explorer_id": "c-1", "target": "door-3"}
```

**Response when door opens** (enough matching-role agents present):
```json
{"success": true, "message": "Door opening! Mage door (3/3 mages present: c-1, c-5, c-9)"}
```

**Response when still waiting**:
```json
{"success": false, "message": "You are now at the door. Mage door (1/3 mages present: c-1)"}
```

**Response when role doesn't match**:
```json
{"success": false, "message": "This door requires a rogue specialist. Your role is mage. Find a rogue to help!", "required_roles": ["rogue"]}
```

## Register the tool

```go
tools.UseAbility{ExplorerID: &explorerID},
```

Rebuild: `CGO_ENABLED=0 go build -o myclaw .` then rejoin.
