package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"rvpodview/internal/auth"
	"rvpodview/internal/podman"
)

// ImageHandler handles image endpoints
type ImageHandler struct {
	client *podman.Client
}

// NewImageHandler creates new image handler
func NewImageHandler(client *podman.Client) *ImageHandler {
	return &ImageHandler{client: client}
}

// List handles GET /api/images
func (h *ImageHandler) List(w http.ResponseWriter, r *http.Request) {
	images, err := h.client.ListImages(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, images)
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
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

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
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "removed"})
}
