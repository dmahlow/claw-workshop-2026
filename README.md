# Go Faster with Agents: Build Your Own Claw in Go

GopherCon Europe 2026 Workshop

## What is this?

A 7-hour hands-on workshop where you build a claw (always-on autonomous AI agent) from scratch in Go, using coding agents to help with implementation. The workshop culminates in a networked maze heist game where all participant-built claws collaborate via the A2A protocol.

## Prerequisites

- Go 1.22+ installed (`go version`)
- A coding agent ready (Claude Code, Gemini CLI, Codex CLI, or Cursor)
- This repo cloned

## Before the Workshop

```bash
git clone https://github.com/dmahlow/gceu-workshop-2026.git
cd gceu-workshop-2026
go version  # should be 1.22+
```

> **Note:** This repo currently contains only the materials for Demo 1. Additional checkpoints, skill files, configuration, and game content will be pushed during the workshop. Run `git pull` when the instructor says to.

## Workshop Flow

The workshop has three demos, each building on the previous one:

1. **Demo 1: The Agent Core** - Build a working CLI agent with tools and streaming
2. **Demo 2: From Agent to Claw** - Add memory, scheduling, and a web UI
3. **Demo 3: A2A + The Maze Heist** - Network your claws, then play the game

Each demo uses skill files you feed to your coding agent. Checkpoints let you catch up at any point.

## Repository Structure

```
skills/
  demo_1/        Skill files for Demo 1 (available now)

checkpoints/
  checkpoint_1_agent_loop/    Basic agent loop
  checkpoint_2_tools/         Tool interface + 2 tools
  checkpoint_3_streaming/     Streaming responses
  checkpoint_4_full_agent/    All 4 tools (end of Demo 1)
```

Materials for Demo 2, Demo 3, and the maze heist game will appear here during the workshop.
