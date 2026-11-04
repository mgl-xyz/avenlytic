package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/avenlytic/avenlytic/internal/core/model"
	"github.com/avenlytic/avenlytic/internal/core/ports"
	"github.com/avenlytic/avenlytic/internal/platform"
)

func main() {
	server := flag.String("server", "http://127.0.0.1:9080", "Avenlytic server base URL")
	token := flag.String("token", "", "API bearer token")
	interval := flag.Duration("interval", 5*time.Second, "collection interval (minimum 1s)")
	name := flag.String("name", "", "optional host display name")
	id := flag.String("id", "", "stable agent ID override")
	dockerSocket := flag.String("docker-socket", "", "optional Docker Engine socket path (disabled by default)")
	hostRoot := flag.String("host-root", "", "optional host filesystem root for Linux containerized collection")
	sysRoot := flag.String("sys-root", "", "optional Linux sysfs root for CPU frequency and hardware sensors")
	flag.Parse()
	if *interval < time.Second {
		log.Fatal("interval must be at least 1s")
	}
	if *token == "" {
		log.Fatal("--token is required")
	}
	hostname, _ := os.Hostname()
	p := platform.Current()
	if *hostRoot != "" || *sysRoot != "" {
		hostProvider, ok := p.(ports.HostRootProvider)
		if !ok {
			log.Fatal("--host-root and --sys-root are currently supported on Linux only")
		}
		if *hostRoot != "" {
			if err := hostProvider.SetHostRoot(*hostRoot); err != nil {
				log.Fatalf("configure host root: %v", err)
			}
		}
		if *sysRoot != "" {
			if err := hostProvider.SetSysRoot(*sysRoot); err != nil {
				log.Fatalf("configure sys root: %v", err)
			}
		}
		if *hostRoot != "" {
			hostHostname, err := readHostHostname(*hostRoot)
			if *name == "" && err != nil {
				log.Fatalf("read host hostname (or provide --name): %v", err)
			}
			if err == nil {
				hostname = hostHostname
			}
		}
	}
	if *name != "" {
		hostname = *name
	}
	agentID := *id
	if agentID == "" {
		agentID = stableID(hostname, *hostRoot)
	}
	var dockerProvider ports.DockerProvider
	if *dockerSocket != "" {
		if runtime.GOOS != "linux" {
			log.Fatal("--docker-socket is currently supported on Linux only")
		}
		var ok bool
		dockerProvider, ok = p.(ports.DockerProvider)
		if !ok {
			log.Fatal("current platform does not support Docker monitoring")
		}
	}
	client := &http.Client{Timeout: 8 * time.Second}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	log.Printf("Avenlytic agent starting: id=%s host=%s platform=%s/%s", agentID, hostname, runtime.GOOS, runtime.GOARCH)
	ticker := time.NewTicker(*interval)
	defer ticker.Stop()
	backoff := time.Second
	var lastSecurityReport, lastPackageAttempt, lastPackageInventory time.Time
	for {
		snapshot, err := p.Snapshot(ctx, hostname)
		if err != nil {
			log.Printf("collect: %v", err)
		} else {
			if dockerProvider != nil {
				dockerCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
				inventory, dockerErr := dockerProvider.CollectDocker(dockerCtx, *dockerSocket)
				cancel()
				snapshot.Docker = &inventory
				if dockerErr != nil {
					snapshot.Docker.Available = false
					snapshot.Docker.Error = dockerErr.Error()
					snapshot.Warnings = append(snapshot.Warnings, "docker: "+dockerErr.Error())
				} else {
					snapshot.Capabilities = append(snapshot.Capabilities, "docker_containers")
				}
			}
			snapshot.AgentID = agentID
			if err = send(ctx, client, strings.TrimRight(*server, "/"), *token, snapshot); err != nil {
				log.Printf("upload: %v", err)
				select {
				case <-ctx.Done():
					return
				case <-time.After(backoff):
				}
				if backoff < 30*time.Second {
					backoff *= 2
				}
			} else {
				backoff = time.Second
			}
			if securityProvider, ok := p.(ports.SecurityProvider); ok &&
				(lastSecurityReport.IsZero() || time.Since(lastSecurityReport) >= time.Minute) {
				includePackages := lastPackageInventory.IsZero() &&
					(lastPackageAttempt.IsZero() || time.Since(lastPackageAttempt) >= 30*time.Minute) ||
					!lastPackageInventory.IsZero() && time.Since(lastPackageInventory) >= 6*time.Hour
				report, securityErr := securityProvider.CollectSecurity(ctx, includePackages)
				if securityErr != nil {
					log.Printf("security collect: %v", securityErr)
				} else {
					report.AgentID = agentID
					securityErr = sendSecurity(ctx, client, strings.TrimRight(*server, "/"), *token, report)
					if securityErr != nil {
						log.Printf("security upload: %v", securityErr)
					} else {
						lastSecurityReport = time.Now()
						if includePackages {
							lastPackageAttempt = time.Now()
						}
						if includePackages && report.PackageInventoryAvailable {
							lastPackageInventory = time.Now()
						}
					}
				}
			}
		}
		select {
		case <-ctx.Done():
			log.Print("agent stopped")
			return
		case <-ticker.C:
		}
	}
}
func stableID(host, hostRoot string) string {
	var seed []byte
	for _, p := range []string{"/etc/machine-id", "/var/lib/dbus/machine-id"} {
		path := p
		if hostRoot != "" {
			path = filepath.Join(hostRoot, strings.TrimPrefix(p, "/"))
		}
		if b, e := os.ReadFile(path); e == nil && len(bytes.TrimSpace(b)) > 0 {
			seed = bytes.TrimSpace(b)
			break
		}
	}
	if len(seed) == 0 {
		if hostRoot != "" {
			seed = []byte(host + "/" + runtime.GOOS + "/" + runtime.GOARCH)
		} else if h, e := os.Hostname(); e == nil {
			seed = []byte(h + "/" + runtime.GOOS + "/" + runtime.GOARCH)
		} else {
			seed = []byte(host + "/" + runtime.GOOS)
		}
	}
	sum := sha256.Sum256(seed)
	return hex.EncodeToString(sum[:16])
}

func readHostHostname(hostRoot string) (string, error) {
	for _, path := range []string{
		filepath.Join(hostRoot, "etc/hostname"),
		filepath.Join(hostRoot, "proc/sys/kernel/hostname"),
	} {
		if b, err := os.ReadFile(path); err == nil {
			if hostname := strings.TrimSpace(string(b)); hostname != "" {
				return hostname, nil
			}
		}
	}
	return "", fmt.Errorf("hostname not found under %s", hostRoot)
}
func send(ctx context.Context, client *http.Client, baseURL, token string, s model.Snapshot) error {
	body, err := json.Marshal(s)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/api/v1/ingest", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("server returned %s", resp.Status)
	}
	return nil
}
func sendSecurity(ctx context.Context, client *http.Client, baseURL, token string, report model.SecurityReport) error {
	body, err := json.Marshal(report)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/api/v1/ingest/security", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("server returned %s", resp.Status)
	}
	return nil
}
