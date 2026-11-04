package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
)

func (a *App) publishUpdate() {
	a.liveMu.Lock()
	defer a.liveMu.Unlock()
	for client := range a.liveClients {
		select {
		case client <- struct{}{}:
		default:
		}
	}
}

func (a *App) liveEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming is not supported", http.StatusInternalServerError)
		return
	}
	if err := http.NewResponseController(w).SetWriteDeadline(time.Time{}); err != nil && !errors.Is(err, http.ErrNotSupported) {
		http.Error(w, "failed to configure event stream", http.StatusInternalServerError)
		return
	}

	updates := make(chan struct{}, 1)
	a.liveMu.Lock()
	a.liveClients[updates] = struct{}{}
	a.liveMu.Unlock()
	defer func() {
		a.liveMu.Lock()
		delete(a.liveClients, updates)
		a.liveMu.Unlock()
	}()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache, no-store")
	w.Header().Set("X-Accel-Buffering", "no")
	fmt.Fprint(w, "retry: 3000\n: connected\n\n")
	if err := writeMonitoringEvent(w, a.monitoringSnapshot()); err != nil {
		return
	}
	flusher.Flush()

	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-updates:
			if err := writeMonitoringEvent(w, a.monitoringSnapshot()); err != nil {
				return
			}
			flusher.Flush()
		case <-heartbeat.C:
			if !a.uiGateAuthorized(r) || !a.authorized(r) {
				fmt.Fprint(w, "event: auth-expired\ndata: {}\n\n")
				flusher.Flush()
				return
			}
			if _, err := fmt.Fprint(w, ": keep-alive\n\n"); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func writeMonitoringEvent(w http.ResponseWriter, snapshot map[string]any) error {
	data, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "event: monitoring\ndata: %s\n\n", data)
	return err
}
