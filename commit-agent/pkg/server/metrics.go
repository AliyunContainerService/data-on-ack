/*
*Copyright (c) 2021, Alibaba Group;
*Licensed under the Apache License, Version 2.0 (the "License");
*you may not use this file except in compliance with the License.
 */

package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	log "github.com/sirupsen/logrus"
)

// metricKey is a (metric name, sorted-label-string) tuple used as the map key
// for counters and histograms with multiple label permutations.
type metricKey struct {
	name   string
	labels string
}

func encodeLabels(pairs map[string]string) string {
	if len(pairs) == 0 {
		return ""
	}
	keys := make([]string, 0, len(pairs))
	for k := range pairs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for i, k := range keys {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(k)
		b.WriteString(`="`)
		b.WriteString(escapeLabelValue(pairs[k]))
		b.WriteByte('"')
	}
	return b.String()
}

func escapeLabelValue(v string) string {
	v = strings.ReplaceAll(v, `\`, `\\`)
	v = strings.ReplaceAll(v, "\n", `\n`)
	v = strings.ReplaceAll(v, `"`, `\"`)
	return v
}

// counter is a monotonically increasing float64.
type counter struct {
	v atomic.Uint64 // stored as math.Float64bits
}

func (c *counter) add(delta float64) {
	for {
		oldBits := c.v.Load()
		oldVal := bitsToFloat(oldBits)
		newVal := oldVal + delta
		if c.v.CompareAndSwap(oldBits, floatToBits(newVal)) {
			return
		}
	}
}

func (c *counter) value() float64 { return bitsToFloat(c.v.Load()) }

// histogram is a fixed-bucket histogram suitable for latency measurements.
// Buckets are upper bounds in seconds.
type histogram struct {
	buckets []float64
	mu      sync.Mutex
	counts  []uint64
	sum     float64
	total   uint64
}

func newHistogram(buckets []float64) *histogram {
	cp := make([]float64, len(buckets))
	copy(cp, buckets)
	return &histogram{
		buckets: cp,
		counts:  make([]uint64, len(cp)),
	}
}

func (h *histogram) observe(seconds float64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for i, ub := range h.buckets {
		if seconds <= ub {
			h.counts[i]++
		}
	}
	h.total++
	h.sum += seconds
}

// gauge is a settable float64.
type gauge struct {
	v atomic.Uint64
}

func (g *gauge) inc() {
	for {
		old := g.v.Load()
		nv := bitsToFloat(old) + 1
		if g.v.CompareAndSwap(old, floatToBits(nv)) {
			return
		}
	}
}

func (g *gauge) dec() {
	for {
		old := g.v.Load()
		nv := bitsToFloat(old) - 1
		if g.v.CompareAndSwap(old, floatToBits(nv)) {
			return
		}
	}
}

func (g *gauge) value() float64 { return bitsToFloat(g.v.Load()) }

func floatToBits(f float64) uint64 { return math.Float64bits(f) }
func bitsToFloat(b uint64) float64 { return math.Float64frombits(b) }

// Registry is the metrics container exposed at /metrics.
type Registry struct {
	mu         sync.Mutex
	counters   map[metricKey]*counter
	gauges     map[metricKey]*gauge
	histograms map[metricKey]*histogram
	help       map[string]string
	histBucket []float64
}

// NewRegistry constructs an empty registry. The latency buckets are tuned for
// commit/push: from sub-second up to 30 minutes.
func NewRegistry() *Registry {
	return &Registry{
		counters:   map[metricKey]*counter{},
		gauges:     map[metricKey]*gauge{},
		histograms: map[metricKey]*histogram{},
		help:       map[string]string{},
		histBucket: []float64{
			0.1, 0.5, 1, 2, 5, 10, 30, 60, 120, 300, 600, 900, 1800,
		},
	}
}

func (r *Registry) docHelp(name, help string) {
	r.mu.Lock()
	r.help[name] = help
	r.mu.Unlock()
}

// IncCounter atomically increments the named counter by 1.
func (r *Registry) IncCounter(name string, labels map[string]string) {
	r.AddCounter(name, labels, 1)
}

// AddCounter atomically increments the named counter by `delta`.
func (r *Registry) AddCounter(name string, labels map[string]string, delta float64) {
	if r == nil {
		return
	}
	key := metricKey{name: name, labels: encodeLabels(labels)}
	r.mu.Lock()
	c, ok := r.counters[key]
	if !ok {
		c = &counter{}
		r.counters[key] = c
	}
	r.mu.Unlock()
	c.add(delta)
}

// IncGauge / DecGauge bump an in-flight style gauge.
func (r *Registry) IncGauge(name string, labels map[string]string) {
	if r == nil {
		return
	}
	r.gaugeFor(name, labels).inc()
}

func (r *Registry) DecGauge(name string, labels map[string]string) {
	if r == nil {
		return
	}
	r.gaugeFor(name, labels).dec()
}

func (r *Registry) gaugeFor(name string, labels map[string]string) *gauge {
	key := metricKey{name: name, labels: encodeLabels(labels)}
	r.mu.Lock()
	defer r.mu.Unlock()
	g, ok := r.gauges[key]
	if !ok {
		g = &gauge{}
		r.gauges[key] = g
	}
	return g
}

// ObserveSeconds records a duration into a histogram.
func (r *Registry) ObserveSeconds(name string, labels map[string]string, seconds float64) {
	if r == nil {
		return
	}
	key := metricKey{name: name, labels: encodeLabels(labels)}
	r.mu.Lock()
	h, ok := r.histograms[key]
	if !ok {
		h = newHistogram(r.histBucket)
		r.histograms[key] = h
	}
	r.mu.Unlock()
	h.observe(seconds)
}

// HTTPHandler exposes the registry in Prometheus text exposition format.
func (r *Registry) HTTPHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		_ = r.write(w)
	})
}

func (r *Registry) write(w io.Writer) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Group by metric name so HELP/TYPE only appears once per family.
	type groupedCounter struct {
		labels string
		value  float64
	}
	type groupedGauge struct {
		labels string
		value  float64
	}

	counterGroups := map[string][]groupedCounter{}
	for k, c := range r.counters {
		counterGroups[k.name] = append(counterGroups[k.name], groupedCounter{k.labels, c.value()})
	}
	gaugeGroups := map[string][]groupedGauge{}
	for k, g := range r.gauges {
		gaugeGroups[k.name] = append(gaugeGroups[k.name], groupedGauge{k.labels, g.value()})
	}

	histNames := map[string]bool{}
	for k := range r.histograms {
		histNames[k.name] = true
	}

	for name, items := range counterGroups {
		if h, ok := r.help[name]; ok {
			fmt.Fprintf(w, "# HELP %s %s\n", name, h)
		}
		fmt.Fprintf(w, "# TYPE %s counter\n", name)
		sort.Slice(items, func(i, j int) bool { return items[i].labels < items[j].labels })
		for _, it := range items {
			writeSample(w, name, it.labels, it.value)
		}
	}
	for name, items := range gaugeGroups {
		if h, ok := r.help[name]; ok {
			fmt.Fprintf(w, "# HELP %s %s\n", name, h)
		}
		fmt.Fprintf(w, "# TYPE %s gauge\n", name)
		sort.Slice(items, func(i, j int) bool { return items[i].labels < items[j].labels })
		for _, it := range items {
			writeSample(w, name, it.labels, it.value)
		}
	}
	for name := range histNames {
		if h, ok := r.help[name]; ok {
			fmt.Fprintf(w, "# HELP %s %s\n", name, h)
		}
		fmt.Fprintf(w, "# TYPE %s histogram\n", name)

		// Collect each label-perm under this family.
		type item struct {
			labels string
			h      *histogram
		}
		var items []item
		for k, h := range r.histograms {
			if k.name == name {
				items = append(items, item{k.labels, h})
			}
		}
		sort.Slice(items, func(i, j int) bool { return items[i].labels < items[j].labels })

		for _, it := range items {
			it.h.mu.Lock()
			labels := it.labels
			for i, ub := range it.h.buckets {
				bucketLabel := fmt.Sprintf(`le="%s"`, formatFloat(ub))
				combined := joinLabel(labels, bucketLabel)
				fmt.Fprintf(w, "%s_bucket{%s} %d\n", name, combined, it.h.counts[i])
			}
			combinedInf := joinLabel(labels, `le="+Inf"`)
			fmt.Fprintf(w, "%s_bucket{%s} %d\n", name, combinedInf, it.h.total)
			if labels == "" {
				fmt.Fprintf(w, "%s_count %d\n", name, it.h.total)
				fmt.Fprintf(w, "%s_sum %s\n", name, formatFloat(it.h.sum))
			} else {
				fmt.Fprintf(w, "%s_count{%s} %d\n", name, labels, it.h.total)
				fmt.Fprintf(w, "%s_sum{%s} %s\n", name, labels, formatFloat(it.h.sum))
			}
			it.h.mu.Unlock()
		}
	}
	return nil
}

func writeSample(w io.Writer, name, labels string, value float64) {
	if labels == "" {
		fmt.Fprintf(w, "%s %s\n", name, formatFloat(value))
		return
	}
	fmt.Fprintf(w, "%s{%s} %s\n", name, labels, formatFloat(value))
}

func joinLabel(a, b string) string {
	switch {
	case a == "" && b == "":
		return ""
	case a == "":
		return b
	case b == "":
		return a
	default:
		return a + "," + b
	}
}

func formatFloat(f float64) string {
	return strconv.FormatFloat(f, 'g', -1, 64)
}

// MetricsServer wraps an http.Server hosting the registry. ListenAndServe
// returns nil if address is empty.
type MetricsServer struct {
	Registry *Registry
	Address  string

	srv *http.Server
}

// Start begins serving in the background. It returns an error channel which
// receives the eventual ListenAndServe error (one value, then closed).
func (m *MetricsServer) Start() chan error {
	errCh := make(chan error, 1)
	if m == nil || m.Address == "" {
		close(errCh)
		return errCh
	}

	mux := http.NewServeMux()
	mux.Handle("/metrics", m.Registry.HTTPHandler())
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "ok")
	})

	m.srv = &http.Server{
		Addr:              m.Address,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		defer close(errCh)
		log.Infof("metrics server listening on %s", m.Address)
		if err := m.srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()
	return errCh
}

// Shutdown gracefully stops the metrics HTTP server.
func (m *MetricsServer) Shutdown(ctx context.Context) error {
	if m == nil || m.srv == nil {
		return nil
	}
	return m.srv.Shutdown(ctx)
}

