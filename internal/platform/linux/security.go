//go:build linux

package linux

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/avenlytic/avenlytic/internal/core/model"
)

const (
	loginWindow          = 15 * time.Minute
	updateCheckInterval  = 30 * time.Minute
	packageScanInterval  = 6 * time.Hour
	maxInventoryPackages = 5000
)

type securityCache struct {
	updatedAt time.Time
	updates   int
	updateErr string
}

func (p *Provider) CollectSecurity(ctx context.Context, includePackages bool) (model.SecurityReport, error) {
	if err := ctx.Err(); err != nil {
		return model.SecurityReport{}, err
	}

	now := time.Now()
	report := model.SecurityReport{
		Collected:          now.UTC(),
		LoginWindowSeconds: int(loginWindow.Seconds()),
	}
	if p.hostRoot != "/" {
		report.UpdateCheckError = "host-root mode does not run host package-manager commands"
		report.PackageInventoryError = "host-root mode does not inspect the host package database"
		return report, nil
	}
	report.FailedLoginAttempts, report.LoginMonitoringAvailable = failedLoginCount(ctx, now)

	p.securityMu.Lock()
	if p.securityCache.updatedAt.IsZero() || now.Sub(p.securityCache.updatedAt) >= updateCheckInterval {
		p.securityCache.updates, p.securityCache.updateErr = availableUpdates(ctx)
		p.securityCache.updatedAt = now
	}
	report.AvailableUpdates = p.securityCache.updates
	report.UpdateCheckError = p.securityCache.updateErr
	report.UpdateCheckAvailable = report.UpdateCheckError == ""

	if includePackages {
		report.Packages, report.PackageInventoryError = installedPackages(ctx)
		report.PackageInventoryAvailable = report.PackageInventoryError == ""
		if report.PackageInventoryAvailable {
			report.PackagesCollected = now.UTC()
		}
	}
	p.securityMu.Unlock()

	return report, nil
}

func failedLoginCount(ctx context.Context, now time.Time) (int, bool) {
	cmdCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	output, err := exec.CommandContext(cmdCtx, "journalctl", "--since=-15 minutes", "--no-pager", "-o", "cat", "--grep=Failed password|Invalid user|authentication failure").Output()
	if err == nil {
		return countLoginFailureLines(string(output)), true
	}
	for _, path := range []string{"/var/log/auth.log", "/var/log/secure"} {
		file, openErr := os.Open(path)
		if openErr != nil {
			continue
		}
		count, scanErr := countRecentAuthLogFailures(file, now, loginWindow)
		_ = file.Close()
		if scanErr == nil {
			return count, true
		}
	}
	return 0, false
}

func countLoginFailureLines(text string) int {
	count := 0
	for _, line := range strings.Split(text, "\n") {
		if isLoginFailure(line) {
			count++
		}
	}
	return count
}

func countRecentAuthLogFailures(file *os.File, now time.Time, window time.Duration) (int, error) {
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 4096), 1<<20)
	year := now.In(time.Local).Year()
	count := 0
	for scanner.Scan() {
		line := scanner.Text()
		if !isLoginFailure(line) {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		stamp, err := time.ParseInLocation("Jan _2 15:04:05", strings.Join(fields[:3], " "), time.Local)
		if err != nil {
			continue
		}
		stamp = stamp.AddDate(year, 0, 0)
		if stamp.After(now) {
			stamp = stamp.AddDate(-1, 0, 0)
		}
		if !stamp.After(now) && now.Sub(stamp) <= window {
			count++
		}
	}
	return count, scanner.Err()
}

func isLoginFailure(line string) bool {
	lower := strings.ToLower(line)
	return strings.Contains(lower, "failed password") ||
		strings.Contains(lower, "invalid user") ||
		strings.Contains(lower, "authentication failure")
}

func availableUpdates(ctx context.Context) (int, string) {
	cmdCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if apt, err := exec.LookPath("apt"); err == nil {
		output, runErr := exec.CommandContext(cmdCtx, apt, "list", "--upgradable").CombinedOutput()
		if runErr != nil {
			return 0, fmt.Sprintf("apt update check: %v", runErr)
		}
		return strings.Count(string(output), "[upgradable from:"), ""
	}
	for _, name := range []string{"dnf", "yum"} {
		bin, err := exec.LookPath(name)
		if err != nil {
			continue
		}
		output, runErr := exec.CommandContext(cmdCtx, bin, "-q", "check-update", "--cacheonly").CombinedOutput()
		if runErr != nil {
			if exitErr, ok := runErr.(*exec.ExitError); !ok || exitErr.ExitCode() != 100 {
				return 0, fmt.Sprintf("%s update check: %v", name, runErr)
			}
		}
		return countRPMUpdates(string(output)), ""
	}
	return 0, "no supported package manager found"
}

func countRPMUpdates(output string) int {
	count := 0
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 3 && strings.Contains(fields[0], ".") && !strings.HasPrefix(fields[0], "Last") {
			count++
		}
	}
	return count
}

func installedPackages(ctx context.Context) ([]model.InstalledPackage, string) {
	osRelease, err := readOSRelease()
	if err != nil {
		return nil, "cannot identify Linux distribution"
	}
	ecosystem := osvEcosystem(osRelease["ID"])
	if ecosystem == "" {
		return nil, "public vulnerability matching is not configured for this distribution"
	}

	var output []byte
	var runErr error
	if dpkg, lookErr := exec.LookPath("dpkg-query"); lookErr == nil {
		format := "${binary:Package}\t${Version}\n"
		output, runErr = exec.CommandContext(ctx, dpkg, "-W", "-f="+format).Output()
	} else if rpm, lookErr := exec.LookPath("rpm"); lookErr == nil {
		output, runErr = exec.CommandContext(ctx, rpm, "-qa", "--qf", "%{NAME}\t%{VERSION}-%{RELEASE}\n").Output()
	} else {
		return nil, "no supported package inventory tool found"
	}
	if runErr != nil {
		return nil, fmt.Sprintf("package inventory: %v", runErr)
	}

	packages := make([]model.InstalledPackage, 0, 512)
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		fields := strings.SplitN(strings.TrimSpace(scanner.Text()), "\t", 2)
		if len(fields) != 2 || fields[0] == "" || fields[1] == "" {
			continue
		}
		name := strings.SplitN(fields[0], ":", 2)[0]
		packages = append(packages, model.InstalledPackage{Name: name, Version: fields[1], Ecosystem: ecosystem})
		if len(packages) > maxInventoryPackages {
			return nil, fmt.Sprintf("package inventory exceeds the %d-package safety limit", maxInventoryPackages)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Sprintf("package inventory: %v", err)
	}
	return packages, ""
}

func osvEcosystem(distribution string) string {
	switch strings.ToLower(distribution) {
	case "debian":
		return "Debian"
	case "ubuntu":
		return "Ubuntu"
	case "fedora":
		return "Fedora"
	case "almalinux":
		return "AlmaLinux"
	case "rocky":
		return "Rocky Linux"
	default:
		return ""
	}
}

func readOSRelease() (map[string]string, error) {
	for _, path := range []string{"/etc/os-release", "/usr/lib/os-release"} {
		file, err := os.Open(filepath.Clean(path))
		if err != nil {
			continue
		}
		values := map[string]string{}
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			key, value, ok := strings.Cut(scanner.Text(), "=")
			if !ok {
				continue
			}
			value = strings.Trim(value, `"'`)
			values[key] = value
		}
		closeErr := file.Close()
		if err := scanner.Err(); err != nil {
			return nil, err
		}
		if closeErr != nil {
			return nil, closeErr
		}
		if values["ID"] != "" {
			return values, nil
		}
	}
	return nil, fmt.Errorf("os-release not found")
}
