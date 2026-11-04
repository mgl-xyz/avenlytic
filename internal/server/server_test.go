package server

import (
	"bytes"
	"encoding/json"
	"github.com/avenlytic/avenlytic/internal/core/model"
	"github.com/avenlytic/avenlytic/internal/notify"
	"github.com/avenlytic/avenlytic/internal/rules"
	"github.com/avenlytic/avenlytic/internal/storage"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func testApp(t *testing.T) *App {
	t.Helper()
	return NewAt("secret", filepath.Join(t.TempDir(), "state.json"))
}
func TestIngestAndList(t *testing.T) {
	a := testApp(t)
	s := model.Snapshot{Protocol: model.ProtocolVersion, AgentID: "host-1", Hostname: "node1", OS: "linux", Collected: time.Now().UTC()}
	b, _ := json.Marshal(s)
	req := httptest.NewRequest("POST", "/api/v1/ingest", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer secret")
	w := httptest.NewRecorder()
	a.Ingest(w, req)
	if w.Code != 202 {
		t.Fatalf("got %d: %s", w.Code, w.Body.String())
	}
	out := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/v1/hosts", nil)
	a.createSession(out, r, "admin")
	for _, c := range out.Result().Cookies() {
		r.AddCookie(c)
	}
	out = httptest.NewRecorder()
	a.Hosts(out, r)
	if out.Code != 200 {
		t.Fatal(out.Code)
	}
}
func TestUnauthorized(t *testing.T) {
	a := testApp(t)
	w := httptest.NewRecorder()
	a.Ingest(w, httptest.NewRequest("POST", "/api/v1/ingest", bytes.NewReader([]byte(`{}`))))
	if w.Code != 401 {
		t.Fatalf("got %d", w.Code)
	}
}

func TestGetChannelsReturnsVisibleSettingsOnly(t *testing.T) {
	a := testApp(t)
	want := notify.Config{
		ID:      "channel-1",
		Name:    "Email",
		Type:    "email",
		Enabled: true,
		Settings: map[string]string{
			"smtp_host":   "smtp.example.test",
			"username":    "alerts",
			"webhook_url": "https://hooks.example.test/secret-path",
			"password":    "keep-this-secret",
		},
	}
	if err := a.store.Update(func(state *storage.State) {
		state.Channels = []notify.Config{want}
	}); err != nil {
		t.Fatal(err)
	}

	response := httptest.NewRecorder()
	a.GetChannels(response, httptest.NewRequest(http.MethodGet, "/api/v1/channels", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("got %d: %s", response.Code, response.Body.String())
	}

	var result struct {
		Items []notify.Config `json:"items"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Items) != 1 {
		t.Fatalf("got %d channels, want 1", len(result.Items))
	}
	settings := result.Items[0].Settings
	if settings["smtp_host"] != "smtp.example.test" ||
		settings["username"] != "alerts" ||
		settings["webhook_url"] != "https://hooks.example.test/secret-path" {
		t.Fatalf("non-sensitive settings were not returned: %#v", settings)
	}
	if _, ok := settings["password"]; ok {
		t.Fatalf("sensitive settings were returned: %#v", settings)
	}
	if bytes.Contains(response.Body.Bytes(), []byte("keep-this-secret")) {
		t.Fatal("response contains a channel secret")
	}
}

func TestGetChannelReturnsDetailsForEditing(t *testing.T) {
	a := testApp(t)
	want := notify.Config{
		ID:      "channel-1",
		Name:    "Webhook",
		Type:    "webhook",
		Enabled: true,
		Settings: map[string]string{
			"url":           "https://hooks.example.test/secret-path",
			"authorization": "Bearer keep-this-secret",
		},
	}
	if err := a.store.Update(func(state *storage.State) {
		state.Channels = []notify.Config{want}
	}); err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(http.MethodGet, "/api/v1/channels/channel-1", nil)
	request.SetPathValue("id", "channel-1")
	response := httptest.NewRecorder()
	a.GetChannel(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("got %d: %s", response.Code, response.Body.String())
	}
	if !bytes.Contains(response.Body.Bytes(), []byte("https://hooks.example.test/secret-path")) {
		t.Fatal("channel URL was not returned")
	}
	var result notify.Config
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if _, ok := result.Settings["authorization"]; ok {
		t.Fatalf("sensitive settings were returned: %#v", result.Settings)
	}

	missing := httptest.NewRequest(http.MethodGet, "/api/v1/channels/missing", nil)
	missing.SetPathValue("id", "missing")
	missingResponse := httptest.NewRecorder()
	a.GetChannel(missingResponse, missing)
	if missingResponse.Code != http.StatusNotFound {
		t.Fatalf("missing channel got %d, want %d", missingResponse.Code, http.StatusNotFound)
	}
}

func TestIngestSecurityReport(t *testing.T) {
	a := testApp(t)
	snapshotBody, err := json.Marshal(model.Snapshot{
		Protocol:  model.ProtocolVersion,
		AgentID:   "host-security",
		Hostname:  "security-node",
		OS:        "linux",
		Collected: time.Now().UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}
	snapshotReq := httptest.NewRequest(http.MethodPost, "/api/v1/ingest", bytes.NewReader(snapshotBody))
	snapshotReq.Header.Set("Authorization", "Bearer secret")
	snapshotResponse := httptest.NewRecorder()
	a.Ingest(snapshotResponse, snapshotReq)
	if snapshotResponse.Code != http.StatusAccepted {
		t.Fatalf("host ingest got %d: %s", snapshotResponse.Code, snapshotResponse.Body.String())
	}

	report := model.SecurityReport{
		AgentID:                  "host-security",
		Collected:                time.Now().UTC(),
		FailedLoginAttempts:      5,
		LoginWindowSeconds:       900,
		LoginMonitoringAvailable: true,
		AvailableUpdates:         3,
		UpdateCheckAvailable:     true,
	}
	body, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ingest/security", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer secret")
	response := httptest.NewRecorder()
	a.Handler().ServeHTTP(response, req)
	if response.Code != http.StatusAccepted {
		t.Fatalf("got %d: %s", response.Code, response.Body.String())
	}
	a.store.View(func(state *storage.State) {
		got := state.Security[report.AgentID]
		if got.FailedLoginAttempts != 5 || got.AvailableUpdates != 3 {
			t.Errorf("unexpected stored report: %+v", got)
		}
	})
}
func TestRuleEvaluation(t *testing.T) {
	r := model.Snapshot{Memory: &model.Memory{UsagePercent: 91}}
	rule := struct {
		Metric, Operator string
		Threshold        float64
	}{"memory.usage_percent", ">", 90}
	_ = r
	_ = rule
}

func TestMemoryAlertDispatchesAndResolves(t *testing.T) {
	var received atomic.Int32
	notifyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { received.Add(1); w.WriteHeader(200) }))
	defer notifyServer.Close()
	a := testApp(t)
	ch := notify.Config{ID: "ch1", Name: "test webhook", Type: "webhook", Enabled: true, Settings: map[string]string{"url": notifyServer.URL}}
	rule := rules.Rule{ID: "r1", Name: "memory high", Enabled: true, Metric: "memory.usage_percent", MessageTemplate: "Alert {host}: {metric}={value} {operator} {threshold}", Operator: ">", Threshold: 80, DurationSeconds: 0, Severity: "warning", ChannelIDs: []string{"ch1"}, NotifyOnRecovery: true}
	if err := a.store.Update(func(s *storage.State) { s.Channels = append(s.Channels, ch); s.Rules = append(s.Rules, rule) }); err != nil {
		t.Fatal(err)
	}
	snap := model.Snapshot{Protocol: model.ProtocolVersion, AgentID: "host1", Hostname: "node1", Collected: time.Now(), Memory: &model.Memory{UsagePercent: 90}}
	a.evaluate(snap)
	waitFor(t, func() bool {
		ok := false
		a.store.View(func(s *storage.State) { ok = len(s.Events) >= 1 && len(s.Deliveries) >= 1 })
		return ok
	})
	foundFiringMessage := false
	a.store.View(func(s *storage.State) {
		for _, event := range s.Events {
			if event.Status == "firing" && event.RuleID == rule.ID {
				foundFiringMessage = true
				if event.Message != "Alert node1: memory.usage_percent=90.00 > 80.00" {
					t.Errorf("unexpected custom alert message: %q", event.Message)
				}
			}
		}
	})
	if !foundFiringMessage {
		t.Fatal("expected a firing event with the custom alert message")
	}
	snap.Memory.UsagePercent = 30
	a.evaluate(snap)
	waitFor(t, func() bool {
		resolved := false
		a.store.View(func(s *storage.State) {
			for _, e := range s.Events {
				if e.Status == "resolved" {
					resolved = true
				}
			}
		})
		return resolved
	})
	if received.Load() < 2 {
		t.Fatalf("expected firing and recovery notifications, got %d", received.Load())
	}
}
func TestOfflineRuleFiresAfterHeartbeatTimeout(t *testing.T) {
	a := testApp(t)
	a.offlineAfter = time.Second
	old := time.Now().Add(-2 * time.Second)
	_ = a.store.Update(func(s *storage.State) {
		s.Hosts["host1"] = model.Snapshot{AgentID: "host1", Hostname: "node1", Collected: old}
		s.Rules = append(s.Rules, rules.Rule{ID: "offline", Name: "agent down", Enabled: true, Metric: "agent.online", Operator: "==", Threshold: 0, DurationSeconds: 0, Severity: "critical"})
	})
	a.CheckOffline()
	waitFor(t, func() bool {
		found := false
		a.store.View(func(s *storage.State) {
			for _, e := range s.Events {
				if e.RuleID == "offline" && e.Status == "firing" {
					found = true
				}
			}
		})
		return found
	})
}
func waitFor(t *testing.T, fn func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if fn() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition did not become true before timeout")
}

func TestFirstRunSetupAndSessionLogin(t *testing.T) {
	a := testApp(t)
	statusRec := httptest.NewRecorder()
	a.Handler().ServeHTTP(statusRec, httptest.NewRequest("GET", "/api/v1/auth/status", nil))
	if statusRec.Code != http.StatusOK || statusRec.Body.String() != "{\"initialized\":false}\n" {
		t.Fatalf("unexpected uninitialized auth status: %d %s", statusRec.Code, statusRec.Body.String())
	}

	tooShort := httptest.NewRecorder()
	a.Handler().ServeHTTP(tooShort, httptest.NewRequest("POST", "/api/v1/auth/setup", bytes.NewReader([]byte(`{"username":"operator","password":"abcde"}`))))
	if tooShort.Code != http.StatusBadRequest {
		t.Fatalf("five-character password should be rejected, got %d", tooShort.Code)
	}

	setupBody := []byte(`{"username":"operator","password":"abcdef"}`)
	setupReq := httptest.NewRequest("POST", "/api/v1/auth/setup", bytes.NewReader(setupBody))
	setupRec := httptest.NewRecorder()
	a.Handler().ServeHTTP(setupRec, setupReq)
	if setupRec.Code != http.StatusCreated {
		t.Fatalf("setup: %d %s", setupRec.Code, setupRec.Body.String())
	}
	statusRec = httptest.NewRecorder()
	a.Handler().ServeHTTP(statusRec, httptest.NewRequest("GET", "/api/v1/auth/status", nil))
	if statusRec.Code != http.StatusOK || statusRec.Body.String() != "{\"initialized\":true}\n" {
		t.Fatalf("auth status exposed data or returned unexpected result: %d %s", statusRec.Code, statusRec.Body.String())
	}
	cookies := setupRec.Result().Cookies()
	if len(cookies) == 0 || cookies[0].Name != sessionCookie {
		t.Fatal("expected session cookie")
	}
	apiReq := httptest.NewRequest("GET", "/api/v1/summary", nil)
	apiReq.AddCookie(cookies[0])
	apiRec := httptest.NewRecorder()
	a.Handler().ServeHTTP(apiRec, apiReq)
	if apiRec.Code != http.StatusOK {
		t.Fatalf("authenticated summary: %d %s", apiRec.Code, apiRec.Body.String())
	}
	secondSetup := httptest.NewRecorder()
	a.Handler().ServeHTTP(secondSetup, httptest.NewRequest("POST", "/api/v1/auth/setup", bytes.NewReader(setupBody)))
	if secondSetup.Code != http.StatusConflict {
		t.Fatalf("second setup should conflict, got %d", secondSetup.Code)
	}
}

func TestJWTBearerAuthorizationAndRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	app := NewAt("", path)
	setup := httptest.NewRequest(http.MethodPost, "/api/v1/auth/setup", bytes.NewReader([]byte(`{"username":"operator","password":"correct-horse-battery-2026"}`)))
	setupResponse := httptest.NewRecorder()
	app.Handler().ServeHTTP(setupResponse, setup)
	if setupResponse.Code != http.StatusCreated {
		t.Fatalf("setup failed: %d %s", setupResponse.Code, setupResponse.Body.String())
	}

	tokenRequest := httptest.NewRequest(http.MethodPost, "/api/v1/auth/token", bytes.NewReader([]byte(`{"username":"operator","password":"correct-horse-battery-2026"}`)))
	tokenResponse := httptest.NewRecorder()
	app.Handler().ServeHTTP(tokenResponse, tokenRequest)
	if tokenResponse.Code != http.StatusOK {
		t.Fatalf("token request failed: %d %s", tokenResponse.Code, tokenResponse.Body.String())
	}
	var issued struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
	}
	if err := json.Unmarshal(tokenResponse.Body.Bytes(), &issued); err != nil {
		t.Fatal(err)
	}
	if issued.AccessToken == "" || issued.TokenType != "Bearer" {
		t.Fatalf("unexpected token response: %+v", issued)
	}

	restarted := NewAt("", path)
	apiRequest := httptest.NewRequest(http.MethodGet, "/api/v1/summary", nil)
	apiRequest.Header.Set("Authorization", "Bearer "+issued.AccessToken)
	apiResponse := httptest.NewRecorder()
	restarted.Handler().ServeHTTP(apiResponse, apiRequest)
	if apiResponse.Code != http.StatusOK {
		t.Fatalf("JWT did not authorize after restart: %d %s", apiResponse.Code, apiResponse.Body.String())
	}

	invalidRequest := httptest.NewRequest(http.MethodGet, "/api/v1/summary", nil)
	invalidRequest.Header.Set("Authorization", "Bearer x"+issued.AccessToken[1:])
	invalidResponse := httptest.NewRecorder()
	restarted.Handler().ServeHTTP(invalidResponse, invalidRequest)
	if invalidResponse.Code != http.StatusUnauthorized {
		t.Fatalf("invalid JWT should be rejected, got %d", invalidResponse.Code)
	}

	loginRequest := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader([]byte(`{"username":"operator","password":"correct-horse-battery-2026"}`)))
	loginResponse := httptest.NewRecorder()
	restarted.Handler().ServeHTTP(loginResponse, loginRequest)
	if loginResponse.Code != http.StatusOK || len(loginResponse.Result().Cookies()) == 0 {
		t.Fatalf("browser login failed before password change: %d", loginResponse.Code)
	}
	changeRequest := httptest.NewRequest(http.MethodPost, "/api/v1/auth/password", bytes.NewReader([]byte(`{"current":"correct-horse-battery-2026","password":"newpass"}`)))
	changeRequest.AddCookie(loginResponse.Result().Cookies()[0])
	changeResponse := httptest.NewRecorder()
	restarted.Handler().ServeHTTP(changeResponse, changeRequest)
	if changeResponse.Code != http.StatusOK {
		t.Fatalf("password change failed: %d %s", changeResponse.Code, changeResponse.Body.String())
	}
	revokedRequest := httptest.NewRequest(http.MethodGet, "/api/v1/summary", nil)
	revokedRequest.Header.Set("Authorization", "Bearer "+issued.AccessToken)
	revokedResponse := httptest.NewRecorder()
	restarted.Handler().ServeHTTP(revokedResponse, revokedRequest)
	if revokedResponse.Code != http.StatusUnauthorized {
		t.Fatalf("password change should revoke JWTs, got %d", revokedResponse.Code)
	}
}

func TestUIGateProtectsUIAndAdminAPI(t *testing.T) {
	a := testApp(t)
	link := "abc"
	sessionWriter := httptest.NewRecorder()
	sessionRequest := httptest.NewRequest(http.MethodPost, "/api/v1/settings", nil)
	a.createSession(sessionWriter, sessionRequest, "admin")
	sessionCookies := sessionWriter.Result().Cookies()

	body := []byte(`{"offline_after_seconds":45,"ui_gate_enabled":true,"ui_gate_path":"` + link + `"}`)
	saveRequest := httptest.NewRequest(http.MethodPost, "/api/v1/settings", bytes.NewReader(body))
	for _, cookie := range sessionCookies {
		saveRequest.AddCookie(cookie)
	}
	saveResponse := httptest.NewRecorder()
	a.Handler().ServeHTTP(saveResponse, saveRequest)
	if saveResponse.Code != http.StatusOK {
		t.Fatalf("enable gate failed: %d %s", saveResponse.Code, saveResponse.Body.String())
	}
	if bytes.Contains(saveResponse.Body.Bytes(), []byte(link)) || bytes.Contains(saveResponse.Body.Bytes(), []byte("ui_gate_hash")) {
		t.Fatal("settings response exposed the private link")
	}
	for _, cookie := range saveResponse.Result().Cookies() {
		if cookie.Name == uiGateCookie {
			t.Fatal("saving settings must not grant a UI gate session")
		}
	}
	saveCookieRequest := httptest.NewRequest(http.MethodGet, "/api/v1/auth/status", nil)
	for _, cookie := range saveResponse.Result().Cookies() {
		saveCookieRequest.AddCookie(cookie)
	}
	saveCookieResponse := httptest.NewRecorder()
	a.Handler().ServeHTTP(saveCookieResponse, saveCookieRequest)
	if saveCookieResponse.Code != http.StatusNotFound {
		t.Fatalf("settings response cookies must not bypass the UI gate, got %d", saveCookieResponse.Code)
	}
	denied := httptest.NewRecorder()
	a.Handler().ServeHTTP(denied, httptest.NewRequest(http.MethodGet, "/api/v1/auth/status", nil))
	if denied.Code != http.StatusNotFound {
		t.Fatalf("ungated API should return 404, got %d", denied.Code)
	}
	deniedStream := httptest.NewRecorder()
	a.Handler().ServeHTTP(deniedStream, httptest.NewRequest(http.MethodGet, "/api/v1/events", nil))
	if deniedStream.Code != http.StatusNotFound {
		t.Fatalf("ungated SSE endpoint should return 404, got %d", deniedStream.Code)
	}
	entryAPI := httptest.NewRequest(http.MethodGet, "/api/v1/ui-gate/entry?path=%2F"+link, nil)
	entryAPIResponse := httptest.NewRecorder()
	a.Handler().ServeHTTP(entryAPIResponse, entryAPI)
	if entryAPIResponse.Code != http.StatusFound || entryAPIResponse.Header().Get("Location") != "/" {
		t.Fatalf("dev-server gate endpoint should grant and redirect, got %d", entryAPIResponse.Code)
	}
	gateAPIRequest := httptest.NewRequest(http.MethodGet, "/api/v1/summary", nil)
	gateAPIRequest.AddCookie(entryAPIResponse.Result().Cookies()[0])
	gateAPIResponse := httptest.NewRecorder()
	a.Handler().ServeHTTP(gateAPIResponse, gateAPIRequest)
	if gateAPIResponse.Code != http.StatusUnauthorized {
		t.Fatalf("gate cookie should not replace admin authentication, got %d", gateAPIResponse.Code)
	}

	page := a.UIHandler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))
	root := httptest.NewRequest(http.MethodGet, "/", nil)
	rootResponse := httptest.NewRecorder()
	page.ServeHTTP(rootResponse, root)
	if rootResponse.Code != http.StatusNotFound {
		t.Fatalf("ungated UI should return 404, got %d", rootResponse.Code)
	}

	entry := httptest.NewRequest(http.MethodGet, "/"+link, nil)
	entryResponse := httptest.NewRecorder()
	page.ServeHTTP(entryResponse, entry)
	if entryResponse.Code != http.StatusFound || entryResponse.Header().Get("Location") != "/" {
		t.Fatalf("entry should redirect to root, got %d %q", entryResponse.Code, entryResponse.Header().Get("Location"))
	}
	entryCookie := entryResponse.Result().Cookies()[0]
	root.AddCookie(entryCookie)
	rootResponse = httptest.NewRecorder()
	page.ServeHTTP(rootResponse, root)
	if rootResponse.Code != http.StatusOK {
		t.Fatalf("gate cookie should allow UI access, got %d", rootResponse.Code)
	}

	agentRequest := httptest.NewRequest(http.MethodPost, "/api/v1/ingest", bytes.NewReader([]byte(`{}`)))
	agentResponse := httptest.NewRecorder()
	a.Handler().ServeHTTP(agentResponse, agentRequest)
	if agentResponse.Code != http.StatusUnauthorized {
		t.Fatalf("Agent ingest should bypass the UI gate and use its own auth, got %d", agentResponse.Code)
	}

	settingsRequest := httptest.NewRequest(http.MethodGet, "/api/v1/settings", nil)
	settingsRequest.AddCookie(entryCookie)
	for _, cookie := range sessionCookies {
		settingsRequest.AddCookie(cookie)
	}
	settingsResponse := httptest.NewRecorder()
	a.Handler().ServeHTTP(settingsResponse, settingsRequest)
	if settingsResponse.Code != http.StatusOK || bytes.Contains(settingsResponse.Body.Bytes(), []byte(link)) {
		t.Fatalf("settings should be accessible without revealing the link: %d %s", settingsResponse.Code, settingsResponse.Body.String())
	}

	disableBody := []byte(`{"offline_after_seconds":45,"ui_gate_enabled":false}`)
	disableRequest := httptest.NewRequest(http.MethodPost, "/api/v1/settings", bytes.NewReader(disableBody))
	disableRequest.AddCookie(entryCookie)
	for _, cookie := range sessionCookies {
		disableRequest.AddCookie(cookie)
	}
	disableResponse := httptest.NewRecorder()
	a.Handler().ServeHTTP(disableResponse, disableRequest)
	if disableResponse.Code != http.StatusOK {
		t.Fatalf("disable gate failed: %d %s", disableResponse.Code, disableResponse.Body.String())
	}
	afterDisable := httptest.NewRecorder()
	a.Handler().ServeHTTP(afterDisable, httptest.NewRequest(http.MethodGet, "/api/v1/auth/status", nil))
	if afterDisable.Code != http.StatusOK {
		t.Fatalf("disabled gate should allow auth status, got %d", afterDisable.Code)
	}
}

func TestAgentTokenCannotAccessAdminAPI(t *testing.T) {
	a := testApp(t)
	req := httptest.NewRequest("GET", "/api/v1/summary", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("agent token must not authorize web API, got %d", rec.Code)
	}
}

func TestSettingsTokenIsPersistedButNeverReturned(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	a := NewAt("", path)
	body := []byte(`{"agent_token":"0123456789abcdef0123456789abcdef","offline_after_seconds":90}`)
	req := httptest.NewRequest("POST", "/api/v1/settings", bytes.NewReader(body))
	sessionRec := httptest.NewRecorder()
	a.createSession(sessionRec, req, "admin")
	cookies := sessionRec.Result().Cookies()
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("save settings: %d %s", rec.Code, rec.Body.String())
	}
	get := httptest.NewRequest("GET", "/api/v1/settings", nil)
	for _, c := range cookies {
		get.AddCookie(c)
	}
	getRec := httptest.NewRecorder()
	a.Handler().ServeHTTP(getRec, get)
	if getRec.Code != 200 {
		t.Fatalf("get settings: %d", getRec.Code)
	}
	if bytes.Contains(getRec.Body.Bytes(), []byte("agent_interval_seconds")) {
		t.Fatal("Agent interval must not be stored as a server setting")
	}
	if bytes.Contains(getRec.Body.Bytes(), []byte("0123456789abcdef")) {
		t.Fatal("agent token must never be returned")
	}
	b := NewAt("", path)
	if !b.authorizedAgent(httptest.NewRequest("POST", "/api/v1/ingest", nil)) { /* request has no header; expected false */
	}
	agentReq := httptest.NewRequest("POST", "/api/v1/ingest", nil)
	agentReq.Header.Set("Authorization", "Bearer 0123456789abcdef0123456789abcdef")
	if !b.authorizedAgent(agentReq) {
		t.Fatal("agent token should survive restart")
	}
}

func TestLoginAfterServerRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	first := NewAt("", path)
	setup := httptest.NewRequest("POST", "/api/v1/auth/setup", bytes.NewReader([]byte(`{"username":"operator","password":"correct-horse-battery-2026"}`)))
	setupRec := httptest.NewRecorder()
	first.Handler().ServeHTTP(setupRec, setup)
	if setupRec.Code != http.StatusCreated {
		t.Fatalf("setup failed: %d", setupRec.Code)
	}
	restarted := NewAt("", path)
	login := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader([]byte(`{"username":"operator","password":"correct-horse-battery-2026"}`)))
	loginRec := httptest.NewRecorder()
	restarted.Handler().ServeHTTP(loginRec, login)
	if loginRec.Code != http.StatusOK {
		t.Fatalf("login after restart failed: %d %s", loginRec.Code, loginRec.Body.String())
	}
	if len(loginRec.Result().Cookies()) == 0 {
		t.Fatal("login did not issue a session cookie")
	}
}
