// Game API types - copy into your tools/ package or point your coding agent here.
//
// These structs match the exact JSON field names returned by the game server.
// Use them to unmarshal responses from gamePost(), or just treat the raw JSON
// string as the tool result (the LLM can read JSON fine).
package tools

// --- /api/join ---

// JoinRequest is the body for POST /api/join.
type JoinRequest struct {
	AgentCardURL string `json:"agent_card_url"`
}

// JoinResponse is returned by POST /api/join.
type JoinResponse struct {
	ExplorerID    string   `json:"explorer_id"`    // e.g. "c-1"
	Role          string   `json:"role"`            // "rogue", "mage", "knight", "sage", "ranger"
	Position      Position `json:"position"`        // starting position
	Message       string   `json:"message"`         // welcome text
	RelayURL      string   `json:"relay_url"`       // relay endpoint for this explorer
	GameServerURL string   `json:"game_server_url"` // base URL of the game server
}

// Position is an (x, y) coordinate in the maze grid.
type Position struct {
	X int `json:"x"`
	Y int `json:"y"`
}

// --- /api/move ---

// MoveRequest is the body for POST /api/move.
type MoveRequest struct {
	ExplorerID string `json:"explorer_id"`
	Direction  string `json:"direction"` // "north", "south", "east", "west"
}

// MoveResponse is returned by POST /api/move.
type MoveResponse struct {
	Success  bool     `json:"success"`
	Position Position `json:"position"` // new position (or same if blocked)
	Message  string   `json:"message"`  // e.g. "Moved north." or "Wall blocks your path."
}

// --- /api/look ---

// LookRequest is the body for POST /api/look.
type LookRequest struct {
	ExplorerID string `json:"explorer_id"`
}

// LookResponse is returned by POST /api/look.
// The description is a multi-line text block, not structured JSON.
// It includes: your identity/position, broadcasts, exits, nearby explorers,
// nearby doors, and jewel status.
type LookResponse struct {
	Description string `json:"description"`
}

// --- /api/ability ---

// AbilityRequest is the body for POST /api/ability.
type AbilityRequest struct {
	ExplorerID string `json:"explorer_id"`
	Target     string `json:"target"` // door ID, e.g. "door-3"
}

// AbilityResponse is returned by POST /api/ability.
// Fields vary depending on whether the ability succeeded, the role matched,
// or a challenge is required.
type AbilityResponse struct {
	Success       bool     `json:"success"`
	Message       string   `json:"message"`
	Status        string   `json:"status,omitempty"`         // presence status text (when not yet open)
	RequiresKey   bool     `json:"requires_key,omitempty"`   // true if a human challenge blocks the door
	DoorID        string   `json:"door_id,omitempty"`        // set when requires_key is true
	RequiredRoles []string `json:"required_roles,omitempty"` // set when role does not match
	Challenge     *struct {
		Description string `json:"description"`
		Hint        string `json:"hint"`
	} `json:"challenge,omitempty"` // set when requires_key is true
}

// --- /api/submit_key ---

// SubmitKeyRequest is the body for POST /api/submit_key.
type SubmitKeyRequest struct {
	ExplorerID string `json:"explorer_id"`
	DoorID     string `json:"door_id"`
	Key        string `json:"key"`
}

// SubmitKeyResponse is returned by POST /api/submit_key.
type SubmitKeyResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// --- /api/peers (GET) ---

// PeersResponse is returned by GET /api/peers.
type PeersResponse struct {
	Peers []PeerInfo `json:"peers"`
}

// PeerInfo describes one explorer in the peer list.
type PeerInfo struct {
	ExplorerID   string `json:"explorer_id"`
	Role         string `json:"role"`
	AgentCardURL string `json:"agent_card_url"`
	RelayURL     string `json:"relay_url"` // present when game server has a public URL
}
