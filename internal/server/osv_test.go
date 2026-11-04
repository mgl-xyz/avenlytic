package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/avenlytic/avenlytic/internal/core/model"
)

func TestLookupOSV(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/querybatch" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		var request struct {
			Queries []osvQuery `json:"queries"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		if len(request.Queries) != 2 || request.Queries[0].Package.Name != "openssl" {
			t.Fatalf("unexpected package query: %+v", request)
		}
		_, _ = w.Write([]byte(`{"results":[{"vulns":[{"id":"CVE-2025-0001","summary":"example"},{"id":"GHSA-0000","aliases":["CVE-2025-0002"]}]},{"vulns":[{"id":"CVE-2025-0001"},{"id":"CVE-2025-0002"}]}]}`))
	}))
	defer server.Close()

	vulnerabilities, err := lookupOSVAt(context.Background(), server.Client(), server.URL+"/querybatch", []model.InstalledPackage{
		{Name: "openssl", Version: "3.0.0", Ecosystem: "Debian"},
		{Name: "curl", Version: "8.0.0", Ecosystem: "Debian"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(vulnerabilities) != 2 || vulnerabilities[1].ID != "CVE-2025-0002" {
		t.Fatalf("expected unique vulnerabilities, got %+v", vulnerabilities)
	}
}

func TestLookupOSVEmptyInventory(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("unexpected OSV request for empty inventory")
	}))
	defer server.Close()

	vulnerabilities, err := lookupOSVAt(context.Background(), server.Client(), server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(vulnerabilities) != 0 {
		t.Fatalf("expected no vulnerabilities, got %+v", vulnerabilities)
	}
}
