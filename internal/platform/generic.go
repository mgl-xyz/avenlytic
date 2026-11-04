package platform

import (
	"context"
	"fmt"
	"github.com/avenlytic/avenlytic/internal/core/model"
	"runtime"
	"time"
)

type genericProvider struct{}

func (genericProvider) ID() string { return runtime.GOOS }
func (genericProvider) Snapshot(ctx context.Context, hostname string) (model.Snapshot, error) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return model.Snapshot{Protocol: model.ProtocolVersion, AgentID: hostname, Hostname: hostname, OS: runtime.GOOS, Arch: runtime.GOARCH, Platform: runtime.GOOS, Collected: time.Now().UTC(), Runtime: model.RuntimeStats{Goroutines: runtime.NumGoroutine(), HeapAlloc: m.HeapAlloc}, Warnings: []string{fmt.Sprintf("native collectors are not implemented for %s", runtime.GOOS)}}, nil
}
