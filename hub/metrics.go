package main

import (
	"sync/atomic"
	"time"
)

type HubMetrics struct {
	startedAt time.Time
	total     atomic.Uint64
	failed    atomic.Uint64
}

func NewHubMetrics() *HubMetrics {
	return &HubMetrics{startedAt: time.Now()}
}

func (m *HubMetrics) RecordRequest(failed bool) {
	m.total.Add(1)
	if failed {
		m.failed.Add(1)
	}
}

func (m *HubMetrics) Total() uint64 {
	return m.total.Load()
}

func (m *HubMetrics) Failed() uint64 {
	return m.failed.Load()
}

func (m *HubMetrics) UptimeSeconds() int {
	return int(time.Since(m.startedAt).Seconds())
}
