# Round 4: The Crown Jewel

> No new tool needed. Your explorer already handles this.

The `parseInteresting` function from Round 2 already detects:

- `"golden glow"` / `"CROWN JEWEL"` - jewel is nearby
- `"CONVERGENCE LOCK"` - endgame: all explorers must gather

When the explorer's tick loop sees these strings, it sends a message to the
LLM with the focused instruction. The LLM then navigates toward the jewel
coordinates and broadcasts them.

## How convergence works

1. First explorer to step on the jewel cell discovers it
2. The game server broadcasts coordinates to everyone via the look response
3. The jewel only opens when enough explorers gather within 3 cells
4. Every explorer's `parseInteresting` will catch the `CONVERGENCE LOCK` text

## What your parseInteresting already covers

```go
if strings.Contains(lower, "convergence lock") {
    return true, "The CONVERGENCE LOCK is active. Navigate to the jewel coordinates and broadcast them."
}
if strings.Contains(lower, "golden glow") || strings.Contains(desc, "CROWN JEWEL") {
    return true, "You can see the CROWN JEWEL nearby. Move toward it and broadcast its location."
}
```

## Optional: tune the instruction

If you want the LLM to be more specific about convergence, update the
instruction string:

```go
if strings.Contains(lower, "convergence lock") {
    return true, "CONVERGENCE LOCK is active. Extract the jewel coordinates from the description. " +
        "Use the move tool to navigate there. Broadcast the coordinates so all peers converge."
}
```

## Key point

No code changes are strictly needed. The architecture from Rounds 1-2
already handles the jewel scenario because `parseInteresting` fires on
the right keywords. The LLM sees the full look description with the
jewel coordinates and the convergence status.

Rebuild if you made changes: `CGO_ENABLED=0 go build -o myclaw .` then rejoin.
