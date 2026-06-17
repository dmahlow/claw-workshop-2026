package agent

import (
	"context"
	"fmt"

	"github.com/openai/openai-go"
)

// StreamEventType indicates the kind of event produced during streaming.
type StreamEventType int

const (
	// EventText is a text content delta.
	EventText StreamEventType = iota
	// EventToolCallDelta carries a tool call argument fragment.
	EventToolCallDelta
	// EventDone signals the stream has finished.
	EventDone
	// EventError signals a stream error.
	EventError
)

// StreamEvent represents a single event from the streaming response.
type StreamEvent struct {
	Type StreamEventType

	// Text content delta (for EventText).
	Text string

	// Accumulated tool calls, populated when EventDone fires.
	ToolCalls []openai.ChatCompletionMessageToolCall

	// Full accumulated content, populated when EventDone fires.
	FullContent string

	// FinishReason from the final chunk.
	FinishReason string

	// Error (for EventError).
	Err error
}

// streamResponse reads from the SSE stream in a goroutine and sends
// StreamEvents to the returned channel. The channel is closed when the
// stream is exhausted or the context is cancelled. The caller is
// responsible for consuming events from the channel.
func streamResponse(ctx context.Context, stream *openai.ChatCompletionAccumulator, sseStream interface {
	Next() bool
	Current() openai.ChatCompletionChunk
	Err() error
	Close() error
}) <-chan StreamEvent {
	ch := make(chan StreamEvent)

	go func() {
		defer close(ch)
		defer sseStream.Close()

		for sseStream.Next() {
			// Check context cancellation.
			select {
			case <-ctx.Done():
				ch <- StreamEvent{Type: EventError, Err: ctx.Err()}
				return
			default:
			}

			chunk := sseStream.Current()
			if !stream.AddChunk(chunk) {
				continue
			}

			if len(chunk.Choices) == 0 {
				continue
			}

			delta := chunk.Choices[0].Delta

			// Stream text content deltas.
			if delta.Content != "" {
				ch <- StreamEvent{Type: EventText, Text: delta.Content}
			}
		}

		if err := sseStream.Err(); err != nil {
			ch <- StreamEvent{Type: EventError, Err: fmt.Errorf("stream error: %w", err)}
			return
		}

		// Build the done event with accumulated data.
		event := StreamEvent{Type: EventDone}
		if len(stream.Choices) > 0 {
			event.FullContent = stream.Choices[0].Message.Content
			event.ToolCalls = stream.Choices[0].Message.ToolCalls
			event.FinishReason = stream.Choices[0].FinishReason
		}
		ch <- event
	}()

	return ch
}
