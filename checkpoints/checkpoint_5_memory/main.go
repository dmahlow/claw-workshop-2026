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
	"myclaw/tools"
)

const defaultModel = "gpt-4o"

const baseSystemPrompt = "You are a helpful assistant. You have access to tools for reading files, listing directories, writing files, and running shell commands. You can also remember information across sessions using the remember tool, and recall it later using the recall tool. Use them when appropriate to help the user. Save important information the user tells you using the remember tool. Check memories before asking questions the user may have already answered."

func main() {
	baseURL := os.Getenv("CLAW_BASE_URL")
	apiKey := os.Getenv("CLAW_API_KEY")
	model := os.Getenv("CLAW_MODEL")
	memoryDir := os.Getenv("CLAW_MEMORY_DIR")

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
	} {
		if err := registry.Register(t); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to register tool %s: %v\n", t.Name(), err)
			os.Exit(1)
		}
	}

	// Set up context with Ctrl+C handling.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	fmt.Println("Agent ready. Type 'exit' or press Ctrl+C to quit.")

	if err := agent.RunAgent(ctx, &client, model, systemPrompt, registry); err != nil {
		fmt.Fprintf(os.Stderr, "Agent error: %v\n", err)
		os.Exit(1)
	}
}
