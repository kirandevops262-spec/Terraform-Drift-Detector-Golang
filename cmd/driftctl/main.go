package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/terraform-drift-detector/golang/internal/cloud"
	"github.com/terraform-drift-detector/golang/internal/cloud/aws"
	"github.com/terraform-drift-detector/golang/internal/config"
	"github.com/terraform-drift-detector/golang/internal/report"
	"github.com/terraform-drift-detector/golang/internal/scan"
)

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "driftctl",
		Short: "Terraform drift detection CLI",
	}
	root.AddCommand(newScanCmd())
	root.AddCommand(newVersionCmd())
	return root
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("driftctl 1.0.0")
		},
	}
}

func newScanCmd() *cobra.Command {
	var (
		configPath string
		statePath  string
		provider   string
		regions    []string
		outputFmt  string
		jsonPath   string
		noConsole    bool
	)

	cmd := &cobra.Command{
		Use:   "scan",
		Short: "Run a drift scan comparing Terraform state to live cloud resources",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.Default()
			if configPath != "" {
				var err error
				cfg, err = config.Load(configPath)
				if err != nil {
					return err
				}
			}
			if statePath != "" {
				cfg.State.Path = statePath
				cfg.State.Source = "local"
			}
			if provider != "" && len(cfg.Providers) > 0 {
				cfg.Providers[0].Name = provider
			}
			if len(regions) > 0 && len(cfg.Providers) > 0 {
				cfg.Providers[0].Regions = regions
			}

			registry := cloud.NewRegistry()
			registry.Register(aws.NewProvider())

			scanner := scan.NewScanner(registry)
			ctx := context.Background()

			driftReport, err := scanner.Run(ctx, cfg)
			if err != nil {
				return err
			}

			if outputFmt == "json" || jsonPath != "" {
				if jsonPath != "" {
					if err := os.MkdirAll(filepath.Dir(jsonPath), 0755); err != nil && filepath.Dir(jsonPath) != "." {
						return err
					}
					if err := report.WriteJSONFile(jsonPath, driftReport); err != nil {
						return err
					}
					fmt.Fprintf(os.Stderr, "JSON report written to %s\n", jsonPath)
				} else {
					if err := report.WriteJSON(os.Stdout, driftReport); err != nil {
						return err
					}
				}
			}

			if !noConsole && cfg.Output.Console && outputFmt != "json" {
				report.WriteConsole(os.Stdout, driftReport)
			}

			if driftReport.Summary.Total > 0 {
				return fmt.Errorf("drift detected: %d finding(s)", driftReport.Summary.Total)
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&configPath, "config", "c", "", "Path to drift.yaml config file")
	cmd.Flags().StringVar(&statePath, "state", "", "Path to terraform.tfstate file")
	cmd.Flags().StringVar(&provider, "provider", "", "Cloud provider (aws)")
	cmd.Flags().StringSliceVar(&regions, "region", nil, "Cloud regions to scan")
	cmd.Flags().StringVar(&outputFmt, "output", "", "Output format: json")
	cmd.Flags().StringVar(&jsonPath, "json", "", "Write JSON report to file")
	cmd.Flags().BoolVar(&noConsole, "no-console", false, "Suppress console output")
	return cmd
}
