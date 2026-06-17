# Skill: Connectivity Test

> **Pacing:** Feed this skill to your agent ONE step at a time. After each "Stop here" marker, wait for the instructor before continuing to the next step.

## Context
Before the maze heist, every claw verifies it can communicate. Good news: the game uses an outbound-only inbox model - your claw polls the game server for messages, so you don't need to be reachable from the network. Only the game server does.

## Step 1: Local A2A loopback test

Verify your A2A implementation works using two local instances:

- Start a second instance of your claw on a different port: `CLAW_PORT=8282 CLAW_MEMORY_DIR=./memory_data_2 CLAW_TASKS_FILE=./scheduler/tasks_2.json ./myclaw`
- In your primary claw's web UI: "Discover the peer at http://localhost:8282"
- Then: "Ask the peer what files it has"
- The response should flow: your claw -> A2A request -> second claw -> tool use -> A2A response -> your claw

### Acceptance criteria
- [ ] Peer discovery succeeds against your local second instance
- [ ] ask_peer returns a real answer produced by the second claw
- [ ] Both web UIs show the message traffic

### Stop here
Your A2A code works. Stop the second instance.

## Step 2: Reach the game server

The instructor will provide the game server URL. Set it as an environment variable:
`export GAME_SERVER_URL=http://ANNOUNCED:9090`

Also make sure `CLAW_PUBLIC_URL` is set to your machine's actual network address (not localhost) so the game server can identify you:
`export CLAW_PUBLIC_URL=http://YOUR_IP:8181`

- `curl $GAME_SERVER_URL/.well-known/agent-card.json` - you should see the game server's Agent Card
- Tell your claw: "Discover the peer at $GAME_SERVER_URL" - it should register the game server in its peer registry

### Acceptance criteria
- [ ] curl returns the game server's Agent Card
- [ ] Your claw discovers the game server as a peer

### Stop here
If curl fails, you have a network problem - flag the instructor. Nothing else will work until this does.

## Step 3: Join and verify the inbox

> **Note:** This step uses the `join_game` tool from the next skill (step_4_game_client.md). If you haven't built it yet, do that first - it's short.

- Tell your claw: "Join the game"
- Your claw registers, gets a role and explorer ID, and starts polling its inbox automatically
- Watch the big screen: your explorer dot should appear
- Tell your claw: "Broadcast a hello to all peers" - other claws receive it through their inboxes and reply

### Acceptance criteria
- [ ] Your explorer appears on the big screen
- [ ] Your broadcast reaches peers and replies come back
- [ ] Incoming messages from other claws appear in your claw's output

### Stop here
You're connected. Wait for the game to start.
