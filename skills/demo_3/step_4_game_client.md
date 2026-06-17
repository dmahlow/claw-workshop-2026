# Skill: Join the Game

> **Pacing:** This is the last skill before the game begins.

## Context
Your claw has A2A networking and connectivity working. Now join the maze heist. The instructor will guide you through adding capabilities round by round during the game.

> **Copied checkpoint 10?** That's fine - it includes join_game with inbox polling and peer refresh already wired up. You have everything you need to join and play. The game rounds will add movement, vision, and coordination on top of what you have.

## Step 1: Join the game

Implement a `join_game` tool:
- POST to `{game_server}/api/join` with `{"agent_card_url": "your_claw_public_url"}`
- Parse the response to get your `explorer_id`, `role`, and starting `position`
- Store the explorer_id (you'll need it for every subsequent game API call)
- After joining, start the inbox poller and peer refresh goroutines (same pattern as your A2A server - start a goroutine, feed the channel)

The join response also includes `relay_url` and `game_server_url` for the message relay.

### Acceptance criteria
- [ ] join_game succeeds and returns your role and position
- [ ] Your dot appears on the big screen
- [ ] Inbox poller is running (broadcasts from other claws arrive)

### Stop here
You're in the game. The instructor will now guide you through 4 rounds of adding capabilities. Each round: the instructor describes what to build, you prompt your coding agent, rebuild, rejoin. Hint files at `hints/01_move.md` through `hints/04_jewel.md` if you get stuck.
