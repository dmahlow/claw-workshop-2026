package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync/atomic"
	"time"

	"myclaw/a2a"
	"myclaw/peers"
)

const gameTimeout = 15 * time.Second

// gameGet sends a GET request to the game server and returns the response body.
func gameGet(ctx context.Context, path string) (string, error) {
	url := gameServerURL() + path

	ctx, cancel := context.WithTimeout(ctx, gameTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("game server request to %s failed: %w", path, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return "", fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("game server returned %d: %s", resp.StatusCode, string(respBody))
	}

	return string(respBody), nil
}

// gameServerURL returns the game server URL from the environment or a default.
func gameServerURL() string {
	if url := os.Getenv("GAME_SERVER_URL"); url != "" {
		return url
	}
	return "http://localhost:9090"
}

// gamePost sends a POST request to the game server and returns the response body.
func gamePost(ctx context.Context, path string, payload interface{}) (string, error) {
	url := gameServerURL() + path

	ctx, cancel := context.WithTimeout(ctx, gameTimeout)
	defer cancel()

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshaling request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("game server request to %s failed: %w", path, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return "", fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("game server returned %d: %s", resp.StatusCode, string(respBody))
	}

	return string(respBody), nil
}

// JoinGame is a tool that joins the maze heist game.
// On join, it also auto-discovers all other peers from the game server's peer
// list and starts the relay inbox poller for incoming A2A messages.
type JoinGame struct {
	AgentCardURL string          // The claw's own Agent Card URL, set at startup.
	ExplorerID   *string         // Shared pointer, set after successful join.
	PeerRegistry *peers.Registry // Peer registry to auto-populate on join.
	A2AHandler   func(text string) (string, error) // Feeds incoming relay messages into the agent loop.
	OnJoin       func()          // Optional callback fired after a successful first join.

	started atomic.Bool // Guards the background goroutines so they start only once.
}

func (t *JoinGame) Name() string { return "join_game" }
func (t *JoinGame) Description() string {
	return "Join the maze heist game. Returns your assigned role, starting position, and explorer ID. Also discovers all other players automatically."
}

func (t *JoinGame) Schema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"properties":          map[string]any{},
		"additionalProperties": false,
	}
}

func (t *JoinGame) Execute(ctx context.Context, _ json.RawMessage) (string, error) {
	payload := map[string]string{
		"agent_card_url": t.AgentCardURL,
	}
	result, err := gamePost(ctx, "/api/join", payload)
	if err != nil {
		return "", err
	}

	// Parse the response to extract explorer_id and set the shared pointer.
	var resp struct {
		ExplorerID string `json:"explorer_id"`
	}
	if jsonErr := json.Unmarshal([]byte(result), &resp); jsonErr == nil && resp.ExplorerID != "" && t.ExplorerID != nil {
		*t.ExplorerID = resp.ExplorerID
	}

	// Start the background goroutines exactly once, even if join_game is
	// called again.
	if resp.ExplorerID != "" && t.started.CompareAndSwap(false, true) {
		// Poll the relay inbox so peers can reach us through the game server.
		if t.A2AHandler != nil {
			StartInboxPoller(ctx, resp.ExplorerID, t.A2AHandler)
		}
		// Discover peers now and keep refreshing so late joiners show up.
		if t.PeerRegistry != nil {
			go t.refreshPeers(ctx)
		}
		// Fire the OnJoin callback (e.g. to start the autonomous explorer).
		if t.OnJoin != nil {
			t.OnJoin()
		}
	}

	return result, nil
}

// refreshPeers discovers peers immediately and then refreshes the peer list
// every 30 seconds so late joiners become visible. It exits when ctx is done.
func (t *JoinGame) refreshPeers(ctx context.Context) {
	t.autoDiscoverPeers(ctx)

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			t.autoDiscoverPeers(ctx)
		}
	}
}

// autoDiscoverPeers fetches the peer list from the game server and registers
// each peer. Peers are registered with their relay URL, so all messages go
// through the game server relay - no direct peer-to-peer connectivity needed.
func (t *JoinGame) autoDiscoverPeers(ctx context.Context) {
	peersJSON, err := gameGet(ctx, "/api/peers")
	if err != nil {
		return
	}

	var peerList struct {
		Peers []struct {
			ExplorerID   string `json:"explorer_id"`
			Role         string `json:"role"`
			AgentCardURL string `json:"agent_card_url"`
			RelayURL     string `json:"relay_url"`
		} `json:"peers"`
	}
	if jsonErr := json.Unmarshal([]byte(peersJSON), &peerList); jsonErr != nil {
		return
	}

	for _, p := range peerList.Peers {
		if p.AgentCardURL == t.AgentCardURL {
			continue // skip self
		}

		if p.RelayURL == "" {
			// No relay available: fall back to fetching the card directly.
			card, discErr := a2a.Discover(ctx, p.AgentCardURL)
			if discErr != nil {
				continue
			}
			t.PeerRegistry.Add(card)
			continue
		}

		// Build a synthetic agent card locally. Tagging the skill with the
		// peer's ROLE means find_peer_with_skill("rogue") finds actual
		// rogues.
		card := &a2a.AgentCard{
			Name:        p.ExplorerID,
			Description: fmt.Sprintf("Maze heist explorer %s (role: %s)", p.ExplorerID, p.Role),
			URL:         p.RelayURL, // all messages go through the game server relay
			Skills: []a2a.AgentSkill{{
				ID:   p.Role,
				Name: p.Role,
				Tags: []string{p.Role, "game", "maze-heist"},
			}},
		}
		t.PeerRegistry.Add(card)
	}
}

// Move is a tool that moves the explorer in the maze.
type Move struct {
	ExplorerID *string // Pointer so it can be set after joining.
}

func (t Move) Name() string { return "move" }
func (t Move) Description() string {
	return "Move your explorer in the maze. Direction must be one of: north, south, east, west."
}

func (t Move) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"direction": map[string]any{
				"type":        "string",
				"description": "Direction to move: north, south, east, or west.",
				"enum":        []string{"north", "south", "east", "west"},
			},
		},
		"required":             []string{"direction"},
		"additionalProperties": false,
	}
}

func (t Move) Execute(ctx context.Context, params json.RawMessage) (string, error) {
	var p struct {
		Direction string `json:"direction"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return "", fmt.Errorf("invalid parameters: %w", err)
	}
	if p.Direction == "" {
		return "", fmt.Errorf("direction is required")
	}

	explorerID := ""
	if t.ExplorerID != nil {
		explorerID = *t.ExplorerID
	}

	payload := map[string]string{
		"explorer_id": explorerID,
		"direction":   p.Direction,
	}
	return gamePost(ctx, "/api/move", payload)
}

// Look is a tool that observes the explorer's surroundings in the maze.
type Look struct {
	ExplorerID *string
}

func (t Look) Name() string { return "look" }
func (t Look) Description() string {
	return "Look around your current position in the maze. Returns visible walls, paths, locked doors, other explorers, and items."
}

func (t Look) Schema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"properties":          map[string]any{},
		"additionalProperties": false,
	}
}

func (t Look) Execute(ctx context.Context, _ json.RawMessage) (string, error) {
	explorerID := ""
	if t.ExplorerID != nil {
		explorerID = *t.ExplorerID
	}

	payload := map[string]string{
		"explorer_id": explorerID,
	}
	return gamePost(ctx, "/api/look", payload)
}

// UseAbility is a tool that uses the explorer's role ability on a target.
type UseAbility struct {
	ExplorerID *string
}

func (t UseAbility) Name() string { return "use_ability" }
func (t UseAbility) Description() string {
	return "Use your role's special ability on a target (e.g., a door ID). Only works if your role matches the door's requirement."
}

func (t UseAbility) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"target": map[string]any{
				"type":        "string",
				"description": "The target to use your ability on (e.g., a door ID).",
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
	if p.Target == "" {
		return "", fmt.Errorf("target is required")
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

// SubmitKey is a tool that submits a key for a human challenge door.
type SubmitKey struct {
	ExplorerID *string
}

func (t SubmitKey) Name() string { return "submit_key" }
func (t SubmitKey) Description() string {
	return "Submit a key obtained from a human challenge to unlock a door."
}

func (t SubmitKey) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"door_id": map[string]any{
				"type":        "string",
				"description": "The ID of the door to unlock.",
			},
			"key": map[string]any{
				"type":        "string",
				"description": "The key string obtained by solving the challenge.",
			},
		},
		"required":             []string{"door_id", "key"},
		"additionalProperties": false,
	}
}

func (t SubmitKey) Execute(ctx context.Context, params json.RawMessage) (string, error) {
	var p struct {
		DoorID string `json:"door_id"`
		Key    string `json:"key"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return "", fmt.Errorf("invalid parameters: %w", err)
	}
	if p.DoorID == "" {
		return "", fmt.Errorf("door_id is required")
	}
	if p.Key == "" {
		return "", fmt.Errorf("key is required")
	}

	explorerID := ""
	if t.ExplorerID != nil {
		explorerID = *t.ExplorerID
	}

	payload := map[string]string{
		"explorer_id": explorerID,
		"door_id":     p.DoorID,
		"key":         p.Key,
	}
	return gamePost(ctx, "/api/submit_key", payload)
}
