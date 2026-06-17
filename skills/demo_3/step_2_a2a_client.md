# Skill: A2A Client

> **Pacing:** Feed this skill to your agent ONE step at a time. After each "Stop here" marker, wait for the instructor before continuing to the next step.

## Context
Our claw can receive A2A messages. Now it needs to send them - discover peers, ask them questions, and delegate tasks. This completes the A2A integration and enables multi-agent collaboration.

### Network note
The game server provides a message relay so you don't need direct peer-to-peer connectivity. When you discover peers via the game server's `/api/peers` endpoint, each peer includes a `relay_url` in addition to their direct `agent_card_url`. Use the `relay_url` for reliable communication - it routes through the game server, which can always reach both sides.

Your A2A client code works the same either way - the relay is transparent. Just POST your JSON-RPC message to the relay URL instead of the direct peer URL.

## Step 1: Peer discovery

Add the ability to discover other agents via their Agent Cards:

- HTTP GET to `{baseURL}/.well-known/agent-card.json` and parse the JSON into your AgentCard struct
- Alternatively, you can use the `a2a-go` SDK: `agentcard.DefaultResolver.Resolve(ctx, baseURL)`
- Create a `peers` package or add to existing code:
  - `Discover(url string) (*AgentCard, error)` - fetch and parse an Agent Card from a URL
  - `Registry` - a map of known peers (URL -> AgentCard)
- Add a tool: **discover_peer**
  - Parameters: `{"url": "string"}` - the base URL of a peer agent
  - Fetches the Agent Card, adds it to the peer registry
  - Returns the peer's name, description, and skills

### Acceptance criteria
- [ ] The claw can fetch and parse a peer's Agent Card
- [ ] Discovered peers are stored in a local registry
- [ ] The discover_peer tool works and shows peer capabilities
- [ ] Invalid URLs return a clear error (not a crash)

### Stop here
Test with a neighbor: have them start their claw, then use the discover_peer tool with their URL. Verify you see their Agent Card.

## Step 2: Send messages to peers

Add the ability to send messages to discovered peers:

- POST a JSON-RPC 2.0 `message/send` request to the peer's A2A endpoint
- Alternatively, you can use the SDK: `a2aclient.NewFromCard(ctx, card)` then `client.SendMessage(ctx, req)`
- Create a tool: **ask_peer**
  - Parameters: `{"peer_url": "string", "message": "string"}`
  - Sends a message to the peer via A2A
  - Waits for the response
  - Returns the peer's response text

The flow: your claw decides it needs help -> calls ask_peer -> sends A2A message to the peer -> peer processes it -> response comes back -> your claw uses the response.

### Acceptance criteria
- [ ] The claw can send a message to a discovered peer and get a response
- [ ] The ask_peer tool returns the peer's response as text
- [ ] Timeouts are handled (if a peer takes too long, return an error after 30s)
- [ ] The claw can have multi-turn conversations with peers (context is maintained on the peer side)

### Stop here
Test with a neighbor: discover their claw, then ask it a question. Verify the response comes back through A2A. Then have them ask your claw something. Two-way communication!

## Step 3: Broadcast and coordination tools

Add tools for broader communication:

**broadcast:**
- Parameters: `{"message": "string"}`
- Sends a message to ALL discovered peers
- Returns a summary of responses
- Uses goroutines to send in parallel (fan-out), collects results via a channel (fan-in)

**find_peer_with_skill:**
- Parameters: `{"skill": "string"}`
- Searches the peer registry for agents that have a matching skill
- Returns a list of matching peers with their URLs

These tools enable the coordination patterns needed for the maze heist.

### Acceptance criteria
- [ ] Broadcast sends to all peers in parallel and collects responses
- [ ] Broadcast has a timeout so one slow peer doesn't block everything
- [ ] find_peer_with_skill searches Agent Card skills correctly
- [ ] The fan-out/fan-in pattern uses goroutines and channels properly
- [ ] `go run -race .` is clean

### Stop here
Test broadcast with at least 2-3 neighboring claws discovered. Verify all responses come back. This is the A2A foundation complete.
