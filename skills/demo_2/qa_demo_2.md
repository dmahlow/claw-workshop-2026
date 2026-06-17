# QA Skill: Review Demo 2 - From Agent to Claw

Use this skill after completing all steps of Demo 2. Feed these prompts to your coding agent.

## 1. Architecture visualization

> Generate a mermaid diagram showing the full claw architecture. Include: the agent loop, tool registry, memory system, scheduler, HTTP server, WebSocket handler, and how they all connect. Show goroutines as separate swim lanes where applicable.

Review the diagram. How many goroutines are running? What channels connect them?

## 2. Concurrency analysis

> Analyze the concurrency model of this claw:
> - How many goroutines are running simultaneously?
> - What channels exist and what do they carry?
> - Is there any shared mutable state? How is it protected?
> - Could any goroutine leak? Under what conditions?
> - Run `go run -race .` - are there any race conditions?
>
> Be specific with file and line references.

This is critical. Race conditions in agent systems are subtle and can cause bizarre behavior.

## 3. Security review

> Review this claw for security concerns:
> - Can the run_command tool be abused? What's the threat model?
> - Can the write_file tool write outside the intended directory?
> - Is the API key exposed anywhere (logs, web UI, error messages)?
> - Can a malicious WebSocket client cause problems?
> - Are there any injection points (user input that reaches shell or filesystem unsanitized)?
>
> For each finding, rate severity (critical/high/medium/low) and suggest a mitigation.

Think about which of these matter for a personal tool vs. a production system.

## 4. Agent, harness, claw - the triptych

> Three things share the same foundation: an agent (tool interface + reasoning loop), a harness (adds intercept hooks, skill loading, extensions - like Claude Code or zot), and a claw (adds memory, scheduling, always-on operation - like OpenClaw). Explain which parts of what you built are shared across all three, and which are claw-specific. Where in this codebase would you add before-tool, before-turn, and before-message intercept hooks to turn it into a harness?

This connects to the broader workshop narrative. The code you just built sits at the intersection of all three.
