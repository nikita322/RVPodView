package api

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/creack/pty"
	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"

	"podmanview/internal/auth"
	"podmanview/internal/events"
	"podmanview/internal/podman"
)

// TerminalHandler handles terminal WebSocket connections
type TerminalHandler struct {
	client       *podman.Client
	wsTokenStore *auth.WSTokenStore
	eventStore   *events.Store
	upgrader     websocket.Upgrader
}

// NewTerminalHandler creates new terminal handler
func NewTerminalHandler(client *podman.Client, wsTokenStore *auth.WSTokenStore, eventStore *events.Store) *TerminalHandler {
	h := &TerminalHandler{
		client:       client,
		wsTokenStore: wsTokenStore,
		eventStore:   eventStore,
	}

	h.upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin:     h.checkOrigin,
	}

	return h
}

// checkOrigin validates WebSocket connection using CSRF token
// This prevents Cross-Site WebSocket Hijacking (CSWSH) attacks
func (h *TerminalHandler) checkOrigin(r *http.Request) bool {
	// Get token from query parameter
	token := r.URL.Query().Get("ws_token")
	if token == "" {
		log.Printf("WebSocket rejected: missing ws_token")
		return false
	}

	// Validate token (one-time use, auto-deleted after validation)
	username, valid := h.wsTokenStore.Validate(token)
	if !valid {
		log.Printf("WebSocket rejected: invalid or expired ws_token")
		return false
	}

	log.Printf("WebSocket connection authorized for user: %s", username)
	return true
}

// ExecMessage represents a WebSocket message
type ExecMessage struct {
	Type string `json:"type"` // "stdin", "resize"
	Data string `json:"data,omitempty"`
	Cols int    `json:"cols,omitempty"`
	Rows int    `json:"rows,omitempty"`
}

// HostTerminal handles WebSocket connection for host terminal
func (h *TerminalHandler) HostTerminal(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUserFromContext(r.Context())
	if !user.IsAdmin() {
		http.Error(w, "Admin access required", http.StatusForbidden)
		return
	}

	// Upgrade HTTP to WebSocket
	ws, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}
	defer ws.Close()

	// Log terminal connection
	h.eventStore.Add(events.EventTerminalHost, user.Username, getClientIP(r), true, "")

	// Start shell process
	cmd := exec.Command("/bin/sh")
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")

	// Get PTY
	ptmx, err := pty.Start(cmd)
	if err != nil {
		log.Printf("Failed to start PTY: %v", err)
		ws.WriteMessage(websocket.TextMessage, []byte("Failed to start shell: "+err.Error()))
		return
	}
	defer func() {
		cmd.Process.Kill()
		ptmx.Close()
	}()

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// Read from PTY -> write to WebSocket
	go func() {
		buf := make([]byte, 1024)
		for {
			select {
			case <-ctx.Done():
				return
			default:
				n, err := ptmx.Read(buf)
				if err != nil {
					cancel()
					return
				}
				if n > 0 {
					if err := ws.WriteMessage(websocket.TextMessage, buf[:n]); err != nil {
						cancel()
						return
					}
				}
			}
		}
	}()

	// Read from WebSocket -> write to PTY
	for {
		select {
		case <-ctx.Done():
			return
		default:
			_, message, err := ws.ReadMessage()
			if err != nil {
				return
			}

			// Parse message
			var msg ExecMessage
			if err := json.Unmarshal(message, &msg); err != nil {
				// Treat as raw stdin
				ptmx.Write(message)
				continue
			}

			switch msg.Type {
			case "stdin":
				ptmx.Write([]byte(msg.Data))
			case "resize":
				if msg.Cols > 0 && msg.Rows > 0 {
					pty.Setsize(ptmx, &pty.Winsize{
						Rows: uint16(msg.Rows),
						Cols: uint16(msg.Cols),
					})
				}
			}
		}
	}
}

// Connect handles WebSocket connection for container terminal
func (h *TerminalHandler) Connect(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUserFromContext(r.Context())
	if !user.IsAdmin() {
		http.Error(w, "Admin access required", http.StatusForbidden)
		return
	}

	containerID := chi.URLParam(r, "id")

	// Create exec instance
	execResp, err := h.client.CreateExec(r.Context(), containerID, []string{"/bin/sh"})
	if err != nil {
		log.Printf("Failed to create exec: %v", err)
		http.Error(w, "Failed to create exec: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Connect to Podman socket for exec start
	socketPath := h.client.GetSocketPath()
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		log.Printf("Failed to connect to socket: %v", err)
		http.Error(w, "Failed to connect to Podman", http.StatusInternalServerError)
		return
	}

	// Send exec start request (hijack connection)
	execStartReq := `{"Detach":false,"Tty":true}`
	httpReq := fmt.Sprintf("POST /v4.0.0/libpod/exec/%s/start HTTP/1.1\r\n"+
		"Host: localhost\r\n"+
		"Content-Type: application/json\r\n"+
		"Content-Length: %d\r\n"+
		"Connection: Upgrade\r\n"+
		"Upgrade: tcp\r\n"+
		"\r\n"+
		"%s", execResp.ID, len(execStartReq), execStartReq)

	_, err = conn.Write([]byte(httpReq))
	if err != nil {
		conn.Close()
		log.Printf("Failed to send exec start: %v", err)
		http.Error(w, "Failed to start exec", http.StatusInternalServerError)
		return
	}

	// Read response header
	reader := bufio.NewReader(conn)
	resp, err := http.ReadResponse(reader, nil)
	if err != nil {
		conn.Close()
		log.Printf("Failed to read response: %v", err)
		http.Error(w, "Failed to start exec", http.StatusInternalServerError)
		return
	}

	log.Printf("Exec start response: %d %s", resp.StatusCode, resp.Status)

	if resp.StatusCode != http.StatusSwitchingProtocols && resp.StatusCode != http.StatusOK {
		conn.Close()
		body, _ := io.ReadAll(resp.Body)
		log.Printf("Exec start failed: %d %s", resp.StatusCode, string(body))
		http.Error(w, "Exec start failed", http.StatusInternalServerError)
		return
	}

	// Upgrade HTTP to WebSocket
	ws, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		conn.Close()
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}

	// Log terminal connection
	h.eventStore.Add(events.EventTerminalContainer, user.Username, getClientIP(r), true, shortID(containerID))

	// Start proxying
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// Read from container -> write to WebSocket
	go func() {
		defer cancel()
		buf := make([]byte, 1024)
		for {
			select {
			case <-ctx.Done():
				return
			default:
				conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
				n, err := conn.Read(buf)
				if err != nil {
					if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
						continue
					}
					if err != io.EOF {
						log.Printf("Read from container error: %v", err)
					}
					return
				}
				if n > 0 {
					if err := ws.WriteMessage(websocket.TextMessage, buf[:n]); err != nil {
						log.Printf("WebSocket write error: %v", err)
						return
					}
				}
			}
		}
	}()

	// Read from WebSocket -> write to container
	for {
		select {
		case <-ctx.Done():
			ws.Close()
			conn.Close()
			return
		default:
			_, message, err := ws.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					log.Printf("WebSocket read error: %v", err)
				}
				ws.Close()
				conn.Close()
				return
			}

			// Parse message
			var msg ExecMessage
			if err := json.Unmarshal(message, &msg); err != nil {
				// Treat as raw stdin
				if _, err := conn.Write(message); err != nil {
					log.Printf("Container write error: %v", err)
					ws.Close()
					conn.Close()
					return
				}
				continue
			}

			switch msg.Type {
			case "stdin":
				if _, err := conn.Write([]byte(msg.Data)); err != nil {
					log.Printf("Container write error: %v", err)
					ws.Close()
					conn.Close()
					return
				}
			case "resize":
				// Resize is more complex with Podman, skip for now
				// Would need to send resize request to exec instance
			}
		}
	}
}

// SimpleTerminal provides a simpler terminal implementation using polling
// This is a fallback if WebSocket doesn't work well
func (h *TerminalHandler) SimpleTerminal(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUserFromContext(r.Context())
	if !user.IsAdmin() {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "Admin access required"})
		return
	}

	containerID := chi.URLParam(r, "id")

	// For simple terminal, just exec a command and return output
	if r.Method == http.MethodPost {
		var req struct {
			Command string `json:"command"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request"})
			return
		}

		// Create and run exec
		cmd := []string{"/bin/sh", "-c", req.Command}
		execResp, err := h.client.CreateExec(r.Context(), containerID, cmd)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		// Start exec and get output
		output, err := h.runExec(execResp.ID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		writeJSON(w, http.StatusOK, map[string]string{"output": output})
		return
	}

	writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
}

// runExec starts an exec instance and returns output
func (h *TerminalHandler) runExec(execID string) (string, error) {
	socketPath := h.client.GetSocketPath()
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return "", err
	}
	defer conn.Close()

	// Send exec start request
	execStartReq := `{"Detach":false,"Tty":false}`
	httpReq := "POST /v4.0.0/libpod/exec/" + execID + "/start HTTP/1.1\r\n" +
		"Host: localhost\r\n" +
		"Content-Type: application/json\r\n" +
		"Content-Length: " + itoa(len(execStartReq)) + "\r\n" +
		"\r\n" +
		execStartReq

	_, err = conn.Write([]byte(httpReq))
	if err != nil {
		return "", err
	}

	// Read response
	reader := bufio.NewReader(conn)
	resp, err := http.ReadResponse(reader, nil)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	// Clean up output (remove stream headers if present)
	output := cleanOutput(string(body))
	return output, nil
}

// itoa converts int to string
func itoa(i int) string {
	return strings.TrimSpace(fmt.Sprintf("%d", i))
}

// cleanOutput removes stream protocol headers from output
func cleanOutput(s string) string {
	// Podman stream format: [stream type (1 byte)][size (3 bytes)][data]
	// For simplicity, just return as-is for now
	return s
}
