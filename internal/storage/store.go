// Package storage persists server state in SQLite with transactional updates.
package storage

import (
	"github.com/avenlytic/avenlytic/internal/core/model"
	"github.com/avenlytic/avenlytic/internal/notify"
	"github.com/avenlytic/avenlytic/internal/rules"
	"time"
)

type ServerSettings struct {
	OfflineAfterSeconds  int    `json:"offline_after_seconds"`
	AgentTokenConfigured bool   `json:"agent_token_configured"`
	AgentToken           string `json:"agent_token,omitempty"`
	UIGateHash           string `json:"ui_gate_hash,omitempty"`
}
type AuthRecord struct {
	Username  string    `json:"username"`
	Salt      string    `json:"salt"`
	Hash      string    `json:"hash"`
	JWTSecret string    `json:"jwt_secret,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}
type State struct {
	Auth       AuthRecord                      `json:"auth"`
	Settings   ServerSettings                  `json:"settings"`
	Hosts      map[string]model.Snapshot       `json:"hosts"`
	Security   map[string]model.SecurityReport `json:"security,omitempty"`
	Rules      []rules.Rule                    `json:"rules"`
	Channels   []notify.Config                 `json:"channels"`
	Events     []Event                         `json:"events"`
	Deliveries []Delivery                      `json:"deliveries"`
}
type Event struct {
	ID         string  `json:"id"`
	RuleID     string  `json:"rule_id"`
	RuleName   string  `json:"rule_name"`
	HostID     string  `json:"host_id"`
	Host       string  `json:"host"`
	Severity   string  `json:"severity"`
	Status     string  `json:"status"`
	Message    string  `json:"message"`
	Value      float64 `json:"value"`
	Threshold  float64 `json:"threshold"`
	CreatedAt  string  `json:"created_at"`
	ResolvedAt string  `json:"resolved_at,omitempty"`
}
type Delivery struct {
	ID          string `json:"id"`
	EventID     string `json:"event_id"`
	ChannelID   string `json:"channel_id"`
	ChannelName string `json:"channel_name"`
	ChannelType string `json:"channel_type"`
	Status      string `json:"status"`
	Attempts    int    `json:"attempts"`
	Error       string `json:"error,omitempty"`
	CreatedAt   string `json:"created_at"`
}

// Repository is the persistence boundary used by the server.
type Repository interface {
	View(func(*State))
	Update(func(*State)) error
}
