package state_test

import (
	"testing"

	"github.com/terraform-drift-detector/golang/internal/state"
)

func TestLocalReader_Read(t *testing.T) {
	reader := &state.LocalReader{Path: "../../testdata/sample.tfstate"}
	raw, err := reader.Read()
	if err != nil {
		t.Fatal(err)
	}
	if raw.Version != 4 {
		t.Fatalf("expected version 4, got %d", raw.Version)
	}
	if len(raw.Resources) < 3 {
		t.Fatalf("expected at least 3 resources, got %d", len(raw.Resources))
	}
}

func TestNewReader_Unsupported(t *testing.T) {
	_, err := state.NewReader("s3", "bucket/key")
	if err == nil {
		t.Fatal("expected error for unsupported source")
	}
}
