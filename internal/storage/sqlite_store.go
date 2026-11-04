package storage

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/avenlytic/avenlytic/internal/core/model"
	"github.com/avenlytic/avenlytic/internal/notify"
	"github.com/avenlytic/avenlytic/internal/rules"
	_ "modernc.org/sqlite"
)

const sqliteHeader = "SQLite format 3\x00"

type Store struct {
	mu    sync.RWMutex
	db    *sql.DB
	path  string
	state State
}

func Open(path string) (*Store, error) {
	if path == "" {
		path = ":memory:"
	}
	if path != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			return nil, fmt.Errorf("create database directory: %w", err)
		}
		if _, err := os.Stat(path); err == nil {
			isSQLite, err := hasSQLiteHeader(path)
			if err != nil {
				return nil, err
			}
			if !isSQLite {
				legacy, err := readLegacyState(path)
				if err != nil {
					return nil, err
				}
				if err := replaceWithSQLite(path, legacy); err != nil {
					return nil, fmt.Errorf("migrate legacy JSON state: %w", err)
				}
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("inspect database path: %w", err)
		} else if legacyPath, err := findLegacyState(path); err != nil {
			return nil, err
		} else if legacyPath != "" {
			legacy, err := readLegacyState(legacyPath)
			if err != nil {
				return nil, err
			}
			if err := createSQLiteFile(path, legacy); err != nil {
				return nil, fmt.Errorf("migrate legacy JSON state from %q: %w", legacyPath, err)
			}
		}
	}

	db, err := openSQLite(path)
	if err != nil {
		return nil, err
	}
	store := &Store{db: db, path: path, state: emptyState()}
	if err := store.load(); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := restrictSQLitePermissions(path); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func emptyState() State {
	return State{
		Hosts:    map[string]model.Snapshot{},
		Security: map[string]model.SecurityReport{},
	}
}

func hasSQLiteHeader(path string) (bool, error) {
	file, err := os.Open(path)
	if err != nil {
		return false, fmt.Errorf("open database path: %w", err)
	}
	defer file.Close()

	header := make([]byte, len(sqliteHeader))
	n, err := file.Read(header)
	if err != nil && !errors.Is(err, io.EOF) {
		return false, fmt.Errorf("read database header: %w", err)
	}
	return n == len(sqliteHeader) && string(header) == sqliteHeader, nil
}

func findLegacyState(databasePath string) (string, error) {
	candidates := []string{databasePath + ".json"}
	if filepath.Base(databasePath) == "avenlytic.db" {
		candidates = append(candidates, filepath.Join(filepath.Dir(databasePath), "avenlytic-state.json"))
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("inspect legacy state %q: %w", candidate, err)
		}
	}
	return "", nil
}

func readLegacyState(path string) (State, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return State{}, fmt.Errorf("read legacy state %q: %w", path, err)
	}
	state := emptyState()
	if err := json.Unmarshal(data, &state); err != nil {
		return State{}, fmt.Errorf("decode legacy state %q: %w", path, err)
	}
	if state.Hosts == nil {
		state.Hosts = map[string]model.Snapshot{}
	}
	if state.Security == nil {
		state.Security = map[string]model.SecurityReport{}
	}
	return state, nil
}

func backupLegacyFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read legacy state for backup: %w", err)
	}
	backupPath := path + ".legacy.json"
	file, err := os.OpenFile(backupPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if errors.Is(err, os.ErrExist) {
		existing, readErr := os.ReadFile(backupPath)
		if readErr == nil && string(existing) == string(data) {
			return nil
		}
		return fmt.Errorf("legacy backup %q already exists and differs", backupPath)
	}
	if err != nil {
		return fmt.Errorf("create legacy backup: %w", err)
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		_ = os.Remove(backupPath)
		return fmt.Errorf("write legacy backup: %w", err)
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		_ = os.Remove(backupPath)
		return fmt.Errorf("sync legacy backup: %w", err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(backupPath)
		return fmt.Errorf("close legacy backup: %w", err)
	}
	return nil
}

func replaceWithSQLite(path string, state State) error {
	if err := backupLegacyFile(path); err != nil {
		return err
	}
	return createSQLiteFile(path, state)
}

func createSQLiteFile(path string, state State) error {
	dir := filepath.Dir(path)
	temp, err := os.CreateTemp(dir, ".avenlytic-migrate-*.db")
	if err != nil {
		return fmt.Errorf("create temporary database: %w", err)
	}
	tempPath := temp.Name()
	if err := temp.Close(); err != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf("close temporary database: %w", err)
	}
	if err := os.Remove(tempPath); err != nil {
		return fmt.Errorf("prepare temporary database: %w", err)
	}
	defer func() {
		_ = os.Remove(tempPath)
		_ = os.Remove(tempPath + "-wal")
		_ = os.Remove(tempPath + "-shm")
	}()

	db, err := openSQLite(tempPath)
	if err != nil {
		return err
	}
	store := &Store{db: db, path: tempPath, state: emptyState()}
	err = store.Update(func(current *State) { *current = state })
	closeErr := db.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return fmt.Errorf("close temporary database: %w", closeErr)
	}
	if err := os.Chmod(tempPath, 0600); err != nil {
		return fmt.Errorf("restrict temporary database permissions: %w", err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("install migrated database: %w", err)
	}
	return nil
}

func openSQLite(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open SQLite database: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	for _, statement := range []string{
		"PRAGMA journal_mode = WAL",
		"PRAGMA synchronous = FULL",
		"PRAGMA busy_timeout = 5000",
		`CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			applied_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS metadata (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			auth TEXT NOT NULL,
			settings TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS hosts (id TEXT PRIMARY KEY, data TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS security_reports (id TEXT PRIMARY KEY, data TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS rules (id TEXT PRIMARY KEY, position INTEGER NOT NULL, data TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS channels (id TEXT PRIMARY KEY, position INTEGER NOT NULL, data TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS events (id TEXT PRIMARY KEY, position INTEGER NOT NULL, data TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS deliveries (id TEXT PRIMARY KEY, position INTEGER NOT NULL, data TEXT NOT NULL)`,
		"INSERT OR IGNORE INTO schema_migrations (version) VALUES (1)",
	} {
		if _, err := db.Exec(statement); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("initialize SQLite schema: %w", err)
		}
	}
	if err := restrictSQLitePermissions(path); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func restrictSQLitePermissions(path string) error {
	if path == ":memory:" {
		return nil
	}
	for _, file := range []string{path, path + "-wal", path + "-shm"} {
		if err := os.Chmod(file, 0600); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("restrict SQLite file permissions for %q: %w", file, err)
		}
	}
	return nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) View(fn func(*State)) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	fn(&s.state)
}

func (s *Store) Update(fn func(*State)) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	next, err := cloneState(s.state)
	if err != nil {
		return fmt.Errorf("copy state for update: %w", err)
	}
	fn(&next)
	if err := s.persist(s.state, next); err != nil {
		return err
	}
	s.state = next
	return nil
}

func cloneState(state State) (State, error) {
	data, err := json.Marshal(state)
	if err != nil {
		return State{}, err
	}
	var result State
	if err := json.Unmarshal(data, &result); err != nil {
		return State{}, err
	}
	if result.Hosts == nil {
		result.Hosts = map[string]model.Snapshot{}
	}
	if result.Security == nil {
		result.Security = map[string]model.SecurityReport{}
	}
	return result, nil
}

func (s *Store) load() error {
	var authData, settingsData string
	err := s.db.QueryRow("SELECT auth, settings FROM metadata WHERE id = 1").Scan(&authData, &settingsData)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return s.Update(func(*State) {})
	case err != nil:
		return fmt.Errorf("read SQLite metadata: %w", err)
	}
	if err := json.Unmarshal([]byte(authData), &s.state.Auth); err != nil {
		return fmt.Errorf("decode SQLite auth metadata: %w", err)
	}
	if err := json.Unmarshal([]byte(settingsData), &s.state.Settings); err != nil {
		return fmt.Errorf("decode SQLite settings metadata: %w", err)
	}
	if err := loadMapRows(s.db, "hosts", &s.state.Hosts); err != nil {
		return err
	}
	if err := loadMapRows(s.db, "security_reports", &s.state.Security); err != nil {
		return err
	}
	if err := loadSliceRows(s.db, "rules", &s.state.Rules); err != nil {
		return err
	}
	if err := loadSliceRows(s.db, "channels", &s.state.Channels); err != nil {
		return err
	}
	if err := loadSliceRows(s.db, "events", &s.state.Events); err != nil {
		return err
	}
	if err := loadSliceRows(s.db, "deliveries", &s.state.Deliveries); err != nil {
		return err
	}
	return nil
}

func loadMapRows[T any](db *sql.DB, table string, target *map[string]T) error {
	rows, err := db.Query("SELECT id, data FROM " + table)
	if err != nil {
		return fmt.Errorf("read SQLite %s: %w", table, err)
	}
	defer rows.Close()
	result := make(map[string]T)
	for rows.Next() {
		var id, data string
		var value T
		if err := rows.Scan(&id, &data); err != nil {
			return fmt.Errorf("read SQLite %s row: %w", table, err)
		}
		if err := json.Unmarshal([]byte(data), &value); err != nil {
			return fmt.Errorf("decode SQLite %s row %q: %w", table, id, err)
		}
		result[id] = value
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate SQLite %s: %w", table, err)
	}
	*target = result
	return nil
}

func loadSliceRows[T any](db *sql.DB, table string, target *[]T) error {
	rows, err := db.Query("SELECT data FROM " + table + " ORDER BY position")
	if err != nil {
		return fmt.Errorf("read SQLite %s: %w", table, err)
	}
	defer rows.Close()
	result := make([]T, 0)
	for rows.Next() {
		var data string
		var value T
		if err := rows.Scan(&data); err != nil {
			return fmt.Errorf("read SQLite %s row: %w", table, err)
		}
		if err := json.Unmarshal([]byte(data), &value); err != nil {
			return fmt.Errorf("decode SQLite %s row: %w", table, err)
		}
		result = append(result, value)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate SQLite %s: %w", table, err)
	}
	*target = result
	return nil
}

func (s *Store) persist(before, after State) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin SQLite transaction: %w", err)
	}
	defer tx.Rollback()

	authData, err := json.Marshal(after.Auth)
	if err != nil {
		return err
	}
	settingsData, err := json.Marshal(after.Settings)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO metadata (id, auth, settings) VALUES (1, ?, ?)
		ON CONFLICT(id) DO UPDATE SET auth = excluded.auth, settings = excluded.settings
		WHERE auth <> excluded.auth OR settings <> excluded.settings`, string(authData), string(settingsData)); err != nil {
		return fmt.Errorf("write SQLite metadata: %w", err)
	}

	oldHosts, err := marshalMapRows(before.Hosts)
	if err != nil {
		return fmt.Errorf("encode previous SQLite host rows: %w", err)
	}
	newHosts, err := marshalMapRows(after.Hosts)
	if err != nil {
		return fmt.Errorf("encode SQLite host rows: %w", err)
	}
	if err := syncMapRows(tx, "hosts", oldHosts, newHosts); err != nil {
		return err
	}
	oldSecurity, err := marshalMapRows(before.Security)
	if err != nil {
		return fmt.Errorf("encode previous SQLite security rows: %w", err)
	}
	newSecurity, err := marshalMapRows(after.Security)
	if err != nil {
		return fmt.Errorf("encode SQLite security rows: %w", err)
	}
	if err := syncMapRows(tx, "security_reports", oldSecurity, newSecurity); err != nil {
		return err
	}
	if err := syncSliceRows(tx, "rules", before.Rules, after.Rules, func(value rules.Rule) string { return value.ID }); err != nil {
		return err
	}
	if err := syncSliceRows(tx, "channels", before.Channels, after.Channels, func(value notify.Config) string { return value.ID }); err != nil {
		return err
	}
	if err := syncSliceRows(tx, "events", before.Events, after.Events, func(value Event) string { return value.ID }); err != nil {
		return err
	}
	if err := syncSliceRows(tx, "deliveries", before.Deliveries, after.Deliveries, func(value Delivery) string { return value.ID }); err != nil {
		return err
	}
	if err := restrictSQLitePermissions(s.path); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit SQLite transaction: %w", err)
	}
	return nil
}

func marshalMapRows[T any](rows map[string]T) (map[string]string, error) {
	result := make(map[string]string, len(rows))
	for id, row := range rows {
		data, err := json.Marshal(row)
		if err != nil {
			return nil, err
		}
		result[id] = string(data)
	}
	return result, nil
}

func syncMapRows(tx *sql.Tx, table string, before, after map[string]string) error {
	query := "INSERT INTO " + table + " (id, data) VALUES (?, ?) ON CONFLICT(id) DO UPDATE SET data = excluded.data WHERE data <> excluded.data"
	for id, data := range after {
		if before[id] == data {
			continue
		}
		if _, err := tx.Exec(query, id, data); err != nil {
			return fmt.Errorf("write SQLite %s row %q: %w", table, id, err)
		}
	}
	for id := range before {
		if _, exists := after[id]; exists {
			continue
		}
		if _, err := tx.Exec("DELETE FROM "+table+" WHERE id = ?", id); err != nil {
			return fmt.Errorf("delete SQLite %s row %q: %w", table, id, err)
		}
	}
	return nil
}

func syncSliceRows[T any](tx *sql.Tx, table string, before, after []T, idOf func(T) string) error {
	oldRows := make(map[string]struct {
		position int
		data     string
	}, len(before))
	for position, row := range before {
		data, err := json.Marshal(row)
		if err != nil {
			return fmt.Errorf("encode SQLite %s row: %w", table, err)
		}
		oldRows[idOf(row)] = struct {
			position int
			data     string
		}{position, string(data)}
	}

	query := "INSERT INTO " + table + " (id, position, data) VALUES (?, ?, ?) ON CONFLICT(id) DO UPDATE SET position = excluded.position, data = excluded.data WHERE position <> excluded.position OR data <> excluded.data"
	newIDs := make(map[string]struct{}, len(after))
	for position, row := range after {
		id := idOf(row)
		if id == "" {
			return fmt.Errorf("SQLite %s row has an empty id", table)
		}
		newIDs[id] = struct{}{}
		data, err := json.Marshal(row)
		if err != nil {
			return fmt.Errorf("encode SQLite %s row %q: %w", table, id, err)
		}
		encoded := string(data)
		if old, exists := oldRows[id]; exists && old.position == position && old.data == encoded {
			continue
		}
		if _, err := tx.Exec(query, id, position, encoded); err != nil {
			return fmt.Errorf("write SQLite %s row %q: %w", table, id, err)
		}
	}
	for id := range oldRows {
		if _, exists := newIDs[id]; exists {
			continue
		}
		if _, err := tx.Exec("DELETE FROM "+table+" WHERE id = ?", id); err != nil {
			return fmt.Errorf("delete SQLite %s row %q: %w", table, id, err)
		}
	}
	return nil
}
