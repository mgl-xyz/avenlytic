package storage

import (
	"encoding/json"
	"github.com/avenlytic/avenlytic/internal/core/model"
	"github.com/avenlytic/avenlytic/internal/notify"
	"github.com/avenlytic/avenlytic/internal/rules"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestSQLiteStatePersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err = s.Update(func(st *State) { st.Hosts["node-a"] = model.Snapshot{AgentID: "node-a", Hostname: "node-a"} }); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0600 {
			t.Fatalf("database permissions = %o, want 600", info.Mode().Perm())
		}
	}
	s2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()
	s2.View(func(st *State) {
		if st.Hosts["node-a"].Hostname != "node-a" {
			t.Fatal("state was not persisted")
		}
	})
}

func TestLegacyJSONMigratesAndIsBackedUp(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "avenlytic-state.json")
	legacy := State{
		Auth:     AuthRecord{Username: "admin", Hash: "password-hash", JWTSecret: "signing-key"},
		Settings: ServerSettings{OfflineAfterSeconds: 75, AgentToken: "agent-secret"},
		Hosts: map[string]model.Snapshot{
			"node-a": {AgentID: "node-a", Hostname: "host-a"},
		},
		Security: map[string]model.SecurityReport{
			"node-a": {AgentID: "node-a", AvailableUpdates: 3},
		},
		Rules:      []rules.Rule{{ID: "rule-a", Name: "memory alert"}},
		Channels:   []notify.Config{{ID: "channel-a", Name: "email", Type: "email", Settings: map[string]string{"password": "smtp-secret"}}},
		Events:     []Event{{ID: "event-a", Status: "firing"}},
		Deliveries: []Delivery{{ID: "delivery-a", Status: "sent"}},
	}
	legacyData, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, legacyData, 0600); err != nil {
		t.Fatal(err)
	}

	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	databaseBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(databaseBytes), sqliteHeader) {
		t.Fatal("legacy path was not replaced by a SQLite database")
	}
	backup, err := os.ReadFile(path + ".legacy.json")
	if err != nil {
		t.Fatalf("legacy backup missing: %v", err)
	}
	if string(backup) != string(legacyData) {
		t.Fatal("legacy backup contents changed")
	}
	store.View(func(state *State) {
		if state.Auth.JWTSecret != "signing-key" || state.Settings.AgentToken != "agent-secret" {
			t.Error("authentication or settings data was not migrated")
		}
		if state.Hosts["node-a"].Hostname != "host-a" || state.Security["node-a"].AvailableUpdates != 3 {
			t.Error("host data was not migrated")
		}
		if len(state.Rules) != 1 || len(state.Channels) != 1 || len(state.Events) != 1 || len(state.Deliveries) != 1 {
			t.Error("related entities were not migrated")
		}
		if state.Channels[0].Settings["password"] != "smtp-secret" {
			t.Error("channel secret was not migrated")
		}
	})
}

func TestDefaultSQLitePathImportsLegacyJSON(t *testing.T) {
	dir := t.TempDir()
	legacyPath := filepath.Join(dir, "avenlytic-state.json")
	legacy := State{Hosts: map[string]model.Snapshot{"node-b": {AgentID: "node-b", Hostname: "legacy-host"}}}
	data, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacyPath, data, 0600); err != nil {
		t.Fatal(err)
	}

	databasePath := filepath.Join(dir, "avenlytic.db")
	store, err := Open(databasePath)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	store.View(func(state *State) {
		if state.Hosts["node-b"].Hostname != "legacy-host" {
			t.Fatal("adjacent legacy state was not imported")
		}
	})
	if _, err := os.Stat(legacyPath); err != nil {
		t.Fatalf("source JSON should remain as a backup: %v", err)
	}
}

func TestFailedUpdateDoesNotChangeStoredState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.db")
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Update(func(state *State) { state.Auth.Username = "before" }); err != nil {
		t.Fatal(err)
	}
	if err := store.Update(func(state *State) { state.Rules = append(state.Rules, rules.Rule{}) }); err == nil {
		t.Fatal("expected invalid empty rule ID to fail")
	}
	store.View(func(state *State) {
		if state.Auth.Username != "before" || len(state.Rules) != 0 {
			t.Fatal("failed transaction leaked into the in-memory state")
		}
	})
	reopened, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	reopened.View(func(state *State) {
		if state.Auth.Username != "before" || len(state.Rules) != 0 {
			t.Fatal("failed transaction changed persisted state")
		}
	})
}

func TestInvalidLegacyJSONIsNotOverwritten(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	original := []byte(`{"auth":`)
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(path); err == nil {
		t.Fatal("expected invalid legacy JSON to fail migration")
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(original) {
		t.Fatal("invalid legacy file was changed")
	}
	if _, err := os.Stat(path + ".legacy.json"); !os.IsNotExist(err) {
		t.Fatal("invalid legacy file should not be backed up as a successful migration")
	}
}
