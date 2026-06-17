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

// RunAgent runs the main agent loop. It reads user input from stdin,
// sends it to the LLM with conversation history, handles tool calls,
// and streams responses back to stdout.
func RunAgent(ctx context.Context, client *openai.Client, model string, systemPrompt string, registry *tools.Registry) error {
	// Build the conversation history starting with the system message.
	messages := []openai.ChatCompletionMessageParamUnion{
		systemMessage(systemPrompt),
	}

	// Build the tool definitions for the LLM.
	toolDefs := buildToolDefs(registry)

	scanner := bufio.NewScanner(os.Stdin)

	for {
		// Prompt indicator.
		fmt.Print("> ")

		// Check context before blocking on input.
		select {
		case <-ctx.Done():
			fmt.Println("\nGoodbye!")
			return nil
		default:
		}

		if !scanner.Scan() {
			// EOF or error.
			if err := scanner.Err(); err != nil {
				return fmt.Errorf("reading input: %w", err)
			}
			fmt.Println()
			return nil
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}
		if input == "exit" {
			fmt.Println("Goodbye!")
			return nil
		}

		// Append user message.
		messages = append(messages, userMessage(input))

		// Run the agent turn: loop until we get a text response (not a tool call).
		var err error
		messages, err = agentTurn(ctx, client, model, messages, toolDefs, registry)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			// Don't exit on error, let the user continue.
		}
	}
}

// agentTurn handles one turn of the agent loop, which may involve
// multiple rounds of tool calls before producing a final text response.
func agentTurn(ctx context.Context, client *openai.Client, model string, messages []openai.ChatCompletionMessageParamUnion, toolDefs []openai.ChatCompletionToolParam, registry *tools.Registry) ([]openai.ChatCompletionMessageParamUnion, error) {
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
				fmt.Print(event.Text)
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
		// Note: we check by tool call presence, not finish reason, because
		// some proxies (e.g., litellm) may return "stop" even with tool calls.
		if len(toolCalls) == 0 {
			fmt.Println() // Final newline after streamed text.
			return messages, nil
		}

		// Execute each tool call and append results.
		fmt.Println() // Newline after any partial content before tool calls.
		for _, tc := range toolCalls {
			toolName := tc.Function.Name
			toolArgs := tc.Function.Arguments

			tool, ok := registry.Get(toolName)
			if !ok {
				// Unknown tool: send error back to the LLM.
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
