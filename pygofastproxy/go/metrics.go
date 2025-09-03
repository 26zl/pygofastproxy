package main

import (
	"sync"
	"time"
)

// Request metrics for monitoring
type Metrics struct {
	mutex         sync.RWMutex
	requestCount  int64
	errorCount    int64
	totalDuration time.Duration
	startTime     time.Time
}

func NewMetrics() *Metrics {
	return &Metrics{
		startTime: time.Now(),
	}
}

// Records a new request
func (m *Metrics) RecordRequest(duration time.Duration, isError bool) {
	m.mutex.Lock()
	m.requestCount++
	m.totalDuration += duration
	if isError {
		m.errorCount++
	}
	m.mutex.Unlock()
}

// Gets the current metrics stats
func (m *Metrics) GetStats() (requestCount, errorCount int64, avgDuration time.Duration, uptime time.Duration) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	uptime = time.Since(m.startTime)
	requestCount = m.requestCount
	errorCount = m.errorCount

	if m.requestCount > 0 {
		avgDuration = m.totalDuration / time.Duration(m.requestCount)
	}
	return
}
