# Validation Skill: Demo 3 Checkpoint

Run the following checks and report results as a checklist. For any failures, explain what's wrong and suggest a fix.

## Checks

1. `go build ./...` succeeds with no errors
2. `go vet ./...` reports no warnings
3. An Agent Card is served at `/.well-known/agent-card.json`
4. The Agent Card contains: name, description, URL, and at least one skill
5. The Agent Card URL uses the public address (not localhost)
6. An A2A handler is registered on the HTTP server
7. Incoming A2A messages are processed and responses returned
8. The `discover_peer` tool can fetch and parse a remote Agent Card
9. A peer registry exists and stores discovered peers
10. The `ask_peer` tool can send a message to a peer and return the response
11. The `broadcast` tool sends to all peers in parallel and collects responses
12. The `find_peer_with_skill` tool searches peer Agent Cards
13. The `join_game` tool is registered (move, look, use_ability, submit_key are added during game rounds)
14. The claw listens on 0.0.0.0 (all network interfaces) - only required for the optional direct peer-to-peer demo; the game itself uses outbound-only inbox polling, so inbound reachability is not required
15. `go run -race .` is clean (no race conditions)

## Report format

```
Demo 3 Validation Results
========================
[PASS] 1. go build succeeds
...

X/15 checks passed
```

For any FAIL, provide the fix.
