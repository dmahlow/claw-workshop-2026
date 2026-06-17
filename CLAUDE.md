# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What This Is

GopherCon Europe 2026 workshop materials for "Go Faster with Agents: Build Your Own Claw in Go." Participants build an autonomous AI agent (claw) from scratch in Go across 3 demos, culminating in a collaborative maze heist game using the A2A protocol.

## Build and Run Commands

All Go modules require `CGO_ENABLED=0` on macOS to avoid LC_UUID linker issues.

```bash
# Reference implementation (the complete claw)
cd reference && CGO_ENABLED=0 go build -o myclaw . && \
  CLAW_BASE_URL="http://localhost:4000/v1" CLAW_API_KEY="dummy" CLAW_MODEL="claude-sonnet" ./myclaw

# Game server
cd game_server && CGO_ENABLED=0 go build -o maze-heist . && \
  GAME_PORT=9090 ./maze-heist

# Maze simulation prototype
cd prototype/maze_viz && CGO_ENABLED=0 go build -o maze-viz . && ./maze-viz

# Verify all checkpoints build
for cp in checkpoints/checkpoint_*; do (cd "$cp" && CGO_ENABLED=0 go build ./... && go vet ./...); done

# LiteLLM proxy (requires ANTHROPIC_API_KEY in env)
cd proxy && ./run.sh   # serves on port 4000
```

## Architecture

### Module Map

| Directory | Module | Purpose |
|---|---|---|
| `reference/` | `myclaw` | Complete claw with all 16 tools, A2A, web UI |
| `checkpoints/checkpoint_1..10/` | `myclaw` | Progressive snapshots, each independently buildable |
| `game_server/` | `github.com/dmahlow/maze-heist` | Maze heist game server with visualization |
| `prototype/maze_viz/` | `github.com/dmahlow/maze-heist-viz` | Standalone simulation for the endgame sequence |

### Reference Implementation Package Structure

- `agent/` - Agent loop with channel-based Message multiplexing, SSE streaming
- `tools/` - 16 tools implementing the `Tool` interface (file ops, memory, scheduling, A2A peers, game client)
- `memory/` - Markdown files with YAML frontmatter on disk
- `scheduler/` - Background goroutine with JSON-persisted tasks
- `web/` - HTTP server, embedded static HTML (`embed.FS`), WebSocket at `/ws`
- `a2a/` - Manual A2A implementation (JSON-RPC 2.0, Agent Card at `/.well-known/agent-card.json`)
- `peers/` - Thread-safe peer registry (`sync.RWMutex`)

### Key Pattern: Message Channel Multiplexing

The agent loop reads from `chan agent.Message`. Four sources feed it:
- CLI (stdin goroutine)
- WebSocket (per-connection goroutine)
- Scheduler (background goroutine)
- A2A (incoming peer messages)

Each `Message` carries `ReplyTo` and `Done` callbacks so the agent responds to the correct source. Adding a new input source is just "start a goroutine, feed the channel."

### Checkpoint Progression

1-4 (Demo 1, 3 steps): agent loop, tool interface, streaming, 4 tools. Checkpoint 1 is a mid-step snapshot.
5-8 (Demo 2, 4 steps): memory, scheduling, web UI + system prompt/config. Step 4 is discussion-only; checkpoint 8 = end of step 3.
9-10 (Demo 3, 4 steps): A2A server/client, connectivity, game client tools

Game phase: checkpoint_10 contains only join_game with inbox poller + peer refresh. Move, look, use_ability tools exist in the reference but participants add them incrementally during 4 game rounds by prompting their coding agent. Auto-explore is NOT built into join_game; participants add it in Round 1.

## Environment Variables

| Var | Default | Purpose |
|---|---|---|
| `CLAW_BASE_URL` | (required) | OpenAI-compatible LLM endpoint |
| `CLAW_API_KEY` | (required) | API key for the endpoint |
| `CLAW_MODEL` | `gpt-4o` | Model name |
| `CLAW_PORT` | `8080` | Web UI and A2A server port |
| `CLAW_PUBLIC_URL` | `http://localhost:PORT` | Externally reachable URL for Agent Card |
| `CLAW_MEMORY_DIR` | `./memory_data` | Persistent memory storage path |
| `CLAW_TASKS_FILE` | `./scheduler/tasks.json` | Scheduler persistence path |
| `GAME_SERVER_URL` | `http://localhost:9090` | Maze heist game server (claw-side) |
| `GAME_PORT` | `9090` | Game server listen port |
| `GAME_MAZE_SEED` | random | Deterministic maze generation |

## Known Gotchas

- **LiteLLM finish_reason:** The proxy returns `finish_reason: "stop"` even when tool calls are present. Check for tool calls by presence, not finish reason.
- **LiteLLM streaming:** Tool calls may arrive in a single chunk instead of deltas. Build `ChatCompletionMessageToolCallParam` manually rather than using `.ToParam()`.
- **Port 8080 conflicts:** The prototype and reference both default to 8080. Use `CLAW_PORT` to avoid collisions.

## Workshop Materials

- `skills/` - Markdown files participants feed to their coding agent (sub-stepped with review gates)
- `docs/workshop_design.md` - Comprehensive design document with all decisions
- `docs/instructor_notes.md` - Teaching moments, known issues, timing guidance
- `docs/game_concept.md` - Maze heist game design including endgame crescendo
- `slides/index.html` - Reveal.js deck (dark theme, Inter + JetBrains Mono)
- `proxy/` - LiteLLM config mapping `claude-sonnet` and `claude-haiku` to Anthropic models
