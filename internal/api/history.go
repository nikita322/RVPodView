package api

import (
	"bufio"
	"encoding/json"
	"log"
	"os"
	"strings"
	"sync"
	"time"
)

// CommandHistoryEntry represents a single command in history
type CommandHistoryEntry struct {
	Command   string    `json:"command"`
	Timestamp time.Time `json:"timestamp"`
}

// HistoryHandler handles command history operations
type HistoryHandler struct {
	historyFile string
	mu          sync.RWMutex
}

// NewHistoryHandler creates new history handler
func NewHistoryHandler(historyFile string) *HistoryHandler {
	return &HistoryHandler{
		historyFile: historyFile,
	}
}

// loadHistory returns command history array (last 50 commands)
func (h *HistoryHandler) loadHistory() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	file, err := os.Open(h.historyFile)
	if err != nil {
		return []string{}
	}
	defer file.Close()

	var commands []string
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var entry CommandHistoryEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}

		commands = append(commands, entry.Command)
	}

	// Return only last 50 commands
	if len(commands) > 50 {
		commands = commands[len(commands)-50:]
	}

	return commands
}

// saveCommand saves a command to history (called from WebSocket)
func (h *HistoryHandler) saveCommand(command string) error {
	command = strings.TrimSpace(command)
	if command == "" {
		return nil
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	// Check if this is a duplicate of the last command
	if lastCmd := h.getLastCommand(h.historyFile); lastCmd == command {
		return nil
	}

	// Append to history file
	file, err := os.OpenFile(h.historyFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer file.Close()

	entry := CommandHistoryEntry{
		Command:   command,
		Timestamp: time.Now(),
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	if _, err := file.WriteString(string(data) + "\n"); err != nil {
		return err
	}

	// Keep only last 500 commands (trim if needed)
	go h.trimHistory(h.historyFile, 500)

	return nil
}

// getLastCommand returns the last command from history file
func (h *HistoryHandler) getLastCommand(historyFile string) string {
	file, err := os.Open(historyFile)
	if err != nil {
		return ""
	}
	defer file.Close()

	var lastLine string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if line != "" {
			lastLine = line
		}
	}

	if lastLine == "" {
		return ""
	}

	var entry CommandHistoryEntry
	if err := json.Unmarshal([]byte(lastLine), &entry); err != nil {
		return ""
	}

	return entry.Command
}

// trimHistory keeps only the last N commands in history file
func (h *HistoryHandler) trimHistory(historyFile string, maxCommands int) {
	file, err := os.Open(historyFile)
	if err != nil {
		return
	}

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if line != "" {
			lines = append(lines, line)
		}
	}
	file.Close()

	// Keep only last maxCommands
	if len(lines) <= maxCommands {
		return
	}

	lines = lines[len(lines)-maxCommands:]

	// Write back to file
	file, err = os.OpenFile(historyFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Printf("Failed to trim history: %v", err)
		return
	}
	defer file.Close()

	for _, line := range lines {
		file.WriteString(line + "\n")
	}
}

