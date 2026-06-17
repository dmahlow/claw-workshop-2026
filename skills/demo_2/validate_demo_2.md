# Validation Skill: Demo 2 Checkpoint

This validates the end state of Demo 2 (4 steps: memory, scheduling, web UI, system prompt + config). Your code should match checkpoint_8_claw. Checks 17-20 cover the final sub-step of step 3 (system prompt extraction, config, logging). Step 4 is discussion-only.

Run the following checks and report results as a checklist. For any failures, explain what's wrong and suggest a fix.

## Checks

1. `go build ./...` succeeds with no errors
2. `go vet ./...` reports no warnings
3. `go run -race .` starts without race detector warnings
4. A `memory/` directory exists (or is created on first run) with markdown files
5. Memory Save, Load, List, and Search functions work correctly
6. `remember` and `recall` tools are registered
7. The system prompt includes loaded memories on startup
8. A scheduler package exists with Task struct and Scheduler
9. Scheduled tasks persist to disk across restarts
10. The `schedule` tool is registered and works
11. Scheduled tasks fire even when the user is idle
12. An HTTP server starts on the configured port
13. `index.html` is served at the root and renders a chat UI
14. WebSocket connection at `/ws` works
15. Messages sent from the web UI reach the agent and responses stream back
16. Tool calls are visible in the web UI
17. The system prompt is loaded from a file (not hardcoded)
18. Configuration comes from environment variables with sensible defaults
19. The API key is not exposed in logs or startup output
20. Ctrl+C triggers graceful shutdown (saves state, closes connections)

## Report format

```
Demo 2 Validation Results
========================
[PASS] 1. go build succeeds
...

X/20 checks passed
```

For any FAIL, provide the fix.
