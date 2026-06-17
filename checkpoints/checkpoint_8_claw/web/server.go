package web

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"sync"

	"myclaw/agent"
	"nhooyr.io/websocket"
)

//go:embed index.html
var staticFiles embed.FS

// wsMessage is the JSON format for messages from the client.
type wsMessage struct {
	Type    string `json:"type"`
	Content string `json:"content"`
}

// wsResponse is the JSON format for messages to the client.
type wsResponse struct {
	Type    string `json:"type"`              // "chunk", "done", "tool_call", "tool_result", "error"
	Content string `json:"content,omitempty"`
	Name    string `json:"name,omitempty"`
	Status  string `json:"status,omitempty"`
}

// Server is the HTTP/WebSocket server for the Claw web UI.
type Server struct {
	msgChan chan agent.Message
	port    string
	mu      sync.Mutex
	clients map[*websocket.Conn]context.CancelFunc
}

// NewServer creates a new web server.
func NewServer(port string, msgChan chan agent.Message) *Server {
	return &Server{
		msgChan: msgChan,
		port:    port,
		clients: make(map[*websocket.Conn]context.CancelFunc),
	}
}

// Start begins serving HTTP on the configured port. It blocks until the
// context is cancelled or the server encounters a fatal error.
func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()

	// Serve static files.
	staticFS, err := fs.Sub(staticFiles, ".")
	if err != nil {
		return fmt.Errorf("creating static FS: %w", err)
	}
	mux.Handle("/", http.FileServer(http.FS(staticFS)))

	// WebSocket endpoint.
	mux.HandleFunc("/ws", s.handleWS)

	srv := &http.Server{
		Addr:    ":" + s.port,
		Handler: mux,
	}

	// Shutdown when context is cancelled.
	go func() {
		<-ctx.Done()
		slog.Info("shutting down web server")
		s.closeAllClients()
		if err := srv.Shutdown(context.Background()); err != nil {
			slog.Error("web server shutdown error", "error", err)
		}
	}()

	slog.Info("web server starting", "port", s.port)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("web server error: %w", err)
	}
	return nil
}

func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true, // Allow connections from any origin for dev.
	})
	if err != nil {
		slog.Error("websocket accept error", "error", err)
		return
	}

	connCtx, connCancel := context.WithCancel(r.Context())

	s.mu.Lock()
	s.clients[conn] = connCancel
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.clients, conn)
		s.mu.Unlock()
		connCancel()
		conn.CloseNow()
	}()

	slog.Info("websocket client connected")

	for {
		_, data, err := conn.Read(connCtx)
		if err != nil {
			slog.Info("websocket client disconnected", "error", err)
			return
		}

		var msg wsMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			slog.Warn("invalid websocket message", "error", err)
			continue
		}

		if msg.Type != "message" || msg.Content == "" {
			continue
		}

		slog.Info("web message received", "content", msg.Content)

		// Send to agent loop via channel.
		s.msgChan <- agent.Message{
			Content: msg.Content,
			Source:  "web",
			ReplyTo: func(text string) {
				s.sendToAll(connCtx, wsResponse{Type: "chunk", Content: text})
			},
			Done: func() {
				s.sendToAll(connCtx, wsResponse{Type: "done"})
			},
			OnTool: func(name, status string) {
				eventType := "tool_call"
				if status == "complete" {
					eventType = "tool_result"
				}
				s.sendToAll(connCtx, wsResponse{Type: eventType, Name: name, Status: status})
			},
		}
	}
}

func (s *Server) sendToAll(ctx context.Context, resp wsResponse) {
	data, err := json.Marshal(resp)
	if err != nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for conn := range s.clients {
		if err := conn.Write(ctx, websocket.MessageText, data); err != nil {
			slog.Debug("failed to write to websocket client", "error", err)
		}
	}
}

func (s *Server) closeAllClients() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for conn, cancel := range s.clients {
		cancel()
		conn.Close(websocket.StatusGoingAway, "server shutting down")
	}
}

