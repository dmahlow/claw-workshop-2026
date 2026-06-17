# Go Faster with Agents: Build Your Own Claw in Go

GopherCon Europe 2026 Workshop - June 17, 2026

Thanks to everyone who attended! You built an autonomous AI agent from scratch in Go, wired it up with A2A, and played a collaborative maze heist. The patterns you learned (tool interfaces, channel multiplexing, fan-out/fan-in, A2A) transfer to any LLM integration you build next.

## What's in this repo

Everything from the workshop - keep building your claw at home.

```
skills/           Step-by-step instructions for each demo
  demo_1/         Agent loop, tools, streaming
  demo_2/         Memory, scheduling, web UI, channel refactor
  demo_3/         A2A server/client, connectivity, game client

checkpoints/      Complete buildable snapshots at each stage
  checkpoint_1_agent_loop/     Basic agent loop
  checkpoint_2_tools/          Tool interface + 2 tools
  checkpoint_3_streaming/      Streaming responses
  checkpoint_4_full_agent/     All 4 tools (end of Demo 1)
  checkpoint_5_memory/         Persistent memory
  checkpoint_6_scheduling/     Scheduler + channel refactor
  checkpoint_7_web_ui/         Web UI + WebSocket
  checkpoint_8_claw/           Full claw (end of Demo 2)
  checkpoint_9_a2a/            A2A server + client
  checkpoint_10_game_ready/    Game client + inbox polling

reference/        The complete claw with all 16 tools and A2A
hints/            Game round hint files (move, look, doors, jewel)
slides/           The slide deck (open slides/index.html)
proxy/            LiteLLM proxy config
```

## Building any checkpoint

```bash
cd checkpoints/checkpoint_8_claw   # or any checkpoint
CGO_ENABLED=0 go build -o myclaw .
```

## Running your claw

You need an OpenAI-compatible LLM endpoint. Point your claw at any provider:

```bash
export CLAW_BASE_URL="https://api.openai.com/v1"   # or any OpenAI-compatible endpoint
export CLAW_API_KEY="your-key"
export CLAW_MODEL="gpt-4o"                          # or any model your endpoint serves
./myclaw
```

## What you built

| Demo | What | Go patterns |
|------|------|-------------|
| 1 | Agent loop, 4 tools, streaming | Interfaces, goroutine+channel, SSE |
| 2 | Memory, scheduling, web UI | embed.FS, channel multiplexing, select |
| 3 | A2A protocol, peer discovery | JSON-RPC, fan-out/fan-in, sync.RWMutex |

## Keep going

- Add tools: Slack, Telegram, email as new input sources (just feed the channel)
- Deploy: `scp myclaw user@server:/usr/local/bin/` - single binary, no runtime
- Extend with MCP: connect to databases, APIs, third-party tools
- Study the reference implementation for the full 16-tool architecture

## Resources

- [A2A Protocol Spec](https://a2a-protocol.org)
- [openai-go SDK](https://github.com/openai/openai-go)
- [a2a-go SDK](https://github.com/a2aproject/a2a-go)
- [nono - agent sandboxing](https://nono.sh)

Daniel Mahlow - [Contiamo](https://contiamo.com) - Berlin
