# Security, Harness Patterns, and What You Built

> This is a discussion step - no coding. Listen and discuss with the instructor.

## What the instructor will cover

### The Agent / Harness / Claw triptych
- **Agent** (Demo 1): Tool interface, reasoning loop, streaming
- **Harness** (Claude Code, zot, Cursor): adds intercept hooks, skill loading, extensions
- **Claw** (what you built): adds memory, scheduling, always-on operation
- Same foundation, different direction. The channel refactor is what makes both extensible.

### Security: the phantom token pattern
- Your claw has `run_command` and `write_file` - powerful and dangerous
- In production: nono uses kernel-level sandboxing (Landlock on Linux, Seatbelt on macOS) with Go bindings
- The phantom token pattern: agent gets a useless session token, a proxy swaps in real API keys at the network boundary. The agent process never has access to secrets.
- The OpenClaw security crisis: 48K exposed nodes, 230 malicious scripts in one week. This is what happens without sandboxing.

### What you built
- Five goroutines, one binary
- 7 tools (read_file, list_directory, write_file, run_command, remember, recall, schedule)
- Persistent memory, autonomous scheduling, real-time web interface
- The same patterns that power production agent systems

## For the game
Some maze doors deep in the maze may ask you to implement a harness pattern - a before-tool guard, output masking. Your coding agent can help. These are optional and depend on how the game is going.
