//go:build linux

package linux

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/avenlytic/avenlytic/internal/core/model"
	ports "github.com/avenlytic/avenlytic/internal/core/ports"
)

type Provider struct {
	prevTotal, prevIdle uint64
	prevAt              time.Time
	prevProc            map[int]uint64
	prevProcAt          time.Time
	hostRoot            string
	procRoot            string
	sysRoot             string
	securityMu          sync.Mutex
	securityCache       securityCache
}

func New() *Provider {
	return &Provider{prevProc: map[int]uint64{}, hostRoot: "/", procRoot: "/proc", sysRoot: "/sys"}
}
func (*Provider) ID() string { return "linux" }

func (p *Provider) SetHostRoot(root string) error {
	if !filepath.IsAbs(root) {
		return fmt.Errorf("host root must be an absolute path")
	}
	root = filepath.Clean(root)
	info, err := os.Stat(root)
	if err != nil {
		return fmt.Errorf("inspect host root: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("host root is not a directory")
	}
	procRoot := filepath.Join(root, "proc")
	info, err = os.Stat(procRoot)
	if err != nil {
		return fmt.Errorf("inspect host proc mount %q: %w", procRoot, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("host proc mount %q is not a directory", procRoot)
	}
	p.hostRoot = root
	p.procRoot = procRoot
	p.sysRoot = filepath.Join(root, "sys")
	return nil
}

func (p *Provider) SetSysRoot(root string) error {
	if !filepath.IsAbs(root) {
		return fmt.Errorf("sys root must be an absolute path")
	}
	info, err := os.Stat(root)
	if err != nil {
		return fmt.Errorf("inspect sys root: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("sys root is not a directory")
	}
	p.sysRoot = filepath.Clean(root)
	return nil
}

func (p *Provider) Snapshot(ctx context.Context, hostname string) (model.Snapshot, error) {
	s := model.Snapshot{Protocol: model.ProtocolVersion, AgentID: hostname, Hostname: hostname, OS: "linux", Arch: runtime.GOARCH, Platform: readPrettyName(p.hostRoot), Collected: time.Now().UTC(), Capabilities: []string{"cpu", "memory", "load", "network", "disk_capacity", "disk_io", "process_count", "processes"}}
	if v, e := readUptime(p.procRoot); e == nil {
		s.UptimeSec = v
	} else {
		s.Warnings = append(s.Warnings, "uptime: "+e.Error())
	}
	if v, e := p.cpu(); e == nil {
		s.CPU = v
		if v.Model != "" || v.Vendor != "" || v.FrequencyMHz > 0 || v.MaxFrequencyMHz > 0 {
			s.Capabilities = append(s.Capabilities, "cpu_details")
		}
	} else {
		s.Warnings = append(s.Warnings, "cpu: "+e.Error())
	}
	if hardware, err := collectHardware(p.sysRoot); err != nil {
		s.Warnings = append(s.Warnings, "hardware: "+err.Error())
	} else if hardware != nil {
		s.Hardware = hardware
		s.Capabilities = append(s.Capabilities, "hardware_sensors")
	} else {
		s.Warnings = append(s.Warnings, "hardware: no readable hwmon or thermal sensors")
	}
	if v, e := memory(p.procRoot); e == nil {
		s.Memory = v
	} else {
		s.Warnings = append(s.Warnings, "memory: "+e.Error())
	}
	if v, e := load(p.procRoot); e == nil {
		s.Load = v
	} else {
		s.Warnings = append(s.Warnings, "load: "+e.Error())
	}
	s.Networks = networks(p.procRoot, p.hostRoot)
	s.Disks = disks(p.procRoot, p.hostRoot)
	s.DiskIO = diskIO(p.procRoot)
	s.ProcessCount = processCount(p.procRoot)
	s.Processes = p.processes(p.procRoot)
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	s.Runtime = model.RuntimeStats{Goroutines: runtime.NumGoroutine(), HeapAlloc: m.HeapAlloc}
	return s, nil
}
func (p *Provider) Capabilities() []ports.Capability {
	ids := []string{"cpu", "memory", "load", "uptime", "network", "disk_capacity", "disk_io", "process_count", "processes"}
	out := make([]ports.Capability, 0, len(ids))
	for _, id := range ids {
		out = append(out, ports.Capability{ID: id, Name: id, Available: true})
	}
	return out
}
func (p *Provider) CollectCPU(ctx context.Context) (*model.CPU, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return p.cpu()
}
func (p *Provider) CollectMemory(ctx context.Context) (*model.Memory, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return memory(p.procRoot)
}
func (p *Provider) CollectDisks(ctx context.Context) ([]model.Disk, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return disks(p.procRoot, p.hostRoot), nil
}
func (p *Provider) CollectDiskIO(ctx context.Context) ([]model.DiskIO, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return diskIO(p.procRoot), nil
}
func (p *Provider) CollectNetworks(ctx context.Context) ([]model.Network, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return networks(p.procRoot, p.hostRoot), nil
}
func (p *Provider) CollectProcesses(ctx context.Context) ([]model.Process, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return p.processes(p.procRoot), nil
}
func (p *Provider) ProcessCount(ctx context.Context) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	return processCount(p.procRoot), nil
}
func (p *Provider) CollectLoad(ctx context.Context) (*model.Load, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return load(p.procRoot)
}
func (p *Provider) Uptime(ctx context.Context) (uint64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	return readUptime(p.procRoot)
}

func (p *Provider) CollectDocker(ctx context.Context, socketPath string) (model.DockerInventory, error) {
	return collectDocker(ctx, socketPath)
}

func (p *Provider) cpu() (*model.CPU, error) {
	f, e := os.Open(filepath.Join(p.procRoot, "stat"))
	if e != nil {
		return nil, e
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	if !sc.Scan() {
		return nil, fmt.Errorf("empty /proc/stat")
	}
	fields := strings.Fields(sc.Text())
	if len(fields) < 5 {
		return nil, fmt.Errorf("invalid cpu line")
	}
	var vals []uint64
	for _, x := range fields[1:] {
		v, err := strconv.ParseUint(x, 10, 64)
		if err != nil {
			return nil, err
		}
		vals = append(vals, v)
	}
	var total, idle uint64
	for i, v := range vals {
		total += v
		if i == 3 || i == 4 {
			idle += v
		}
	}
	now := time.Now()
	usage := 0.0
	if p.prevAt.IsZero() {
		p.prevTotal, p.prevIdle, p.prevAt = total, idle, now
	} else if total > p.prevTotal && idle >= p.prevIdle {
		dt := total - p.prevTotal
		di := idle - p.prevIdle
		if dt > 0 {
			usage = 100 * float64(dt-di) / float64(dt)
		}
		p.prevTotal, p.prevIdle, p.prevAt = total, idle, now
	}
	cpuModel, vendor, procFrequency := cpuIdentity(p.procRoot)
	current, minimum, maximum := cpuFrequencies(p.sysRoot)
	if current == 0 {
		current = procFrequency
	}
	return &model.CPU{
		UsagePercent:    usage,
		Cores:           cpuCoreCount(p.procRoot),
		Model:           cpuModel,
		Vendor:          vendor,
		FrequencyMHz:    current,
		MinFrequencyMHz: minimum,
		MaxFrequencyMHz: maximum,
	}, nil
}

func cpuCoreCount(procRoot string) int {
	file, err := os.Open(filepath.Join(procRoot, "stat"))
	if err != nil {
		return runtime.NumCPU()
	}
	defer file.Close()
	cores := 0
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		name := strings.Fields(scanner.Text())
		if len(name) > 0 && strings.HasPrefix(name[0], "cpu") && name[0] != "cpu" {
			if _, err := strconv.Atoi(strings.TrimPrefix(name[0], "cpu")); err == nil {
				cores++
			}
		}
	}
	if cores == 0 {
		return runtime.NumCPU()
	}
	return cores
}
func memory(procRoot string) (*model.Memory, error) {
	f, e := os.Open(filepath.Join(procRoot, "meminfo"))
	if e != nil {
		return nil, e
	}
	defer f.Close()
	vals := map[string]uint64{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		a := strings.Fields(sc.Text())
		if len(a) >= 2 {
			v, _ := strconv.ParseUint(a[1], 10, 64)
			vals[strings.TrimSuffix(a[0], ":")] = v * 1024
		}
	}
	total := vals["MemTotal"]
	avail := vals["MemAvailable"]
	if total == 0 {
		return nil, fmt.Errorf("MemTotal unavailable")
	}
	used := total - avail
	return &model.Memory{TotalBytes: total, AvailableBytes: avail, UsedBytes: used, UsagePercent: 100 * float64(used) / float64(total)}, nil
}
func load(procRoot string) (*model.Load, error) {
	b, e := os.ReadFile(filepath.Join(procRoot, "loadavg"))
	if e != nil {
		return nil, e
	}
	a := strings.Fields(string(b))
	if len(a) < 3 {
		return nil, fmt.Errorf("invalid loadavg")
	}
	v := make([]float64, 3)
	for i := range v {
		v[i], e = strconv.ParseFloat(a[i], 64)
		if e != nil {
			return nil, e
		}
	}
	return &model.Load{One: v[0], Five: v[1], Fifteen: v[2]}, nil
}
func networks(procRoot, hostRoot string) []model.Network {
	netPath := filepath.Join(procRoot, "net/dev")
	if hostRoot != "/" {
		netPath = filepath.Join(procRoot, "1/net/dev")
	}
	f, e := os.Open(netPath)
	if e != nil {
		return nil
	}
	defer f.Close()
	var out []model.Network
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if !strings.Contains(line, ":") {
			continue
		}
		a := strings.SplitN(line, ":", 2)
		v := strings.Fields(a[1])
		if len(v) < 9 {
			continue
		}
		rx, _ := strconv.ParseUint(v[0], 10, 64)
		tx, _ := strconv.ParseUint(v[8], 10, 64)
		name := strings.TrimSpace(a[0])
		if name != "lo" {
			out = append(out, model.Network{Name: name, RXBytes: rx, TXBytes: tx})
		}
	}
	return out
}
func disks(procRoot, hostRoot string) []model.Disk {
	mountsPath := filepath.Join(procRoot, "mounts")
	if hostRoot != "/" {
		mountsPath = filepath.Join(procRoot, "1/mounts")
	}
	f, e := os.Open(mountsPath)
	if e != nil {
		return nil
	}
	defer f.Close()
	skip := map[string]bool{"proc": true, "sysfs": true, "devtmpfs": true, "devpts": true, "tmpfs": true, "cgroup": true, "cgroup2": true, "overlay": true, "squashfs": true, "tracefs": true, "securityfs": true, "pstore": true, "debugfs": true, "mqueue": true, "fusectl": true, "configfs": true}
	seen := map[string]bool{}
	var out []model.Disk
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		a := strings.Fields(sc.Text())
		if len(a) < 3 || skip[a[2]] {
			continue
		}
		mount := strings.ReplaceAll(strings.ReplaceAll(a[1], "\\040", " "), "\\134", "\\")
		if seen[mount] {
			continue
		}
		seen[mount] = true
		statPath := mount
		if hostRoot != "/" {
			statPath = filepath.Join(hostRoot, strings.TrimPrefix(mount, "/"))
		}
		var st syscall.Statfs_t
		if syscall.Statfs(statPath, &st) != nil {
			continue
		}
		total := st.Blocks * uint64(st.Bsize)
		free := st.Bavail * uint64(st.Bsize)
		if total == 0 {
			continue
		}
		used := total - free
		out = append(out, model.Disk{Device: filepath.Base(a[0]), Mountpoint: mount, TotalBytes: total, FreeBytes: free, UsedBytes: used, UsagePercent: 100 * float64(used) / float64(total)})
	}
	return out
}
func diskIO(procRoot string) []model.DiskIO {
	b, e := os.ReadFile(filepath.Join(procRoot, "diskstats"))
	if e != nil {
		return nil
	}
	var out []model.DiskIO
	for _, line := range strings.Split(string(b), "\n") {
		a := strings.Fields(line)
		if len(a) < 14 {
			continue
		}
		name := a[2]
		if strings.HasPrefix(name, "loop") || strings.HasPrefix(name, "ram") {
			continue
		}
		reads, _ := strconv.ParseUint(a[5], 10, 64)
		writes, _ := strconv.ParseUint(a[9], 10, 64)
		out = append(out, model.DiskIO{Device: name, ReadBytes: reads * 512, WriteBytes: writes * 512})
	}
	return out
}
func processCount(procRoot string) int {
	entries, e := os.ReadDir(procRoot)
	if e != nil {
		return 0
	}
	n := 0
	for _, x := range entries {
		if !x.IsDir() {
			continue
		}
		if _, e := strconv.Atoi(x.Name()); e == nil {
			n++
		}
	}
	return n
}
func (p *Provider) processes(procRoot string) []model.Process {
	entries, err := os.ReadDir(procRoot)
	if err != nil {
		return nil
	}
	now := time.Now()
	elapsed := now.Sub(p.prevProcAt).Seconds()
	cores := cpuCoreCount(procRoot)
	current := map[int]uint64{}
	var out []model.Process
	for _, entry := range entries {
		pid, e := strconv.Atoi(entry.Name())
		if e != nil || !entry.IsDir() {
			continue
		}
		b, e := os.ReadFile(filepath.Join(procRoot, entry.Name(), "stat"))
		if e != nil {
			continue
		}
		line := string(b)
		right := strings.LastIndex(line, ")")
		if right < 0 {
			continue
		}
		left := strings.Index(line, "(")
		name := strings.TrimSpace(line[left+1 : right])
		rest := strings.Fields(line[right+1:])
		if len(rest) < 22 {
			continue
		}
		state := rest[0]
		ut, _ := strconv.ParseUint(rest[11], 10, 64)
		st, _ := strconv.ParseUint(rest[12], 10, 64)
		ticks := ut + st
		current[pid] = ticks
		cpu := 0.0
		if prev, ok := p.prevProc[pid]; ok && elapsed > 0 && ticks >= prev {
			cpu = 100 * float64(ticks-prev) / 100 / elapsed / float64(cores)
		}
		status, _ := os.ReadFile(filepath.Join(procRoot, entry.Name(), "status"))
		rss := uint64(0)
		for _, l := range strings.Split(string(status), "\n") {
			if strings.HasPrefix(l, "VmRSS:") {
				f := strings.Fields(l)
				if len(f) > 1 {
					v, _ := strconv.ParseUint(f[1], 10, 64)
					rss = v * 1024
				}
				break
			}
		}
		out = append(out, model.Process{PID: pid, Name: name, State: state, RSSBytes: rss, CPUPercent: cpu})
	}
	p.prevProc = current
	p.prevProcAt = now
	sort.Slice(out, func(i, j int) bool { return out[i].CPUPercent > out[j].CPUPercent })
	if len(out) > 20 {
		out = out[:20]
	}
	return out
}

func readUptime(procRoot string) (uint64, error) {
	b, e := os.ReadFile(filepath.Join(procRoot, "uptime"))
	if e != nil {
		return 0, e
	}
	a := strings.Fields(string(b))
	if len(a) == 0 {
		return 0, fmt.Errorf("empty uptime")
	}
	v, e := strconv.ParseFloat(a[0], 64)
	return uint64(v), e
}
func readPrettyName(hostRoot string) string {
	b, err := os.ReadFile(filepath.Join(hostRoot, "etc/os-release"))
	if err != nil {
		return "Linux"
	}
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(line, "PRETTY_NAME=") {
			return strings.Trim(strings.TrimPrefix(line, "PRETTY_NAME="), "\"")
		}
	}
	return "Linux"
}
