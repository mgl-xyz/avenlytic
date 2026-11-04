// Package collector defines the platform-neutral collector registry.
package collector

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/avenlytic/avenlytic/internal/core/model"
	"github.com/avenlytic/avenlytic/internal/core/ports"
)

type Descriptor struct {
	ID                     string   `json:"id"`
	Name                   string   `json:"name"`
	Description            string   `json:"description,omitempty"`
	RequiredCapabilities   []string `json:"required_capabilities"`
	DefaultIntervalSeconds int      `json:"default_interval_seconds"`
	TimeoutSeconds         int      `json:"timeout_seconds"`
}

type Collector interface {
	Descriptor() Descriptor
	Collect(context.Context, ports.Provider) ([]model.Metric, error)
}

type Registry struct {
	mu    sync.RWMutex
	items map[string]Collector
}

func NewRegistry() *Registry { return &Registry{items: map[string]Collector{}} }
func (r *Registry) Register(c Collector) error {
	d := c.Descriptor()
	if d.ID == "" {
		return fmt.Errorf("collector ID is required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.items[d.ID]; ok {
		return fmt.Errorf("collector %q already registered", d.ID)
	}
	r.items[d.ID] = c
	return nil
}
func (r *Registry) Get(id string) (Collector, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	c, ok := r.items[id]
	return c, ok
}
func (r *Registry) List() []Descriptor {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Descriptor, 0, len(r.items))
	for _, c := range r.items {
		out = append(out, c.Descriptor())
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
func (r *Registry) ForCapabilities(available []string) []Collector {
	set := map[string]bool{}
	for _, x := range available {
		set[x] = true
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []Collector
	for _, c := range r.items {
		ok := true
		for _, required := range c.Descriptor().RequiredCapabilities {
			if !set[required] {
				ok = false
				break
			}
		}
		if ok {
			out = append(out, c)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Descriptor().ID < out[j].Descriptor().ID })
	return out
}
