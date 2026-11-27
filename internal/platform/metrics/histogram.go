package metrics

import (
	"math"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
)

// Histogram counts observations in fixed buckets.
type Histogram struct {
	bounds []float64       // upper bounds, ascending
	counts []atomic.Uint64 // per bucket, last one is +Inf; not cumulative
	count  atomic.Uint64
	sum    atomic.Uint64 // float64 bits
}

// Observe records v. It does not allocate.
func (h *Histogram) Observe(v float64) {
	h.counts[sort.SearchFloat64s(h.bounds, v)].Add(1)
	h.count.Add(1)
	for {
		old := h.sum.Load()
		if h.sum.CompareAndSwap(old, math.Float64bits(math.Float64frombits(old)+v)) {
			return
		}
	}
}

// Histogram returns the histogram for name and label pairs, creating it on first use.
// It panics on empty or non-ascending buckets, or buckets that differ from the name's first series.
func (r *Registry) Histogram(name, help string, buckets []float64, labels ...string) *Histogram {
	if len(buckets) == 0 || !strictlyAscending(buckets) {
		panic("metrics: " + name + " needs non-empty strictly ascending buckets")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if f, ok := r.fams[name]; ok && f.typ == "histogram" {
		for _, o := range f.objs {
			if !slices.Equal(o.(*Histogram).bounds, buckets) {
				panic("metrics: " + name + " already registered with other buckets")
			}
			break
		}
	}
	return r.get(name, help, "histogram", labels, func() (any, func() int64) {
		h := &Histogram{bounds: slices.Clone(buckets), counts: make([]atomic.Uint64, len(buckets)+1)}
		return h, func() int64 { return int64(h.count.Load()) } //nolint:gosec // counts never approach MaxInt64
	}).(*Histogram)
}

// writeHistogram appends the series of one histogram family; labels are sorted label strings.
func writeHistogram(b *strings.Builder, name string, labels []string, hists map[string]any) {
	for _, ls := range labels {
		h := hists[ls].(*Histogram)
		var cum uint64
		for i := range h.counts {
			cum += h.counts[i].Load()
			le := "+Inf"
			if i < len(h.bounds) {
				le = strconv.FormatFloat(h.bounds[i], 'g', -1, 64)
			}
			b.WriteString(name + "_bucket" + withLE(ls, le) + " " + strconv.FormatUint(cum, 10) + "\n")
		}
		sum := strconv.FormatFloat(math.Float64frombits(h.sum.Load()), 'g', -1, 64)
		b.WriteString(name + "_sum" + ls + " " + sum + "\n")
		b.WriteString(name + "_count" + ls + " " + strconv.FormatUint(h.count.Load(), 10) + "\n")
	}
}

func withLE(ls, le string) string {
	if ls == "" {
		return `{le="` + le + `"}`
	}
	return ls[:len(ls)-1] + `,le="` + le + `"}`
}

func strictlyAscending(b []float64) bool {
	for i := 1; i < len(b); i++ {
		if b[i] <= b[i-1] {
			return false
		}
	}
	return true
}
