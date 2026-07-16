package state

import (
	"encoding/json"
	"fmt"
	"os"
)

// RawState represents parsed Terraform state JSON.
type RawState struct {
	Version  int          `json:"version"`
	Resources []RawResource `json:"resources"`
}

// RawResource is a resource entry from Terraform state.
type RawResource struct {
	Mode     string        `json:"mode"`
	Type     string        `json:"type"`
	Name     string        `json:"name"`
	Provider string        `json:"provider"`
	Instances []RawInstance `json:"instances"`
}

// RawInstance holds resource instance attributes.
type RawInstance struct {
	Attributes map[string]any `json:"attributes"`
}

// Reader loads Terraform state from a backend.
type Reader interface {
	Read() (*RawState, error)
}

// LocalReader reads state from a local file.
type LocalReader struct {
	Path string
}

// Read loads and parses a local terraform.tfstate file.
func (r *LocalReader) Read() (*RawState, error) {
	data, err := os.ReadFile(r.Path)
	if err != nil {
		return nil, fmt.Errorf("read state file %s: %w", r.Path, err)
	}
	var raw RawState
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse state file: %w", err)
	}
	return &raw, nil
}

// NewReader creates a state reader for the given source.
func NewReader(source, path string) (Reader, error) {
	switch source {
	case "local", "":
		return &LocalReader{Path: path}, nil
	default:
		return nil, fmt.Errorf("unsupported state source: %s", source)
	}
}
