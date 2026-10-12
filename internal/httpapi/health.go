package httpapi

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/BrOrlandi/whatsapp-mcp/internal/health"
	"github.com/BrOrlandi/whatsapp-mcp/internal/version"
)

func Handler(state *health.State, freshness time.Duration) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { writeHealth(w, state.Snapshot(), freshness, true) })
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) { writeHealth(w, state.Snapshot(), freshness, false) })
	return mux
}

// FullHandler mounts the probes, the remote MCP endpoint and the control panel
// on one server. The MCP endpoint authenticates every request on its own and
// shares nothing with the panel's session cookie.
func FullHandler(state *health.State, freshness time.Duration, web, remoteMCP, media http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { writeHealth(w, state.Snapshot(), freshness, true) })
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) { writeHealth(w, state.Snapshot(), freshness, false) })
	if remoteMCP != nil {
		mux.Handle("/mcp", remoteMCP)
	}
	// Media links authorise themselves with the token in the path, so they sit
	// beside /mcp rather than behind the panel session or the bearer key.
	if media != nil {
		mux.Handle("/media/", media)
	}
	mux.Handle("/", web)
	return mux
}

// writeHealth reports the session state and the per-queue consumption, so a
// probe failure can be diagnosed from the response alone. The probes answer
// anyone who can reach the host, so the account's identity (number, push name),
// its message activity times and raw queue errors stay out: the panel and
// whatsapp_status show those behind a credential.
func writeHealth(w http.ResponseWriter, snapshot health.Snapshot, freshness time.Duration, live bool) {
	stale := snapshot.Stale(freshness)
	queues := make([]health.Queue, len(snapshot.Queues))
	for i, queue := range snapshot.Queues {
		queue.LastError = ""
		queues[i] = queue
	}
	body := map[string]any{
		// The version is here so that "which version is that box running" has an
		// answer a monitor can read, without a session on the panel.
		"version":             version.String(),
		"evolution_connected": snapshot.EvolutionConnected,
		"last_event_at":       snapshot.LastEventAt,
		"rabbit_connected":    snapshot.RabbitConnected,
		"database_connected":  snapshot.DatabaseConnected,
		"whatsapp":            map[string]any{"state": snapshot.WhatsApp.State, "changed_at": snapshot.WhatsApp.ChangedAt},
		"queues":              queues,
		"stale":               stale,
	}
	if problems := snapshot.Problems(); len(problems) > 0 {
		body["problems"] = problems
	}
	if stale {
		body["warning"] = health.FreshnessWarning
	}
	status := http.StatusOK
	if !live && !snapshot.Ready(freshness) {
		status = http.StatusServiceUnavailable
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
