// Package plugins provides HTTP handlers for plugin management.
package plugins

import (
	"encoding/json"
	"net/http"
)

// Handler provides HTTP handlers for plugin endpoints
type Handler struct {
	loader *Loader
}

// NewHandler creates a new plugin handler
func NewHandler(loader *Loader) *Handler {
	return &Handler{loader: loader}
}

// RegisterRoutes registers plugin management routes
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/plugins", h.ListPlugins)
	mux.HandleFunc("GET /api/v1/plugins/{name}", h.GetPlugin)
	mux.HandleFunc("GET /api/v1/plugins/{name}/health", h.GetPluginHealth)

	// Mount plugin routes
	mux.Handle("/plugins/", h.loader.Handler())
}

// ListPlugins returns all registered plugins
func (h *Handler) ListPlugins(w http.ResponseWriter, r *http.Request) {
	plugins := h.loader.List()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"plugins": plugins,
		"count":   len(plugins),
	})
}

// GetPlugin returns a specific plugin
func (h *Handler) GetPlugin(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		http.Error(w, `{"error":"plugin name required"}`, http.StatusBadRequest)
		return
	}

	plugins := h.loader.List()
	for _, p := range plugins {
		if p.Info.Name == name {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(p)
			return
		}
	}

	http.Error(w, `{"error":"plugin not found"}`, http.StatusNotFound)
}

// GetPluginHealth returns the health of a specific plugin
func (h *Handler) GetPluginHealth(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		http.Error(w, `{"error":"plugin name required"}`, http.StatusBadRequest)
		return
	}

	plugin, exists := h.loader.Get(name)
	if !exists {
		http.Error(w, `{"error":"plugin not found"}`, http.StatusNotFound)
		return
	}

	health := plugin.Health()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(health)
}
