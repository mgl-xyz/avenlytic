package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/avenlytic/avenlytic/internal/core/model"
)

const osvEndpoint = "https://api.osv.dev/v1/querybatch"

type osvQuery struct {
	Package osvPackage `json:"package"`
	Version string     `json:"version"`
}
type osvPackage struct {
	Name      string `json:"name"`
	Ecosystem string `json:"ecosystem"`
}
type osvResponse struct {
	Results []struct {
		Vulnerabilities []struct {
			ID      string   `json:"id"`
			Summary string   `json:"summary"`
			Aliases []string `json:"aliases"`
		} `json:"vulns"`
	} `json:"results"`
}

func lookupOSV(ctx context.Context, packages []model.InstalledPackage) ([]model.Vulnerability, error) {
	client := &http.Client{Timeout: 20 * time.Second}
	return lookupOSVAt(ctx, client, osvEndpoint, packages)
}

func lookupOSVAt(ctx context.Context, client *http.Client, endpoint string, packages []model.InstalledPackage) ([]model.Vulnerability, error) {
	if len(packages) == 0 {
		return []model.Vulnerability{}, nil
	}
	const batchSize = 1000
	found := map[string]string{}
	for start := 0; start < len(packages); start += batchSize {
		end := min(start+batchSize, len(packages))
		queries := make([]osvQuery, 0, end-start)
		for _, pkg := range packages[start:end] {
			queries = append(queries, osvQuery{
				Package: osvPackage{Name: pkg.Name, Ecosystem: pkg.Ecosystem},
				Version: pkg.Version,
			})
		}
		body, err := json.Marshal(map[string]any{"queries": queries})
		if err != nil {
			return nil, fmt.Errorf("encode OSV query: %w", err)
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("create OSV request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "Avenlytic vulnerability monitor")
		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("query OSV: %w", err)
		}
		if resp.StatusCode != http.StatusOK {
			_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
			resp.Body.Close()
			return nil, fmt.Errorf("OSV returned %s", resp.Status)
		}
		var result osvResponse
		decodeErr := json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(&result)
		resp.Body.Close()
		if decodeErr != nil {
			return nil, fmt.Errorf("decode OSV response: %w", decodeErr)
		}
		if len(result.Results) != len(queries) {
			return nil, fmt.Errorf("OSV returned %d results for %d packages", len(result.Results), len(queries))
		}
		for _, packageResult := range result.Results {
			for _, vulnerability := range packageResult.Vulnerabilities {
				if vulnerability.ID == "" {
					continue
				}
				id := canonicalOSVID(vulnerability.ID, vulnerability.Aliases)
				summary := strings.TrimSpace(vulnerability.Summary)
				if existing, exists := found[id]; !exists || existing == "" && summary != "" {
					found[id] = summary
				}
			}
		}
	}
	vulnerabilities := make([]model.Vulnerability, 0, len(found))
	for id, summary := range found {
		vulnerabilities = append(vulnerabilities, model.Vulnerability{ID: id, Summary: summary})
	}
	sort.Slice(vulnerabilities, func(i, j int) bool { return vulnerabilities[i].ID < vulnerabilities[j].ID })
	return vulnerabilities, nil
}

func canonicalOSVID(id string, aliases []string) string {
	cves := make([]string, 0, len(aliases))
	for _, alias := range aliases {
		alias = strings.TrimSpace(alias)
		if strings.HasPrefix(strings.ToUpper(alias), "CVE-") {
			cves = append(cves, alias)
		}
	}
	if len(cves) == 0 {
		return id
	}
	sort.Strings(cves)
	return cves[0]
}
