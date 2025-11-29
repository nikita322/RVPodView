package api

import (
	"net/http"
	"os/exec"

	"rvpodview/internal/auth"
	"rvpodview/internal/podman"
)

// SystemHandler handles system endpoints
type SystemHandler struct {
	client *podman.Client
}

// NewSystemHandler creates new system handler
func NewSystemHandler(client *podman.Client) *SystemHandler {
	return &SystemHandler{client: client}
}

// DashboardInfo represents dashboard summary
type DashboardInfo struct {
	System     *DashboardSystemInfo `json:"system"`
	HostStats  *HostStats           `json:"hostStats"`
	Containers ContainerCounts      `json:"containers"`
	Images     int                  `json:"images"`
	Volumes    int                  `json:"volumes"`
	Networks   int                  `json:"networks"`
}

// DashboardSystemInfo contains only used system fields
type DashboardSystemInfo struct {
	Host    DashboardHostInfo    `json:"host"`
	Version DashboardVersionInfo `json:"version"`
}

// DashboardHostInfo contains only used host fields
type DashboardHostInfo struct {
	Arch     string `json:"arch"`
	Hostname string `json:"hostname"`
	Kernel   string `json:"kernel"`
	MemTotal int64  `json:"memTotal"`
	MemFree  int64  `json:"memFree"`
}

// DashboardVersionInfo contains only used version fields
type DashboardVersionInfo struct {
	Version string `json:"Version"`
}

// ContainerCounts represents container count statistics
type ContainerCounts struct {
	Total   int `json:"total"`
	Running int `json:"running"`
	Stopped int `json:"stopped"`
}

// Dashboard handles GET /api/system/dashboard
func (h *SystemHandler) Dashboard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get system info
	sysInfo, err := h.client.GetSystemInfo(ctx)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// Get containers count
	containers, err := h.client.ListContainers(ctx)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	containerCounts := ContainerCounts{Total: len(containers)}
	for _, c := range containers {
		if c.State == "running" {
			containerCounts.Running++
		} else {
			containerCounts.Stopped++
		}
	}

	// Get images count
	images, err := h.client.ListImages(ctx)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// Get volumes count
	volumes, err := h.client.ListVolumes(ctx)
	if err != nil {
		volumes = nil // Ignore error, just set to 0
	}

	// Get networks count
	networks, err := h.client.ListNetworks(ctx)
	if err != nil {
		networks = nil
	}

	// Get host stats (CPU, temperatures)
	hostStats := GetHostStats()

	// Build optimized system info with only used fields
	systemInfo := &DashboardSystemInfo{
		Host: DashboardHostInfo{
			Arch:     sysInfo.Host.Arch,
			Hostname: sysInfo.Host.Hostname,
			Kernel:   sysInfo.Host.Kernel,
			MemTotal: sysInfo.Host.MemTotal,
			MemFree:  sysInfo.Host.MemFree,
		},
		Version: DashboardVersionInfo{
			Version: sysInfo.Version.Version,
		},
	}

	dashboard := DashboardInfo{
		System:     systemInfo,
		HostStats:  hostStats,
		Containers: containerCounts,
		Images:     len(images),
		Volumes:    len(volumes),
		Networks:   len(networks),
	}

	writeJSON(w, http.StatusOK, dashboard)
}

// Info handles GET /api/system/info
func (h *SystemHandler) Info(w http.ResponseWriter, r *http.Request) {
	info, err := h.client.GetSystemInfo(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, info)
}

// DiskUsage handles GET /api/system/df
func (h *SystemHandler) DiskUsage(w http.ResponseWriter, r *http.Request) {
	df, err := h.client.GetSystemDF(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, df)
}

// Prune handles POST /api/system/prune
func (h *SystemHandler) Prune(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUserFromContext(r.Context())
	if !user.IsAdmin() {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "Admin access required"})
		return
	}

	if err := h.client.SystemPrune(r.Context()); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "pruned"})
}

// Reboot handles POST /api/system/reboot
func (h *SystemHandler) Reboot(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUserFromContext(r.Context())
	if !user.IsAdmin() {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "Admin access required"})
		return
	}

	// Send response before rebooting
	writeJSON(w, http.StatusOK, map[string]string{"status": "rebooting"})

	// Reboot in background
	go func() {
		exec.Command("systemctl", "reboot").Run()
	}()
}

// Shutdown handles POST /api/system/shutdown
func (h *SystemHandler) Shutdown(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUserFromContext(r.Context())
	if !user.IsAdmin() {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "Admin access required"})
		return
	}

	// Send response before shutdown
	writeJSON(w, http.StatusOK, map[string]string{"status": "shutting down"})

	// Shutdown in background
	go func() {
		exec.Command("systemctl", "poweroff").Run()
	}()
}
