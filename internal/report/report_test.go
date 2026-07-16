package report_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/terraform-drift-detector/golang/internal/report"
	"github.com/terraform-drift-detector/golang/pkg/models"
)

func TestWriteJSON(t *testing.T) {
	r := &models.DriftReport{
		ReportVersion: models.ReportVersion,
		ScanID:        "test",
		StartedAt:     time.Now(),
		CompletedAt:   time.Now(),
		Summary:       models.DriftSummary{Total: 1, MissingInCloud: 1},
		Drifts: []models.DriftItem{
			{DriftType: models.DriftMissingInCloud, Type: "aws_vpc", TerraformRef: "aws_vpc.main"},
		},
	}
	var buf bytes.Buffer
	if err := report.WriteJSON(&buf, r); err != nil {
		t.Fatal(err)
	}
	var decoded models.DriftReport
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Summary.Total != 1 {
		t.Fatalf("expected total 1, got %d", decoded.Summary.Total)
	}
}

func TestWriteConsole(t *testing.T) {
	r := &models.DriftReport{
		ScanID:      "test",
		StartedAt:   time.Now(),
		CompletedAt: time.Now().Add(time.Second),
		Summary:     models.DriftSummary{Total: 0},
		Resources:   models.ScanStats{ExpectedCount: 1, ActualCount: 1, MatchedCount: 1},
	}
	var buf bytes.Buffer
	report.WriteConsole(&buf, r)
	if !strings.Contains(buf.String(), "No drift detected") {
		t.Fatalf("expected no drift message, got: %s", buf.String())
	}
}
