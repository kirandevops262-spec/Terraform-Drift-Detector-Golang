package models

import "time"

const ReportVersion = "1.0"

// Resource is the normalized representation of infrastructure.
type Resource struct {
	ID           string            `json:"id"`
	TerraformRef string            `json:"terraform_ref,omitempty"`
	Type         string            `json:"type"`
	Provider     string            `json:"provider"`
	Region       string            `json:"region,omitempty"`
	CloudID      string            `json:"cloud_id,omitempty"`
	Attributes   map[string]any    `json:"attributes,omitempty"`
	Tags         map[string]string `json:"tags,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// DriftType classifies a detected difference.
type DriftType string

const (
	DriftMissingInCloud  DriftType = "missing_in_cloud"
	DriftMissingInState  DriftType = "missing_in_state"
	DriftAttributeChanged DriftType = "attribute_changed"
	DriftTagsChanged     DriftType = "tags_changed"
	DriftTypeMismatch    DriftType = "type_mismatch"
)

// FieldChange records a single attribute or tag difference.
type FieldChange struct {
	Path     string `json:"path"`
	Expected any    `json:"expected,omitempty"`
	Actual   any    `json:"actual,omitempty"`
}

// DriftItem is one drift finding for a resource.
type DriftItem struct {
	ResourceID   string        `json:"resource_id"`
	TerraformRef string        `json:"terraform_ref,omitempty"`
	Type         string        `json:"type"`
	Provider     string        `json:"provider"`
	Region       string        `json:"region,omitempty"`
	CloudID      string        `json:"cloud_id,omitempty"`
	DriftType    DriftType     `json:"drift_type"`
	Changes      []FieldChange `json:"changes,omitempty"`
	Message      string        `json:"message,omitempty"`
}

// DriftSummary aggregates drift counts.
type DriftSummary struct {
	Total              int            `json:"total"`
	MissingInCloud     int            `json:"missing_in_cloud"`
	MissingInState     int            `json:"missing_in_state"`
	AttributeChanged   int            `json:"attribute_changed"`
	TagsChanged        int            `json:"tags_changed"`
	TypeMismatch       int            `json:"type_mismatch"`
	ByResourceType     map[string]int `json:"by_resource_type,omitempty"`
}

// ScanStats captures resource counts for a scan.
type ScanStats struct {
	ExpectedCount int `json:"expected_count"`
	ActualCount   int `json:"actual_count"`
	MatchedCount  int `json:"matched_count"`
}

// DriftReport is the full output of a drift scan.
type DriftReport struct {
	ReportVersion string       `json:"report_version"`
	ScanID        string       `json:"scan_id"`
	StartedAt     time.Time    `json:"started_at"`
	CompletedAt   time.Time    `json:"completed_at"`
	Summary       DriftSummary `json:"summary"`
	Drifts        []DriftItem  `json:"drifts"`
	Resources     ScanStats    `json:"resources"`
}

// ScanStatus represents scan lifecycle.
type ScanStatus string

const (
	ScanStatusPending   ScanStatus = "pending"
	ScanStatusRunning   ScanStatus = "running"
	ScanStatusCompleted ScanStatus = "completed"
	ScanStatusFailed    ScanStatus = "failed"
)

// ScanRecord persists scan metadata.
type ScanRecord struct {
	ID          string       `json:"id"`
	Status      ScanStatus   `json:"status"`
	StartedAt   time.Time    `json:"started_at"`
	CompletedAt *time.Time   `json:"completed_at,omitempty"`
	Error       string       `json:"error,omitempty"`
	Report      *DriftReport `json:"report,omitempty"`
	Config      string       `json:"config,omitempty"`
}

// Schedule defines a recurring scan.
type Schedule struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Cron      string    `json:"cron"`
	Enabled   bool      `json:"enabled"`
	Config    string    `json:"config"`
	LastRunAt *time.Time `json:"last_run_at,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}
