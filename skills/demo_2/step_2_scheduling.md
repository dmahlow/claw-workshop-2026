# Skill: Autonomous Scheduling

> **Pacing:** Feed this skill to your agent ONE step at a time. After each "Stop here" marker, wait for the instructor before continuing to the next step.

## Context
A claw doesn't just respond - it acts on its own. We'll add a scheduler that lets the agent schedule future tasks. "Remind me in 30 minutes" or "check this URL every hour" become possible. This is where Go's goroutines and timers shine.

## Step 1: Task scheduler

Create a `scheduler` package with a simple task scheduler:

- A `Task` struct: `ID string`, `Description string`, `ExecuteAt time.Time`, `Recurring bool`, `Interval time.Duration`
- A `Scheduler` struct that manages a list of tasks
- The scheduler runs in its own goroutine, checking for due tasks on a regular tick (you decide the interval - shorter means more responsive but more CPU)
- When a task is due, it sends the task description to a callback function
- Recurring tasks reschedule themselves after execution
- Tasks persist to disk (a simple JSON file) so they survive restarts

### Acceptance criteria
- [ ] Tasks can be scheduled for a specific time
- [ ] Recurring tasks re-fire at their interval
- [ ] The scheduler runs in a background goroutine
- [ ] Tasks persist across restarts (saved to `scheduler/tasks.json`)
- [ ] The scheduler respects context cancellation (clean shutdown)
- [ ] No race conditions (`go run -race .` is clean)

### Stop here
Write a test: schedule a task 5 seconds in the future, verify the callback fires.

## Step 2: Schedule tool

Create a new tool:

**schedule:**
- Parameters: `{"description": "string", "delay": "string", "recurring": "boolean"}`
- `delay` is a human-readable duration: "30m", "1h", "24h", etc. (parsed with `time.ParseDuration`)
- Creates a task in the scheduler
- Returns confirmation with the scheduled time

Also modify the agent loop: when the scheduler fires a task, inject the task description as a new user message into the agent loop, triggering the agent to act on it.

### Acceptance criteria
- [ ] The LLM can schedule tasks via the tool
- [ ] When a task fires, the agent processes it as if a user sent the message
- [ ] "Remind me in 1 minute to check the time" actually works
- [ ] Recurring tasks fire repeatedly
- [ ] The agent can list scheduled tasks (extend the schedule tool with an `action: "list"` parameter)

### Stop here
Test: Ask your agent to "remind me in 1 minute that I should take a break." Wait for the reminder. Verify it appears and the agent acts on it.

## Step 3: Background operation - the big refactor

**This is the most important architectural change in the workshop.** The agent loop currently blocks on stdin. It needs to accept input from multiple sources: CLI, scheduled tasks, and (in the next step) WebSocket clients.

Refactor the agent loop to use a `Message` channel:

```go
type Message struct {
    Content  string
    Source   string            // "cli", "web", "scheduler"
    ReplyTo  func(string)     // callback to send response chunks to the source
    Done     func()           // callback when the response is complete
    OnTool   func(name, status string) // callback for tool call notifications
}
```

- The agent loop reads from `chan Message` using `select`, not from stdin directly
- stdin becomes one goroutine feeding messages into the channel (`StartCLIInput`)
- The scheduler becomes another goroutine feeding messages when tasks fire
- The `ReplyTo` callback lets the agent send responses back to whoever asked (CLI prints to stdout, WebSocket sends JSON, scheduler logs the output)
- This same channel will accept WebSocket messages in the next step - no further refactoring needed

### Acceptance criteria
- [ ] Scheduled tasks fire and produce output even when the user is idle
- [ ] The user can still type input at any time (input and scheduled tasks don't block each other)
- [ ] Multiple scheduled tasks firing close together are handled correctly
- [ ] The terminal output is clean (scheduled task output doesn't garble user input)

### Stop here
Schedule two tasks 30 seconds apart, then just wait. Verify both fire and produce output without you typing anything.

## Mandatory review (5 minutes)

Before continuing to the next skill, run these two prompts with your agent:

> Generate a mermaid diagram showing all goroutines in this claw and the channels connecting them. Label each goroutine with what it does.

> Explain how a message from the scheduler reaches the agent loop and gets a response. Then explain how a WebSocket message (which we'll add next) would follow the same path.

This is the most important architectural moment in the workshop. If the diagram makes sense, everything else will too.
