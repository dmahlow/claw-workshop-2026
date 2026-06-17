package agent

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/shared"
	"myclaw/tools"
)

// Message represents an input from any source (CLI, WebSocket, scheduler).
type Message struct {
	Content string
	Source  string         // "cli", "web", "scheduler"
	ReplyTo func(string)  // callback to send response chunks to the source
	Done    func()         // callback when the response is complete
}

// ResponseEvent is sent to sources to stream response data.
type ResponseEvent struct {
	Type string // "chunk", "done", "tool_call", "tool_result", "error"
	Content string
	ToolName string
}

// RunAgent runs the main agent loop with channel-based message multiplexing.
// It reads from the provided message channel and processes each message.
func RunAgent(ctx context.Context, client *openai.Client, model string, systemPrompt string, registry *tools.Registry, messages chan Message) error {
	// Build the conversation history starting with the system message.
	history := []openai.ChatCompletionMessageParamUnion{
		systemMessage(systemPrompt),
	}

	// Build the tool definitions for the LLM.
	toolDefs := buildToolDefs(registry)

	for {
		select {
		case <-ctx.Done():
			fmt.Println("\nGoodbye!")
			return nil
		case msg, ok := <-messages:
			if !ok {
				return nil
			}

			// Append user message.
			history = append(history, userMessage(msg.Content))

			// Run the agent turn.
			var err error
			history, err = agentTurn(ctx, client, model, history, toolDefs, registry, msg)
			if err != nil {
				errMsg := fmt.Sprintf("Error: %v", err)
				if msg.ReplyTo != nil {
					msg.ReplyTo(errMsg)
				}
				fmt.Fprintf(os.Stderr, "%s\n", errMsg)
			}
			if msg.Done != nil {
				msg.Done()
			}
		}
	}
}

// StartCLIInput starts a goroutine that reads from stdin and sends messages
// to the provided channel. It closes the channel when stdin is exhausted.
func StartCLIInput(ctx context.Context, ch chan Message) {
	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		for {
			fmt.Print("> ")

			select {
			case <-ctx.Done():
				return
			default:
			}

			if !scanner.Scan() {
				return
			}

			input := strings.TrimSpace(scanner.Text())
			if input == "" {
				continue
			}
			if input == "exit" {
				fmt.Println("Goodbye!")
				// Signal to close by sending a context cancellation
				// or just return - the main loop will handle it.
				return
			}

			ch <- Message{
				Content: input,
				Source:  "cli",
				ReplyTo: func(s string) {
					fmt.Print(s)
				},
				Done: func() {
					fmt.Println()
				},
			}
		}
	}()
}

// agentTurn handles one turn of the agent loop, which may involve
// multiple rounds of tool calls before producing a final text response.
func agentTurn(ctx context.Context, client *openai.Client, model string, messages []openai.ChatCompletionMessageParamUnion, toolDefs []openai.ChatCompletionToolParam, registry *tools.Registry, msg Message) ([]openai.ChatCompletionMessageParamUnion, error) {
	for {
		params := openai.ChatCompletionNewParams{
			Model:    model,
			Messages: messages,
		}
		if len(toolDefs) > 0 {
			params.Tools = toolDefs
		}

		// Start streaming.
		sseStream := client.Chat.Completions.NewStreaming(ctx, params)
		acc := openai.ChatCompletionAccumulator{}
		events := streamResponse(ctx, &acc, sseStream)

		// Consume events.
		var fullContent string
		var toolCalls []openai.ChatCompletionMessageToolCall

		for event := range events {
			switch event.Type {
			case EventText:
				if msg.ReplyTo != nil {
					msg.ReplyTo(event.Text)
				}
			case EventDone:
				fullContent = event.FullContent
				toolCalls = event.ToolCalls
			case EventError:
				return messages, event.Err
			}
		}

		// Append the assistant message to history.
		messages = append(messages, assistantMessage(fullContent, toolCalls))

		// If there are no tool calls, we're done with this turn.
		if len(toolCalls) == 0 {
			return messages, nil
		}

		// Execute each tool call and append results.
		for _, tc := range toolCalls {
			toolName := tc.Function.Name
			toolArgs := tc.Function.Arguments

			tool, ok := registry.Get(toolName)
			if !ok {
				errMsg := fmt.Sprintf("unknown tool: %s", toolName)
				fmt.Fprintf(os.Stderr, "[tool error: %s]\n", errMsg)
				messages = append(messages, toolMessage(tc.ID, errMsg))
				continue
			}

			fmt.Fprintf(os.Stderr, "[calling tool: %s]\n", toolName)

			result, err := tool.Execute(ctx, []byte(toolArgs))
			if err != nil {
				errMsg := fmt.Sprintf("tool error: %v", err)
				fmt.Fprintf(os.Stderr, "[%s]\n", errMsg)
				messages = append(messages, toolMessage(tc.ID, errMsg))
				continue
			}

			messages = append(messages, toolMessage(tc.ID, result))
		}

		// Loop back to send tool results to the LLM.
	}
}

// buildToolDefs converts registered tools into the OpenAI tool format.
func buildToolDefs(registry *tools.Registry) []openai.ChatCompletionToolParam {
	allTools := registry.All()
	defs := make([]openai.ChatCompletionToolParam, 0, len(allTools))
	for _, t := range allTools {
		defs = append(defs, openai.ChatCompletionToolParam{
			Function: shared.FunctionDefinitionParam{
				Name:        t.Name(),
				Description: openai.String(t.Description()),
				Parameters:  shared.FunctionParameters(t.Schema()),
			},
		})
	}
	return defs
}

// Helper functions to construct message param unions.

func systemMessage(content string) openai.ChatCompletionMessageParamUnion {
	return openai.ChatCompletionMessageParamUnion{
		OfSystem: &openai.ChatCompletionSystemMessageParam{
			Content: openai.ChatCompletionSystemMessageParamContentUnion{
				OfString: openai.String(content),
			},
		},
	}
}

func userMessage(content string) openai.ChatCompletionMessageParamUnion {
	return openai.ChatCompletionMessageParamUnion{
		OfUser: &openai.ChatCompletionUserMessageParam{
			Content: openai.ChatCompletionUserMessageParamContentUnion{
				OfString: openai.String(content),
			},
		},
	}
}

func assistantMessage(content string, toolCalls []openai.ChatCompletionMessageToolCall) openai.ChatCompletionMessageParamUnion {
	msg := &openai.ChatCompletionAssistantMessageParam{
		Content: openai.ChatCompletionAssistantMessageParamContentUnion{
			OfString: openai.String(content),
		},
	}
	if len(toolCalls) > 0 {
		tcParams := make([]openai.ChatCompletionMessageToolCallParam, len(toolCalls))
		for i, tc := range toolCalls {
			tcParams[i] = openai.ChatCompletionMessageToolCallParam{
				ID:   tc.ID,
				Type: "function",
				Function: openai.ChatCompletionMessageToolCallFunctionParam{
					Name:      tc.Function.Name,
					Arguments: tc.Function.Arguments,
				},
			}
		}
		msg.ToolCalls = tcParams
	}
	return openai.ChatCompletionMessageParamUnion{
		OfAssistant: msg,
	}
}

func toolMessage(toolCallID, content string) openai.ChatCompletionMessageParamUnion {
	return openai.ChatCompletionMessageParamUnion{
		OfTool: &openai.ChatCompletionToolMessageParam{
			ToolCallID: toolCallID,
			Content: openai.ChatCompletionToolMessageParamContentUnion{
				OfString: openai.String(content),
			},
		},
	}
}
