// Package metrics is a tiny stdlib-only registry with Prometheus text exposition.
package metrics

import (
	"fmt"
	"net/http"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
)

// Counter is a monotonically increasing value.
type Counter struct{ v atomic.Uint64 }

// Inc adds 1.
func (c *Counter) Inc() { c.v.Add(1) }

// Add adds n.
func (c *Counter) Add(n uint64) { c.v.Add(n) }

// Gauge is a value that can go up and down.
type Gauge struct{ v atomic.Int64 }

// Set replaces the value.
func (g *Gauge) Set(v int64) { g.v.Store(v) }

// Add adds d (may be negative).
func (g *Gauge) Add(d int64) { g.v.Add(d) }

type family struct {
	help   string
	typ    string
	series map[string]func() int64 // label string -> value
	objs   map[string]any          // label string -> *Counter / *Gauge
	fn     bool
}

// Registry holds metric families by name.
type Registry struct {
	mu   sync.Mutex
	fams map[string]*family
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry { return &Registry{fams: map[string]*family{}} }

var escaper = strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`)

func labelString(labels []string) string {
	if len(labels)%2 != 0 {
		panic("metrics: odd number of label arguments")
	}
	if len(labels) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteByte('{')
	for i := 0; i < len(labels); i += 2 {
		if i > 0 {
			b.WriteByte(',')
		}
		fmt.Fprintf(&b, `%s="%s"`, labels[i], escaper.Replace(labels[i+1]))
	}
	b.WriteByte('}')
	return b.String()
}

// get returns the existing or newly made object for name+labels; caller holds r.mu.
func (r *Registry) get(name, help, typ string, labels []string, mk func() (any, func() int64)) any {
	ls := labelString(labels)
	f, ok := r.fams[name]
	if !ok {
		f = &family{help: help, typ: typ, series: map[string]func() int64{}, objs: map[string]any{}}
		r.fams[name] = f
	}
	if f.typ != typ || f.fn {
		panic("metrics: " + name + " already registered with another type")
	}
	if o, ok := f.objs[ls]; ok {
		return o
	}
	o, val := mk()
	f.objs[ls] = o
	f.series[ls] = val
	return o
}

// Counter returns the counter for name and label pairs, creating it on first use.
func (r *Registry) Counter(name, help string, labels ...string) *Counter {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.get(name, help, "counter", labels, func() (any, func() int64) {
		c := &Counter{}
		return c, func() int64 { return int64(c.v.Load()) } //nolint:gosec // counters never approach MaxInt64
	}).(*Counter)
}

// Gauge returns the gauge for name and label pairs, creating it on first use.
func (r *Registry) Gauge(name, help string, labels ...string) *Gauge {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.get(name, help, "gauge", labels, func() (any, func() int64) {
		g := &Gauge{}
		return g, g.v.Load
	}).(*Gauge)
}

// GaugeFunc registers a gauge whose value fn returns on every scrape.
func (r *Registry) GaugeFunc(name, help string, fn func() int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.fams[name]; ok {
		panic("metrics: " + name + " already registered")
	}
	r.fams[name] = &family{help: help, typ: "gauge", fn: true, series: map[string]func() int64{"": fn}}
}

// Handler serves the Prometheus text format.
func (r *Registry) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		r.mu.Lock()
		var b strings.Builder
		names := make([]string, 0, len(r.fams))
		for n := range r.fams {
			names = append(names, n)
		}
		slices.Sort(names)
		for _, n := range names {
			f := r.fams[n]
			fmt.Fprintf(&b, "# HELP %s %s\n# TYPE %s %s\n", n, f.help, n, f.typ)
			ls := make([]string, 0, len(f.series))
			for l := range f.series {
				ls = append(ls, l)
			}
			slices.Sort(ls)
			for _, l := range ls {
				fmt.Fprintf(&b, "%s%s %d\n", n, l, f.series[l]())
			}
		}
		r.mu.Unlock()
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		_, _ = w.Write([]byte(b.String()))
	})
}
