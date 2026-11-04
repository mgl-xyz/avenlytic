package collector

import (
	"context"
	"github.com/avenlytic/avenlytic/internal/core/model"
	"github.com/avenlytic/avenlytic/internal/core/ports"
	"testing"
)

type fakeCollector struct{ d Descriptor }

func (f fakeCollector) Descriptor() Descriptor { return f.d }
func (f fakeCollector) Collect(context.Context, ports.Provider) ([]model.Metric, error) {
	return nil, nil
}
func TestRegistryCapabilitySelection(t *testing.T) {
	r := NewRegistry()
	if err := r.Register(fakeCollector{Descriptor{ID: "cpu", RequiredCapabilities: []string{"cpu"}}}); err != nil {
		t.Fatal(err)
	}
	if err := r.Register(fakeCollector{Descriptor{ID: "smart", RequiredCapabilities: []string{"smart"}}}); err != nil {
		t.Fatal(err)
	}
	got := r.ForCapabilities([]string{"cpu"})
	if len(got) != 1 || got[0].Descriptor().ID != "cpu" {
		t.Fatalf("unexpected collectors: %+v", got)
	}
}
func TestRegistryRejectsDuplicate(t *testing.T) {
	r := NewRegistry()
	c := fakeCollector{Descriptor{ID: "cpu"}}
	if err := r.Register(c); err != nil {
		t.Fatal(err)
	}
	if err := r.Register(c); err == nil {
		t.Fatal("expected duplicate error")
	}
}
