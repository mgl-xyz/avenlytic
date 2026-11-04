package model

import "time"

const ProtocolVersion = "v1"

type Snapshot struct {
	Protocol     string           `json:"protocol"`
	AgentID      string           `json:"agent_id"`
	Hostname     string           `json:"hostname"`
	OS           string           `json:"os"`
	Arch         string           `json:"arch"`
	Platform     string           `json:"platform"`
	Collected    time.Time        `json:"collected_at"`
	UptimeSec    uint64           `json:"uptime_seconds,omitempty"`
	CPU          *CPU             `json:"cpu,omitempty"`
	Hardware     *Hardware        `json:"hardware,omitempty"`
	Memory       *Memory          `json:"memory,omitempty"`
	Load         *Load            `json:"load,omitempty"`
	Networks     []Network        `json:"networks,omitempty"`
	Disks        []Disk           `json:"disks,omitempty"`
	DiskIO       []DiskIO         `json:"disk_io,omitempty"`
	ProcessCount int              `json:"process_count,omitempty"`
	Processes    []Process        `json:"processes,omitempty"`
	Docker       *DockerInventory `json:"docker,omitempty"`
	Runtime      RuntimeStats     `json:"runtime"`
	Capabilities []string         `json:"capabilities"`
	Warnings     []string         `json:"warnings,omitempty"`
}

type DockerInventory struct {
	Available  bool              `json:"available"`
	Error      string            `json:"error,omitempty"`
	Containers []DockerContainer `json:"containers"`
}

type DockerContainer struct {
	ID               string       `json:"id"`
	Name             string       `json:"name"`
	Image            string       `json:"image"`
	State            string       `json:"state"`
	Status           string       `json:"status"`
	CreatedAt        time.Time    `json:"created_at"`
	Ports            []DockerPort `json:"ports,omitempty"`
	CPUPercent       *float64     `json:"cpu_percent,omitempty"`
	MemoryUsageBytes *uint64      `json:"memory_usage_bytes,omitempty"`
	MemoryLimitBytes *uint64      `json:"memory_limit_bytes,omitempty"`
	MemoryPercent    *float64     `json:"memory_percent,omitempty"`
	StatsError       string       `json:"stats_error,omitempty"`
}

type DockerPort struct {
	IP          string `json:"ip,omitempty"`
	PrivatePort uint16 `json:"private_port"`
	PublicPort  uint16 `json:"public_port,omitempty"`
	Type        string `json:"type"`
}

type SecurityReport struct {
	AgentID                   string             `json:"agent_id"`
	Collected                 time.Time          `json:"collected_at"`
	FailedLoginAttempts       int                `json:"failed_login_attempts"`
	LoginWindowSeconds        int                `json:"login_window_seconds"`
	LoginMonitoringAvailable  bool               `json:"login_monitoring_available"`
	AvailableUpdates          int                `json:"available_updates"`
	UpdateCheckAvailable      bool               `json:"update_check_available"`
	UpdateCheckError          string             `json:"update_check_error,omitempty"`
	PackageInventoryAvailable bool               `json:"package_inventory_available"`
	PackageInventoryError     string             `json:"package_inventory_error,omitempty"`
	PackagesCollected         time.Time          `json:"packages_collected_at,omitempty"`
	Packages                  []InstalledPackage `json:"packages,omitempty"`
	Vulnerabilities           []Vulnerability    `json:"vulnerabilities,omitempty"`
	VulnerabilityChecked      time.Time          `json:"vulnerability_checked_at,omitempty"`
	VulnerabilityError        string             `json:"vulnerability_error,omitempty"`
}
type InstalledPackage struct {
	Name      string `json:"name"`
	Version   string `json:"version"`
	Ecosystem string `json:"ecosystem"`
}
type Vulnerability struct {
	ID      string `json:"id"`
	Summary string `json:"summary,omitempty"`
}
type CPU struct {
	UsagePercent    float64 `json:"usage_percent"`
	Cores           int     `json:"cores"`
	Model           string  `json:"model,omitempty"`
	Vendor          string  `json:"vendor,omitempty"`
	FrequencyMHz    float64 `json:"frequency_mhz,omitempty"`
	MinFrequencyMHz float64 `json:"min_frequency_mhz,omitempty"`
	MaxFrequencyMHz float64 `json:"max_frequency_mhz,omitempty"`
}
type Hardware struct {
	Sensors []HardwareSensor `json:"sensors,omitempty"`
}
type HardwareSensor struct {
	Name     string   `json:"name"`
	Kind     string   `json:"kind"`
	Value    float64  `json:"value"`
	Unit     string   `json:"unit"`
	Critical *float64 `json:"critical,omitempty"`
}
type Memory struct {
	TotalBytes     uint64  `json:"total_bytes"`
	AvailableBytes uint64  `json:"available_bytes"`
	UsedBytes      uint64  `json:"used_bytes"`
	UsagePercent   float64 `json:"usage_percent"`
}
type Load struct {
	One     float64 `json:"one"`
	Five    float64 `json:"five"`
	Fifteen float64 `json:"fifteen"`
}
type Network struct {
	Name    string `json:"name"`
	RXBytes uint64 `json:"rx_bytes"`
	TXBytes uint64 `json:"tx_bytes"`
}
type Disk struct {
	Device       string  `json:"device,omitempty"`
	Mountpoint   string  `json:"mountpoint"`
	TotalBytes   uint64  `json:"total_bytes"`
	FreeBytes    uint64  `json:"free_bytes"`
	UsedBytes    uint64  `json:"used_bytes"`
	UsagePercent float64 `json:"usage_percent"`
}
type Process struct {
	PID        int     `json:"pid"`
	Name       string  `json:"name"`
	State      string  `json:"state"`
	RSSBytes   uint64  `json:"rss_bytes"`
	CPUPercent float64 `json:"cpu_percent"`
}
type DiskIO struct {
	Device     string `json:"device"`
	ReadBytes  uint64 `json:"read_bytes"`
	WriteBytes uint64 `json:"write_bytes"`
}
type RuntimeStats struct {
	Goroutines int    `json:"goroutines"`
	HeapAlloc  uint64 `json:"heap_alloc_bytes"`
}

// Metric is the normalized unit used by independent collectors and future
// historical storage. Snapshot remains the compact v1 wire payload.
type Metric struct {
	Name      string            `json:"name"`
	Kind      string            `json:"kind"` // gauge or counter
	Value     float64           `json:"value"`
	Unit      string            `json:"unit"`
	Timestamp time.Time         `json:"timestamp"`
	Labels    map[string]string `json:"labels,omitempty"`
}
