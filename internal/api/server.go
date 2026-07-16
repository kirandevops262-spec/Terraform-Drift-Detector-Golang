package api

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"

	"github.com/terraform-drift-detector/golang/internal/config"
	"github.com/terraform-drift-detector/golang/internal/scan"
	"github.com/terraform-drift-detector/golang/internal/schedule"
	"github.com/terraform-drift-detector/golang/internal/store"
	"github.com/terraform-drift-detector/golang/pkg/models"
)

// Server is the HTTP API for drift detection.
type Server struct {
	cfg      *config.Config
	store    *store.Store
	scanner  *scan.Scanner
	scheduler *schedule.Runner
	apiKey   string
}

// NewServer creates an API server.
func NewServer(cfg *config.Config, st *store.Store, scanner *scan.Scanner, sched *schedule.Runner) *Server {
	return &Server{
		cfg:       cfg,
		store:     st,
		scanner:   scanner,
		scheduler: sched,
		apiKey:    cfg.API.APIKey,
	}
}

// Handler returns the HTTP handler.
func (s *Server) Handler() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/api/v1/health", s.handleHealth)

	r.Route("/api/v1", func(r chi.Router) {
		r.Use(s.authMiddleware)
		r.Post("/scans", s.handleCreateScan)
		r.Get("/scans", s.handleListScans)
		r.Get("/scans/{id}", s.handleGetScan)
		r.Get("/scans/{id}/report", s.handleGetReport)
		r.Get("/scans/{id}/drifts", s.handleGetDrifts)
		r.Post("/schedules", s.handleCreateSchedule)
		r.Get("/schedules", s.handleListSchedules)
	})

	// Dashboard static files
	r.Get("/", s.handleDashboard)
	r.Get("/dashboard/*", s.handleDashboardStatic)

	return r
}

func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.apiKey == "" {
			next.ServeHTTP(w, r)
			return
		}
		key := r.Header.Get("X-API-Key")
		if key == "" {
			key = r.URL.Query().Get("api_key")
		}
		if key != s.apiKey {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleCreateScan(w http.ResponseWriter, r *http.Request) {
	cfg := s.cfg
	if r.Body != nil && r.ContentLength > 0 {
		var override config.Config
		if err := json.NewDecoder(r.Body).Decode(&override); err == nil {
			if override.State.Path != "" {
				cfg.State = override.State
			}
			if len(override.Providers) > 0 {
				cfg.Providers = override.Providers
			}
		}
	}

	rec := &models.ScanRecord{
		ID:        uuid.New().String(),
		Status:    models.ScanStatusRunning,
		StartedAt: time.Now().UTC(),
	}
	_ = s.store.SaveScan(rec)

	report, err := s.scanner.Run(r.Context(), cfg)
	now := time.Now().UTC()
	rec.CompletedAt = &now
	if err != nil {
		rec.Status = models.ScanStatusFailed
		rec.Error = err.Error()
		_ = s.store.SaveScan(rec)
		writeJSON(w, http.StatusInternalServerError, rec)
		return
	}
	rec.Status = models.ScanStatusCompleted
	rec.Report = report
	rec.ID = report.ScanID
	_ = s.store.SaveScan(rec)
	writeJSON(w, http.StatusCreated, rec)
}

func (s *Server) handleListScans(w http.ResponseWriter, r *http.Request) {
	scans, err := s.store.ListScans(50)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, scans)
}

func (s *Server) handleGetScan(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	rec, err := s.store.GetScan(id)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, rec)
}

func (s *Server) handleGetReport(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	rec, err := s.store.GetScan(id)
	if err != nil || rec.Report == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, rec.Report)
}

func (s *Server) handleGetDrifts(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	rec, err := s.store.GetScan(id)
	if err != nil || rec.Report == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	driftType := r.URL.Query().Get("type")
	drifts := rec.Report.Drifts
	if driftType != "" {
		filtered := make([]models.DriftItem, 0)
		for _, d := range drifts {
			if string(d.DriftType) == driftType {
				filtered = append(filtered, d)
			}
		}
		drifts = filtered
	}
	writeJSON(w, http.StatusOK, drifts)
}

type createScheduleRequest struct {
	Name    string `json:"name"`
	Cron    string `json:"cron"`
	Enabled bool   `json:"enabled"`
	Config  string `json:"config"`
}

func (s *Server) handleCreateSchedule(w http.ResponseWriter, r *http.Request) {
	var req createScheduleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	cfgJSON := req.Config
	if cfgJSON == "" {
		b, _ := json.Marshal(s.cfg)
		cfgJSON = string(b)
	}
	sch := &models.Schedule{
		ID:        uuid.New().String(),
		Name:      req.Name,
		Cron:      req.Cron,
		Enabled:   req.Enabled,
		Config:    cfgJSON,
		CreatedAt: time.Now().UTC(),
	}
	if err := s.store.SaveSchedule(sch); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if s.scheduler != nil {
		_ = s.scheduler.Register(sch)
	}
	writeJSON(w, http.StatusCreated, sch)
}

func (s *Server) handleListSchedules(w http.ResponseWriter, r *http.Request) {
	schedules, err := s.store.ListSchedules()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, schedules)
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "web/dashboard/index.html")
}

func (s *Server) handleDashboardStatic(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/dashboard/")
	http.ServeFile(w, r, "web/dashboard/"+path)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// Run starts the HTTP server.
func (s *Server) Run(ctx context.Context) error {
	srv := &http.Server{
		Addr:    s.cfg.API.Addr,
		Handler: s.Handler(),
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

// Ensure web directory exists at startup.
func init() {
	_ = os.MkdirAll("web/dashboard", 0755)
}
