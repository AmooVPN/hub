package models

import "testing"

func TestIsValidSyncJobStatus(t *testing.T) {
	valid := []string{SyncJobStatusQueued, SyncJobStatusRunning, SyncJobStatusSuccess, SyncJobStatusFailed, SyncJobStatusCancelled}
	for _, status := range valid {
		if !IsValidSyncJobStatus(status) {
			t.Fatalf("expected valid status: %s", status)
		}
	}
	if IsValidSyncJobStatus("unknown") {
		t.Fatal("expected unknown status to be invalid")
	}
}
