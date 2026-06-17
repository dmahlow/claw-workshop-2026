# Round 2: Look + Event-Driven LLM Calls

> Stuck for more than 2 minutes? Paste this to your coding agent.

## Goal

Upgrade the explorer from Round 1 to detect **interesting events** in the
look response and only then send a message to the LLM. This is the hybrid
architecture: Go code handles routine movement, the LLM handles decisions.

## What counts as interesting

Simple string matching on the look description text:

- `"locked door"` - a door is found
- `"golden glow"` or `"CROWN JEWEL"` - the jewel is nearby
- `"CONVERGENCE LOCK"` - endgame sequence
- `">> "` with actionable keywords - broadcast messages needing a response

Everything else: keep moving deterministically, no LLM call.

## Updated Explorer struct

Add an `OnEvent` callback and a `parseInteresting` check to the tick method:

```go
type Explorer struct {
	ExplorerID *string
	OnEvent    func(content string) // callback to send interesting events to agent loop

	visited map[string]int
	lastDir string
	mu      sync.Mutex
}
```

Note: we use a callback (`OnEvent`) instead of a typed channel to avoid an
import cycle between `tools` and `agent` packages.

In `tick()`, after parsing the look response, check for interesting events
**before** doing deterministic movement:

```go
func (e *Explorer) tick(ctx context.Context) {
	// ... look call and JSON parse same as Round 1 ...

	desc := lookResp.Description

	// Check for interesting events first
	interesting, instruction := parseInteresting(desc)
	if interesting {
		// Send to LLM for a decision
		if e.OnEvent != nil {
			e.OnEvent(fmt.Sprintf("[Explorer event] %s\n\nCurrent view:\n%s", instruction, desc))
		}
		return // don't move deterministically this tick
	}

	// Nothing interesting - deterministic movement (same as Round 1)
	exits := parseExits(desc)
	// ... pick direction and move ...
}
```

## parseInteresting

```go
func parseInteresting(desc string) (bool, string) {
	lower := strings.ToLower(desc)

	if strings.Contains(lower, "convergence lock") {
		return true, "The CONVERGENCE LOCK is active. Navigate to the jewel coordinates and broadcast them."
	}
	if strings.Contains(lower, "golden glow") || strings.Contains(desc, "CROWN JEWEL") {
		return true, "You can see the CROWN JEWEL nearby. Move toward it and broadcast its location."
	}
	if strings.Contains(lower, "locked door") {
		return true, "You found a locked door. If it matches your role, use your ability. Otherwise broadcast its location."
	}
	if strings.Contains(desc, ">> ") {
		if strings.Contains(lower, "need") || strings.Contains(lower, "converge") || strings.Contains(lower, "jewel") {
			return true, "Broadcast messages have actionable info. Decide what to do."
		}
	}

	return false, ""
}
```

## Updated wiring in main.go

The explorer now needs a callback to feed events into the agent loop:

```go
explorer := &tools.Explorer{
	ExplorerID: &explorerID,
	OnEvent: func(content string) {
		msgChan <- agent.Message{
			Content: content,
			Source:  "explorer",
			ReplyTo: func(s string) { fmt.Print(s) },
			Done:    func() { fmt.Println() },
		}
	},
}

&tools.JoinGame{
	// ...
	OnJoin: func() {
		go explorer.Start(ctx)
	},
},
```

## How it works

- **~95% of ticks**: no interesting event, explorer moves on its own, zero LLM calls
- **~5% of ticks**: door/jewel/broadcast found, one LLM call with focused context
- The LLM gets the full look description plus a focused instruction telling it what to decide
- The LLM responds using the existing move/use_ability/broadcast tools

This is the core pattern: **deterministic fast loop + event-driven LLM calls**.

Rebuild: `CGO_ENABLED=0 go build -o myclaw .` then rejoin.
