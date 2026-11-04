package rules

import (
	"fmt"
	"github.com/avenlytic/avenlytic/internal/core/model"
	"math"
	"unicode/utf8"
)

type Rule struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	Enabled          bool     `json:"enabled"`
	Metric           string   `json:"metric"`
	MessageTemplate  string   `json:"message_template,omitempty"`
	Operator         string   `json:"operator"`
	Threshold        float64  `json:"threshold"`
	DurationSeconds  int      `json:"duration_seconds"`
	Severity         string   `json:"severity"`
	ChannelIDs       []string `json:"channel_ids"`
	NotifyOnRecovery bool     `json:"notify_on_recovery"`
}
type Result struct {
	Value     float64
	Supported bool
}

func Evaluate(r Rule, s model.Snapshot) Result {
	switch r.Metric {
	case "cpu.usage_percent":
		if s.CPU != nil {
			return Result{s.CPU.UsagePercent, true}
		}
	case "memory.usage_percent":
		if s.Memory != nil {
			return Result{s.Memory.UsagePercent, true}
		}
	case "load.1m":
		if s.Load != nil {
			return Result{s.Load.One, true}
		}
	case "load.5m":
		if s.Load != nil {
			return Result{s.Load.Five, true}
		}
	case "load.15m":
		if s.Load != nil {
			return Result{s.Load.Fifteen, true}
		}
	case "disk.usage_percent":
		var max float64
		for _, d := range s.Disks {
			if d.UsagePercent > max {
				max = d.UsagePercent
			}
		}
		if len(s.Disks) > 0 {
			return Result{max, true}
		}
	case "process.count":
		return Result{float64(s.ProcessCount), true}
	case "agent.online":
		return Result{1, true}
	}
	return Result{}
}
func EvaluateWithSecurity(r Rule, s model.Snapshot, security model.SecurityReport) Result {
	switch r.Metric {
	case "security.login_failures_15m":
		if security.LoginMonitoringAvailable && security.LoginWindowSeconds == 900 {
			return Result{float64(security.FailedLoginAttempts), true}
		}
	case "system.updates_available":
		if security.UpdateCheckAvailable {
			return Result{float64(security.AvailableUpdates), true}
		}
	case "security.vulnerabilities":
		if !security.VulnerabilityChecked.IsZero() && security.VulnerabilityError == "" {
			return Result{float64(len(security.Vulnerabilities)), true}
		}
	}
	return Evaluate(r, s)
}
func Match(op string, a, b float64) bool {
	switch op {
	case ">":
		return a > b
	case ">=":
		return a >= b
	case "<":
		return a < b
	case "<=":
		return a <= b
	case "==":
		return a == b
	case "!=":
		return a != b
	}
	return false
}
func Validate(r Rule) error {
	if r.ID == "" || r.Name == "" || r.Metric == "" {
		return fmt.Errorf("id, name and metric are required")
	}
	if utf8.RuneCountInString(r.MessageTemplate) > 2000 {
		return fmt.Errorf("message_template must be 2000 characters or fewer")
	}
	switch r.Operator {
	case ">", ">=", "<", "<=", "==", "!=":
	default:
		return fmt.Errorf("invalid operator")
	}
	switch r.Metric {
	case "cpu.usage_percent", "memory.usage_percent", "disk.usage_percent", "load.1m", "load.5m", "load.15m", "process.count", "agent.online", "security.login_failures_15m", "system.updates_available", "security.vulnerabilities":
	default:
		return fmt.Errorf("unsupported metric")
	}
	if r.DurationSeconds < 0 || r.DurationSeconds > 86400 {
		return fmt.Errorf("duration_seconds must be between 0 and 86400")
	}
	if math.IsNaN(r.Threshold) || math.IsInf(r.Threshold, 0) {
		return fmt.Errorf("threshold must be finite")
	}
	if r.Metric == "cpu.usage_percent" || r.Metric == "memory.usage_percent" || r.Metric == "disk.usage_percent" {
		if r.Threshold < 0 || r.Threshold > 100 {
			return fmt.Errorf("percentage threshold must be between 0 and 100")
		}
	}
	switch r.Severity {
	case "", "info", "warning", "critical":
	default:
		return fmt.Errorf("invalid severity")
	}
	return nil
}
