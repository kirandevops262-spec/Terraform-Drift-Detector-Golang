package schedule

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/robfig/cron/v3"

	"github.com/terraform-drift-detector/golang/internal/config"
	"github.com/terraform-drift-detector/golang/internal/scan"
	"github.com/terraform-drift-detector/golang/internal/store"
	"github.com/terraform-drift-detector/golang/pkg/models"
)

// Runner manages scheduled drift scans.
type Runner struct {
	cron    *cron.Cron
	store   *store.Store
	scanner *scan.Scanner
	mu      sync.Mutex
	entries map[string]cron.EntryID
}

// NewRunner creates a schedule runner.
func NewRunner(st *store.Store, scanner *scan.Scanner) *Runner {
	return &Runner{
		cron:    cron.New(),
		store:   st,
		scanner: scanner,
		entries: make(map[string]cron.EntryID),
	}
}

// LoadFromConfig registers schedules from config file entries.
func (r *Runner) LoadFromConfig(cfg *config.Config, configPath string) error {
	for _, sch := range cfg.Schedules {
		if !sch.Enabled {
			continue
		}
		cfgCopy := *cfg
		b, _ := json.Marshal(&cfgCopy)
		schedule := &models.Schedule{
			ID:        uuid.New().String(),
			Name:      sch.Name,
			Cron:      sch.Cron,
			Enabled:   true,
			Config:    string(b),
			CreatedAt: time.Now().UTC(),
		}
		if err := r.store.SaveSchedule(schedule); err != nil {
			return err
		}
		if err := r.Register(schedule); err != nil {
			return err
		}
	}
	return nil
}

// Register adds a schedule to the cron runner.
func (r *Runner) Register(sch *models.Schedule) error {
	if !sch.Enabled {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	if id, ok := r.entries[sch.ID]; ok {
		r.cron.Remove(id)
	}

	schCopy := *sch
	entryID, err := r.cron.AddFunc(sch.Cron, func() {
		r.runSchedule(context.Background(), &schCopy)
	})
	if err != nil {
		return err
	}
	r.entries[sch.ID] = entryID
	return nil
}

// Start begins the cron scheduler.
func (r *Runner) Start() {
	r.cron.Start()
}

// Stop halts the cron scheduler.
func (r *Runner) Stop() {
	r.cron.Stop()
}

func (r *Runner) runSchedule(ctx context.Context, sch *models.Schedule) {
	log.Printf("running scheduled scan: %s", sch.Name)
	cfg := config.Default()
	if sch.Config != "" {
		if err := json.Unmarshal([]byte(sch.Config), cfg); err != nil {
			log.Printf("schedule %s config error: %v", sch.Name, err)
			return
		}
	}
	rec := &models.ScanRecord{
		ID:        uuid.New().String(),
		Status:    models.ScanStatusRunning,
		StartedAt: time.Now().UTC(),
		Config:    sch.Config,
	}
	_ = r.store.SaveScan(rec)

	report, err := r.scanner.Run(ctx, cfg)
	now := time.Now().UTC()
	rec.CompletedAt = &now
	if err != nil {
		rec.Status = models.ScanStatusFailed
		rec.Error = err.Error()
		log.Printf("schedule %s failed: %v", sch.Name, err)
	} else {
		rec.Status = models.ScanStatusCompleted
		rec.Report = report
		rec.ID = report.ScanID
		log.Printf("schedule %s completed: %d drifts", sch.Name, report.Summary.Total)
	}
	_ = r.store.SaveScan(rec)

	sch.LastRunAt = &now
	_ = r.store.SaveSchedule(sch)
}
