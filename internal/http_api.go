package internal

import (
	"encoding/json"
	"net/http"
	"strings"
)

// registerMusicHTTPAPI mounts fixture-friendly JSON list/detail stubs on the
// module health HTTP server (no MediaAdmin gRPC client required).
func (m *Module) registerMusicHTTPAPI(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/artists", m.handleListArtistsHTTP)
	mux.HandleFunc("GET /api/artists/{id}", m.handleGetArtistHTTP)
}

func (m *Module) handleListArtistsHTTP(w http.ResponseWriter, r *http.Request) {
	if m.store == nil {
		http.Error(w, `{"error":"store not open"}`, http.StatusServiceUnavailable)
		return
	}
	items, err := m.store.ListArtists(r.URL.Query().Get("q"))
	if err != nil {
		http.Error(w, fmtJSONError(err), http.StatusInternalServerError)
		return
	}
	out := make([]artistJSON, 0, len(items))
	for _, a := range items {
		out = append(out, toArtistJSON(a))
	}
	writeJSON(w, out)
}

func (m *Module) handleGetArtistHTTP(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, `{"error":"id required"}`, http.StatusBadRequest)
		return
	}
	if m.store == nil {
		http.Error(w, `{"error":"store not open"}`, http.StatusServiceUnavailable)
		return
	}
	a, err := m.store.GetArtist(id)
	if err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		}
		http.Error(w, fmtJSONError(err), status)
		return
	}
	albums, err := m.store.ListAlbums(id)
	if err != nil {
		http.Error(w, fmtJSONError(err), http.StatusInternalServerError)
		return
	}
	detail := artistDetailJSON{Artist: toArtistJSON(a), Albums: make([]albumJSON, 0, len(albums))}
	for _, al := range albums {
		detail.Albums = append(detail.Albums, toAlbumJSON(al))
	}
	writeJSON(w, detail)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(true)
	_ = enc.Encode(v)
}

func fmtJSONError(err error) string {
	b, _ := json.Marshal(map[string]string{"error": err.Error()})
	return string(b)
}
