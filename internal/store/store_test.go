package store_test

import (
	"os"
	"testing"
	"time"

	"github.com/terraform-drift-detector/golang/internal/store"
	"github.com/terraform-drift-detector/golang/pkg/models"
)

func TestStore_ScanRoundTrip(t *testing.T) {
	path := t.TempDir() + "/test.db"
	st, err := store.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	defer os.Remove(path)

	now := time.Now().UTC()
	completed := now.Add(time.Second)
	rec := &models.ScanRecord{
		ID:          "scan-1",
		Status:      models.ScanStatusCompleted,
		StartedAt:   now,
		CompletedAt: &completed,
		Report: &models.DriftReport{
			ScanID:        "scan-1",
			ReportVersion: models.ReportVersion,
			Summary:       models.DriftSummary{Total: 1},
			Drifts: []models.DriftItem{
				{DriftType: models.DriftMissingInCloud, Type: "aws_vpc"},
			},
		},
	}
	if err := st.SaveScan(rec); err != nil {
		t.Fatal(err)
	}
	got, err := st.GetScan("scan-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Report == nil || got.Report.Summary.Total != 1 {
		t.Fatalf("unexpected report: %+v", got.Report)
	}
	list, err := st.ListScans(10)
	if err != nil || len(list) != 1 {
		t.Fatalf("expected 1 scan, got %d", len(list))
	}
}

func TestStore_ScheduleRoundTrip(t *testing.T) {
	path := t.TempDir() + "/test.db"
	st, err := store.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	sch := &models.Schedule{
		ID:        "sch-1",
		Name:      "nightly",
		Cron:      "0 2 * * *",
		Enabled:   true,
		Config:    `{}`,
		CreatedAt: time.Now().UTC(),
	}
	if err := st.SaveSchedule(sch); err != nil {
		t.Fatal(err)
	}
	got, err := st.GetSchedule("sch-1")
	if err != nil || got.Name != "nightly" {
		t.Fatalf("unexpected schedule: %+v", got)
	}
}
