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

// Explorer handles autonomous maze exploration with deterministic movement.
// It calls /api/look every 3 seconds, parses the text response in Go code,
// and only sends a message to the LLM when something interesting happens
// (doors, other explorers, jewel, convergence). Routine navigation is handled
// entirely in Go with no LLM calls.
type Explorer struct {
	ExplorerID *string                          // shared pointer, set after join
	OnEvent    func(content string)             // callback to send interesting events to the agent loop

	visited              map[string]int      // "x,y" -> visit count
	knownExits           map[string][]string // "x,y" -> open directions from that cell
	knownLocked          map[string][]string // "x,y" -> locked door directions from that cell
	mazeW, mazeH         int                 // maze dimensions (parsed from look or set by server)
	lastDir              string
	targetX              int    // navigation target (-1 = none)
	targetY              int
	lastEventTime        int64  // unix seconds of last LLM event fire
	lastBroadcastDoor    string // dedup: last door ID we fired an LLM broadcast for
	lastBroadcastDoorAt  int64  // unix seconds when lastBroadcastDoor was set
	lastBubble           string // dedup: last bubble text sent
	mu                   sync.Mutex
}

// Start launches the explore goroutine. It ticks every 3 seconds until ctx
// is cancelled.
func (e *Explorer) Start(ctx context.Context) {
	e.mu.Lock()
	e.visited = make(map[string]int)
	e.knownExits = make(map[string][]string)
	e.knownLocked = make(map[string][]string)
	e.targetX = -1
	e.targetY = -1
	e.mu.Unlock()

	slog.Info("explorer started", "explorer_id", e.getID())

	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("explorer stopped")
			return
		case <-ticker.C:
			e.tick(ctx)
		}
	}
}

// getID safely reads the explorer ID.
func (e *Explorer) getID() string {
	if e.ExplorerID == nil {
		return ""
	}
	return *e.ExplorerID
}

// tick runs one exploration cycle: look, parse, decide, act.
func (e *Explorer) tick(ctx context.Context) {
	id := e.getID()
	if id == "" {
		return
	}

	// 1. Look (plain HTTP, no LLM)
	payload := map[string]string{"explorer_id": id}
	result, err := gamePost(ctx, "/api/look", payload)
	if err != nil {
		slog.Warn("explorer look failed", "error", err)
		return
	}

	// Parse the JSON response to get the description text.
	var lookResp struct {
		Description string `json:"description"`
	}
	if err := json.Unmarshal([]byte(result), &lookResp); err != nil {
		slog.Warn("explorer: failed to parse look response", "error", err)
		return
	}
	desc := lookResp.Description

	// 2. Handle interesting events directly in Go where possible.
	//    Only escalate to the LLM for genuinely complex decisions.
	if e.handleEvents(ctx, id, desc) {
		return // event handled; skip deterministic move this tick
	}

	// 3. Deterministic movement - no LLM call.
	openExits, lockedExits := parseAllExits(desc)
	if len(openExits) == 0 && len(lockedExits) == 0 {
		return
	}

	// Parse current position from the description for visit tracking.
	pos := parsePosition(desc)

	// Record current position, exits, and locked doors in local map.
	var cx, cy int
	fmt.Sscanf(pos, "%d,%d", &cx, &cy)

	e.mu.Lock()
	e.visited[pos]++
	e.knownExits[pos] = openExits
	if len(lockedExits) > 0 {
		e.knownLocked[pos] = lockedExits
	}
	if cx+1 > e.mazeW {
		e.mazeW = cx + 1
	}
	if cy+1 > e.mazeH {
		e.mazeH = cy + 1
	}

	// Pick direction: BFS to target if set, otherwise explore.
	dir := e.pickDirection(openExits, pos)
	e.lastDir = dir
	e.mu.Unlock()

	// 4. Move (plain HTTP, no LLM)
	movePayload := map[string]string{
		"explorer_id": id,
		"direction":   dir,
	}
	_, err = gamePost(ctx, "/api/move", movePayload)
	if err != nil {
		slog.Warn("explorer move failed", "error", err)
	}
}

// handleEvents checks for interesting situations and handles them directly
// in Go where possible. Returns true if an event was handled (skip the
// normal deterministic move this tick). Only escalates to the LLM for
// broadcasts that need interpretation.
func (e *Explorer) handleEvents(ctx context.Context, id, desc string) bool {
	lower := strings.ToLower(desc)

	// Cooldown: only fire LLM events at most once per 25 seconds per claw.
	// Go-only actions (use_ability, setting target) always run.
	now := time.Now().Unix()
	e.mu.Lock()
	canFireLLM := (now - e.lastEventTime) >= 25
	e.mu.Unlock()

	// Convergence lock - parse coordinates and set navigation target
	if strings.Contains(lower, "convergence lock") {
		if coords := extractCoords(desc, "("); coords != "" {
			parts := strings.Split(coords, ",")
			if len(parts) == 2 {
				x, y := 0, 0
				fmt.Sscanf(parts[0], "%d", &x)
				fmt.Sscanf(parts[1], "%d", &y)
				e.mu.Lock()
				e.targetX = x
				e.targetY = y
				e.mu.Unlock()
				slog.Info("explorer targeting jewel", "x", x, "y", y)
			}
		}
		// Don't return true - let the deterministic mover navigate toward the target
	}

	// Crown jewel / golden glow nearby - set target and broadcast
	if strings.Contains(lower, "golden glow") || strings.Contains(desc, "CROWN JEWEL") {
		// Parse our position to estimate jewel direction
		pos := parsePosition(desc)
		parts := strings.Split(pos, ",")
		if len(parts) == 2 {
			x, y := 0, 0
			fmt.Sscanf(parts[0], "%d", &x)
			fmt.Sscanf(parts[1], "%d", &y)
			e.mu.Lock()
			e.targetX = x
			e.targetY = y
			e.mu.Unlock()
		}
		slog.Info("explorer sees jewel glow")
		if canFireLLM && e.OnEvent != nil {
			e.mu.Lock()
			e.lastEventTime = now
			e.mu.Unlock()
			e.sendBubble(ctx, id, "I see the CROWN JEWEL!")
			e.OnEvent("[Explorer] I can see the CROWN JEWEL! Broadcasting to all peers. Reply with ONLY: 'broadcast \"Crown jewel spotted near my position!\"' then STOP.")
		}
		return true
	}

	// Locked door - handle directly: use ability if matching role, broadcast if not
	if strings.Contains(lower, "locked door") {
		doorID, doorRole := parseDoorInfo(desc)
		if doorID != "" {
			// Try use_ability (works if our role matches, server rejects otherwise)
			result, err := gamePost(ctx, "/api/ability", map[string]string{
				"explorer_id": id,
				"target":      doorID,
			})
			if err == nil {
				slog.Info("explorer used ability on door", "door", doorID, "result", truncate(result, 80))
			}
			// Broadcast the door location via LLM (rate-limited + dedup)
			e.mu.Lock()
			isDupe := e.lastBroadcastDoor == doorID && (now-e.lastBroadcastDoorAt) < 120
			e.mu.Unlock()
			if canFireLLM && !isDupe && e.OnEvent != nil {
				e.mu.Lock()
				e.lastEventTime = now
				e.lastBroadcastDoor = doorID
				e.lastBroadcastDoorAt = now
				e.mu.Unlock()
				e.sendBubble(ctx, id, fmt.Sprintf("Found %s door %s!", doorRole, doorID))
			e.OnEvent(fmt.Sprintf("[Explorer] Found %s door %s. Reply with ONLY: 'broadcast \"Found %s door %s, need help!\"' - then STOP. Do not explore.", doorRole, doorID, doorRole, doorID))
			}
			return true
		}
	}

	// Broadcasts from peers - only escalate truly actionable ones to LLM
	if strings.Contains(desc, "BROADCASTS") && strings.Contains(desc, ">> ") {
		// Parse jewel coordinates from auto-converge broadcast (no LLM needed)
		if strings.Contains(lower, "converge") || strings.Contains(lower, "convergence") {
			for _, line := range strings.Split(desc, "\n") {
				lineLower := strings.ToLower(line)
				if (strings.Contains(lineLower, "converge") || strings.Contains(lineLower, "convergence")) && strings.Contains(line, "at (") {
					// Extract coordinates from "at (X, Y)" pattern
					idx := strings.LastIndex(line, "at (")
					if idx >= 0 {
						rest := line[idx+4:]
						end := strings.Index(rest, ")")
						if end > 0 {
							coordStr := strings.ReplaceAll(rest[:end], " ", "")
							parts := strings.Split(coordStr, ",")
							if len(parts) == 2 {
								x, y := 0, 0
								fmt.Sscanf(parts[0], "%d", &x)
								fmt.Sscanf(parts[1], "%d", &y)
								if x > 0 || y > 0 {
									e.mu.Lock()
									e.targetX = x
									e.targetY = y
									e.hotReloadDoors()
									e.mu.Unlock()
									e.sendBubble(ctx, id, fmt.Sprintf("Heading to jewel at (%d,%d)!", x, y))
								slog.Info("explorer: targeting jewel from broadcast", "x", x, "y", y)
								}
							}
						}
					}
				}
			}
			// Target set, let the movement code navigate there
			return false
		}
		if strings.Contains(lower, "crown jewel") {
			if canFireLLM && e.OnEvent != nil {
				e.mu.Lock()
				e.lastEventTime = now
				e.mu.Unlock()
				e.sendBubble(ctx, id, "Crown jewel found! Heading there!")
			e.OnEvent("[Explorer] A peer found the CROWN JEWEL. Reply with ONLY the direction you should move to reach it, nothing else. One word: north, south, east, or west.")
			}
			return true
		}
		// Other broadcasts (door locations etc.) - ignore and keep exploring
		// The deterministic mover handles navigation; we don't need LLM for "go to X,Y"
	}

	return false
}

// parseDoorInfo extracts door ID and role from a look description.
// Looks for patterns like "locked door (door-3)" and "Rogue door (2/3 rogues present)"
func parseDoorInfo(desc string) (string, string) {
	// Find door ID: "door-N" pattern
	doorID := ""
	for _, word := range strings.Fields(desc) {
		clean := strings.Trim(word, "(),")
		if strings.HasPrefix(clean, "door-") {
			doorID = clean
			break
		}
	}
	// Find role from "X door" pattern
	role := ""
	for _, r := range []string{"rogue", "mage", "knight", "sage", "ranger"} {
		if strings.Contains(strings.ToLower(desc), r+" door") {
			role = r
			break
		}
	}
	return doorID, role
}

// extractCoords pulls "X,Y" from a pattern like "at (X, Y)" near a keyword.
func extractCoords(desc, keyword string) string {
	idx := strings.Index(desc, keyword)
	if idx == -1 {
		return ""
	}
	rest := desc[idx:]
	paren := strings.Index(rest, "(")
	if paren == -1 {
		return ""
	}
	end := strings.Index(rest[paren:], ")")
	if end == -1 {
		return ""
	}
	return strings.ReplaceAll(rest[paren+1:paren+end], " ", "")
}

// sendBubble posts a speech bubble to the game server viz. Deduplicates
// by tracking the last bubble text on the explorer.
func (e *Explorer) sendBubble(ctx context.Context, explorerID, text string) {
	if explorerID == "" || text == "" {
		return
	}
	e.mu.Lock()
	if e.lastBubble == text {
		e.mu.Unlock()
		return
	}
	e.lastBubble = text
	e.mu.Unlock()
	gamePost(ctx, "/api/bubble", map[string]string{
		"explorer_id": explorerID,
		"text":        text,
	})
}

// truncate shortens a string to n chars.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// parseAllExits extracts open and locked exit directions from the look description.
func parseAllExits(desc string) (open []string, locked []string) {
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
			for _, dir := range []string{"north", "south", "east", "west"} {
				if strings.HasPrefix(trimmed, "- "+dir+":") {
					if strings.Contains(trimmed, "locked door") {
						locked = append(locked, dir)
					} else {
						open = append(open, dir)
					}
				}
			}
		}
	}
	return
}

// hotReloadDoors promotes all known locked doors to open exits.
// Called when convergence opens all doors. Must be called with e.mu held.
func (e *Explorer) hotReloadDoors() {
	for pos, dirs := range e.knownLocked {
		existing := e.knownExits[pos]
		has := make(map[string]bool)
		for _, d := range existing {
			has[d] = true
		}
		for _, d := range dirs {
			if !has[d] {
				existing = append(existing, d)
			}
		}
		e.knownExits[pos] = existing
	}
	e.knownLocked = make(map[string][]string)
	slog.Info("explorer: hot-reloaded locked doors into known exits")
}

// parsePosition extracts "x,y" from "at position (x, y)" in the description.
func parsePosition(desc string) string {
	// Look for "at position (X, Y)"
	idx := strings.Index(desc, "at position (")
	if idx == -1 {
		return "0,0"
	}
	rest := desc[idx+len("at position ("):]
	end := strings.Index(rest, ")")
	if end == -1 {
		return "0,0"
	}
	// "3, 2" -> "3,2"
	coords := strings.ReplaceAll(rest[:end], " ", "")
	return coords
}

// reverse returns the opposite direction.
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

// pickDirection chooses a direction from available exits. When a target is
// set (jewel convergence), it prefers directions that reduce distance to
// the target. Otherwise it prefers unvisited cells and avoids backtracking.
// Must be called with e.mu held.
func (e *Explorer) pickDirection(exits []string, currentPos string) string {
	if len(exits) == 0 {
		return "north"
	}

	// Parse current position.
	var cx, cy int
	fmt.Sscanf(currentPos, "%d,%d", &cx, &cy)

	// If we have a navigation target, use optimistic BFS.
	if e.targetX >= 0 && e.targetY >= 0 {
		// Clear target only when standing on it.
		if cx == e.targetX && cy == e.targetY {
			slog.Info("explorer reached target", "target", fmt.Sprintf("%d,%d", e.targetX, e.targetY))
			e.targetX = -1
			e.targetY = -1
		} else if path := e.optimisticBFS(cx, cy, e.targetX, e.targetY); len(path) > 0 {
			for _, exit := range exits {
				if exit == path[0] {
					return path[0]
				}
			}
		}
		// Near target but BFS direction was blocked: random walk to break orbit
		dist := abs(cx-e.targetX) + abs(cy-e.targetY)
		if dist <= 4 && len(exits) > 0 {
			return exits[rand.Intn(len(exits))]
		}
	}

	// No target: explore mode. Avoid backtracking, prefer unvisited.
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

	type scored struct {
		dir   string
		count int
	}
	var scoredDirs []scored
	for _, d := range candidates {
		destPos := offsetPosition(currentPos, d)
		count := e.visited[destPos]
		scoredDirs = append(scoredDirs, scored{d, count})
	}

	minCount := scoredDirs[0].count
	for _, s := range scoredDirs[1:] {
		if s.count < minCount {
			minCount = s.count
		}
	}

	var best []string
	for _, s := range scoredDirs {
		if s.count == minCount {
			best = append(best, s.dir)
		}
	}

	return best[rand.Intn(len(best))]
}

// bfsPath finds the shortest path from (fx,fy) to (tx,ty) through cells
// the explorer has visited (knownExits). Returns the direction sequence, or
// nil if no path exists in explored territory. Must be called with e.mu held.
func (e *Explorer) bfsPath(fx, fy, tx, ty int) []string {
	type node struct {
		x, y int
		path []string
	}

	start := fmt.Sprintf("%d,%d", fx, fy)
	target := fmt.Sprintf("%d,%d", tx, ty)
	if start == target {
		return nil
	}

	queue := []node{{fx, fy, nil}}
	seen := map[string]bool{start: true}

	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]

		pos := fmt.Sprintf("%d,%d", curr.x, curr.y)
		exits := e.knownExits[pos]

		for _, dir := range exits {
			nx, ny := curr.x, curr.y
			switch dir {
			case "north":
				ny--
			case "south":
				ny++
			case "east":
				nx++
			case "west":
				nx--
			}

			npos := fmt.Sprintf("%d,%d", nx, ny)
			if seen[npos] {
				continue
			}
			seen[npos] = true

			step := append(append([]string{}, curr.path...), dir)

			if npos == target {
				return step
			}

			if _, known := e.knownExits[npos]; known {
				queue = append(queue, node{nx, ny, step})
			}
		}
	}

	return nil
}

// optimisticBFS plans a path assuming unknown cells have all 4 exits open.
// Known cells use actual exits. This lets the explorer plan through unexplored
// territory and correct on the next tick when walls are discovered.
// Must be called with e.mu held.
func (e *Explorer) optimisticBFS(fx, fy, tx, ty int) []string {
	type node struct {
		x, y int
		path []string
	}

	start := fmt.Sprintf("%d,%d", fx, fy)
	target := fmt.Sprintf("%d,%d", tx, ty)
	if start == target {
		return nil
	}

	// Use a generous bound: max of known extent and target position + margin
	maxX := e.mazeW + 5
	maxY := e.mazeH + 5
	if tx+5 > maxX {
		maxX = tx + 5
	}
	if ty+5 > maxY {
		maxY = ty + 5
	}

	allDirs := []string{"north", "south", "east", "west"}
	queue := []node{{fx, fy, nil}}
	seen := map[string]bool{start: true}

	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]

		pos := fmt.Sprintf("%d,%d", curr.x, curr.y)
		exits, known := e.knownExits[pos]
		if !known {
			exits = allDirs
		}

		for _, dir := range exits {
			nx, ny := curr.x, curr.y
			switch dir {
			case "north":
				ny--
			case "south":
				ny++
			case "east":
				nx++
			case "west":
				nx--
			}

			if nx < 0 || ny < 0 || nx >= maxX || ny >= maxY {
				continue
			}

			npos := fmt.Sprintf("%d,%d", nx, ny)
			if seen[npos] {
				continue
			}
			seen[npos] = true

			step := append(append([]string{}, curr.path...), dir)

			if npos == target {
				return step
			}

			queue = append(queue, node{nx, ny, step})
		}
	}

	return nil
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// offsetPosition computes the approximate destination "x,y" string from a
// current position and a direction. Used for visit-count lookups.
func offsetPosition(pos, dir string) string {
	// Parse "x,y"
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
