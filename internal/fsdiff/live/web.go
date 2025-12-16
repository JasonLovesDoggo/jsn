package live

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"strconv"
	"sync"
	"time"

	"pkg.jsn.cam/jsn/internal/fsdiff/live/priority"
	"pkg.jsn.cam/jsn/internal/fsdiff/ui"
)

// WebServer provides a web UI for the recording session
type WebServer struct {
	session *Session
	addr    string
	server  *http.Server

	// SSE clients
	clients   map[chan *Change]struct{}
	clientsMu sync.RWMutex
}

// NewWebServer creates a new web server for the session
func NewWebServer(session *Session, addr string) *WebServer {
	ws := &WebServer{
		session: session,
		addr:    addr,
		clients: make(map[chan *Change]struct{}),
	}

	// Register for change notifications
	session.OnChange(func(changes []*Change) {
		ws.broadcast(changes)
	})

	return ws
}

// Start starts the web server
func (ws *WebServer) Start(ctx context.Context) error {
	mux := http.NewServeMux()

	// API endpoints
	mux.HandleFunc("GET /api/changes", ws.handleAPIChanges)
	mux.HandleFunc("GET /api/scans", ws.handleAPIScans)
	mux.HandleFunc("GET /api/config", ws.handleGetConfig)
	mux.HandleFunc("PUT /api/config", ws.handleUpdateConfig)
	mux.HandleFunc("GET /content/{hash}", ws.handleContent)
	mux.HandleFunc("GET /events", ws.handleSSE)
	mux.HandleFunc("GET /export", ws.handleExport)

	// Legacy htmx endpoints (can be removed later)
	mux.HandleFunc("GET /changes", ws.handleChanges)
	mux.HandleFunc("GET /changes/{idx}", ws.handleChangeDetail)
	mux.HandleFunc("GET /stats", ws.handleStats)

	// Serve embedded Svelte UI
	distFS, _ := fs.Sub(ui.Dist, "dist")
	fileServer := http.FileServer(http.FS(distFS))
	mux.Handle("GET /assets/", fileServer)
	mux.HandleFunc("GET /", ws.handleSPA(distFS))

	ws.server = &http.Server{
		Addr:         ws.addr,
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second, // Longer for SSE
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		ws.server.Shutdown(shutdownCtx)
	}()

	if err := ws.server.ListenAndServe(); err != http.ErrServerClosed {
		return err
	}
	return nil
}

// handleSPA returns a handler that serves index.html for SPA routing
func (ws *WebServer) handleSPA(fsys fs.FS) http.HandlerFunc {
	indexHTML, _ := fs.ReadFile(fsys, "index.html")
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(indexHTML)
	}
}

// handleChanges returns the changes list as HTML partial
func (ws *WebServer) handleChanges(w http.ResponseWriter, r *http.Request) {
	changes := ws.session.GetChanges()

	// Apply filter if provided
	filter := r.URL.Query().Get("filter")

	// Reverse to show newest first
	reversed := make([]*Change, len(changes))
	for i, c := range changes {
		reversed[len(changes)-1-i] = c
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	component := ChangesList(reversed, filter)
	component.Render(r.Context(), w)
}

// handleChangeDetail returns detail for a specific change
func (ws *WebServer) handleChangeDetail(w http.ResponseWriter, r *http.Request) {
	idxStr := r.PathValue("idx")
	idx, err := strconv.Atoi(idxStr)
	if err != nil {
		http.Error(w, "invalid index", http.StatusBadRequest)
		return
	}

	// Get a safe copy of changes
	changes := ws.session.GetChanges()
	n := len(changes)
	if idx < 0 || idx >= n {
		http.Error(w, "index out of range", http.StatusBadRequest)
		return
	}

	// The UI shows changes in reverse order (newest first)
	// So idx 0 in UI = last element in slice
	realIdx := n - 1 - idx
	if realIdx < 0 || realIdx >= n {
		http.Error(w, "index out of range", http.StatusBadRequest)
		return
	}

	change := changes[realIdx]

	// Load content if available
	var content string
	if change.ContentKey != "" {
		data, err := ws.session.store.LoadContent(change.ContentKey)
		if err == nil {
			content = string(data)
		}
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	component := ChangeDetail(change, content, idx)
	component.Render(r.Context(), w)
}

// handleContent returns raw file content by hash
func (ws *WebServer) handleContent(w http.ResponseWriter, r *http.Request) {
	hash := r.PathValue("hash")
	if hash == "" {
		http.Error(w, "missing hash", http.StatusBadRequest)
		return
	}

	content, err := ws.session.store.LoadContent(hash)
	if err != nil {
		http.Error(w, "content not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write(content)
}

// APIChange extends Change with priority classification
type APIChange struct {
	*Change
	Priority priority.Level `json:"priority"`
}

// APIChangesResponse is the response for /api/changes
type APIChangesResponse struct {
	Changes []*APIChange `json:"changes"`
	Total   int          `json:"total"`
	Offset  int          `json:"offset"`
	Limit   int          `json:"limit"`
}

// handleAPIChanges returns filtered changes as JSON
// Query params:
//   - since: unix timestamp (seconds or ms)
//   - until: unix timestamp
//   - priority: "critical" | "interesting" | "all" (default: "all")
//   - exclude_bulk: bool (default: true)
//   - scanId: int (filter by scan ID)
//   - limit: int (default: 1000)
//   - offset: int (default: 0)
func (ws *WebServer) handleAPIChanges(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	var since, until time.Time
	if s := q.Get("since"); s != "" {
		since = parseTimestamp(s)
	}
	if u := q.Get("until"); u != "" {
		until = parseTimestamp(u)
	}

	priorityFilter := q.Get("priority")
	if priorityFilter == "" {
		priorityFilter = "all"
	}

	excludeBulk := true
	if eb := q.Get("exclude_bulk"); eb == "false" || eb == "0" {
		excludeBulk = false
	}

	scanID := 0
	if s := q.Get("scanId"); s != "" {
		if parsed, err := strconv.Atoi(s); err == nil {
			scanID = parsed
		}
	}

	limit := 1000
	if l := q.Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	offset := 0
	if o := q.Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	allChanges := ws.session.GetChanges()

	var filtered []*APIChange
	for _, c := range allChanges {
		if !since.IsZero() && c.Timestamp.Before(since) {
			continue
		}
		if !until.IsZero() && c.Timestamp.After(until) {
			continue
		}
		if excludeBulk && c.BulkID > 0 {
			continue
		}
		if scanID > 0 && c.ScanID != scanID {
			continue
		}

		p := priority.Classify(c.Path, c.Mode, c.Type == ChangeAdded, c.BulkID > 0)

		switch priorityFilter {
		case "critical":
			if p != priority.Critical {
				continue
			}
		case "interesting":
			if p != priority.Critical && p != priority.Interesting {
				continue
			}
		}

		filtered = append(filtered, &APIChange{Change: c, Priority: p})
	}

	total := len(filtered)
	if offset > len(filtered) {
		filtered = nil
	} else {
		filtered = filtered[offset:]
		if len(filtered) > limit {
			filtered = filtered[:limit]
		}
	}

	resp := APIChangesResponse{
		Changes: filtered,
		Total:   total,
		Offset:  offset,
		Limit:   limit,
	}
	if resp.Changes == nil {
		resp.Changes = []*APIChange{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func parseTimestamp(s string) time.Time {
	if ts, err := strconv.ParseInt(s, 10, 64); err == nil {
		if ts > 4102444800 {
			return time.UnixMilli(ts)
		}
		return time.Unix(ts, 0)
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t
	}
	return time.Time{}
}

// ConfigResponse is the response for /api/config
type ConfigResponse struct {
	Interval     int   `json:"interval"`     // seconds
	LastScanTime int64 `json:"lastScanTime"` // unix ms
	NextScanTime int64 `json:"nextScanTime"` // unix ms
}

// handleGetConfig returns current config and timing
func (ws *WebServer) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	lastScan, nextScan, interval := ws.session.GetScanTiming()
	resp := ConfigResponse{
		Interval:     int(interval.Seconds()),
		LastScanTime: lastScan.UnixMilli(),
		NextScanTime: nextScan.UnixMilli(),
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// ConfigUpdate is the request body for PUT /api/config
type ConfigUpdate struct {
	Interval int `json:"interval"` // seconds
}

// handleUpdateConfig updates the scan interval
func (ws *WebServer) handleUpdateConfig(w http.ResponseWriter, r *http.Request) {
	var update ConfigUpdate
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if update.Interval < 5 || update.Interval > 3600 {
		http.Error(w, "interval must be 5-3600 seconds", http.StatusBadRequest)
		return
	}
	ws.session.SetInterval(time.Duration(update.Interval) * time.Second)

	// Return updated config
	ws.handleGetConfig(w, r)
}

// handleAPIScans returns all scan metadata
func (ws *WebServer) handleAPIScans(w http.ResponseWriter, r *http.Request) {
	scans := ws.session.GetScans()
	if scans == nil {
		scans = []*Scan{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(scans)
}

// handleSSE handles Server-Sent Events for live updates
func (ws *WebServer) handleSSE(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE not supported", http.StatusInternalServerError)
		return
	}

	// Create client channel
	clientChan := make(chan *Change, 100)
	ws.clientsMu.Lock()
	ws.clients[clientChan] = struct{}{}
	ws.clientsMu.Unlock()

	defer func() {
		ws.clientsMu.Lock()
		delete(ws.clients, clientChan)
		ws.clientsMu.Unlock()
		close(clientChan)
	}()

	// Send initial ping
	fmt.Fprintf(w, "event: ping\ndata: connected\n\n")
	flusher.Flush()

	for {
		select {
		case <-r.Context().Done():
			return
		case change := <-clientChan:
			data, _ := json.Marshal(change)
			fmt.Fprintf(w, "event: change\ndata: %s\n\n", data)
			flusher.Flush()
		}
	}
}

// handleExport exports the session as JSON
func (ws *WebServer) handleExport(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=session.json")

	if err := ws.session.store.ExportJSON(w); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// handleStats returns current stats as HTML partial
func (ws *WebServer) handleStats(w http.ResponseWriter, r *http.Request) {
	changes := ws.session.GetChanges()

	added, modified, deleted := 0, 0, 0
	for _, c := range changes {
		switch c.Type {
		case ChangeAdded:
			added++
		case ChangeModified:
			modified++
		case ChangeDeleted:
			deleted++
		}
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	component := Stats(len(changes), added, modified, deleted)
	component.Render(r.Context(), w)
}

// broadcast sends changes to all SSE clients
func (ws *WebServer) broadcast(changes []*Change) {
	ws.clientsMu.RLock()
	defer ws.clientsMu.RUnlock()

	for clientChan := range ws.clients {
		for _, change := range changes {
			select {
			case clientChan <- change:
			default:
				// Client too slow, skip
			}
		}
	}
}
