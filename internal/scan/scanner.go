package scan

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/terraform-drift-detector/golang/internal/cloud"
	"github.com/terraform-drift-detector/golang/internal/config"
	"github.com/terraform-drift-detector/golang/internal/drift"
	"github.com/terraform-drift-detector/golang/internal/extract"
	"github.com/terraform-drift-detector/golang/internal/state"
	"github.com/terraform-drift-detector/golang/pkg/models"
)

// Scanner orchestrates drift detection scans.
type Scanner struct {
	registry *cloud.Registry
}

// NewScanner creates a scanner with the given provider registry.
func NewScanner(registry *cloud.Registry) *Scanner {
	return &Scanner{registry: registry}
}

// Run executes a full drift scan.
func (s *Scanner) Run(ctx context.Context, cfg *config.Config) (*models.DriftReport, error) {
	scanID := uuid.New().String()
	started := time.Now().UTC()

	reader, err := state.NewReader(cfg.State.Source, cfg.State.Path)
	if err != nil {
		return nil, err
	}
	rawState, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("load state: %w", err)
	}

	providerCfg := cfg.PrimaryProvider()
	if providerCfg == nil {
		return nil, fmt.Errorf("no cloud provider configured")
	}

	prov, ok := s.registry.Get(providerCfg.Name)
	if !ok {
		return nil, fmt.Errorf("provider not registered: %s", providerCfg.Name)
	}

	extractOpts := extract.Options{
		Provider: providerCfg.Name,
	}
	if len(providerCfg.Regions) > 0 {
		extractOpts.Region = providerCfg.Regions[0]
	}

	expected := extract.FromState(rawState, extractOpts)

	scope := cloud.FetchScope{
		Regions:       providerCfg.Regions,
		ResourceTypes: providerCfg.ResourceTypes,
	}

	rawCloud, err := prov.Fetch(ctx, scope)
	if err != nil {
		return nil, fmt.Errorf("fetch cloud resources: %w", err)
	}
	actual := extract.FromCloud(rawCloud)

	engineOpts := drift.DefaultOptions()
	if cfg.Comparison.IgnoreAttributes != nil {
		if global, ok := cfg.Comparison.IgnoreAttributes["global"]; ok {
			engineOpts.IgnoreGlobal = global
		}
		for k, v := range cfg.Comparison.IgnoreAttributes {
			if k != "global" {
				engineOpts.IgnoreByType[k] = v
			}
		}
	}

	engine := drift.NewEngine(engineOpts)
	report := engine.Compare(scanID, expected, actual)
	report.StartedAt = started
	report.CompletedAt = time.Now().UTC()
	return report, nil
}
