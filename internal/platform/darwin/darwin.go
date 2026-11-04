//go:build darwin

package darwin

import (
	"context"
	"github.com/avenlytic/avenlytic/internal/core/model"
	"runtime"
	"time"
)

type Provider struct{}

func New() *Provider         { return &Provider{} }
func (*Provider) ID() string { return "darwin" }
func (*Provider) Snapshot(ctx context.Context, hostname string) (model.Snapshot, error) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return model.Snapshot{Protocol: model.ProtocolVersion, AgentID: hostname, Hostname: hostname, OS: "darwin", Arch: runtime.GOARCH, Platform: "macOS", Collected: time.Now().UTC(), Runtime: model.RuntimeStats{Goroutines: runtime.NumGoroutine(), HeapAlloc: m.HeapAlloc}, Warnings: []string{"macOS native CPU/memory/disk/network collectors are not implemented yet"}}, nil
}
