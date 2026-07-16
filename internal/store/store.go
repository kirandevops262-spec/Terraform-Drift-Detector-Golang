package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/terraform-drift-detector/golang/pkg/models"
)

// Store persists scans and schedules.
type Store struct {
	db *sql.DB
}

// Open connects to the database and runs migrations.
func Open(driver, dsn string) (*Store, error) {
	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, err
	}
	if driver == "sqlite3" {
		if _, err := db.Exec("PRAGMA journal_mode=WAL;"); err != nil {
			return nil, err
		}
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) migrate() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS scans (
			id TEXT PRIMARY KEY,
			status TEXT NOT NULL,
			started_at TEXT NOT NULL,
			completed_at TEXT,
			error TEXT,
			report_json TEXT,
			config_json TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS schedules (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			cron TEXT NOT NULL,
			enabled INTEGER NOT NULL DEFAULT 1,
			config_json TEXT NOT NULL,
			last_run_at TEXT,
			created_at TEXT NOT NULL
		)`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
	}
	return nil
}

// Close closes the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// SaveScan persists a scan record.
func (s *Store) SaveScan(rec *models.ScanRecord) error {
	var reportJSON sql.NullString
	if rec.Report != nil {
		b, err := json.Marshal(rec.Report)
		if err != nil {
			return err
		}
		reportJSON = sql.NullString{String: string(b), Valid: true}
	}
	var completed sql.NullString
	if rec.CompletedAt != nil {
		completed = sql.NullString{String: rec.CompletedAt.Format(time.RFC3339), Valid: true}
	}
	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO scans (id, status, started_at, completed_at, error, report_json, config_json)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		rec.ID, rec.Status, rec.StartedAt.Format(time.RFC3339), completed, rec.Error, reportJSON, rec.Config,
	)
	return err
}

// GetScan retrieves a scan by ID.
func (s *Store) GetScan(id string) (*models.ScanRecord, error) {
	row := s.db.QueryRow(`SELECT id, status, started_at, completed_at, error, report_json, config_json FROM scans WHERE id = ?`, id)
	return scanRow(row)
}

// ListScans returns recent scans.
func (s *Store) ListScans(limit int) ([]*models.ScanRecord, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.Query(`SELECT id, status, started_at, completed_at, error, report_json, config_json FROM scans ORDER BY started_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*models.ScanRecord
	for rows.Next() {
		rec, err := scanRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

type scannable interface {
	Scan(dest ...any) error
}

func scanRow(row scannable) (*models.ScanRecord, error) {
	var rec models.ScanRecord
	var started, completed, reportJSON, configJSON sql.NullString
	var errMsg sql.NullString
	if err := row.Scan(&rec.ID, &rec.Status, &started, &completed, &errMsg, &reportJSON, &configJSON); err != nil {
		return nil, err
	}
	t, err := time.Parse(time.RFC3339, started.String)
	if err != nil {
		return nil, err
	}
	rec.StartedAt = t
	if completed.Valid {
		ct, err := time.Parse(time.RFC3339, completed.String)
		if err != nil {
			return nil, err
		}
		rec.CompletedAt = &ct
	}
	if errMsg.Valid {
		rec.Error = errMsg.String
	}
	if configJSON.Valid {
		rec.Config = configJSON.String
	}
	if reportJSON.Valid && reportJSON.String != "" {
		var report models.DriftReport
		if err := json.Unmarshal([]byte(reportJSON.String), &report); err != nil {
			return nil, err
		}
		rec.Report = &report
	}
	return &rec, nil
}

// SaveSchedule persists a schedule.
func (s *Store) SaveSchedule(sch *models.Schedule) error {
	var lastRun sql.NullString
	if sch.LastRunAt != nil {
		lastRun = sql.NullString{String: sch.LastRunAt.Format(time.RFC3339), Valid: true}
	}
	enabled := 0
	if sch.Enabled {
		enabled = 1
	}
	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO schedules (id, name, cron, enabled, config_json, last_run_at, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		sch.ID, sch.Name, sch.Cron, enabled, sch.Config, lastRun, sch.CreatedAt.Format(time.RFC3339),
	)
	return err
}

// ListSchedules returns all schedules.
func (s *Store) ListSchedules() ([]*models.Schedule, error) {
	rows, err := s.db.Query(`SELECT id, name, cron, enabled, config_json, last_run_at, created_at FROM schedules ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*models.Schedule
	for rows.Next() {
		var sch models.Schedule
		var enabled int
		var lastRun, created sql.NullString
		if err := rows.Scan(&sch.ID, &sch.Name, &sch.Cron, &enabled, &sch.Config, &lastRun, &created); err != nil {
			return nil, err
		}
		sch.Enabled = enabled == 1
		if lastRun.Valid {
			t, _ := time.Parse(time.RFC3339, lastRun.String)
			sch.LastRunAt = &t
		}
		if created.Valid {
			sch.CreatedAt, _ = time.Parse(time.RFC3339, created.String)
		}
		out = append(out, &sch)
	}
	return out, rows.Err()
}

// GetSchedule retrieves a schedule by ID.
func (s *Store) GetSchedule(id string) (*models.Schedule, error) {
	row := s.db.QueryRow(`SELECT id, name, cron, enabled, config_json, last_run_at, created_at FROM schedules WHERE id = ?`, id)
	var sch models.Schedule
	var enabled int
	var lastRun, created sql.NullString
	if err := row.Scan(&sch.ID, &sch.Name, &sch.Cron, &enabled, &sch.Config, &lastRun, &created); err != nil {
		return nil, err
	}
	sch.Enabled = enabled == 1
	if lastRun.Valid {
		t, _ := time.Parse(time.RFC3339, lastRun.String)
		sch.LastRunAt = &t
	}
	if created.Valid {
		sch.CreatedAt, _ = time.Parse(time.RFC3339, created.String)
	}
	return &sch, nil
}
