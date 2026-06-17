# Skill: Web UI

> **Pacing:** Feed this skill to your agent ONE step at a time. After each "Stop here" marker, wait for the instructor before continuing to the next step.

## Context
Our claw works from the terminal, but a web UI makes it more accessible and sets us up for the A2A demo later (the claw needs an HTTP server anyway). We'll build a minimal chat interface served by the Go binary.

## Step 1: HTTP server and static page

Add an HTTP server to the claw binary:

- Serve on a configurable port (e.g., `CLAW_PORT` env var, default 8080)
- Serve a static `index.html` at the root
- The HTML page should be a minimal chat UI - design it however you think looks good. At minimum: a scrolling message area, a text input with send button, and visual distinction between user and agent messages.
- The HTTP server runs alongside the agent loop (both in their own goroutines)
- For now, the page is just static HTML - no backend connection yet

### Acceptance criteria
- [ ] `go run .` starts both the agent (CLI) and the web server
- [ ] Opening http://localhost:8080 shows the chat UI
- [ ] The page looks clean and professional on a laptop screen
- [ ] The HTML/CSS is embedded in the Go binary (use `embed.FS`)

### Stop here
Open the page in a browser. Verify it looks right. The send button won't do anything yet - that's next.

## Step 2: WebSocket connection

Add a WebSocket endpoint that connects the chat UI to the agent:

- `/ws` endpoint using a WebSocket library (gorilla/websocket or nhooyr.io/websocket)
- When the client sends a message over the WebSocket, feed it into the agent loop as user input
- When the agent produces output (text, tool calls, or scheduled task results), send it to the WebSocket client
- Stream tokens over the WebSocket as they arrive (each chunk is a small JSON message)

Message format (client to server):
```json
{"type": "message", "content": "hello"}
```

Message format (server to client):
```json
{"type": "chunk", "content": "Hello"}
{"type": "chunk", "content": " there!"}
{"type": "done"}
{"type": "tool_call", "name": "read_file", "status": "executing"}
{"type": "tool_result", "name": "read_file", "status": "complete"}
```

### Acceptance criteria
- [ ] Messages sent from the web UI reach the agent
- [ ] Agent responses stream back to the web UI token by token
- [ ] Tool calls are visible in the UI (show which tool is being called)
- [ ] The CLI still works in parallel (both interfaces feed the same agent)
- [ ] Multiple browser tabs can connect (each gets the streamed output)

### Stop here
Open the web UI and have a conversation. Ask the agent to use a tool. Verify streaming works and tool calls are visible.

## Step 3: Polish the UI

Enhance the web UI:

- Auto-scroll to the bottom when new messages arrive
- Show a typing indicator while the agent is generating
- Render markdown in agent messages (basic: bold, code blocks, lists)
- Show a subtle indicator for tool calls (e.g., a collapsible section showing "Used read_file on main.go")
- Show connection status (connected/disconnected) in the header
- Handle reconnection if the WebSocket drops

### Acceptance criteria
- [ ] Auto-scroll works
- [ ] Typing indicator appears during generation
- [ ] Code blocks render with monospace font and a subtle background
- [ ] Tool calls are shown but don't clutter the conversation
- [ ] Disconnection/reconnection is handled gracefully

### Stop here
This is a good time to step back and use the web UI for a few minutes. Does it feel good? Is there anything annoying? Fix any rough edges.

## Step 4: System prompt and configuration

Before we wrap up Demo 2, let's clean up the claw:

- Store the system prompt in a `system_prompt.md` file and embed it into the binary with `//go:embed`
- Create a Config struct that reads all settings from environment variables with sensible defaults: CLAW_BASE_URL, CLAW_API_KEY, CLAW_MODEL, CLAW_PORT, CLAW_MEMORY_DIR, CLAW_TASKS_FILE
- Print a configuration summary on startup with the API key redacted
- Add structured logging via `slog` for tool calls, errors, and scheduled task executions
- Wire up graceful shutdown: on Ctrl+C, save scheduler state, close WebSocket connections, then exit

### Acceptance criteria
- [ ] System prompt loaded from embedded file, not hardcoded
- [ ] All config from environment variables with defaults
- [ ] API key never appears in logs
- [ ] Ctrl+C shuts down cleanly

### Stop here
This is the end of Demo 2 coding. The instructor will now cover security and the harness connection on screen.
