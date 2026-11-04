//go:build linux

package linux

import (
	"context"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCollectDockerUsesReadOnlyAPIAndCollectsInventory(t *testing.T) {
	socketPath := filepath.Join(t.TempDir(), "docker.sock")
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("Docker API method = %s, want GET", r.Method)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		switch r.URL.Path {
		case "/version":
			_, _ = w.Write([]byte(`{"ApiVersion":"1.41"}`))
		case "/v1.41/containers/json":
			_, _ = w.Write([]byte(`[{
				"Id":"0123456789abcdef0123456789abcdef",
				"Names":["/web"],
				"Image":"nginx:latest",
				"State":"running",
				"Status":"Up 2 minutes",
				"Created":1793800000,
				"Ports":[{"IP":"0.0.0.0","PrivatePort":80,"PublicPort":8080,"Type":"tcp"}]
			},{"Id":"fedcba9876543210","Names":["/stopped"],"Image":"alpine:latest","State":"exited","Status":"Exited (0)"}]`))
		case "/v1.41/containers/0123456789abcdef0123456789abcdef/stats":
			_, _ = w.Write([]byte(`{
				"cpu_stats":{"cpu_usage":{"total_usage":200,"percpu_usage":[1]},"system_cpu_usage":1400,"online_cpus":1},
				"precpu_stats":{"cpu_usage":{"total_usage":100},"system_cpu_usage":1000},
				"memory_stats":{"usage":25,"limit":100}
			}`))
		default:
			http.NotFound(w, r)
		}
	})}
	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			t.Errorf("Docker test server: %v", err)
		}
	}()
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("close Docker test server: %v", err)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	inventory, err := collectDocker(ctx, socketPath)
	if err != nil {
		t.Fatal(err)
	}
	if !inventory.Available || len(inventory.Containers) != 2 {
		t.Fatalf("unexpected inventory: %+v", inventory)
	}
	running := inventory.Containers[0]
	if running.Name != "web" || running.Image != "nginx:latest" || running.State != "running" {
		t.Fatalf("unexpected container details: %+v", running)
	}
	if len(running.Ports) != 1 || running.Ports[0].PublicPort != 8080 {
		t.Fatalf("unexpected port mapping: %+v", running.Ports)
	}
	if running.CPUPercent == nil || *running.CPUPercent != 25 {
		t.Fatalf("CPU percent = %v, want 25", running.CPUPercent)
	}
	if running.MemoryPercent == nil || *running.MemoryPercent != 25 {
		t.Fatalf("memory percent = %v, want 25", running.MemoryPercent)
	}
	if inventory.Containers[1].MemoryUsageBytes != nil {
		t.Fatal("stopped container should not have live resource stats")
	}
}

func TestValidDockerAPIVersion(t *testing.T) {
	for _, version := range []string{"1.24", "1.41", "1.99"} {
		if !validDockerAPIVersion(version) {
			t.Errorf("version %q should be accepted", version)
		}
	}
	for _, version := range []string{"", "1.0", "2.41", "1.x", "1.256", "1.41.1"} {
		if validDockerAPIVersion(version) {
			t.Errorf("version %q should be rejected", version)
		}
	}
}

func TestValidDockerContainerID(t *testing.T) {
	for _, id := range []string{"0123456789ab", "0123456789abcdef0123456789abcdef"} {
		if !validDockerContainerID(id) {
			t.Errorf("container ID %q should be accepted", id)
		}
	}
	for _, id := range []string{"", "123", "../container", strings.Repeat("a", 66)} {
		if validDockerContainerID(id) {
			t.Errorf("container ID %q should be rejected", id)
		}
	}
}

func TestCollectDockerRejectsInvalidSocket(t *testing.T) {
	if _, err := collectDocker(context.Background(), ""); err == nil {
		t.Fatal("expected empty socket path error")
	}
	if _, err := collectDocker(context.Background(), filepath.Join(t.TempDir(), "missing-docker.sock")); err == nil {
		t.Fatal("expected unavailable socket error")
	}
}

func TestDockerCPUPercentMissingOrResetCounters(t *testing.T) {
	if got := dockerCPUPercent(dockerStats{}); got != nil {
		t.Fatal("missing counters should not produce a CPU percentage")
	}
	var stats dockerStats
	currentCPU, previousCPU := uint64(10), uint64(20)
	currentSystem, previousSystem := uint64(100), uint64(50)
	cpuCount := uint32(1)
	stats.CPUStats.CPUUsage.TotalUsage = &currentCPU
	stats.CPUStats.CPUUsage.PerCPU = []uint64{1}
	stats.CPUStats.OnlineCPUs = &cpuCount
	stats.CPUStats.SystemCPUUsage = &currentSystem
	stats.PreCPUStats.CPUUsage.TotalUsage = &previousCPU
	stats.PreCPUStats.SystemCPUUsage = &previousSystem
	if got := dockerCPUPercent(stats); got != nil {
		t.Fatal("reset CPU counter should not produce a percentage")
	}
}
