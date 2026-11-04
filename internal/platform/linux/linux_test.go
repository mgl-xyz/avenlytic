//go:build linux

package linux

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/avenlytic/avenlytic/internal/core/model"
)

func TestSnapshotCollectsNativeMetrics(t *testing.T) {
	p := New()
	s, err := p.Snapshot(context.Background(), "test-host")
	if err != nil {
		t.Fatal(err)
	}
	if s.CPU == nil || s.Memory == nil || s.Load == nil {
		t.Fatalf("missing base metrics: %+v", s.Warnings)
	}
	if s.ProcessCount <= 0 {
		t.Fatalf("expected process count, got %d", s.ProcessCount)
	}
	if len(s.Processes) == 0 {
		t.Fatal("expected process list")
	}
	if s.AgentID != "test-host" {
		t.Fatalf("agent id mismatch: %s", s.AgentID)
	}
}

func TestLinuxProviderCapabilities(t *testing.T) {
	p := New()
	if len(p.Capabilities()) < 5 {
		t.Fatal("expected capability metadata")
	}
}

func TestHostRootCollectionUsesHostPaths(t *testing.T) {
	root := t.TempDir()
	proc := root + "/proc"
	if err := os.MkdirAll(proc, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(proc+"/stat", []byte("cpu 10 0 0 10 0 0 0 0 0 0\ncpu0 5 0 0 5 0 0 0 0 0 0\ncpu1 5 0 0 5 0 0 0 0 0 0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(proc+"/meminfo", []byte("MemTotal: 1024 kB\nMemAvailable: 512 kB\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(proc+"/uptime", []byte("123.0 0.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(proc+"/loadavg", []byte("0.10 0.20 0.30 1/100 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(proc+"/cpuinfo", []byte("vendor_id : GenuineTest\nmodel name : Test CPU 3000\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sysRoot := root + "/sys"
	hwmon := sysRoot + "/class/hwmon/hwmon0"
	cgroupRoot := sysRoot + "/fs/cgroup"
	if err := os.MkdirAll(hwmon, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(cgroupRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	for path, content := range map[string]string{
		hwmon + "/name":                    "coretemp\n",
		hwmon + "/temp1_input":             "42000\n",
		cgroupRoot + "/cgroup.controllers": "cpu memory pids\n",
		cgroupRoot + "/memory.current":     "4096\n",
	} {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(proc+"/1/net", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(proc+"/1/net/dev", []byte("header\nheader\neth0: 100 0 0 0 0 0 0 0 200 0 0 0 0 0 0 0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(proc+"/diskstats", []byte("8 0 sda 1 0 2 0 3 0 4 0 0 0 0 0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(proc+"/1", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(proc+"/1/mounts", []byte("host-root / ext4 rw 0 0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(root+"/etc", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(root+"/etc/os-release", []byte("PRETTY_NAME=\"Host Linux\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	p := New()
	if err := p.SetHostRoot(root); err != nil {
		t.Fatal(err)
	}
	cpu, err := p.cpu()
	if err != nil {
		t.Fatal(err)
	}
	if cpu.Cores != 2 {
		t.Fatalf("expected host CPU core count 2, got %d", cpu.Cores)
	}
	mem, err := memory(p.procRoot)
	if err != nil {
		t.Fatal(err)
	}
	if mem.TotalBytes != 1024*1024 || mem.AvailableBytes != 512*1024 {
		t.Fatalf("unexpected host memory values: %+v", mem)
	}
	uptime, err := readUptime(p.procRoot)
	if err != nil || uptime != 123 {
		t.Fatalf("unexpected host uptime %d, error %v", uptime, err)
	}
	if got := readPrettyName(p.hostRoot); got != "Host Linux" {
		t.Fatalf("expected host distribution name, got %q", got)
	}
	disks := disks(p.procRoot, p.hostRoot)
	if len(disks) != 1 || disks[0].Mountpoint != "/" {
		t.Fatalf("expected host root filesystem, got %+v", disks)
	}
	snapshot, err := p.Snapshot(context.Background(), "host")
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Platform != "Host Linux" || snapshot.Memory == nil || snapshot.Load == nil ||
		len(snapshot.Networks) != 1 || len(snapshot.DiskIO) != 1 {
		t.Fatalf("snapshot did not use host metric sources: %+v", snapshot)
	}
	if snapshot.CPU.Model != "Test CPU 3000" || snapshot.CPU.Vendor != "GenuineTest" {
		t.Fatalf("snapshot did not include host CPU identity: %+v", snapshot.CPU)
	}
	if snapshot.Hardware == nil || len(snapshot.Hardware.Sensors) != 1 || snapshot.Cgroup == nil ||
		snapshot.Cgroup.Version != "v2" || snapshot.Cgroup.MemoryUsageBytes == nil || *snapshot.Cgroup.MemoryUsageBytes != 4096 {
		t.Fatalf("snapshot did not include host sysfs and cgroup metrics: hardware=%+v cgroup=%+v", snapshot.Hardware, snapshot.Cgroup)
	}
}

func TestCollectHardware(t *testing.T) {
	root := t.TempDir()
	hwmon := root + "/class/hwmon/hwmon0"
	cpuFreq := root + "/devices/system/cpu/cpu0/cpufreq"
	for _, path := range []string{hwmon, cpuFreq} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for path, content := range map[string]string{
		hwmon + "/name":               "coretemp\n",
		hwmon + "/temp1_input":        "42000\n",
		hwmon + "/temp1_label":        "Package id 0\n",
		hwmon + "/temp1_crit":         "100000\n",
		hwmon + "/fan1_input":         "1200\n",
		cpuFreq + "/scaling_cur_freq": "2400000\n",
		cpuFreq + "/cpuinfo_min_freq": "800000\n",
		cpuFreq + "/cpuinfo_max_freq": "3600000\n",
	} {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	hardware, err := collectHardware(root)
	if err != nil {
		t.Fatal(err)
	}
	if hardware == nil || len(hardware.Sensors) != 2 {
		t.Fatalf("expected temperature and fan sensors, got %+v", hardware)
	}
	var temperature *model.HardwareSensor
	for i := range hardware.Sensors {
		if hardware.Sensors[i].Kind == "temperature" {
			temperature = &hardware.Sensors[i]
			break
		}
	}
	if temperature == nil || temperature.Name != "coretemp Package id 0" ||
		temperature.Value != 42 || temperature.Critical == nil || *temperature.Critical != 100 {
		t.Fatalf("unexpected temperature sensor: %+v", hardware.Sensors)
	}
	current, minimum, maximum := cpuFrequencies(root)
	if current != 2400 || minimum != 800 || maximum != 3600 {
		t.Fatalf("unexpected CPU frequencies: current=%v min=%v max=%v", current, minimum, maximum)
	}
}

func TestCollectThermalSensorsFallback(t *testing.T) {
	root := t.TempDir()
	zone := root + "/class/thermal/thermal_zone0"
	if err := os.MkdirAll(zone, 0o755); err != nil {
		t.Fatal(err)
	}
	for path, content := range map[string]string{
		zone + "/temp":              "55000\n",
		zone + "/type":              "acpitz\n",
		zone + "/trip_point_0_temp": "95000\n",
		zone + "/trip_point_0_type": "critical\n",
	} {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	hardware, err := collectHardware(root)
	if err != nil {
		t.Fatal(err)
	}
	if hardware == nil || len(hardware.Sensors) != 1 || hardware.Sensors[0].Name != "acpitz" ||
		hardware.Sensors[0].Value != 55 || hardware.Sensors[0].Critical == nil || *hardware.Sensors[0].Critical != 95 {
		t.Fatalf("unexpected thermal-zone fallback data: %+v", hardware)
	}
}

func TestCollectHardwareReportsNoSensors(t *testing.T) {
	hardware, err := collectHardware(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if hardware != nil {
		t.Fatalf("expected no hardware sensor inventory, got %+v", hardware)
	}
}

func TestCollectCgroupV2(t *testing.T) {
	root := t.TempDir()
	for path, content := range map[string]string{
		root + "/cgroup.controllers": "cpu memory pids\n",
		root + "/cpu.stat":           "usage_usec 123456\nuser_usec 100000\n",
		root + "/cpu.max":            "200000 100000\n",
		root + "/memory.current":     "4096\n",
		root + "/memory.max":         "8192\n",
		root + "/pids.current":       "5\n",
		root + "/pids.max":           "100\n",
		root + "/io.stat":            "8:0 rbytes=1024 wbytes=2048 rios=1 wios=2\n8:16 rbytes=512 wbytes=256\n",
	} {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cgroup, err := collectCgroup(root)
	if err != nil {
		t.Fatal(err)
	}
	if cgroup == nil || cgroup.Version != "v2" || cgroup.CPUUsageUsec == nil || *cgroup.CPUUsageUsec != 123456 ||
		cgroup.CPUQuotaCores == nil || *cgroup.CPUQuotaCores != 2 ||
		cgroup.MemoryUsageBytes == nil || *cgroup.MemoryUsageBytes != 4096 ||
		cgroup.MemoryLimitBytes == nil || *cgroup.MemoryLimitBytes != 8192 ||
		cgroup.PIDsCurrent == nil || *cgroup.PIDsCurrent != 5 ||
		cgroup.PIDsLimit == nil || *cgroup.PIDsLimit != 100 ||
		cgroup.IOReadBytes == nil || *cgroup.IOReadBytes != 1536 ||
		cgroup.IOWriteBytes == nil || *cgroup.IOWriteBytes != 2304 {
		t.Fatalf("unexpected cgroup v2 stats: %+v", cgroup)
	}
}

func TestCollectCgroupV1SeparateCPUControllers(t *testing.T) {
	root := t.TempDir()
	for _, path := range []string{root + "/cpu", root + "/cpuacct", root + "/blkio"} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for path, content := range map[string]string{
		root + "/cpu/cpu.cfs_quota_us":                  "25000\n",
		root + "/cpu/cpu.cfs_period_us":                 "100000\n",
		root + "/cpuacct/cpuacct.usage":                 "1000000\n",
		root + "/blkio/blkio.throttle.io_service_bytes": "8:0 Read 1024\n8:0 Write 2048\nTotal Read 1024\nTotal Write 2048\n",
	} {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cgroup, err := collectCgroup(root)
	if err != nil {
		t.Fatal(err)
	}
	if cgroup == nil || cgroup.CPUQuotaCores == nil || *cgroup.CPUQuotaCores != 0.25 ||
		cgroup.CPUUsageUsec == nil || *cgroup.CPUUsageUsec != 1000 ||
		cgroup.IOReadBytes == nil || *cgroup.IOReadBytes != 1024 ||
		cgroup.IOWriteBytes == nil || *cgroup.IOWriteBytes != 2048 {
		t.Fatalf("unexpected separate cgroup v1 CPU controllers: %+v", cgroup)
	}
}

func TestCollectCgroupV1(t *testing.T) {
	root := t.TempDir()
	cpuRoot, memoryRoot, pidsRoot := root+"/cpu,cpuacct", root+"/memory", root+"/pids"
	for _, path := range []string{cpuRoot, memoryRoot, pidsRoot} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for path, content := range map[string]string{
		cpuRoot + "/cpuacct.usage":            "2500000\n",
		cpuRoot + "/cpu.cfs_quota_us":         "50000\n",
		cpuRoot + "/cpu.cfs_period_us":        "100000\n",
		memoryRoot + "/memory.usage_in_bytes": "4096\n",
		memoryRoot + "/memory.limit_in_bytes": "8192\n",
		pidsRoot + "/pids.current":            "3\n",
		pidsRoot + "/pids.max":                "max\n",
		root + "/blkio.io_service_bytes":      "8:0 Read 4096\n8:0 Write 8192\n",
	} {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cgroup, err := collectCgroup(root)
	if err != nil {
		t.Fatal(err)
	}
	if cgroup == nil || cgroup.Version != "v1" || cgroup.CPUUsageUsec == nil || *cgroup.CPUUsageUsec != 2500 ||
		cgroup.CPUQuotaCores == nil || *cgroup.CPUQuotaCores != 0.5 ||
		cgroup.MemoryLimitBytes == nil || *cgroup.MemoryLimitBytes != 8192 ||
		cgroup.PIDsCurrent == nil || *cgroup.PIDsCurrent != 3 || cgroup.PIDsLimit != nil ||
		cgroup.IOReadBytes == nil || *cgroup.IOReadBytes != 4096 ||
		cgroup.IOWriteBytes == nil || *cgroup.IOWriteBytes != 8192 {
		t.Fatalf("unexpected cgroup v1 stats: %+v", cgroup)
	}
}

func TestSetHostRootRequiresProcMount(t *testing.T) {
	p := New()
	if err := p.SetHostRoot(t.TempDir()); err == nil {
		t.Fatal("expected missing host proc mount to fail")
	}
}

func TestCountRecentAuthLogFailures(t *testing.T) {
	now := time.Date(2026, time.November, 3, 19, 55, 0, 0, time.Local)
	log := strings.Join([]string{
		"Nov  3 19:50:00 host sshd: Failed password for invalid user root",
		"Nov  3 19:20:00 host sshd: Invalid user test",
		"Nov  3 19:56:00 host sshd: authentication failure",
		"Nov  3 19:53:00 host sshd: Accepted publickey for admin",
	}, "\n")
	file, err := os.CreateTemp(t.TempDir(), "auth.log")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString(log); err != nil {
		t.Fatal(err)
	}
	if _, err := file.Seek(0, 0); err != nil {
		t.Fatal(err)
	}
	count, err := countRecentAuthLogFailures(file, now, 15*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected one recent failed login, got %d", count)
	}
	_ = file.Close()
}

func TestCountRPMUpdates(t *testing.T) {
	output := "Updating Subscription Management repositories.\n\nopenssl.x86_64 1:3.0.0 repo\ncurl.x86_64 8.0.0 repo\n"
	if got := countRPMUpdates(output); got != 2 {
		t.Fatalf("expected two updates, got %d", got)
	}
}

func TestOSVEcosystem(t *testing.T) {
	for distribution, want := range map[string]string{"debian": "Debian", "ubuntu": "Ubuntu", "fedora": "Fedora", "rocky": "Rocky Linux"} {
		if got := osvEcosystem(distribution); got != want {
			t.Errorf("%s: got %q, want %q", distribution, got, want)
		}
	}
	if got := osvEcosystem("unknown"); got != "" {
		t.Fatalf("unsupported distribution should not be matched, got %q", got)
	}
}
