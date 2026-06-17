# QA Skill: Review Demo 3 - A2A and The Maze Heist

Use this skill after the game (or during a break). Feed these prompts to your coding agent.

## 1. A2A architecture

> Generate a mermaid sequence diagram showing the A2A communication flow during the maze heist. Include: your claw, the game server, and at least two peer claws. Show: game join, look/move commands, a help request for a locked door, map sharing, and a human challenge flow.

Review the diagram. Does it match what you saw happening on the big screen?

## 2. Network analysis

> Analyze the A2A implementation:
> - How many HTTP connections does our claw maintain to peers?
> - What happens if a peer goes offline mid-game?
> - How efficient is the broadcast (sending to all peers)? Could it be improved?
> - Is there any risk of message loops (A broadcasts to B, B broadcasts back to A)?
> - How does the timeout strategy affect game performance?

Think about scaling: what if there were 500 claws instead of 50?

## 3. Full system review

> Look at the entire claw codebase as a whole. We've built:
> - An agent reasoning loop with tool dispatch
> - A streaming response system
> - 10+ tools (file, command, memory, scheduling, A2A, game)
> - Persistent memory
> - Autonomous scheduling
> - A web UI with WebSocket streaming
> - A2A server and client
> - Game client
>
> Rate the overall code quality 1-10. What are the top 3 things you'd improve for production use?

This is the retrospective moment. What did the coding agent do well? What did you have to fix?

## 4. What you built today

> Write a one-paragraph summary of what this claw is and what it can do. Target audience: a colleague who wasn't at the workshop.

Save this to memory - you might want it later.
