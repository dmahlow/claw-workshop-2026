# Workshop Setup

## 1. Clone the repo

```bash
git clone https://github.com/dmahlow/gceu26_workshop.git
cd gceu26_workshop
```

## 2. Your claw connects to the shared LLM proxy

```bash
export CLAW_BASE_URL="https://macbook-pro-3.taila8c4.ts.net/v1"
export CLAW_API_KEY="dummy"
export CLAW_MODEL="qwen"
```

## 3. Verify the proxy is reachable

```bash
curl -s "$CLAW_BASE_URL/chat/completions" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $CLAW_API_KEY" \
  -d '{"model":"qwen","messages":[{"role":"user","content":"hello"}]}'
```

You should get a JSON response with a message.

## 4. Your coding agent

Use your own coding agent (Claude Code, Cursor, Gemini CLI, Codex) with your own API key/subscription. The proxy above is only for the claw you build during the workshop.

## 5. Build and run

```bash
CGO_ENABLED=0 go build -o myclaw .
./myclaw
```
