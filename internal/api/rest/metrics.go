package rest

import (
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

type Metrics struct {
	requestCount   atomic.Int64
	errorCount     atomic.Int64
	totalLatencyMs atomic.Int64

	mu           sync.RWMutex
	pathCounts   map[string]*atomic.Int64
	statusCounts map[int]*atomic.Int64
}

var globalMetrics = &Metrics{
	pathCounts:   make(map[string]*atomic.Int64),
	statusCounts: make(map[int]*atomic.Int64),
}

// GetMetrics returns the global metrics instance.
func GetMetrics() *Metrics {
	return globalMetrics
}

// Snapshot returns current metrics as a map.
func (m *Metrics) Snapshot() map[string]any {
	m.mu.RLock()
	defer m.mu.RUnlock()

	paths := make(map[string]int64)
	for path, count := range m.pathCounts {
		paths[path] = count.Load()
	}

	statuses := make(map[string]int64)
	for code, count := range m.statusCounts {
		statuses[fmt.Sprintf("%d", code)] = count.Load()
	}

	total := m.requestCount.Load()
	avgLatency := float64(0)
	if total > 0 {
		avgLatency = float64(m.totalLatencyMs.Load()) / float64(total)
	}

	return map[string]any{
		"requests_total": total,
		"errors_total":   m.errorCount.Load(),
		"avg_latency_ms": avgLatency,
		"by_path":        paths,
		"by_status":      statuses,
	}
}

func (m *Metrics) record(path string, status int, latency time.Duration) {
	m.requestCount.Add(1)
	m.totalLatencyMs.Add(latency.Milliseconds())

	if status >= 500 {
		m.errorCount.Add(1)
	}

	m.mu.Lock()
	if _, ok := m.pathCounts[path]; !ok {
		m.pathCounts[path] = &atomic.Int64{}
	}
	m.pathCounts[path].Add(1)

	if _, ok := m.statusCounts[status]; !ok {
		m.statusCounts[status] = &atomic.Int64{}
	}
	m.statusCounts[status].Add(1)
	m.mu.Unlock()
}

// MetricsMiddleware records request metrics.
func MetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := &responseWriter{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(ww, r)

		globalMetrics.record(r.URL.Path, ww.status, time.Since(start))
	})
}

// MetricsHandler returns current metrics as JSON.
func MetricsHandler(w http.ResponseWriter, r *http.Request) {
	respondOK(w, http.StatusOK, globalMetrics.Snapshot())
}
