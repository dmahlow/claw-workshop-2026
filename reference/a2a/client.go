package a2a

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// defaultTimeout is generous because relayed responses include a full LLM turn
// on the other side.
const defaultTimeout = 60 * time.Second

// Discover fetches and parses an Agent Card from the given base URL.
// It requests /.well-known/agent-card.json from the provided URL.
func Discover(ctx context.Context, baseURL string) (*AgentCard, error) {
	baseURL = strings.TrimRight(baseURL, "/")
	cardURL := baseURL + "/.well-known/agent-card.json"

	ctx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cardURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching agent card from %s: %w", cardURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("agent card request returned %d: %s", resp.StatusCode, string(body))
	}

	var card AgentCard
	if err := json.NewDecoder(resp.Body).Decode(&card); err != nil {
		return nil, fmt.Errorf("decoding agent card: %w", err)
	}

	// If the card URL is empty, set it to the base URL.
	if card.URL == "" {
		card.URL = baseURL
	}

	return &card, nil
}

// SendMessage sends a text message to a peer agent via A2A JSON-RPC and returns the response text.
func SendMessage(ctx context.Context, peerURL string, message string) (string, error) {
	peerURL = strings.TrimRight(peerURL, "/")
	endpoint := peerURL + "/a2a"

	ctx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()

	rpcReq := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "message/send",
		ID:      1,
		Params: SendMessageParams{
			Message: Message{
				Role: "user",
				Parts: []Part{
					{Type: "text", Text: message},
				},
			},
		},
	}

	body, err := json.Marshal(rpcReq)
	if err != nil {
		return "", fmt.Errorf("marshaling request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("sending message to %s: %w", endpoint, err)
	}
	defer resp.Body.Close()

	var rpcResp JSONRPCResponse
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return "", fmt.Errorf("decoding response: %w", err)
	}

	if rpcResp.Error != nil {
		return "", fmt.Errorf("A2A error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}

	// Parse the result to extract the response text.
	resultBytes, err := json.Marshal(rpcResp.Result)
	if err != nil {
		return "", fmt.Errorf("marshaling result: %w", err)
	}

	var result SendMessageResult
	if err := json.Unmarshal(resultBytes, &result); err != nil {
		return "", fmt.Errorf("decoding result: %w", err)
	}

	var textParts []string
	for _, part := range result.Message.Parts {
		if part.Type == "text" && part.Text != "" {
			textParts = append(textParts, part.Text)
		}
	}

	return strings.Join(textParts, "\n"), nil
}
