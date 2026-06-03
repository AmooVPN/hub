package services

import (
	"sync/atomic"
	"time"
)

type MetricsCollector struct {
	requestTotal         atomic.Int64
	requestDurationNanos atomic.Int64
	subscriptionTotal    atomic.Int64
	backupExportsTotal   atomic.Int64
	backupImportsTotal   atomic.Int64
}

type MetricsSnapshot struct {
	RequestTotal       int64
	RequestDuration    time.Duration
	SubscriptionTotal  int64
	BackupExportsTotal int64
	BackupImportsTotal int64
}

func NewMetricsCollector() *MetricsCollector { return &MetricsCollector{} }

func (m *MetricsCollector) ObserveRequest(duration time.Duration) {
	if m == nil {
		return
	}
	m.requestTotal.Add(1)
	m.requestDurationNanos.Add(duration.Nanoseconds())
}

func (m *MetricsCollector) IncSubscriptionRequest() {
	if m == nil {
		return
	}
	m.subscriptionTotal.Add(1)
}

func (m *MetricsCollector) IncBackupExport() {
	if m == nil {
		return
	}
	m.backupExportsTotal.Add(1)
}

func (m *MetricsCollector) IncBackupImport() {
	if m == nil {
		return
	}
	m.backupImportsTotal.Add(1)
}

func (m *MetricsCollector) Snapshot() MetricsSnapshot {
	if m == nil {
		return MetricsSnapshot{}
	}
	return MetricsSnapshot{
		RequestTotal:       m.requestTotal.Load(),
		RequestDuration:    time.Duration(m.requestDurationNanos.Load()),
		SubscriptionTotal:  m.subscriptionTotal.Load(),
		BackupExportsTotal: m.backupExportsTotal.Load(),
		BackupImportsTotal: m.backupImportsTotal.Load(),
	}
}
