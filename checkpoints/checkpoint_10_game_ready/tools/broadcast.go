package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"myclaw/a2a"
	"myclaw/peers"
)

const broadcastTimeout = 30 * time.Second

// Broadcast is a tool that sends a message to all discovered peers in parallel.
type Broadcast struct {
	Registry *peers.Registry
}

func (t Broadcast) Name() string { return "broadcast" }
func (t Broadcast) Description() string {
	return "Send a message to ALL discovered peers in parallel and collect their responses. Useful for coordination and announcements."
}

func (t Broadcast) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"message": map[string]any{
				"type":        "string",
				"description": "The message to broadcast to all peers.",
			},
		},
		"required":             []string{"message"},
		"additionalProperties": false,
	}
}

// peerResult holds the response from a single peer.
type peerResult struct {
	URL      string
	Name     string
	Response string
	Err      error
}

func (t Broadcast) Execute(ctx context.Context, params json.RawMessage) (string, error) {
	var p struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return "", fmt.Errorf("invalid parameters: %w", err)
	}
	if p.Message == "" {
		return "", fmt.Errorf("message is required")
	}

	allPeers := t.Registry.All()
	if len(allPeers) == 0 {
		return "No peers discovered yet. Use discover_peer first.", nil
	}

	// Fan-out: send to all peers in parallel.
	ctx, cancel := context.WithTimeout(ctx, broadcastTimeout)
	defer cancel()

	results := make(chan peerResult, len(allPeers))
	var wg sync.WaitGroup

	for url, card := range allPeers {
		wg.Add(1)
		go func(peerURL, peerName string) {
			defer wg.Done()
			response, err := a2a.SendMessage(ctx, peerURL, p.Message)
			results <- peerResult{
				URL:      peerURL,
				Name:     peerName,
				Response: response,
				Err:      err,
			}
		}(url, card.Name)
	}

	// Close the results channel when all goroutines finish.
	go func() {
		wg.Wait()
		close(results)
	}()

	// Fan-in: collect all results.
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Broadcast to %d peers:\n\n", len(allPeers)))

	for result := range results {
		if result.Err != nil {
			sb.WriteString(fmt.Sprintf("[%s] (%s): ERROR - %v\n", result.Name, result.URL, result.Err))
		} else {
			sb.WriteString(fmt.Sprintf("[%s] (%s): %s\n", result.Name, result.URL, result.Response))
		}
	}

	return sb.String(), nil
}
