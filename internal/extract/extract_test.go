package extract_test

import (
	"testing"

	"github.com/terraform-drift-detector/golang/internal/extract"
	"github.com/terraform-drift-detector/golang/internal/state"
)

func TestFromState_ManagedOnly(t *testing.T) {
	reader := &state.LocalReader{Path: "../../testdata/sample.tfstate"}
	raw, err := reader.Read()
	if err != nil {
		t.Fatal(err)
	}
	resources := extract.FromState(raw, extract.Options{Provider: "aws"})
	if len(resources) != 3 {
		t.Fatalf("expected 3 managed resources, got %d", len(resources))
	}

	types := map[string]bool{}
	for _, r := range resources {
		types[r.Type] = true
		if r.Provider != "aws" {
			t.Fatalf("expected aws provider, got %s", r.Provider)
		}
	}
	for _, want := range []string{"aws_instance", "aws_s3_bucket", "aws_vpc"} {
		if !types[want] {
			t.Fatalf("missing resource type %s", want)
		}
	}
}

func TestFromState_TagsExtracted(t *testing.T) {
	reader := &state.LocalReader{Path: "../../testdata/sample.tfstate"}
	raw, _ := reader.Read()
	resources := extract.FromState(raw, extract.Options{Provider: "aws"})
	for _, r := range resources {
		if r.Type == "aws_instance" {
			if r.Tags["Environment"] != "production" {
				t.Fatalf("expected production tag, got %v", r.Tags)
			}
			if r.CloudID != "i-abc123def456" {
				t.Fatalf("unexpected cloud id: %s", r.CloudID)
			}
		}
	}
}
