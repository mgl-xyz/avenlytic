//go:build windows

package windows

import (
	"context"
	"github.com/avenlytic/avenlytic/internal/core/model"
	"runtime"
	"time"
)

type Provider struct{}

func New() *Provider         { return &Provider{} }
func (*Provider) ID() string { return "windows" }
func (*Provider) Snapshot(ctx context.Context, hostname string) (model.Snapshot, error) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return model.Snapshot{Protocol: model.ProtocolVersion, AgentID: hostname, Hostname: hostname, OS: "windows", Arch: runtime.GOARCH, Platform: "Windows", Collected: time.Now().UTC(), Runtime: model.RuntimeStats{Goroutines: runtime.NumGoroutine(), HeapAlloc: m.HeapAlloc}, Warnings: []string{"Windows native CPU/memory/disk/network collectors are not implemented yet"}}, nil
}
