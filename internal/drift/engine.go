package drift

import (
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/terraform-drift-detector/golang/pkg/models"
)

// Options configures drift comparison.
type Options struct {
	IgnoreGlobal     []string
	IgnoreByType     map[string][]string
}

// DefaultOptions returns default ignore lists.
func DefaultOptions() Options {
	return Options{
		IgnoreGlobal: []string{
			"id", "arn", "tags_all", "tags", "timeouts",
			"region", "account_id", "unique_id", "owner_id",
			"private_dns", "public_dns", "primary_network_interface_id",
		},
		IgnoreByType: map[string][]string{
			"aws_instance": {"cpu_core_count", "instance_state"},
		},
	}
}

// Engine compares expected and actual resource sets.
type Engine struct {
	opts Options
}

// NewEngine creates a drift engine.
func NewEngine(opts Options) *Engine {
	return &Engine{opts: opts}
}

type matchKey struct {
	cloudID string
	resType string
	region  string
}

// Compare produces a drift report from two resource sets.
func (e *Engine) Compare(scanID string, expected, actual []models.Resource) *models.DriftReport {
	report := &models.DriftReport{
		ReportVersion: models.ReportVersion,
		ScanID:        scanID,
		Drifts:        []models.DriftItem{},
		Summary: models.DriftSummary{
			ByResourceType: make(map[string]int),
		},
		Resources: models.ScanStats{
			ExpectedCount: len(expected),
			ActualCount:   len(actual),
		},
	}

	expByKey := indexResources(expected)
	actByKey := indexResources(actual)

	matched := make(map[string]bool)

	for key, exp := range expByKey {
		act, ok := actByKey[key]
		if !ok {
			// try matching by cloud ID only
			act, ok = findByCloudID(actByKey, exp.CloudID, exp.Type)
		}
		if !ok {
			report.Drifts = append(report.Drifts, models.DriftItem{
				ResourceID:   exp.ID,
				TerraformRef: exp.TerraformRef,
				Type:         exp.Type,
				Provider:     exp.Provider,
				Region:       exp.Region,
				CloudID:      exp.CloudID,
				DriftType:    models.DriftMissingInCloud,
				Message:      "Resource exists in Terraform state but was not found in cloud",
			})
			e.incSummary(report, models.DriftMissingInCloud, exp.Type)
			continue
		}
		matched[act.ID] = true
		report.Resources.MatchedCount++

		if exp.Type != act.Type {
			report.Drifts = append(report.Drifts, models.DriftItem{
				ResourceID:   exp.ID,
				TerraformRef: exp.TerraformRef,
				Type:         exp.Type,
				Provider:     exp.Provider,
				Region:       exp.Region,
				CloudID:      exp.CloudID,
				DriftType:    models.DriftTypeMismatch,
				Message:      fmt.Sprintf("Type mismatch: expected %s, actual %s", exp.Type, act.Type),
			})
			e.incSummary(report, models.DriftTypeMismatch, exp.Type)
			continue
		}

		tagChanges := diffTags(exp.Tags, act.Tags)
		attrChanges := e.diffAttributes(exp.Type, exp.Attributes, act.Attributes)

		if len(tagChanges) > 0 {
			report.Drifts = append(report.Drifts, models.DriftItem{
				ResourceID:   exp.ID,
				TerraformRef: exp.TerraformRef,
				Type:         exp.Type,
				Provider:     exp.Provider,
				Region:       exp.Region,
				CloudID:      exp.CloudID,
				DriftType:    models.DriftTagsChanged,
				Changes:      tagChanges,
				Message:      fmt.Sprintf("%d tag change(s) detected", len(tagChanges)),
			})
			e.incSummary(report, models.DriftTagsChanged, exp.Type)
		}

		if len(attrChanges) > 0 {
			report.Drifts = append(report.Drifts, models.DriftItem{
				ResourceID:   exp.ID,
				TerraformRef: exp.TerraformRef,
				Type:         exp.Type,
				Provider:     exp.Provider,
				Region:       exp.Region,
				CloudID:      exp.CloudID,
				DriftType:    models.DriftAttributeChanged,
				Changes:      attrChanges,
				Message:      fmt.Sprintf("%d attribute change(s) detected", len(attrChanges)),
			})
			e.incSummary(report, models.DriftAttributeChanged, exp.Type)
		}
	}

	for key, act := range actByKey {
		if matched[act.ID] {
			continue
		}
		// skip if matched via cloud ID alias
		if _, inState := expByKey[key]; inState {
			continue
		}
		report.Drifts = append(report.Drifts, models.DriftItem{
			ResourceID: act.ID,
			Type:       act.Type,
			Provider:   act.Provider,
			Region:     act.Region,
			CloudID:    act.CloudID,
			DriftType:  models.DriftMissingInState,
			Message:    "Resource exists in cloud but is not managed in Terraform state",
		})
		e.incSummary(report, models.DriftMissingInState, act.Type)
	}

	report.Summary.Total = len(report.Drifts)
	return report
}

func indexResources(resources []models.Resource) map[matchKey]models.Resource {
	m := make(map[matchKey]models.Resource)
	for _, r := range resources {
		key := matchKey{
			cloudID: r.CloudID,
			resType: r.Type,
			region:  r.Region,
		}
		if r.CloudID == "" {
			key.cloudID = r.TerraformRef
		}
		m[key] = r
	}
	return m
}

func findByCloudID(m map[matchKey]models.Resource, cloudID, resType string) (models.Resource, bool) {
	if cloudID == "" {
		return models.Resource{}, false
	}
	for k, r := range m {
		if k.cloudID == cloudID && k.resType == resType {
			return r, true
		}
	}
	return models.Resource{}, false
}

func (e *Engine) diffAttributes(resType string, expected, actual map[string]any) []models.FieldChange {
	ignore := e.ignoreSet(resType)
	var changes []models.FieldChange
	seen := make(map[string]bool)

	for k, expVal := range expected {
		if ignore[k] {
			continue
		}
		seen[k] = true
		actVal, ok := actual[k]
		if !ok {
			if expVal == nil || fmt.Sprint(expVal) == "" {
				continue
			}
			changes = append(changes, models.FieldChange{
				Path:     "attributes." + k,
				Expected: expVal,
				Actual:   nil,
			})
			continue
		}
		if !valuesEqual(expVal, actVal) {
			changes = append(changes, models.FieldChange{
				Path:     "attributes." + k,
				Expected: expVal,
				Actual:   actVal,
			})
		}
	}

	for k, actVal := range actual {
		if ignore[k] || seen[k] {
			continue
		}
		if actVal == nil || fmt.Sprint(actVal) == "" {
			continue
		}
		changes = append(changes, models.FieldChange{
			Path:     "attributes." + k,
			Expected: nil,
			Actual:   actVal,
		})
	}

	sort.Slice(changes, func(i, j int) bool {
		return changes[i].Path < changes[j].Path
	})
	return changes
}

func diffTags(expected, actual map[string]string) []models.FieldChange {
	var changes []models.FieldChange
	seen := make(map[string]bool)

	for k, expVal := range expected {
		seen[k] = true
		actVal, ok := actual[k]
		if !ok {
			changes = append(changes, models.FieldChange{
				Path:     "tags." + k,
				Expected: expVal,
				Actual:   nil,
			})
			continue
		}
		if expVal != actVal {
			changes = append(changes, models.FieldChange{
				Path:     "tags." + k,
				Expected: expVal,
				Actual:   actVal,
			})
		}
	}

	for k, actVal := range actual {
		if seen[k] {
			continue
		}
		changes = append(changes, models.FieldChange{
			Path:     "tags." + k,
			Expected: nil,
			Actual:   actVal,
		})
	}

	sort.Slice(changes, func(i, j int) bool {
		return changes[i].Path < changes[j].Path
	})
	return changes
}

func (e *Engine) ignoreSet(resType string) map[string]bool {
	set := make(map[string]bool)
	for _, k := range e.opts.IgnoreGlobal {
		set[k] = true
	}
	for _, k := range e.opts.IgnoreByType[resType] {
		set[k] = true
	}
	return set
}

func (e *Engine) incSummary(report *models.DriftReport, dt models.DriftType, resType string) {
	switch dt {
	case models.DriftMissingInCloud:
		report.Summary.MissingInCloud++
	case models.DriftMissingInState:
		report.Summary.MissingInState++
	case models.DriftAttributeChanged:
		report.Summary.AttributeChanged++
	case models.DriftTagsChanged:
		report.Summary.TagsChanged++
	case models.DriftTypeMismatch:
		report.Summary.TypeMismatch++
	}
	report.Summary.ByResourceType[resType]++
}

func valuesEqual(a, b any) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}

	// Normalize numeric types
	if fa, ok := toFloat64(a); ok {
		if fb, ok := toFloat64(b); ok {
			return fa == fb
		}
	}

	av := reflect.ValueOf(a)
	bv := reflect.ValueOf(b)

	if av.Kind() == reflect.Slice || av.Kind() == reflect.Array {
		if bv.Kind() != reflect.Slice && bv.Kind() != reflect.Array {
			return fmt.Sprint(a) == fmt.Sprint(b)
		}
		if av.Len() != bv.Len() {
			return false
		}
		for i := 0; i < av.Len(); i++ {
			if !valuesEqual(av.Index(i).Interface(), bv.Index(i).Interface()) {
				return false
			}
		}
		return true
	}

	if av.Kind() == reflect.Map && bv.Kind() == reflect.Map {
		if av.Len() != bv.Len() {
			return false
		}
		for _, k := range av.MapKeys() {
			if !valuesEqual(av.MapIndex(k).Interface(), bv.MapIndex(k).Interface()) {
				return false
			}
		}
		return true
	}

	return strings.EqualFold(fmt.Sprint(a), fmt.Sprint(b))
}

func toFloat64(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case int32:
		return float64(n), true
	default:
		return 0, false
	}
}
