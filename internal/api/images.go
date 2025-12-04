package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"rvpodview/internal/auth"
	"rvpodview/internal/events"
	"rvpodview/internal/podman"
)

// ImageHandler handles image endpoints
type ImageHandler struct {
	client     *podman.Client
	eventStore *events.Store
}

// NewImageHandler creates new image handler
func NewImageHandler(client *podman.Client, eventStore *events.Store) *ImageHandler {
	return &ImageHandler{client: client, eventStore: eventStore}
}

// ImageWithUsage extends Image with usage info
type ImageWithUsage struct {
	ID       string   `json:"Id"`
	RepoTags []string `json:"RepoTags"`
	Created  int64    `json:"Created"`
	Size     int64    `json:"Size"`
	InUse    bool     `json:"InUse"`
}

// List handles GET /api/images
func (h *ImageHandler) List(w http.ResponseWriter, r *http.Request) {
	images, err := h.client.ListImages(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// Get containers to check which images are in use
	containers, _ := h.client.ListContainers(r.Context())
	usedImageIDs := make(map[string]bool)
	for _, c := range containers {
		if c.ImageID != "" {
			usedImageIDs[c.ImageID] = true
		}
	}

	// Build response with usage info
	result := make([]ImageWithUsage, len(images))
	for i, img := range images {
		result[i] = ImageWithUsage{
			ID:       img.ID,
			RepoTags: img.RepoTags,
			Created:  img.Created,
			Size:     img.Size,
			InUse:    usedImageIDs[img.ID],
		}
	}

	writeJSON(w, http.StatusOK, result)
}

// Inspect handles GET /api/images/{id}
func (h *ImageHandler) Inspect(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	info, err := h.client.InspectImage(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, info)
}

// PullRequest represents image pull request
type PullRequest struct {
	Reference string `json:"reference"`
}

// Pull handles POST /api/images/pull
func (h *ImageHandler) Pull(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUserFromContext(r.Context())
	if !user.IsAdmin() {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "Admin access required"})
		return
	}

	var req PullRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
		return
	}

	if req.Reference == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Reference is required"})
		return
	}

	if err := h.client.PullImage(r.Context(), req.Reference); err != nil {
		h.eventStore.Add(events.EventImagePull, user.Username, getClientIP(r), false, req.Reference)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	h.eventStore.Add(events.EventImagePull, user.Username, getClientIP(r), true, req.Reference)
	writeJSON(w, http.StatusOK, map[string]string{"status": "pulled"})
}

// Remove handles DELETE /api/images/{id}
func (h *ImageHandler) Remove(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUserFromContext(r.Context())
	if !user.IsAdmin() {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "Admin access required"})
		return
	}

	id := chi.URLParam(r, "id")
	force := r.URL.Query().Get("force") == "true"

	if err := h.client.RemoveImage(r.Context(), id, force); err != nil {
		h.eventStore.Add(events.EventImageRemove, user.Username, getClientIP(r), false, shortID(id))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	h.eventStore.Add(events.EventImageRemove, user.Username, getClientIP(r), true, shortID(id))
	writeJSON(w, http.StatusOK, map[string]string{"status": "removed"})
}
