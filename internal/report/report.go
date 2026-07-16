package report

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/terraform-drift-detector/golang/pkg/models"
)

// WriteJSON writes a drift report as JSON.
func WriteJSON(w io.Writer, report *models.DriftReport) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

// WriteJSONFile writes a drift report to a file.
func WriteJSONFile(path string, report *models.DriftReport) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return WriteJSON(f, report)
}

// WriteConsole prints a human-readable drift report.
func WriteConsole(w io.Writer, report *models.DriftReport) {
	fmt.Fprintf(w, "\nTerraform Drift Report (scan: %s)\n", report.ScanID)
	fmt.Fprintf(w, "Duration: %s\n\n", report.CompletedAt.Sub(report.StartedAt).Round(1e6))

	fmt.Fprintf(w, "Resources: expected=%d actual=%d matched=%d\n",
		report.Resources.ExpectedCount,
		report.Resources.ActualCount,
		report.Resources.MatchedCount)

	fmt.Fprintf(w, "Drift Summary: total=%d missing_in_cloud=%d missing_in_state=%d attribute_changed=%d tags_changed=%d\n\n",
		report.Summary.Total,
		report.Summary.MissingInCloud,
		report.Summary.MissingInState,
		report.Summary.AttributeChanged,
		report.Summary.TagsChanged)

	if len(report.Drifts) == 0 {
		fmt.Fprintln(w, "No drift detected.")
		return
	}

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "DRIFT TYPE\tRESOURCE\tTYPE\tREGION\tDETAILS")
	for _, d := range report.Drifts {
		ref := d.TerraformRef
		if ref == "" {
			ref = d.CloudID
		}
		details := d.Message
		if len(d.Changes) > 0 {
			parts := make([]string, 0, len(d.Changes))
			for _, c := range d.Changes {
				parts = append(parts, fmt.Sprintf("%s: %v -> %v", c.Path, c.Expected, c.Actual))
			}
			details = strings.Join(parts, "; ")
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
			d.DriftType, ref, d.Type, d.Region, details)
	}
	tw.Flush()
	fmt.Fprintln(w)
}
