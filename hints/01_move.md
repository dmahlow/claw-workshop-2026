# Round 1: Move + Deterministic Exploration

> Stuck for more than 2 minutes? Paste this to your coding agent.

## Goal

Make your claw explore the maze autonomously with **no LLM calls** for basic
movement. Every 3 seconds the explorer looks around, parses exits from the
text response, picks a random valid direction, and moves. Pure Go code.

## Shared explorerID

Your join_game tool sets `explorerID` via a shared pointer. Your new Explorer
struct uses the same pointer: `ExplorerID: &explorerID`

## Helper functions

If you already have `gamePost` and `gameServerURL` from your join_game
implementation, skip these. Otherwise add them to `tools/` (e.g.
`tools/game_helpers.go`):

```go
package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
)

func gameServerURL() string {
	if url := os.Getenv("GAME_SERVER_URL"); url != "" {
		return url
	}
	return "http://localhost:9090"
}

func gamePost(ctx context.Context, path string, payload interface{}) (string, error) {
	url := gameServerURL() + path
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	return string(respBody), nil
}
```

## Explorer struct

Create `tools/explorer.go`. This handles autonomous movement with zero LLM
calls. It calls /api/look, parses the text response for exits, and moves
deterministically.

```go
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand"
	"strings"
	"sync"
	"time"
)

type Explorer struct {
	ExplorerID *string // shared pointer, set after join

	visited map[string]int // "x,y" -> visit count
	lastDir string
	mu      sync.Mutex
}

func (e *Explorer) Start(ctx context.Context) {
	e.mu.Lock()
	e.visited = make(map[string]int)
	e.mu.Unlock()

	slog.Info("explorer started")

	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			e.tick(ctx)
		}
	}
}

func (e *Explorer) tick(ctx context.Context) {
	id := ""
	if e.ExplorerID != nil {
		id = *e.ExplorerID
	}
	if id == "" {
		return
	}

	// 1. Look (plain HTTP, no LLM)
	result, err := gamePost(ctx, "/api/look", map[string]string{"explorer_id": id})
	if err != nil {
		return
	}

	var lookResp struct {
		Description string `json:"description"`
	}
	if err := json.Unmarshal([]byte(result), &lookResp); err != nil {
		return
	}

	// 2. Parse exits from the text description
	exits := parseExits(lookResp.Description)
	if len(exits) == 0 {
		return
	}

	// 3. Track position and pick direction
	pos := parsePosition(lookResp.Description)
	e.mu.Lock()
	e.visited[pos]++
	dir := e.pickDirection(exits, pos)
	e.lastDir = dir
	e.mu.Unlock()

	// 4. Move (plain HTTP, no LLM)
	gamePost(ctx, "/api/move", map[string]string{
		"explorer_id": id,
		"direction":   dir,
	})
}

// parseExits extracts open passage directions from the look description.
func parseExits(desc string) []string {
	var exits []string
	inExits := false
	for _, line := range strings.Split(desc, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "Exits:" {
			inExits = true
			continue
		}
		if inExits {
			if trimmed == "" || (!strings.HasPrefix(trimmed, "-") && !strings.HasPrefix(trimmed, "None")) {
				break
			}
			if strings.Contains(trimmed, "open passage") {
				for _, dir := range []string{"north", "south", "east", "west"} {
					if strings.HasPrefix(trimmed, "- "+dir+":") {
						exits = append(exits, dir)
					}
				}
			}
		}
	}
	return exits
}

// parsePosition extracts "x,y" from "at position (x, y)".
func parsePosition(desc string) string {
	idx := strings.Index(desc, "at position (")
	if idx == -1 {
		return "0,0"
	}
	rest := desc[idx+len("at position ("):]
	end := strings.Index(rest, ")")
	if end == -1 {
		return "0,0"
	}
	return strings.ReplaceAll(rest[:end], " ", "")
}

func reverse(dir string) string {
	switch dir {
	case "north":
		return "south"
	case "south":
		return "north"
	case "east":
		return "west"
	case "west":
		return "east"
	}
	return ""
}

// pickDirection prefers unvisited cells and avoids immediate backtracking.
func (e *Explorer) pickDirection(exits []string, currentPos string) string {
	// Filter out reverse of last direction
	rev := reverse(e.lastDir)
	candidates := make([]string, 0, len(exits))
	for _, d := range exits {
		if d != rev {
			candidates = append(candidates, d)
		}
	}
	if len(candidates) == 0 {
		candidates = exits
	}

	// Score by visit count of destination
	minCount := -1
	for _, d := range candidates {
		dest := offsetPosition(currentPos, d)
		count := e.visited[dest]
		if minCount == -1 || count < minCount {
			minCount = count
		}
	}

	var best []string
	for _, d := range candidates {
		dest := offsetPosition(currentPos, d)
		if e.visited[dest] == minCount {
			best = append(best, d)
		}
	}
	return best[rand.Intn(len(best))]
}

func offsetPosition(pos, dir string) string {
	var x, y int
	fmt.Sscanf(pos, "%d,%d", &x, &y)
	switch dir {
	case "north":
		y--
	case "south":
		y++
	case "east":
		x++
	case "west":
		x--
	}
	return fmt.Sprintf("%d,%d", x, y)
}
```

## Wiring in main.go

Your JoinGame struct has an `OnJoin` callback. Use it to start the explorer
after a successful join:

```go
explorer := &tools.Explorer{
	ExplorerID: &explorerID,
}

// In your JoinGame registration:
&tools.JoinGame{
	AgentCardURL: cfg.PublicURL,
	ExplorerID:   &explorerID,
	PeerRegistry: peerRegistry,
	A2AHandler:   a2aHandler,
	OnJoin: func() {
		go explorer.Start(ctx)
	},
},
```

## API reference

**POST /api/look** returns JSON with a `description` field (plain text):
```json
{"description": "You are c-1 (role: mage) at position (3, 2).\n\nExits:\n  - north: open passage\n  - south: locked door (door-5)\n  - east: open passage\n\n..."}
```

**POST /api/move**:
```json
{"explorer_id": "c-1", "direction": "north"}
```

## Key point

No `agent.Message`, no LLM call. The explorer just looks and moves on its
own. The LLM only handles interactive commands from the user or peers.

Rebuild: `CGO_ENABLED=0 go build -o myclaw .` then rejoin.
