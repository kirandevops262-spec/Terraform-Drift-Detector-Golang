package drift_test

import (
	"testing"
	"time"

	"github.com/terraform-drift-detector/golang/internal/drift"
	"github.com/terraform-drift-detector/golang/pkg/models"
)

func TestCompare_MissingInCloud(t *testing.T) {
	engine := drift.NewEngine(drift.DefaultOptions())
	expected := []models.Resource{
		{
			ID: "aws:aws_vpc:vpc-deleted999", Type: "aws_vpc", Provider: "aws",
			CloudID: "vpc-deleted999", Region: "us-east-1",
			TerraformRef: "aws_vpc.main",
			Attributes:   map[string]any{"cidr_block": "10.0.0.0/16"},
			Tags:         map[string]string{"Name": "main-vpc"},
		},
	}
	actual := []models.Resource{}

	report := engine.Compare("test-scan", expected, actual)
	if report.Summary.MissingInCloud != 1 {
		t.Fatalf("expected 1 missing_in_cloud, got %d", report.Summary.MissingInCloud)
	}
}

func TestCompare_AttributeChanged(t *testing.T) {
	engine := drift.NewEngine(drift.DefaultOptions())
	expected := []models.Resource{
		{
			ID: "aws:aws_instance:i-abc123", Type: "aws_instance", Provider: "aws",
			CloudID: "i-abc123", Region: "us-east-1",
			TerraformRef: "aws_instance.web",
			Attributes:   map[string]any{"instance_type": "t3.micro"},
			Tags:         map[string]string{"Environment": "production"},
		},
	}
	actual := []models.Resource{
		{
			ID: "aws:aws_instance:i-abc123", Type: "aws_instance", Provider: "aws",
			CloudID: "i-abc123", Region: "us-east-1",
			Attributes: map[string]any{"instance_type": "t3.small"},
			Tags:       map[string]string{"Environment": "production"},
		},
	}

	report := engine.Compare("test-scan", expected, actual)
	if report.Summary.AttributeChanged != 1 {
		t.Fatalf("expected 1 attribute_changed, got %d", report.Summary.AttributeChanged)
	}
	if report.Drifts[0].Changes[0].Path != "attributes.instance_type" {
		t.Fatalf("unexpected change path: %s", report.Drifts[0].Changes[0].Path)
	}
}

func TestCompare_TagsChanged(t *testing.T) {
	engine := drift.NewEngine(drift.DefaultOptions())
	expected := []models.Resource{
		{
			ID: "aws:aws_s3_bucket:my-bucket", Type: "aws_s3_bucket", Provider: "aws",
			CloudID: "my-bucket", Region: "us-east-1",
			TerraformRef: "aws_s3_bucket.logs",
			Attributes:   map[string]any{"bucket": "my-bucket"},
			Tags:         map[string]string{"Environment": "production"},
		},
	}
	actual := []models.Resource{
		{
			ID: "aws:aws_s3_bucket:my-bucket", Type: "aws_s3_bucket", Provider: "aws",
			CloudID: "my-bucket", Region: "us-east-1",
			Attributes: map[string]any{"bucket": "my-bucket"},
			Tags:       map[string]string{"Environment": "staging"},
		},
	}

	report := engine.Compare("test-scan", expected, actual)
	if report.Summary.TagsChanged != 1 {
		t.Fatalf("expected 1 tags_changed, got %d", report.Summary.TagsChanged)
	}
}

func TestCompare_MissingInState(t *testing.T) {
	engine := drift.NewEngine(drift.DefaultOptions())
	expected := []models.Resource{}
	actual := []models.Resource{
		{
			ID: "aws:aws_instance:i-orphan", Type: "aws_instance", Provider: "aws",
			CloudID: "i-orphan", Region: "us-east-1",
			Attributes: map[string]any{"instance_type": "t3.micro"},
		},
	}

	report := engine.Compare("test-scan", expected, actual)
	if report.Summary.MissingInState != 1 {
		t.Fatalf("expected 1 missing_in_state, got %d", report.Summary.MissingInState)
	}
}

func TestCompare_NoDrift(t *testing.T) {
	engine := drift.NewEngine(drift.DefaultOptions())
	r := models.Resource{
		ID: "aws:aws_instance:i-match", Type: "aws_instance", Provider: "aws",
		CloudID: "i-match", Region: "us-east-1",
		TerraformRef: "aws_instance.web",
		Attributes:   map[string]any{"instance_type": "t3.micro"},
		Tags:         map[string]string{"Name": "web"},
	}
	report := engine.Compare("test-scan", []models.Resource{r}, []models.Resource{r})
	if report.Summary.Total != 0 {
		t.Fatalf("expected no drift, got %d", report.Summary.Total)
	}
}

func TestCompare_ReportMetadata(t *testing.T) {
	engine := drift.NewEngine(drift.DefaultOptions())
	report := engine.Compare("scan-123", nil, nil)
	if report.ScanID != "scan-123" {
		t.Fatalf("unexpected scan id: %s", report.ScanID)
	}
	if report.ReportVersion != models.ReportVersion {
		t.Fatalf("unexpected report version: %s", report.ReportVersion)
	}
	_ = time.Now()
}
