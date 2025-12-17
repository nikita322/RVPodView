package api

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"podmanview/internal/auth"
	"podmanview/internal/events"
)

// FileManagerHandler handles file operations
type FileManagerHandler struct {
	eventStore *events.Store
	baseDir    string // Base directory for file operations (e.g., /home)
	maxUploadSize int64 // Maximum upload size in bytes (default 100MB)
}

// NewFileManagerHandler creates new file manager handler
func NewFileManagerHandler(eventStore *events.Store, baseDir string) *FileManagerHandler {
	// If baseDir is empty, use user's home directory or root
	if baseDir == "" {
		if homeDir, err := os.UserHomeDir(); err == nil {
			baseDir = homeDir
		} else {
			baseDir = "/"
		}
	}

	// Clean and resolve absolute path
	baseDir, err := filepath.Abs(baseDir)
	if err != nil {
		log.Printf("Warning: failed to resolve base directory, using /: %v", err)
		baseDir = "/"
	}

	return &FileManagerHandler{
		eventStore:    eventStore,
		baseDir:       baseDir,
		maxUploadSize: 100 * 1024 * 1024, // 100MB default
	}
}

// FileInfo represents file or directory information
type FileInfo struct {
	Name    string    `json:"name"`
	IsDir   bool      `json:"is_dir"`
	Size    int64     `json:"size"`
	ModTime time.Time `json:"mod_time"`
	Mode    string    `json:"mode"`
	Path    string    `json:"path"` // Full path relative to baseDir
}

// BrowseResponse represents directory browsing response
type BrowseResponse struct {
	Path   string     `json:"path"`   // Current path relative to baseDir
	Parent string     `json:"parent"` // Parent directory path
	Items  []FileInfo `json:"items"`  // Files and directories
}

// validatePath checks if path is safe and within baseDir
// Returns cleaned absolute path and error
func (h *FileManagerHandler) validatePath(requestedPath string) (string, error) {
	// Special case: "/" or empty path means baseDir root
	if requestedPath == "" || requestedPath == "/" {
		return h.baseDir, nil
	}

	// Clean the path to remove .. and other unsafe elements
	cleanPath := filepath.Clean(requestedPath)

	// Remove leading slash for relative path construction
	cleanPath = strings.TrimPrefix(cleanPath, "/")
	cleanPath = strings.TrimPrefix(cleanPath, "\\")

	// Join with baseDir to get absolute path
	absPath := filepath.Join(h.baseDir, cleanPath)

	// Resolve to absolute path (handles symlinks)
	absPath, err := filepath.Abs(absPath)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}

	// Ensure path is within baseDir (prevent path traversal)
	// Use filepath.Clean to normalize both paths for comparison
	normalizedBase := filepath.Clean(h.baseDir)
	normalizedAbs := filepath.Clean(absPath)

	if !strings.HasPrefix(normalizedAbs, normalizedBase) {
		return "", fmt.Errorf("access denied: path outside base directory")
	}

	return absPath, nil
}

// getRelativePath returns path relative to baseDir
func (h *FileManagerHandler) getRelativePath(absPath string) string {
	relPath, err := filepath.Rel(h.baseDir, absPath)
	if err != nil {
		return "/"
	}
	if relPath == "." {
		return "/"
	}
	return "/" + filepath.ToSlash(relPath)
}

// Browse lists files and directories
func (h *FileManagerHandler) Browse(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUserFromContext(r.Context())
	if !user.IsAdmin() {
		http.Error(w, "Admin access required", http.StatusForbidden)
		return
	}

	requestedPath := r.URL.Query().Get("path")
	if requestedPath == "" {
		requestedPath = "/"
	}

	// Validate and get absolute path
	absPath, err := h.validatePath(requestedPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Check if path exists and is a directory
	stat, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "Directory not found", http.StatusNotFound)
		} else {
			http.Error(w, "Failed to access directory", http.StatusInternalServerError)
		}
		return
	}

	if !stat.IsDir() {
		http.Error(w, "Path is not a directory", http.StatusBadRequest)
		return
	}

	// Read directory contents
	entries, err := os.ReadDir(absPath)
	if err != nil {
		http.Error(w, "Failed to read directory", http.StatusInternalServerError)
		log.Printf("Failed to read directory %s: %v", absPath, err)
		return
	}

	// Build file list
	items := make([]FileInfo, 0, len(entries))
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue // Skip files we can't stat
		}

		itemPath := filepath.Join(absPath, entry.Name())
		relPath := h.getRelativePath(itemPath)

		items = append(items, FileInfo{
			Name:    entry.Name(),
			IsDir:   entry.IsDir(),
			Size:    info.Size(),
			ModTime: info.ModTime(),
			Mode:    info.Mode().String(),
			Path:    relPath,
		})
	}

	// Get parent directory
	parentPath := "/"
	if absPath != h.baseDir {
		parentAbs := filepath.Dir(absPath)
		parentPath = h.getRelativePath(parentAbs)
	}

	response := BrowseResponse{
		Path:   h.getRelativePath(absPath),
		Parent: parentPath,
		Items:  items,
	}

	// Log browse event
	h.eventStore.Add(events.EventFileBrowse, user.Username, getClientIP(r), true,
		fmt.Sprintf("path=%s", h.getRelativePath(absPath)))

	writeJSON(w, http.StatusOK, response)
}

// Download serves a file for download
func (h *FileManagerHandler) Download(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUserFromContext(r.Context())
	if !user.IsAdmin() {
		http.Error(w, "Admin access required", http.StatusForbidden)
		return
	}

	requestedPath := r.URL.Query().Get("path")
	if requestedPath == "" {
		http.Error(w, "Path is required", http.StatusBadRequest)
		return
	}

	// Validate and get absolute path
	absPath, err := h.validatePath(requestedPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Check if file exists
	stat, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "File not found", http.StatusNotFound)
		} else {
			http.Error(w, "Failed to access file", http.StatusInternalServerError)
		}
		return
	}

	if stat.IsDir() {
		http.Error(w, "Cannot download directory", http.StatusBadRequest)
		return
	}

	// Open file
	file, err := os.Open(absPath)
	if err != nil {
		http.Error(w, "Failed to open file", http.StatusInternalServerError)
		log.Printf("Failed to open file %s: %v", absPath, err)
		return
	}
	defer file.Close()

	// Detect content type
	contentType := mime.TypeByExtension(filepath.Ext(absPath))
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	// Set headers
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filepath.Base(absPath)))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", stat.Size()))

	// Stream file to client
	_, err = io.Copy(w, file)
	if err != nil {
		log.Printf("Failed to send file %s: %v", absPath, err)
		return
	}

	// Log download event
	h.eventStore.Add(events.EventFileDownload, user.Username, getClientIP(r), true,
		fmt.Sprintf("file=%s size=%d", filepath.Base(absPath), stat.Size()))
}

// Upload handles file uploads (multipart form)
func (h *FileManagerHandler) Upload(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUserFromContext(r.Context())
	if !user.IsAdmin() {
		http.Error(w, "Admin access required", http.StatusForbidden)
		return
	}

	// Limit request body size
	r.Body = http.MaxBytesReader(w, r.Body, h.maxUploadSize)

	// Parse multipart form
	err := r.ParseMultipartForm(h.maxUploadSize)
	if err != nil {
		http.Error(w, "File too large or invalid form data", http.StatusBadRequest)
		return
	}

	// Get target directory
	targetPath := r.FormValue("path")
	if targetPath == "" {
		targetPath = "/"
	}

	// Validate target directory
	absTargetDir, err := h.validatePath(targetPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Check if target is a directory
	stat, err := os.Stat(absTargetDir)
	if err != nil || !stat.IsDir() {
		http.Error(w, "Target path is not a directory", http.StatusBadRequest)
		return
	}

	// Get uploaded files
	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		http.Error(w, "No files uploaded", http.StatusBadRequest)
		return
	}

	uploadedFiles := []string{}
	var uploadErr error

	for _, fileHeader := range files {
		// Validate filename (prevent path traversal)
		filename := filepath.Base(fileHeader.Filename)
		if filename == "" || filename == "." || filename == ".." {
			uploadErr = fmt.Errorf("invalid filename: %s", fileHeader.Filename)
			break
		}

		// Open uploaded file
		file, err := fileHeader.Open()
		if err != nil {
			uploadErr = fmt.Errorf("failed to open uploaded file: %w", err)
			break
		}

		// Create destination file
		destPath := filepath.Join(absTargetDir, filename)
		destFile, err := os.Create(destPath)
		if err != nil {
			file.Close()
			uploadErr = fmt.Errorf("failed to create file: %w", err)
			break
		}

		// Copy file contents
		_, err = io.Copy(destFile, file)
		file.Close()
		destFile.Close()

		if err != nil {
			os.Remove(destPath) // Clean up partial file
			uploadErr = fmt.Errorf("failed to write file: %w", err)
			break
		}

		uploadedFiles = append(uploadedFiles, filename)
	}

	if uploadErr != nil {
		// Clean up successfully uploaded files on error
		for _, filename := range uploadedFiles {
			os.Remove(filepath.Join(absTargetDir, filename))
		}
		http.Error(w, uploadErr.Error(), http.StatusInternalServerError)
		log.Printf("Upload failed: %v", uploadErr)
		return
	}

	// Log upload event
	h.eventStore.Add(events.EventFileUpload, user.Username, getClientIP(r), true,
		fmt.Sprintf("files=%d path=%s", len(uploadedFiles), h.getRelativePath(absTargetDir)))

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"files":   uploadedFiles,
		"count":   len(uploadedFiles),
	})
}

// Delete removes a file or directory
func (h *FileManagerHandler) Delete(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUserFromContext(r.Context())
	if !user.IsAdmin() {
		http.Error(w, "Admin access required", http.StatusForbidden)
		return
	}

	requestedPath := r.URL.Query().Get("path")
	if requestedPath == "" || requestedPath == "/" {
		http.Error(w, "Cannot delete root directory", http.StatusBadRequest)
		return
	}

	// Validate and get absolute path
	absPath, err := h.validatePath(requestedPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Prevent deleting baseDir
	if absPath == h.baseDir {
		http.Error(w, "Cannot delete base directory", http.StatusBadRequest)
		return
	}

	// Check if path exists
	stat, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "File or directory not found", http.StatusNotFound)
		} else {
			http.Error(w, "Failed to access path", http.StatusInternalServerError)
		}
		return
	}

	// Remove file or directory (recursively if directory)
	err = os.RemoveAll(absPath)
	if err != nil {
		http.Error(w, "Failed to delete", http.StatusInternalServerError)
		log.Printf("Failed to delete %s: %v", absPath, err)
		return
	}

	// Log delete event
	itemType := "file"
	if stat.IsDir() {
		itemType = "directory"
	}
	h.eventStore.Add(events.EventFileDelete, user.Username, getClientIP(r), true,
		fmt.Sprintf("type=%s path=%s", itemType, h.getRelativePath(absPath)))

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("%s deleted successfully", itemType),
	})
}

// MkDir creates a new directory
func (h *FileManagerHandler) MkDir(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUserFromContext(r.Context())
	if !user.IsAdmin() {
		http.Error(w, "Admin access required", http.StatusForbidden)
		return
	}

	var req struct {
		Path string `json:"path"`
		Name string `json:"name"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		http.Error(w, "Directory name is required", http.StatusBadRequest)
		return
	}

	// Validate directory name (prevent path traversal)
	dirName := filepath.Base(req.Name)
	if dirName == "" || dirName == "." || dirName == ".." || strings.Contains(dirName, "/") || strings.Contains(dirName, "\\") {
		http.Error(w, "Invalid directory name", http.StatusBadRequest)
		return
	}

	// Validate parent path
	parentPath := req.Path
	if parentPath == "" {
		parentPath = "/"
	}

	absParentDir, err := h.validatePath(parentPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Check if parent is a directory
	stat, err := os.Stat(absParentDir)
	if err != nil || !stat.IsDir() {
		http.Error(w, "Parent path is not a directory", http.StatusBadRequest)
		return
	}

	// Create new directory
	newDirPath := filepath.Join(absParentDir, dirName)
	err = os.Mkdir(newDirPath, 0755)
	if err != nil {
		if os.IsExist(err) {
			http.Error(w, "Directory already exists", http.StatusConflict)
		} else {
			http.Error(w, "Failed to create directory", http.StatusInternalServerError)
			log.Printf("Failed to create directory %s: %v", newDirPath, err)
		}
		return
	}

	// Log mkdir event
	h.eventStore.Add(events.EventFileMkdir, user.Username, getClientIP(r), true,
		fmt.Sprintf("path=%s", h.getRelativePath(newDirPath)))

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"path":    h.getRelativePath(newDirPath),
		"name":    dirName,
	})
}

// Rename renames a file or directory
func (h *FileManagerHandler) Rename(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUserFromContext(r.Context())
	if !user.IsAdmin() {
		http.Error(w, "Admin access required", http.StatusForbidden)
		return
	}

	var req struct {
		OldPath string `json:"old_path"`
		NewName string `json:"new_name"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.OldPath == "" || req.NewName == "" {
		http.Error(w, "Both old_path and new_name are required", http.StatusBadRequest)
		return
	}

	// Validate new name (prevent path traversal)
	newName := filepath.Base(req.NewName)
	if newName == "" || newName == "." || newName == ".." || strings.Contains(newName, "/") || strings.Contains(newName, "\\") {
		http.Error(w, "Invalid new name", http.StatusBadRequest)
		return
	}

	// Validate old path
	absOldPath, err := h.validatePath(req.OldPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Prevent renaming baseDir
	if absOldPath == h.baseDir {
		http.Error(w, "Cannot rename base directory", http.StatusBadRequest)
		return
	}

	// Check if old path exists
	if _, err := os.Stat(absOldPath); err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "File or directory not found", http.StatusNotFound)
		} else {
			http.Error(w, "Failed to access path", http.StatusInternalServerError)
		}
		return
	}

	// Build new path (same parent directory)
	parentDir := filepath.Dir(absOldPath)
	absNewPath := filepath.Join(parentDir, newName)

	// Check if new path already exists
	if _, err := os.Stat(absNewPath); err == nil {
		http.Error(w, "A file or directory with that name already exists", http.StatusConflict)
		return
	}

	// Rename
	err = os.Rename(absOldPath, absNewPath)
	if err != nil {
		http.Error(w, "Failed to rename", http.StatusInternalServerError)
		log.Printf("Failed to rename %s to %s: %v", absOldPath, absNewPath, err)
		return
	}

	// Log rename event
	h.eventStore.Add(events.EventFileRename, user.Username, getClientIP(r), true,
		fmt.Sprintf("from=%s to=%s", filepath.Base(absOldPath), newName))

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"path":    h.getRelativePath(absNewPath),
		"name":    newName,
	})
}

// ReadFile reads file content for editing
func (h *FileManagerHandler) ReadFile(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUserFromContext(r.Context())
	if !user.IsAdmin() {
		http.Error(w, "Admin access required", http.StatusForbidden)
		return
	}

	requestedPath := r.URL.Query().Get("path")
	if requestedPath == "" {
		http.Error(w, "Path is required", http.StatusBadRequest)
		return
	}

	// Validate and get absolute path
	absPath, err := h.validatePath(requestedPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Check if file exists
	stat, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "File not found", http.StatusNotFound)
		} else {
			http.Error(w, "Failed to access file", http.StatusInternalServerError)
		}
		return
	}

	if stat.IsDir() {
		http.Error(w, "Cannot read directory as file", http.StatusBadRequest)
		return
	}

	// Check file size (limit to 10MB for editing)
	const maxEditSize = 10 * 1024 * 1024
	if stat.Size() > maxEditSize {
		http.Error(w, "File too large to edit (max 10MB)", http.StatusBadRequest)
		return
	}

	// Read file content
	content, err := os.ReadFile(absPath)
	if err != nil {
		http.Error(w, "Failed to read file", http.StatusInternalServerError)
		log.Printf("Failed to read file %s: %v", absPath, err)
		return
	}

	// Log read event
	h.eventStore.Add(events.EventFileRead, user.Username, getClientIP(r), true,
		fmt.Sprintf("file=%s size=%d", filepath.Base(absPath), stat.Size()))

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"content": string(content),
		"name":    filepath.Base(absPath),
		"size":    stat.Size(),
	})
}

// WriteFile saves file content after editing
func (h *FileManagerHandler) WriteFile(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUserFromContext(r.Context())
	if !user.IsAdmin() {
		http.Error(w, "Admin access required", http.StatusForbidden)
		return
	}

	var req struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Path == "" {
		http.Error(w, "Path is required", http.StatusBadRequest)
		return
	}

	// Validate and get absolute path
	absPath, err := h.validatePath(req.Path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Check if file exists and is not a directory
	stat, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "File not found", http.StatusNotFound)
		} else {
			http.Error(w, "Failed to access file", http.StatusInternalServerError)
		}
		return
	}

	if stat.IsDir() {
		http.Error(w, "Cannot write to directory", http.StatusBadRequest)
		return
	}

	// Write file content
	err = os.WriteFile(absPath, []byte(req.Content), stat.Mode())
	if err != nil {
		http.Error(w, "Failed to write file", http.StatusInternalServerError)
		log.Printf("Failed to write file %s: %v", absPath, err)
		return
	}

	// Log write event
	h.eventStore.Add(events.EventFileWrite, user.Username, getClientIP(r), true,
		fmt.Sprintf("file=%s size=%d", filepath.Base(absPath), len(req.Content)))

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"name":    filepath.Base(absPath),
		"size":    len(req.Content),
	})
}
