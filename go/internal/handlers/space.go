package handlers

import (
	"encoding/json"
	"net/http"
)

type SpaceHandler struct {
	store Store
}

func NewSpaceHandler(store Store) *SpaceHandler {
	return &SpaceHandler{store: store}
}

func (h *SpaceHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	total, used, free, err := h.store.SpaceInfo()
	if err != nil {
		http.Error(w, "space info unavailable", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"total": total,
		"used":  used,
		"free":  free,
	})
}
