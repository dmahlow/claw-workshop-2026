package main

import (
	"context"
	_ "embed"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"myclaw/agent"
	"myclaw/memory"
	"myclaw/scheduler"
	"myclaw/tools"
	"myclaw/web"
)

//go:embed system_prompt.md
var systemPromptTemplate string

// Config holds all configuration for the claw.
type Config struct {
	BaseURL   string
	APIKey    string
	Model     string
	Port      string
	MemoryDir string
	TasksFile string
}

func loadConfig() Config {
	c := Config{
		BaseURL:   os.Getenv("CLAW_BASE_URL"),
		APIKey:    os.Getenv("CLAW_API_KEY"),
		Model:     os.Getenv("CLAW_MODEL"),
		Port:      os.Getenv("CLAW_PORT"),
		MemoryDir: os.Getenv("CLAW_MEMORY_DIR"),
		TasksFile: os.Getenv("CLAW_TASKS_FILE"),
	}

	// Apply defaults.
	if c.Model == "" {
		c.Model = "gpt-4o"
	}
	if c.Port == "" {
		c.Port = "8080"
	}
	if c.MemoryDir == "" {
		c.MemoryDir = "./memory_data"
	}
	if c.TasksFile == "" {
		c.TasksFile = "./scheduler/tasks.json"
	}

	return c
}

func redactKey(key string) string {
	if len(key) <= 8 {
		return "***"
	}
	return key[:4] + "..." + key[len(key)-4:]
}

func printConfig(cfg Config) {
	fmt.Println("=== Claw Configuration ===")
	fmt.Printf("  Base URL:   %s\n", cfg.BaseURL)
	fmt.Printf("  API Key:    %s\n", redactKey(cfg.APIKey))
	fmt.Printf("  Model:      %s\n", cfg.Model)
	fmt.Printf("  Web Port:   %s\n", cfg.Port)
	fmt.Printf("  Memory Dir: %s\n", cfg.MemoryDir)
	fmt.Printf("  Tasks File: %s\n", cfg.TasksFile)
	fmt.Println("==========================")
}

func main() {
	// Set up structured logging.
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	cfg := loadConfig()

	if cfg.APIKey == "" {
		fmt.Fprintln(os.Stderr, "CLAW_API_KEY environment variable is required")
		os.Exit(1)
	}

	printConfig(cfg)

	// Initialize memory store.
	memStore, err := memory.NewStore(cfg.MemoryDir)
	if err != nil {
		slog.Error("failed to initialize memory", "error", err)
		os.Exit(1)
	}

	// Build system prompt with injected memories.
	systemPrompt := systemPromptTemplate
	if memories := memStore.Dump(2000); memories != "" {
		systemPrompt += "\n\n## Known facts from previous sessions\n" + memories
	}

	// Create the message channel.
	msgChan := make(chan agent.Message, 16)

	// Initialize scheduler.
	sched, err := scheduler.New(cfg.TasksFile, func(description string) {
		slog.Info("scheduled task firing", "description", description)
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
		slog.Error("failed to initialize scheduler", "error", err)
		os.Exit(1)
	}

	// Build client options.
	opts := []option.RequestOption{
		option.WithAPIKey(cfg.APIKey),
	}
	if cfg.BaseURL != "" {
		opts = append(opts, option.WithBaseURL(cfg.BaseURL))
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
			slog.Error("failed to register tool", "tool", t.Name(), "error", err)
			os.Exit(1)
		}
	}

	// Log registered tools.
	toolNames := make([]string, 0)
	for _, t := range registry.All() {
		toolNames = append(toolNames, t.Name())
	}
	slog.Info("tools registered", "tools", strings.Join(toolNames, ", "))

	// Set up context with Ctrl+C handling.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// Start the scheduler in the background.
	go sched.Run(ctx)

	// Start the web server in the background.
	webServer := web.NewServer(cfg.Port, msgChan)
	go func() {
		if err := webServer.Start(ctx); err != nil {
			slog.Error("web server error", "error", err)
		}
	}()

	// Start CLI input reader.
	agent.StartCLIInput(ctx, msgChan)

	fmt.Printf("\nClaw ready. Web UI at http://localhost:%s\n", cfg.Port)
	fmt.Println("Type 'exit' or press Ctrl+C to quit.")

	if err := agent.RunAgent(ctx, &client, cfg.Model, systemPrompt, registry, msgChan); err != nil {
		slog.Error("agent error", "error", err)
		os.Exit(1)
	}

	// Graceful shutdown.
	slog.Info("shutting down")
	if err := sched.Save(); err != nil {
		slog.Error("failed to save scheduler state", "error", err)
	}
	slog.Info("shutdown complete")
}
