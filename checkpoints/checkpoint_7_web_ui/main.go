package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"myclaw/agent"
	"myclaw/memory"
	"myclaw/scheduler"
	"myclaw/tools"
	"myclaw/web"
)

const defaultModel = "gpt-4o"

const baseSystemPrompt = "You are a helpful assistant. You have access to tools for reading files, listing directories, writing files, and running shell commands. You can also remember information across sessions using the remember tool, and recall it later using the recall tool. You can schedule tasks for later using the schedule tool. Use them when appropriate to help the user. Save important information the user tells you using the remember tool. Check memories before asking questions the user may have already answered. When the user asks you to do something later or on a recurring basis, use the schedule tool."

func main() {
	baseURL := os.Getenv("CLAW_BASE_URL")
	apiKey := os.Getenv("CLAW_API_KEY")
	model := os.Getenv("CLAW_MODEL")
	memoryDir := os.Getenv("CLAW_MEMORY_DIR")
	tasksFile := os.Getenv("CLAW_TASKS_FILE")
	port := os.Getenv("CLAW_PORT")

	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "CLAW_API_KEY environment variable is required")
		os.Exit(1)
	}

	if model == "" {
		model = defaultModel
	}
	if memoryDir == "" {
		memoryDir = "./memory_data"
	}
	if tasksFile == "" {
		tasksFile = "./scheduler/tasks.json"
	}
	if port == "" {
		port = "8080"
	}

	// Initialize memory store.
	memStore, err := memory.NewStore(memoryDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize memory: %v\n", err)
		os.Exit(1)
	}

	// Build system prompt with injected memories.
	systemPrompt := baseSystemPrompt
	if memories := memStore.Dump(2000); memories != "" {
		systemPrompt += "\n\n## Known facts from previous sessions\n" + memories
	}

	// Create the message channel.
	msgChan := make(chan agent.Message, 16)

	// Initialize scheduler.
	sched, err := scheduler.New(tasksFile, func(description string) {
		fmt.Fprintf(os.Stderr, "\n[scheduled task firing: %s]\n", description)
		msgChan <- agent.Message{
			Content: fmt.Sprintf("[Scheduled task] %s", description),
			Source:  "scheduler",
			ReplyTo: func(s string) {
				fmt.Print(s)
			},
			Done: func() {
				fmt.Println()
				fmt.Print("> ")
			},
		}
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize scheduler: %v\n", err)
		os.Exit(1)
	}

	// Build client options.
	opts := []option.RequestOption{
		option.WithAPIKey(apiKey),
	}
	if baseURL != "" {
		opts = append(opts, option.WithBaseURL(baseURL))
	}

	client := openai.NewClient(opts...)

	// Register tools.
	registry := tools.NewRegistry()
	for _, t := range []tools.Tool{
		tools.ReadFile{},
		tools.ListDirectory{},
		tools.WriteFile{},
		tools.RunCommand{},
		tools.Remember{Store: memStore},
		tools.Recall{Store: memStore},
		tools.Schedule{Scheduler: sched},
	} {
		if err := registry.Register(t); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to register tool %s: %v\n", t.Name(), err)
			os.Exit(1)
		}
	}

	// Set up context with Ctrl+C handling.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// Start the scheduler in the background.
	go sched.Run(ctx)

	// Start the web server in the background.
	webServer := web.NewServer(port, msgChan)
	go func() {
		if err := webServer.Start(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "Web server error: %v\n", err)
		}
	}()

	// Start CLI input reader.
	agent.StartCLIInput(ctx, msgChan)

	fmt.Printf("Agent ready. Web UI at http://localhost:%s\n", port)
	fmt.Println("Type 'exit' or press Ctrl+C to quit.")

	if err := agent.RunAgent(ctx, &client, model, systemPrompt, registry, msgChan); err != nil {
		fmt.Fprintf(os.Stderr, "Agent error: %v\n", err)
		os.Exit(1)
	}

	// Graceful shutdown: save scheduler state.
	if err := sched.Save(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to save scheduler state: %v\n", err)
	}
}
