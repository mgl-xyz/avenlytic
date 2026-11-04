// Package ports defines OS-neutral interfaces implemented by platform adapters.
package ports

import (
	"context"
	"github.com/avenlytic/avenlytic/internal/core/model"
)

type Provider interface {
	ID() string
	Snapshot(context.Context, string) (model.Snapshot, error)
}
type HostRootProvider interface {
	SetHostRoot(string) error
	SetSysRoot(string) error
}
type Capability struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Available  bool   `json:"available"`
	Privileged bool   `json:"privileged"`
	Reason     string `json:"reason,omitempty"`
}
type CapabilityProvider interface{ Capabilities() []Capability }
type CPUProvider interface {
	CollectCPU(context.Context) (*model.CPU, error)
}
type MemoryProvider interface {
	CollectMemory(context.Context) (*model.Memory, error)
}
type DiskProvider interface {
	CollectDisks(context.Context) ([]model.Disk, error)
	CollectDiskIO(context.Context) ([]model.DiskIO, error)
}
type NetworkProvider interface {
	CollectNetworks(context.Context) ([]model.Network, error)
}
type ProcessProvider interface {
	CollectProcesses(context.Context) ([]model.Process, error)
	ProcessCount(context.Context) (int, error)
}
type SystemProvider interface {
	CollectLoad(context.Context) (*model.Load, error)
	Uptime(context.Context) (uint64, error)
}
type SecurityProvider interface {
	CollectSecurity(context.Context, bool) (model.SecurityReport, error)
}
type DockerProvider interface {
	CollectDocker(context.Context, string) (model.DockerInventory, error)
}
