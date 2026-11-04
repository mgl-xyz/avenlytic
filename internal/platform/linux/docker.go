//go:build linux

package linux

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/avenlytic/avenlytic/internal/core/model"
)

const (
	dockerRequestTimeout = 4 * time.Second
	dockerResponseLimit  = 4 << 20
	dockerStatsWorkers   = 8
)

type dockerVersion struct {
	APIVersion string `json:"ApiVersion"`
}

type dockerContainerSummary struct {
	ID      string   `json:"Id"`
	Names   []string `json:"Names"`
	Image   string   `json:"Image"`
	State   string   `json:"State"`
	Status  string   `json:"Status"`
	Created int64    `json:"Created"`
	Ports   []struct {
		IP          string `json:"IP"`
		PrivatePort uint16 `json:"PrivatePort"`
		PublicPort  uint16 `json:"PublicPort"`
		Type        string `json:"Type"`
	} `json:"Ports"`
}

type dockerStats struct {
	CPUStats struct {
		CPUUsage struct {
			TotalUsage *uint64  `json:"total_usage"`
			PerCPU     []uint64 `json:"percpu_usage"`
		} `json:"cpu_usage"`
		SystemCPUUsage *uint64 `json:"system_cpu_usage"`
		OnlineCPUs     *uint32 `json:"online_cpus"`
	} `json:"cpu_stats"`
	PreCPUStats struct {
		CPUUsage struct {
			TotalUsage *uint64 `json:"total_usage"`
		} `json:"cpu_usage"`
		SystemCPUUsage *uint64 `json:"system_cpu_usage"`
	} `json:"precpu_stats"`
	MemoryStats struct {
		Usage *uint64 `json:"usage"`
		Limit *uint64 `json:"limit"`
	} `json:"memory_stats"`
}

type dockerAPI struct {
	client  *http.Client
	version string
}

func collectDocker(ctx context.Context, socketPath string) (model.DockerInventory, error) {
	if strings.TrimSpace(socketPath) == "" {
		return model.DockerInventory{}, fmt.Errorf("Docker socket path is empty")
	}

	client := &http.Client{
		Timeout: dockerRequestTimeout,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(ctx, "unix", socketPath)
			},
		},
	}
	defer client.CloseIdleConnections()

	api := dockerAPI{client: client}
	var version dockerVersion
	if err := api.get(ctx, "/version", &version); err != nil {
		return model.DockerInventory{}, fmt.Errorf("read Docker API version: %w", err)
	}
	if !validDockerAPIVersion(version.APIVersion) {
		return model.DockerInventory{}, fmt.Errorf("Docker returned invalid API version %q", version.APIVersion)
	}
	api.version = version.APIVersion

	var summaries []dockerContainerSummary
	if err := api.get(ctx, api.path("/containers/json?all=1&limit=100"), &summaries); err != nil {
		return model.DockerInventory{}, fmt.Errorf("list Docker containers: %w", err)
	}

	inventory := model.DockerInventory{
		Available:  true,
		Containers: make([]model.DockerContainer, len(summaries)),
	}
	for i, summary := range summaries {
		name := summary.ID
		if len(name) > 12 {
			name = name[:12]
		}
		if len(summary.Names) > 0 {
			name = strings.TrimPrefix(summary.Names[0], "/")
		}
		container := model.DockerContainer{
			ID:        summary.ID,
			Name:      name,
			Image:     summary.Image,
			State:     summary.State,
			Status:    summary.Status,
			CreatedAt: time.Unix(summary.Created, 0).UTC(),
			Ports:     make([]model.DockerPort, 0, len(summary.Ports)),
		}
		for _, port := range summary.Ports {
			container.Ports = append(container.Ports, model.DockerPort{
				IP:          port.IP,
				PrivatePort: port.PrivatePort,
				PublicPort:  port.PublicPort,
				Type:        port.Type,
			})
		}
		inventory.Containers[i] = container
	}

	collectDockerStats(ctx, &api, inventory.Containers)
	return inventory, nil
}

func (api *dockerAPI) path(path string) string {
	return "/v" + api.version + path
}

func (api *dockerAPI) get(ctx context.Context, path string, result any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://docker"+path, nil)
	if err != nil {
		return err
	}
	resp, err := api.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("Docker API returned %s", resp.Status)
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, dockerResponseLimit)).Decode(result); err != nil {
		return fmt.Errorf("decode Docker response: %w", err)
	}
	return nil
}

func validDockerAPIVersion(version string) bool {
	parts := strings.Split(version, ".")
	if len(parts) != 2 || parts[0] != "1" {
		return false
	}
	minor, err := strconv.ParseUint(parts[1], 10, 8)
	return err == nil && minor > 0
}

func collectDockerStats(ctx context.Context, api *dockerAPI, containers []model.DockerContainer) {
	type workItem struct {
		index int
		id    string
	}

	jobs := make(chan workItem)
	var workers sync.WaitGroup
	workerCount := min(dockerStatsWorkers, len(containers))
	for range workerCount {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for job := range jobs {
				var stats dockerStats
				path := api.path("/containers/" + url.PathEscape(job.id) + "/stats?stream=false")
				if err := api.get(ctx, path, &stats); err != nil {
					containers[job.index].StatsError = err.Error()
					continue
				}
				containers[job.index].CPUPercent = dockerCPUPercent(stats)
				if stats.MemoryStats.Usage != nil {
					usage := *stats.MemoryStats.Usage
					containers[job.index].MemoryUsageBytes = &usage
				}
				if stats.MemoryStats.Limit != nil {
					limit := *stats.MemoryStats.Limit
					containers[job.index].MemoryLimitBytes = &limit
				}
				if stats.MemoryStats.Usage != nil && stats.MemoryStats.Limit != nil && *stats.MemoryStats.Limit > 0 {
					percent := float64(*stats.MemoryStats.Usage) * 100 / float64(*stats.MemoryStats.Limit)
					containers[job.index].MemoryPercent = &percent
				}
			}
		}()
	}

	for i, container := range containers {
		if container.State != "running" {
			continue
		}
		if !validDockerContainerID(container.ID) {
			containers[i].StatsError = "Docker returned an invalid container ID"
			continue
		}
		select {
		case jobs <- workItem{index: i, id: container.ID}:
		case <-ctx.Done():
			containers[i].StatsError = ctx.Err().Error()
		}
	}
	close(jobs)
	workers.Wait()
}

func validDockerContainerID(id string) bool {
	if len(id) < 12 || len(id) > 64 || len(id)%2 != 0 {
		return false
	}
	_, err := hex.DecodeString(id)
	return err == nil
}

func dockerCPUPercent(stats dockerStats) *float64 {
	if stats.CPUStats.CPUUsage.TotalUsage == nil || stats.PreCPUStats.CPUUsage.TotalUsage == nil ||
		stats.CPUStats.SystemCPUUsage == nil || stats.PreCPUStats.SystemCPUUsage == nil {
		return nil
	}
	currentCPU, previousCPU := *stats.CPUStats.CPUUsage.TotalUsage, *stats.PreCPUStats.CPUUsage.TotalUsage
	currentSystem, previousSystem := *stats.CPUStats.SystemCPUUsage, *stats.PreCPUStats.SystemCPUUsage
	if currentCPU < previousCPU || currentSystem <= previousSystem {
		return nil
	}
	cpuDelta := currentCPU - previousCPU
	systemDelta := currentSystem - previousSystem
	cpuCount := uint32(len(stats.CPUStats.CPUUsage.PerCPU))
	if stats.CPUStats.OnlineCPUs != nil && *stats.CPUStats.OnlineCPUs > 0 {
		cpuCount = *stats.CPUStats.OnlineCPUs
	}
	if cpuCount == 0 {
		return nil
	}
	percent := float64(cpuDelta) / float64(systemDelta) * float64(cpuCount) * 100
	return &percent
}
