package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/terraform-drift-detector/golang/internal/api"
	"github.com/terraform-drift-detector/golang/internal/cloud"
	"github.com/terraform-drift-detector/golang/internal/cloud/aws"
	"github.com/terraform-drift-detector/golang/internal/config"
	"github.com/terraform-drift-detector/golang/internal/schedule"
	"github.com/terraform-drift-detector/golang/internal/scan"
	"github.com/terraform-drift-detector/golang/internal/store"
)

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	var configPath string
	root := &cobra.Command{
		Use:   "driftd",
		Short: "Terraform drift detection API server",
	}
	root.PersistentFlags().StringVarP(&configPath, "config", "c", "configs/drift.example.yaml", "Path to config file")

	root.RunE = func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(configPath)
		if err != nil {
			cfg = config.Default()
			log.Printf("using default config: %v", err)
		}

		st, err := store.Open("sqlite3", cfg.Database.DSN)
		if err != nil {
			return fmt.Errorf("open database: %w", err)
		}
		defer st.Close()

		registry := cloud.NewRegistry()
		registry.Register(aws.NewProvider())
		scanner := scan.NewScanner(registry)
		sched := schedule.NewRunner(st, scanner)

		if err := sched.LoadFromConfig(cfg, configPath); err != nil {
			log.Printf("warning: schedule load: %v", err)
		}
		sched.Start()
		defer sched.Stop()

		srv := api.NewServer(cfg, st, scanner, sched)
		log.Printf("driftd listening on %s", cfg.API.Addr)

		ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer stop()
		return srv.Run(ctx)
	}
	return root
}
