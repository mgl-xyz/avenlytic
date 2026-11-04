package rules

import (
	"testing"
	"time"

	"github.com/avenlytic/avenlytic/internal/core/model"
)

func TestEvaluateMemory(t *testing.T) {
	s := model.Snapshot{Memory: &model.Memory{UsagePercent: 91}}
	v := Evaluate(Rule{Metric: "memory.usage_percent"}, s)
	if !v.Supported || v.Value != 91 {
		t.Fatalf("unexpected result: %+v", v)
	}
	if !Match(">", v.Value, 90) {
		t.Fatal("expected threshold match")
	}
}

func TestUnsupportedMetric(t *testing.T) {
	if Evaluate(Rule{Metric: "disk.usage_percent"}, model.Snapshot{}).Supported {
		t.Fatal("expected unsupported")
	}
}

func TestEvaluateLoadAverages(t *testing.T) {
	s := model.Snapshot{Load: &model.Load{One: 1.25, Five: 2.5, Fifteen: 3.75}}
	for _, test := range []struct {
		metric string
		want   float64
	}{
		{metric: "load.1m", want: 1.25},
		{metric: "load.5m", want: 2.5},
		{metric: "load.15m", want: 3.75},
	} {
		got := Evaluate(Rule{Metric: test.metric}, s)
		if !got.Supported || got.Value != test.want {
			t.Errorf("%s: got %+v, want %v", test.metric, got, test.want)
		}
	}
}

func TestValidateLoadAverageRules(t *testing.T) {
	for _, metric := range []string{"load.1m", "load.5m", "load.15m"} {
		rule := Rule{ID: "test", Name: "Load", Metric: metric, Operator: ">", Severity: "warning"}
		if err := Validate(rule); err != nil {
			t.Errorf("%s: %v", metric, err)
		}
	}
}

func TestEvaluateSecurityMetrics(t *testing.T) {
	security := model.SecurityReport{
		FailedLoginAttempts:      7,
		LoginWindowSeconds:       900,
		LoginMonitoringAvailable: true,
		AvailableUpdates:         3,
		UpdateCheckAvailable:     true,
		Vulnerabilities:          []model.Vulnerability{{ID: "CVE-2025-0001"}},
		VulnerabilityChecked:     time.Now(),
	}
	for _, test := range []struct {
		metric string
		want   float64
	}{
		{metric: "security.login_failures_15m", want: 7},
		{metric: "system.updates_available", want: 3},
		{metric: "security.vulnerabilities", want: 1},
	} {
		got := EvaluateWithSecurity(Rule{Metric: test.metric}, model.Snapshot{}, security)
		if !got.Supported || got.Value != test.want {
			t.Errorf("%s: got %+v, want %v", test.metric, got, test.want)
		}
	}
	if got := EvaluateWithSecurity(Rule{Metric: "security.vulnerabilities"}, model.Snapshot{}, model.SecurityReport{VulnerabilityError: "offline"}); got.Supported {
		t.Fatal("expected unavailable vulnerability scan to be unsupported")
	}
}

func TestValidateSecurityMetrics(t *testing.T) {
	for _, metric := range []string{"security.login_failures_15m", "system.updates_available", "security.vulnerabilities"} {
		rule := Rule{ID: "test", Name: "Security", Metric: metric, Operator: ">", Severity: "warning"}
		if err := Validate(rule); err != nil {
			t.Errorf("%s: %v", metric, err)
		}
	}
}
