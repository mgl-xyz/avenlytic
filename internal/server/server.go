package server

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/avenlytic/avenlytic/internal/core/model"
	"github.com/avenlytic/avenlytic/internal/notify"
	"github.com/avenlytic/avenlytic/internal/rules"
	"github.com/avenlytic/avenlytic/internal/storage"
)

type App struct {
	token        string
	store        storage.Repository
	registry     *notify.Registry
	mu           sync.Mutex
	pending      map[string]time.Time
	active       map[string]string
	offlineAfter time.Duration
	configMu     sync.RWMutex
	settingsMu   sync.Mutex
	authMu       sync.Mutex
	sessions     map[string]sessionRecord
	gateMu       sync.Mutex
	gateSessions map[string]time.Time
	liveMu       sync.Mutex
	liveClients  map[chan struct{}]struct{}
	hostOnline   map[string]bool
	scanMu       sync.Mutex
}

func New(token string) *App { return NewAt(token, "data/avenlytic.db") }
func NewAt(token, path string) *App {
	s, err := storage.Open(path)
	if err != nil {
		panic("open state store: " + err.Error())
	}
	return NewWithStore(token, s)
}
func NewWithStore(token string, s storage.Repository) *App {
	a := &App{token: token, store: s, registry: notify.NewRegistry(), pending: map[string]time.Time{}, active: map[string]string{}, offlineAfter: 45 * time.Second, sessions: map[string]sessionRecord{}, gateSessions: map[string]time.Time{}, liveClients: map[chan struct{}]struct{}{}, hostOnline: map[string]bool{}}
	s.View(func(st *storage.State) {
		if st.Settings.AgentToken != "" {
			a.token = st.Settings.AgentToken
		}
		if st.Settings.OfflineAfterSeconds >= 15 {
			a.offlineAfter = time.Duration(st.Settings.OfflineAfterSeconds) * time.Second
		}
		for _, e := range st.Events {
			if e.Status == "firing" || e.Status == "acknowledged" {
				a.active[e.RuleID+":"+e.HostID] = e.ID
			}
		}
		for id, snapshot := range st.Hosts {
			a.hostOnline[id] = time.Since(snapshot.Collected) <= a.offlineAfter
		}
	})
	return a
}
func (a *App) Close() error {
	if closer, ok := a.store.(interface{ Close() error }); ok {
		return closer.Close()
	}
	return nil
}
func (a *App) Handler() http.Handler {
	m := http.NewServeMux()
	m.HandleFunc("GET /healthz", a.Health)
	m.HandleFunc("GET /api/v1/auth/status", a.authStatus)
	m.HandleFunc("GET /api/v1/ui-gate/entry", a.uiGateEntry)
	m.HandleFunc("GET /api/v1/events", a.liveEvents)
	m.HandleFunc("GET /api/v1/settings", a.GetSettings)
	m.HandleFunc("POST /api/v1/settings", a.SaveSettings)
	m.HandleFunc("POST /api/v1/auth/setup", a.setupAdmin)
	m.HandleFunc("POST /api/v1/auth/login", a.login)
	m.HandleFunc("POST /api/v1/auth/token", a.issueJWT)
	m.HandleFunc("POST /api/v1/auth/logout", a.logout)
	m.HandleFunc("GET /api/v1/auth/me", a.me)
	m.HandleFunc("POST /api/v1/auth/password", a.changePassword)
	m.HandleFunc("POST /api/v1/ingest", a.Ingest)
	m.HandleFunc("POST /api/v1/ingest/security", a.IngestSecurity)
	m.HandleFunc("GET /api/v1/hosts", a.Hosts)
	m.HandleFunc("GET /api/v1/summary", a.Summary)
	m.HandleFunc("GET /api/v1/rules", a.GetRules)
	m.HandleFunc("POST /api/v1/rules", a.SaveRule)
	m.HandleFunc("DELETE /api/v1/rules/{id}", a.DeleteRule)
	m.HandleFunc("GET /api/v1/channels", a.GetChannels)
	m.HandleFunc("GET /api/v1/channels/{id}", a.GetChannel)
	m.HandleFunc("POST /api/v1/channels", a.SaveChannel)
	m.HandleFunc("POST /api/v1/channels/test", a.TestChannel)
	m.HandleFunc("POST /api/v1/channels/{id}/test", a.TestSavedChannel)
	m.HandleFunc("DELETE /api/v1/channels/{id}", a.DeleteChannel)
	m.HandleFunc("GET /api/v1/alerts", a.Alerts)
	m.HandleFunc("GET /api/v1/deliveries", a.Deliveries)
	m.HandleFunc("POST /api/v1/alerts/{id}/ack", a.AckAlert)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") && r.URL.Path != "/api/v1/ui-gate/entry" && r.URL.Path != "/api/v1/ingest" && r.URL.Path != "/api/v1/ingest/security" && !a.uiGateAuthorized(r) {
			http.NotFound(w, r)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/") && !strings.HasPrefix(r.URL.Path, "/api/v1/auth/") && r.URL.Path != "/api/v1/ui-gate/entry" && r.URL.Path != "/api/v1/ingest" && r.URL.Path != "/api/v1/ingest/security" && !a.authorized(r) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/") && r.Body != nil && r.Method != http.MethodGet {
			r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		}
		m.ServeHTTP(w, r)
	})
}
func (a *App) authorized(r *http.Request) bool {
	if a.hasSession(r) {
		return true
	}
	_, ok := a.bearerJWT(r)
	return ok
}

func (a *App) authorizedAgent(r *http.Request) bool {
	a.authMu.Lock()
	configuredToken := a.token
	a.authMu.Unlock()
	if configuredToken == "" {
		return false
	}
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		return false
	}
	got := strings.TrimPrefix(header, "Bearer ")
	if len(got) != len(configuredToken) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(configuredToken)) == 1
}
func (a *App) GetSettings(w http.ResponseWriter, r *http.Request) {
	var s storage.ServerSettings
	a.store.View(func(st *storage.State) { s = st.Settings })
	gateEnabled := s.UIGateHash != ""
	s.AgentToken = ""
	s.UIGateHash = ""
	a.authMu.Lock()
	s.AgentTokenConfigured = a.token != ""
	a.authMu.Unlock()
	if s.OfflineAfterSeconds == 0 {
		s.OfflineAfterSeconds = 45
	}
	safe := struct {
		OfflineAfterSeconds  int  `json:"offline_after_seconds"`
		AgentTokenConfigured bool `json:"agent_token_configured"`
		UIGateEnabled        bool `json:"ui_gate_enabled"`
	}{s.OfflineAfterSeconds, s.AgentTokenConfigured, gateEnabled}
	writeJSON(w, 200, safe)
}
func (a *App) SaveSettings(w http.ResponseWriter, r *http.Request) {
	var in struct {
		AgentToken          string `json:"agent_token"`
		OfflineAfterSeconds int    `json:"offline_after_seconds"`
		UIGateEnabled       *bool  `json:"ui_gate_enabled"`
		UIGatePath          string `json:"ui_gate_path"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil {
		http.Error(w, "invalid settings", 400)
		return
	}
	a.settingsMu.Lock()
	defer a.settingsMu.Unlock()
	if in.OfflineAfterSeconds < 15 || in.OfflineAfterSeconds > 3600 {
		http.Error(w, "offline timeout must be 15-3600 seconds", 400)
		return
	}
	var currentGateHash string
	a.store.View(func(st *storage.State) { currentGateHash = st.Settings.UIGateHash })
	gateHash := currentGateHash
	gateEnabled := currentGateHash != ""
	if in.UIGateEnabled != nil && !*in.UIGateEnabled {
		gateEnabled = false
		gateHash = ""
	} else if in.UIGateEnabled != nil {
		gateEnabled = *in.UIGateEnabled
	}
	if in.UIGatePath != "" {
		if !validUIGatePath(in.UIGatePath) {
			http.Error(w, "security link must be 3-128 URL-safe characters", 400)
			return
		}
		gateHash = hashUIGatePath(in.UIGatePath)
		gateEnabled = true
	}
	if gateEnabled && gateHash == "" {
		http.Error(w, "security link is required", 400)
		return
	}
	if !gateEnabled {
		gateHash = ""
	}
	if in.AgentToken != "" && len(in.AgentToken) < 24 {
		http.Error(w, "agent token must contain at least 24 characters", 400)
		return
	}
	if err := a.store.Update(func(st *storage.State) {
		st.Settings.OfflineAfterSeconds = in.OfflineAfterSeconds
		st.Settings.UIGateHash = gateHash
		if in.AgentToken != "" {
			st.Settings.AgentTokenConfigured = true
			st.Settings.AgentToken = in.AgentToken
		}
	}); err != nil {
		http.Error(w, "save failed", 500)
		return
	}
	gateChanged := gateHash != currentGateHash
	if gateChanged {
		a.gateMu.Lock()
		a.gateSessions = map[string]time.Time{}
		a.gateMu.Unlock()
	}
	if !gateEnabled && gateChanged {
		clearUIGateCookie(w, r)
	}
	if in.AgentToken != "" {
		a.authMu.Lock()
		a.token = in.AgentToken
		a.authMu.Unlock()
	}
	a.configMu.Lock()
	a.offlineAfter = time.Duration(in.OfflineAfterSeconds) * time.Second
	timeout := a.offlineAfter
	a.configMu.Unlock()
	a.authMu.Lock()
	configured := a.token != ""
	a.authMu.Unlock()
	a.publishUpdate()
	writeJSON(w, 200, map[string]any{"ok": true, "agent_token_configured": configured, "offline_after_seconds": timeout / time.Second, "ui_gate_enabled": gateEnabled})
}

func (a *App) offlineTimeout() time.Duration {
	a.configMu.RLock()
	defer a.configMu.RUnlock()
	return a.offlineAfter
}
func (a *App) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{"status": "ok", "protocol": model.ProtocolVersion, "version": "0.4.1"})
}
func (a *App) Ingest(w http.ResponseWriter, r *http.Request) {
	if !a.authorizedAgent(r) {
		http.Error(w, "unauthorized", 401)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	defer r.Body.Close()
	var s model.Snapshot
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if e := d.Decode(&s); e != nil {
		http.Error(w, "invalid snapshot: "+e.Error(), 400)
		return
	}
	if s.Protocol != model.ProtocolVersion || strings.TrimSpace(s.AgentID) == "" || strings.TrimSpace(s.Hostname) == "" {
		http.Error(w, "invalid protocol or identity", 400)
		return
	}
	if s.Collected.After(time.Now().Add(2 * time.Minute)) {
		http.Error(w, "timestamp too far in future", 400)
		return
	}
	if err := a.store.Update(func(st *storage.State) { st.Hosts[s.AgentID] = s }); err != nil {
		http.Error(w, "state persistence failed", http.StatusInternalServerError)
		return
	}
	a.mu.Lock()
	a.hostOnline[s.AgentID] = true
	a.mu.Unlock()
	a.evaluate(s)
	a.resolveOffline(s)
	a.publishUpdate()
	writeJSON(w, 202, map[string]string{"status": "accepted"})
}
func (a *App) IngestSecurity(w http.ResponseWriter, r *http.Request) {
	if !a.authorizedAgent(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	defer r.Body.Close()
	var report model.SecurityReport
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&report); err != nil {
		http.Error(w, "invalid security report: "+err.Error(), http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(report.AgentID) == "" {
		http.Error(w, "agent_id is required", http.StatusBadRequest)
		return
	}
	agentID := strings.TrimSpace(report.AgentID)
	now := time.Now()
	if report.Collected.IsZero() || report.Collected.After(now.Add(2*time.Minute)) {
		http.Error(w, "invalid report timestamp", http.StatusBadRequest)
		return
	}
	if report.FailedLoginAttempts < 0 || report.LoginWindowSeconds != 900 || report.AvailableUpdates < 0 {
		http.Error(w, "invalid security metric values", http.StatusBadRequest)
		return
	}
	if len(report.Packages) > 5000 {
		http.Error(w, "package inventory exceeds 5000 entries", http.StatusRequestEntityTooLarge)
		return
	}
	if report.PackageInventoryAvailable && (report.PackagesCollected.IsZero() || report.PackagesCollected.After(now.Add(2*time.Minute))) {
		http.Error(w, "invalid package inventory timestamp", http.StatusBadRequest)
		return
	}
	if !report.PackageInventoryAvailable && len(report.Packages) != 0 {
		http.Error(w, "package inventory cannot be included when unavailable", http.StatusBadRequest)
		return
	}
	for _, pkg := range report.Packages {
		if strings.TrimSpace(pkg.Name) == "" || strings.TrimSpace(pkg.Version) == "" || strings.TrimSpace(pkg.Ecosystem) == "" || len(pkg.Name) > 256 || len(pkg.Version) > 256 || len(pkg.Ecosystem) > 128 {
			http.Error(w, "invalid package inventory entry", http.StatusBadRequest)
			return
		}
	}

	var previous model.SecurityReport
	var snapshot model.Snapshot
	a.store.View(func(state *storage.State) {
		previous = state.Security[agentID]
		snapshot = state.Hosts[agentID]
	})
	if snapshot.AgentID == "" {
		http.Error(w, "agent must report host metrics before security data", http.StatusConflict)
		return
	}
	scanInventory := report.PackageInventoryAvailable &&
		!report.PackagesCollected.IsZero() &&
		report.PackagesCollected.After(previous.PackagesCollected)
	if !scanInventory {
		report.Packages = previous.Packages
		report.PackagesCollected = previous.PackagesCollected
		if report.PackageInventoryError == "" && !report.PackageInventoryAvailable {
			report.PackageInventoryAvailable = previous.PackageInventoryAvailable
			report.PackageInventoryError = previous.PackageInventoryError
		}
		report.Vulnerabilities = previous.Vulnerabilities
		report.VulnerabilityChecked = previous.VulnerabilityChecked
		report.VulnerabilityError = previous.VulnerabilityError
	} else {
		report.VulnerabilityError = "checking"
		report.Vulnerabilities = nil
		report.VulnerabilityChecked = time.Time{}
	}
	if !report.PackageInventoryAvailable {
		report.PackageInventoryError = boundedText(report.PackageInventoryError, 512)
	}
	report.UpdateCheckError = boundedText(report.UpdateCheckError, 512)
	if err := a.store.Update(func(state *storage.State) {
		if state.Security == nil {
			state.Security = map[string]model.SecurityReport{}
		}
		state.Security[agentID] = report
	}); err != nil {
		http.Error(w, "security report persistence failed", http.StatusInternalServerError)
		return
	}

	a.evaluateSecurity(snapshot, report)
	a.publishUpdate()
	if scanInventory {
		go a.scanVulnerabilities(agentID, report.PackagesCollected, report.Packages)
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "accepted"})
}
func (a *App) Hosts(w http.ResponseWriter, r *http.Request) {
	if !a.authorized(r) {
		http.Error(w, "unauthorized", 401)
		return
	}
	var list []map[string]any
	now := time.Now()
	a.store.View(func(st *storage.State) {
		for id, s := range st.Hosts {
			last := s.Collected
			status := "online"
			if now.Sub(last) > a.offlineTimeout() {
				status = "offline"
			}
			security := st.Security[id]
			list = append(list, map[string]any{"snapshot": s, "status": status, "last_seen": last, "agent_id": id, "security": securitySummary(security)})
		}
	})
	sort.Slice(list, func(i, j int) bool { return fmt.Sprint(list[i]["agent_id"]) < fmt.Sprint(list[j]["agent_id"]) })
	writeJSON(w, 200, map[string]any{"items": list, "count": len(list)})
}

func (a *App) monitoringSnapshot() map[string]any {
	var list []map[string]any
	now := time.Now()
	var snapshots []model.Snapshot
	online := 0
	activeAlerts := 0
	var events []storage.Event
	a.store.View(func(st *storage.State) {
		for id, s := range st.Hosts {
			snapshots = append(snapshots, s)
			last := s.Collected
			status := "online"
			if now.Sub(last) > a.offlineTimeout() {
				status = "offline"
			} else {
				online++
			}
			security := st.Security[id]
			list = append(list, map[string]any{"snapshot": s, "status": status, "last_seen": last, "agent_id": id, "security": securitySummary(security)})
		}
		for _, event := range st.Events {
			events = append(events, event)
			if event.Status == "firing" || event.Status == "acknowledged" {
				activeAlerts++
			}
		}
	})
	sort.Slice(list, func(i, j int) bool { return fmt.Sprint(list[i]["agent_id"]) < fmt.Sprint(list[j]["agent_id"]) })
	sort.Slice(events, func(i, j int) bool { return events[i].CreatedAt > events[j].CreatedAt })
	var cpu, mem float64
	nc, nm := 0, 0
	for _, h := range snapshots {
		if time.Since(h.Collected) > a.offlineTimeout() {
			continue
		}
		if h.CPU != nil {
			cpu += h.CPU.UsagePercent
			nc++
		}
		if h.Memory != nil {
			mem += h.Memory.UsagePercent
			nm++
		}
	}
	if nc > 0 {
		cpu /= float64(nc)
	}
	if nm > 0 {
		mem /= float64(nm)
	}
	return map[string]any{
		"hosts":   map[string]any{"items": list, "count": len(list)},
		"summary": map[string]any{"hosts": len(snapshots), "online": online, "offline": len(snapshots) - online, "alerts": activeAlerts, "avg_cpu": cpu, "avg_memory": mem},
		"alerts":  map[string]any{"items": events},
	}
}

func (a *App) Summary(w http.ResponseWriter, r *http.Request) {
	var hosts []model.Snapshot
	online := 0
	alerts := 0
	a.store.View(func(st *storage.State) {
		for _, h := range st.Hosts {
			hosts = append(hosts, h)
			if time.Since(h.Collected) <= a.offlineTimeout() {
				online++
			}
		}
		for _, e := range st.Events {
			if e.Status == "firing" || e.Status == "acknowledged" {
				alerts++
			}
		}
	})
	var cpu, mem float64
	nc, nm := 0, 0
	for _, h := range hosts {
		if time.Since(h.Collected) > a.offlineTimeout() {
			continue
		}
		if h.CPU != nil {
			cpu += h.CPU.UsagePercent
			nc++
		}
		if h.Memory != nil {
			mem += h.Memory.UsagePercent
			nm++
		}
	}
	if nc > 0 {
		cpu /= float64(nc)
	}
	if nm > 0 {
		mem /= float64(nm)
	}
	writeJSON(w, 200, map[string]any{"hosts": len(hosts), "online": online, "offline": len(hosts) - online, "alerts": alerts, "avg_cpu": cpu, "avg_memory": mem})
}
func (a *App) GetRules(w http.ResponseWriter, r *http.Request) {
	var v []rules.Rule
	a.store.View(func(s *storage.State) { v = append(v, s.Rules...) })
	writeJSON(w, 200, map[string]any{"items": v})
}
func (a *App) SaveRule(w http.ResponseWriter, r *http.Request) {
	if !a.authorized(r) {
		http.Error(w, "unauthorized", 401)
		return
	}
	var v rules.Rule
	if e := json.NewDecoder(r.Body).Decode(&v); e != nil {
		http.Error(w, e.Error(), 400)
		return
	}
	if v.ID == "" {
		v.ID = randomID()
	}
	if v.Severity == "" {
		v.Severity = "warning"
	}
	if e := rules.Validate(v); e != nil {
		http.Error(w, e.Error(), 400)
		return
	}
	var channelIDs = map[string]bool{}
	a.store.View(func(st *storage.State) {
		for _, c := range st.Channels {
			channelIDs[c.ID] = true
		}
	})
	for _, id := range v.ChannelIDs {
		if !channelIDs[id] {
			http.Error(w, "unknown notification channel: "+id, 400)
			return
		}
	}
	if err := a.store.Update(func(s *storage.State) {
		for i, x := range s.Rules {
			if x.ID == v.ID {
				s.Rules[i] = v
				return
			}
		}
		s.Rules = append(s.Rules, v)
	}); err != nil {
		http.Error(w, "state persistence failed", 500)
		return
	}
	a.publishUpdate()
	writeJSON(w, 200, v)
}
func (a *App) DeleteRule(w http.ResponseWriter, r *http.Request) {
	if !a.authorized(r) {
		http.Error(w, "unauthorized", 401)
		return
	}
	id := r.PathValue("id")
	_ = a.store.Update(func(s *storage.State) {
		out := s.Rules[:0]
		for _, v := range s.Rules {
			if v.ID != id {
				out = append(out, v)
			}
		}
		s.Rules = out
	})
	a.publishUpdate()
	writeJSON(w, 200, map[string]bool{"ok": true})
}
func (a *App) GetChannels(w http.ResponseWriter, r *http.Request) {
	var v []notify.Config
	a.store.View(func(s *storage.State) {
		for _, c := range s.Channels {
			x := c
			x.Settings = visibleChannelSettings(c.Settings)
			v = append(v, x)
		}
	})
	writeJSON(w, 200, map[string]any{"items": v})
}

func (a *App) GetChannel(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var channel *notify.Config
	a.store.View(func(s *storage.State) {
		for _, c := range s.Channels {
			if c.ID == id {
				x := c
				x.Settings = visibleChannelSettings(c.Settings)
				channel = &x
				break
			}
		}
	})
	if channel == nil {
		http.NotFound(w, r)
		return
	}
	writeJSON(w, http.StatusOK, channel)
}

func visibleChannelSettings(settings map[string]string) map[string]string {
	visible := make(map[string]string, len(settings))
	for key, value := range settings {
		normalized := strings.ToLower(key)
		if strings.Contains(normalized, "password") ||
			strings.Contains(normalized, "secret") ||
			strings.Contains(normalized, "token") ||
			strings.Contains(normalized, "authorization") {
			continue
		}
		visible[key] = value
	}
	return visible
}

func (a *App) SaveChannel(w http.ResponseWriter, r *http.Request) {
	if !a.authorized(r) {
		http.Error(w, "unauthorized", 401)
		return
	}
	var v notify.Config
	if err := json.NewDecoder(r.Body).Decode(&v); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	if v.ID == "" {
		v.ID = randomID()
	}
	if v.Name == "" || v.Type == "" {
		http.Error(w, "name and type required", 400)
		return
	}
	switch v.Type {
	case "email", "dingtalk", "webhook", "feishu", "wecom", "slack", "telegram":
	default:
		http.Error(w, "unsupported channel type", 400)
		return
	}
	var old *notify.Config
	a.store.View(func(st *storage.State) {
		for _, c := range st.Channels {
			if c.ID == v.ID {
				x := c
				old = &x
				break
			}
		}
	})
	if old != nil {
		if v.Type != old.Type {
			http.Error(w, "channel type cannot be changed; create a new channel", 400)
			return
		}
		if v.Settings == nil {
			v.Settings = map[string]string{}
		}
		for k, oldValue := range old.Settings {
			if strings.TrimSpace(v.Settings[k]) == "" {
				v.Settings[k] = oldValue
			}
		}
	}
	if err := validateChannel(v); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	_ = a.store.Update(func(st *storage.State) {
		updated := false
		for i, c := range st.Channels {
			if c.ID == v.ID {
				st.Channels[i] = v
				updated = true
				break
			}
		}
		if !updated {
			st.Channels = append(st.Channels, v)
		}
	})
	a.publishUpdate()
	v.Settings = nil
	writeJSON(w, 200, v)
}
func validateChannel(c notify.Config) error {
	required := map[string][]string{"dingtalk": {"webhook_url"}, "feishu": {"webhook_url"}, "wecom": {"webhook_url"}, "slack": {"webhook_url"}, "webhook": {"url"}, "telegram": {"bot_token", "chat_id"}, "email": {"smtp_host", "smtp_port", "from", "to"}}
	for _, key := range required[c.Type] {
		if strings.TrimSpace(c.Settings[key]) == "" {
			return fmt.Errorf("%s is required", key)
		}
	}
	return nil
}
func (a *App) TestChannel(w http.ResponseWriter, r *http.Request) {
	if !a.authorized(r) {
		http.Error(w, "unauthorized", 401)
		return
	}
	var v notify.Config
	if e := json.NewDecoder(r.Body).Decode(&v); e != nil {
		http.Error(w, e.Error(), 400)
		return
	}
	e := a.registry.Send(r.Context(), v, notify.Message{Title: "Avenlytic test notification", Text: "Notification channel test succeeded.", Severity: "info", Status: "test", At: time.Now()})
	if e != nil {
		http.Error(w, e.Error(), 502)
		return
	}
	writeJSON(w, 200, map[string]bool{"sent": true})
}
func (a *App) TestSavedChannel(w http.ResponseWriter, r *http.Request) {
	if !a.authorized(r) {
		http.Error(w, "unauthorized", 401)
		return
	}
	id := r.PathValue("id")
	var cfg notify.Config
	a.store.View(func(st *storage.State) {
		for _, c := range st.Channels {
			if c.ID == id {
				cfg = c
				break
			}
		}
	})
	if cfg.ID == "" {
		http.Error(w, "channel not found", 404)
		return
	}
	if err := a.registry.Send(r.Context(), cfg, notify.Message{Title: "Avenlytic test notification", Text: "Notification channel test succeeded.", Severity: "info", Status: "test", At: time.Now()}); err != nil {
		http.Error(w, err.Error(), 502)
		return
	}
	writeJSON(w, 200, map[string]bool{"sent": true})
}
func (a *App) DeleteChannel(w http.ResponseWriter, r *http.Request) {
	if !a.authorized(r) {
		http.Error(w, "unauthorized", 401)
		return
	}
	id := r.PathValue("id")
	_ = a.store.Update(func(s *storage.State) {
		out := s.Channels[:0]
		for _, v := range s.Channels {
			if v.ID != id {
				out = append(out, v)
			}
		}
		s.Channels = out
	})
	a.publishUpdate()
	writeJSON(w, 200, map[string]bool{"ok": true})
}
func (a *App) Alerts(w http.ResponseWriter, r *http.Request) {
	var v []storage.Event
	a.store.View(func(s *storage.State) { v = append(v, s.Events...) })
	sort.Slice(v, func(i, j int) bool { return v[i].CreatedAt > v[j].CreatedAt })
	writeJSON(w, 200, map[string]any{"items": v})
}
func (a *App) Deliveries(w http.ResponseWriter, r *http.Request) {
	var v []storage.Delivery
	a.store.View(func(s *storage.State) { v = append(v, s.Deliveries...) })
	sort.Slice(v, func(i, j int) bool { return v[i].CreatedAt > v[j].CreatedAt })
	writeJSON(w, 200, map[string]any{"items": v})
}
func (a *App) AckAlert(w http.ResponseWriter, r *http.Request) {
	if !a.authorized(r) {
		http.Error(w, "unauthorized", 401)
		return
	}
	id := r.PathValue("id")
	_ = a.store.Update(func(s *storage.State) {
		for i := range s.Events {
			if s.Events[i].ID == id {
				s.Events[i].Status = "acknowledged"
			}
		}
	})
	a.publishUpdate()
	writeJSON(w, 200, map[string]bool{"ok": true})
}
func (a *App) evaluate(s model.Snapshot) {
	var security model.SecurityReport
	a.store.View(func(state *storage.State) { security = state.Security[s.AgentID] })
	a.evaluateSecurity(s, security)
}
func (a *App) evaluateSecurity(s model.Snapshot, security model.SecurityReport) {
	a.store.View(func(st *storage.State) {
		for _, rule := range st.Rules {
			if !rule.Enabled || rule.Metric == "agent.online" {
				continue
			}
			v := rules.EvaluateWithSecurity(rule, s, security)
			if !v.Supported {
				continue
			}
			key := rule.ID + ":" + s.AgentID
			a.mu.Lock()
			since, exists := a.pending[key]
			if rules.Match(rule.Operator, v.Value, rule.Threshold) {
				if !exists {
					a.pending[key] = time.Now()
					since = time.Now()
				}
				ready := time.Since(since) >= time.Duration(rule.DurationSeconds)*time.Second
				_, firing := a.active[key]
				if ready && !firing {
					message := fmt.Sprintf("%s is %.2f (threshold %s %.2f)", rule.Metric, v.Value, rule.Operator, rule.Threshold)
					event := storage.Event{ID: randomID(), RuleID: rule.ID, RuleName: rule.Name, HostID: s.AgentID, Host: s.Hostname, Severity: rule.Severity, Status: "firing", Message: renderRuleMessage(rule, s.Hostname, v.Value, message), Value: v.Value, Threshold: rule.Threshold, CreatedAt: time.Now().UTC().Format(time.RFC3339)}
					a.active[key] = event.ID
					channels := append([]notify.Config(nil), st.Channels...)
					go a.persistAndNotify(event, rule, channels)
				}
			} else {
				delete(a.pending, key)
				if eventID, ok := a.active[key]; ok {
					delete(a.active, key)
					channels := append([]notify.Config(nil), st.Channels...)
					go a.recoverEvent(eventID, rule, s, channels, rule.NotifyOnRecovery)
				}
			}
			a.mu.Unlock()
		}
	})
}
func (a *App) scanVulnerabilities(agentID string, collected time.Time, packages []model.InstalledPackage) {
	a.scanMu.Lock()
	defer a.scanMu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	vulnerabilities, err := lookupOSV(ctx, packages)
	if err != nil {
		err = fmt.Errorf("public vulnerability lookup failed: %w", err)
	}
	var report model.SecurityReport
	if updateErr := a.store.Update(func(state *storage.State) {
		report = state.Security[agentID]
		if !report.PackagesCollected.Equal(collected) {
			return
		}
		if err != nil {
			report.VulnerabilityError = err.Error()
			report.Vulnerabilities = nil
		} else {
			report.VulnerabilityError = ""
			report.Vulnerabilities = vulnerabilities
			report.VulnerabilityChecked = time.Now().UTC()
		}
		state.Security[agentID] = report
	}); updateErr != nil {
		log.Printf("persist OSV results for agent %q: %v", agentID, updateErr)
		return
	}
	var snapshot model.Snapshot
	a.store.View(func(state *storage.State) { snapshot = state.Hosts[agentID] })
	if snapshot.AgentID != "" {
		a.evaluateSecurity(snapshot, report)
	}
	a.publishUpdate()
}

func securitySummary(report model.SecurityReport) map[string]any {
	return map[string]any{
		"collected_at":                report.Collected,
		"failed_login_attempts":       report.FailedLoginAttempts,
		"login_window_seconds":        report.LoginWindowSeconds,
		"login_monitoring_available":  report.LoginMonitoringAvailable,
		"available_updates":           report.AvailableUpdates,
		"update_check_available":      report.UpdateCheckAvailable,
		"update_check_error":          report.UpdateCheckError,
		"package_inventory_available": report.PackageInventoryAvailable,
		"package_inventory_error":     report.PackageInventoryError,
		"vulnerabilities":             report.Vulnerabilities,
		"vulnerability_count":         len(report.Vulnerabilities),
		"vulnerability_checked_at":    report.VulnerabilityChecked,
		"vulnerability_error":         report.VulnerabilityError,
	}
}

func boundedText(value string, maxLength int) string {
	if len(value) > maxLength {
		return value[:maxLength]
	}
	return value
}
func (a *App) persistAndNotify(e storage.Event, r rules.Rule, channels []notify.Config) {
	_ = a.store.Update(func(s *storage.State) {
		s.Events = append(s.Events, e)
		if len(s.Events) > 2000 {
			s.Events = s.Events[len(s.Events)-2000:]
		}
	})
	a.publishUpdate()
	a.dispatch(e, r, channels)
}
func renderRuleMessage(rule rules.Rule, host string, value float64, fallback string) string {
	if strings.TrimSpace(rule.MessageTemplate) == "" {
		return fallback
	}
	valueText := strconv.FormatFloat(value, 'f', 2, 64)
	thresholdText := strconv.FormatFloat(rule.Threshold, 'f', 2, 64)
	return strings.NewReplacer(
		"{host}", host,
		"{metric}", rule.Metric,
		"{value}", valueText,
		"{operator}", rule.Operator,
		"{threshold}", thresholdText,
	).Replace(rule.MessageTemplate)
}
func (a *App) recoverEvent(id string, r rules.Rule, s model.Snapshot, channels []notify.Config, send bool) {
	e := storage.Event{ID: randomID(), RuleID: r.ID, RuleName: r.Name, HostID: s.AgentID, Host: s.Hostname, Severity: r.Severity, Status: "resolved", Message: "Condition returned to normal", CreatedAt: time.Now().UTC().Format(time.RFC3339)}
	_ = a.store.Update(func(st *storage.State) {
		for i := range st.Events {
			if st.Events[i].ID == id {
				st.Events[i].Status = "resolved"
				st.Events[i].ResolvedAt = e.CreatedAt
			}
		}
		st.Events = append(st.Events, e)
	})
	a.publishUpdate()
	if send {
		a.dispatch(e, r, channels)
	}
}
func (a *App) dispatch(e storage.Event, r rules.Rule, channels []notify.Config) {
	for _, c := range channels {
		if !c.Enabled || !contains(r.ChannelIDs, c.ID) {
			continue
		}
		delivery := storage.Delivery{ID: randomID(), EventID: e.ID, ChannelID: c.ID, ChannelName: c.Name, ChannelType: c.Type, Status: "failed", CreatedAt: time.Now().UTC().Format(time.RFC3339)}
		var err error
		for attempt := 1; attempt <= 3; attempt++ {
			delivery.Attempts = attempt
			ctx, cancel := timeLimit()
			err = a.registry.Send(ctx, c, notify.Message{Title: "[" + strings.ToUpper(e.Severity) + "] " + e.RuleName, Text: e.Message, Severity: e.Severity, EventID: e.ID, Status: e.Status, Host: e.Host, Rule: e.RuleName, At: time.Now()})
			cancel()
			if err == nil {
				delivery.Status = "sent"
				break
			}
			delivery.Error = redactError(err.Error(), c)
			if attempt < 3 {
				time.Sleep(time.Duration(attempt) * time.Second)
			}
		}
		_ = a.store.Update(func(st *storage.State) {
			st.Deliveries = append(st.Deliveries, delivery)
			if len(st.Deliveries) > 2000 {
				st.Deliveries = st.Deliveries[len(st.Deliveries)-2000:]
			}
		})
		a.publishUpdate()
	}
}
func (a *App) CheckOffline() {
	now := time.Now()
	var hosts []model.Snapshot
	var ruleset []rules.Rule
	var channels []notify.Config
	a.store.View(func(s *storage.State) {
		for _, h := range s.Hosts {
			hosts = append(hosts, h)
		}
		ruleset = append(ruleset, s.Rules...)
		channels = append(channels, s.Channels...)
	})
	statusChanged := false
	a.mu.Lock()
	for _, host := range hosts {
		online := now.Sub(host.Collected) <= a.offlineTimeout()
		if previous, known := a.hostOnline[host.AgentID]; known && previous != online {
			statusChanged = true
		}
		a.hostOnline[host.AgentID] = online
	}
	a.mu.Unlock()
	if statusChanged {
		a.publishUpdate()
	}
	for _, h := range hosts {
		if now.Sub(h.Collected) <= a.offlineTimeout() {
			continue
		}
		for _, r := range ruleset {
			if !r.Enabled || r.Metric != "agent.online" {
				continue
			}
			key := r.ID + ":" + h.AgentID
			a.mu.Lock()
			since, pending := a.pending[key]
			if !pending {
				a.pending[key] = now
				since = now
			}
			_, firing := a.active[key]
			if !firing && now.Sub(since) >= time.Duration(r.DurationSeconds)*time.Second {
				fallback := "Agent heartbeat timeout; last seen " + h.Collected.Format(time.RFC3339)
				e := storage.Event{ID: randomID(), RuleID: r.ID, RuleName: r.Name, HostID: h.AgentID, Host: h.Hostname, Severity: r.Severity, Status: "firing", Message: renderRuleMessage(r, h.Hostname, 0, fallback), CreatedAt: now.UTC().Format(time.RFC3339)}
				a.active[key] = e.ID
				ch := append([]notify.Config(nil), channels...)
				go a.persistAndNotify(e, r, ch)
			}
			a.mu.Unlock()
		}
	}
}
func (a *App) RunMonitor(ctxDone <-chan struct{}) {
	t := time.NewTicker(10 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctxDone:
			return
		case <-t.C:
			a.CheckOffline()
		}
	}
}
func (a *App) resolveOffline(s model.Snapshot) {
	var rs []rules.Rule
	var cs []notify.Config
	a.store.View(func(st *storage.State) { rs = append(rs, st.Rules...); cs = append(cs, st.Channels...) })
	for _, r := range rs {
		if r.Metric != "agent.online" {
			continue
		}
		key := r.ID + ":" + s.AgentID
		a.mu.Lock()
		eventID, ok := a.active[key]
		if ok {
			delete(a.active, key)
		}
		delete(a.pending, key)
		a.mu.Unlock()
		if !ok {
			continue
		}
		now := time.Now().UTC().Format(time.RFC3339)
		e := storage.Event{ID: randomID(), RuleID: r.ID, RuleName: r.Name, HostID: s.AgentID, Host: s.Hostname, Severity: r.Severity, Status: "resolved", Message: "Agent heartbeat restored", CreatedAt: now}
		_ = a.store.Update(func(st *storage.State) {
			for i := range st.Events {
				if st.Events[i].ID == eventID {
					st.Events[i].Status = "resolved"
					st.Events[i].ResolvedAt = now
				}
			}
			st.Events = append(st.Events, e)
		})
		if r.NotifyOnRecovery {
			go a.dispatch(e, r, append([]notify.Config(nil), cs...))
		}
	}
}
func redactError(message string, c notify.Config) string {
	for _, key := range []string{"webhook_url", "url", "bot_token", "password", "secret", "authorization", "username"} {
		if value := c.Settings[key]; value != "" {
			message = strings.ReplaceAll(message, value, "[redacted]")
		}
	}
	return message
}
func contains(a []string, s string) bool {
	for _, v := range a {
		if v == s {
			return true
		}
	}
	return false
}
func randomID() string { b := make([]byte, 8); _, _ = rand.Read(b); return hex.EncodeToString(b) }
func timeLimit() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 8*time.Second)
}
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
