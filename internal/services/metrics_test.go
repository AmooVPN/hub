package services

import (
	"testing"
	"time"
)

func TestMetricsCollectorSnapshot(t *testing.T) {
	collector := NewMetricsCollector()
	collector.ObserveRequest(25 * time.Millisecond)
	collector.IncSubscriptionRequest()
	collector.IncBackupExport()
	collector.IncBackupImport()
	snap := collector.Snapshot()
	if snap.RequestTotal != 1 || snap.SubscriptionTotal != 1 || snap.BackupExportsTotal != 1 || snap.BackupImportsTotal != 1 {
		t.Fatalf("unexpected snapshot: %+v", snap)
	}
	if snap.RequestDuration <= 0 {
		t.Fatalf("expected request duration to be tracked: %+v", snap)
	}
}
