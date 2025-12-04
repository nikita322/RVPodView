package api

import (
	"net/http"
	"strconv"

	"rvpodview/internal/events"
)

// EventsHandler handles event log endpoints
type EventsHandler struct {
	store *events.Store
}

// NewEventsHandler creates new events handler
func NewEventsHandler(store *events.Store) *EventsHandler {
	return &EventsHandler{store: store}
}

// List returns events from the store
// GET /api/events?limit=50&since=123
func (h *EventsHandler) List(w http.ResponseWriter, r *http.Request) {
	// Check for since parameter (get events after ID)
	if sinceStr := r.URL.Query().Get("since"); sinceStr != "" {
		sinceID, err := strconv.ParseInt(sinceStr, 10, 64)
		if err == nil {
			eventList := h.store.GetSince(sinceID)
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"events": eventList,
				"lastId": h.store.LastID(),
			})
			return
		}
	}

	// Check for limit parameter
	limit := 50 // default
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	eventList := h.store.GetLast(limit)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"events": eventList,
		"lastId": h.store.LastID(),
	})
}
