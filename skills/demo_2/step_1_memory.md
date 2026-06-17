# Skill: Persistent Memory

> **Pacing:** Feed this skill to your agent ONE step at a time. After each "Stop here" marker, wait for the instructor before continuing to the next step.

## Context
Our agent forgets everything when it restarts. A claw remembers. We'll add a markdown-based memory system on disk - similar to how OpenClaw and NanoClaw handle memory. The agent can store facts, recall them in future sessions, and the system prompt includes relevant memories.

## Step 1: Memory storage

Create a `memory` package with functions to store and retrieve memories.

- Memories are stored as individual markdown files in a `memory/` directory
- Each file has YAML frontmatter with metadata:
  ```yaml
  ---
  key: "user_name"
  created: "2026-06-15T10:30:00Z"
  updated: "2026-06-15T10:30:00Z"
  ---
  The user's name is Daniel.
  ```
- Implement: `Save(key, content string) error` and `Load(key string) (string, error)`
- Implement: `List() ([]string, error)` to list all memory keys
- Implement: `Search(query string) ([]string, error)` for simple substring search across all memories

### Acceptance criteria
- [ ] Memories persist to disk as markdown files
- [ ] Save, Load, List, and Search all work correctly
- [ ] Overwriting an existing key updates the content and the `updated` timestamp
- [ ] The memory directory is created automatically if it doesn't exist
- [ ] File names are sanitized (no path traversal, no special characters)

### Stop here
Write a quick test: save a memory, restart the program, load it back. Verify it persists.

## Step 2: Memory tools

Create two new tools and register them:

**remember:**
- Parameters: `{"key": "string", "content": "string"}`
- Saves a memory using the memory package
- Returns confirmation

**recall:**
- Parameters: `{"query": "string"}` (optional - if empty, list all memories)
- If query is provided, search memories and return matching content
- If no query, list all memory keys
- Returns the memory content or a list of keys

### Acceptance criteria
- [ ] Both tools are registered in the tool registry
- [ ] The LLM can decide on its own to save important information
- [ ] The LLM can recall previously saved information
- [ ] Tools have clear descriptions so the LLM knows when to use them

### Stop here
Test: Tell your agent "My name is [your name], please remember that." End the session. Start a new session and ask "What's my name?" Verify it uses the recall tool to find the answer.

## Step 3: System prompt injection

Load relevant memories into the system prompt at startup:

- On agent start, load memories and append them to the system prompt
- This gives the agent immediate context without needing to call the recall tool
- You'll need a token budget to prevent the system prompt from growing unbounded - decide what's reasonable and how to format the injected memories

### Acceptance criteria
- [ ] The system prompt includes known memories on startup
- [ ] The agent can reference memories without explicitly calling the recall tool
- [ ] Memory injection respects the token budget
- [ ] New memories saved during a session are available in the next session's system prompt

### Stop here
Save a few memories, restart the agent, and ask a question that requires memory context. Verify the agent answers correctly without calling the recall tool.
