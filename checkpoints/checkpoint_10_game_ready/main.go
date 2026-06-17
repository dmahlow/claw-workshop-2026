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
	"myclaw/a2a"
	"myclaw/agent"
	"myclaw/memory"
	"myclaw/peers"
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
	PublicURL string
}

func loadConfig() Config {
	c := Config{
		BaseURL:   os.Getenv("CLAW_BASE_URL"),
		APIKey:    os.Getenv("CLAW_API_KEY"),
		Model:     os.Getenv("CLAW_MODEL"),
		Port:      os.Getenv("CLAW_PORT"),
		MemoryDir: os.Getenv("CLAW_MEMORY_DIR"),
		TasksFile: os.Getenv("CLAW_TASKS_FILE"),
		PublicURL: os.Getenv("CLAW_PUBLIC_URL"),
	}

	// Apply defaults.
	if c.Model == "" {
		c.Model = "claude-haiku"
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
	if c.PublicURL == "" {
		c.PublicURL = "http://localhost:" + c.Port
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
	fmt.Printf("  Base URL:    %s\n", cfg.BaseURL)
	fmt.Printf("  API Key:     %s\n", redactKey(cfg.APIKey))
	fmt.Printf("  Model:       %s\n", cfg.Model)
	fmt.Printf("  Web Port:    %s\n", cfg.Port)
	fmt.Printf("  Public URL:  %s\n", cfg.PublicURL)
	fmt.Printf("  Memory Dir:  %s\n", cfg.MemoryDir)
	fmt.Printf("  Tasks File:  %s\n", cfg.TasksFile)
	fmt.Println("==========================")
}

func buildAgentCard(cfg Config) a2a.AgentCard {
	return a2a.AgentCard{
		Name:        "Claw",
		Description: "A personal AI assistant built with Go. Can read/write files, run commands, remember things, schedule tasks, and collaborate with other agents via A2A.",
		URL:         cfg.PublicURL,
		Version:     "1.0.0",
		Skills: []a2a.AgentSkill{
			{
				ID:          "file_operations",
				Name:        "File Operations",
				Description: "Read, write, and list files on the local filesystem.",
				Tags:        []string{"files", "filesystem", "read", "write"},
			},
			{
				ID:          "command_execution",
				Name:        "Command Execution",
				Description: "Execute shell commands and return output.",
				Tags:        []string{"shell", "command", "execute"},
			},
			{
				ID:          "memory",
				Name:        "Persistent Memory",
				Description: "Remember and recall information across sessions.",
				Tags:        []string{"memory", "remember", "recall"},
			},
			{
				ID:          "scheduling",
				Name:        "Task Scheduling",
				Description: "Schedule tasks for later or recurring execution.",
				Tags:        []string{"schedule", "recurring", "tasks"},
			},
			{
				ID:          "peer_communication",
				Name:        "Peer Communication",
				Description: "Discover, message, and broadcast to other agents via A2A.",
				Tags:        []string{"a2a", "peers", "communication", "broadcast"},
			},
			{
				ID:          "maze_heist",
				Name:        "Maze Heist",
				Description: "Explore a maze, coordinate with peers, and solve challenges in the maze heist game.",
				Tags:        []string{"game", "maze", "heist", "explore"},
			},
		},
		DefaultInputModes:  []string{"text/plain"},
		DefaultOutputModes: []string{"text/plain"},
	}
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

	// Initialize peer registry.
	peerRegistry := peers.NewRegistry()

	// Shared explorer ID for game tools. Set when the agent joins the game.
	explorerID := ""

	// a2aHandler feeds incoming A2A messages (from the A2A server or the game
	// relay inbox) into the agent loop and waits for the response.
	a2aHandler := func(text string) (string, error) {
		// Use a channel to wait for the response.
		responseCh := make(chan string, 1)
		var responseBuilder strings.Builder

		msgChan <- agent.Message{
			Content: fmt.Sprintf("[A2A message] %s", text),
			Source:  "a2a",
			ReplyTo: func(s string) {
				responseBuilder.WriteString(s)
			},
			Done: func() {
				responseCh <- responseBuilder.String()
			},
		}

		// Wait for the response.
		response := <-responseCh
		return response, nil
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
		// A2A peer tools.
		tools.DiscoverPeer{Registry: peerRegistry},
		tools.AskPeer{Registry: peerRegistry},
		tools.Broadcast{Registry: peerRegistry},
		tools.FindPeerWithSkill{Registry: peerRegistry},
		// Game tools - only join_game is pre-registered. Participants add
		// move, look, use_ability, submit_key during game rounds by prompting
		// their coding agent. The reference implementation has all tools for
		// testing purposes; checkpoint_10 is the participant starting point.
		&tools.JoinGame{
			AgentCardURL: cfg.PublicURL,
			ExplorerID:   &explorerID,
			PeerRegistry: peerRegistry,
			A2AHandler:   a2aHandler,
		},
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

	// Set up the web server.
	webServer := web.NewServer(cfg.Port, msgChan)

	// Build the Agent Card and set up A2A server.
	card := buildAgentCard(cfg)
	a2aServer := a2a.NewServer(card, a2aHandler)
	a2aServer.RegisterRoutes(webServer.Mux())

	// Set up context with Ctrl+C handling.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// Start the scheduler in the background.
	go sched.Run(ctx)

	// Start the web server in the background.
	go func() {
		if err := webServer.Start(ctx); err != nil {
			slog.Error("web server error", "error", err)
		}
	}()

	// Start CLI input reader.
	agent.StartCLIInput(ctx, msgChan)

	fmt.Printf("\nClaw ready. Web UI at http://localhost:%s\n", cfg.Port)
	fmt.Printf("Agent Card at %s/.well-known/agent-card.json\n", cfg.PublicURL)
	fmt.Printf("A2A endpoint at %s/a2a\n", cfg.PublicURL)
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
