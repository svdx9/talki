package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/svdx9/talki/backend/internal/config"
	"github.com/svdx9/talki/backend/internal/skills"
)

type Server struct {
	cfg     config.Config
	log     *slog.Logger
	mux     *http.ServeMux
	httpSrv *http.Server
	sp      skills.Repository
}

func New(cfg config.Config, log *slog.Logger, sp skills.Repository) (*Server, error) {
	s := &Server{
		cfg:     cfg,
		log:     log,
		mux:     http.NewServeMux(),
		httpSrv: nil,
		sp:      sp,
	}

	s.registerRoutes(sp)

	//exhaustruct:ignore
	s.httpSrv = &http.Server{
		Addr:         ":" + strconv.Itoa(cfg.Port),
		Handler:      s.mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return s, nil
}

func (s *Server) registerRoutes(sp skills.Repository) {
	cs := &chatServer{
		skills: sp,
		l:      s.log,
		cfg:    s.cfg,
	}
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
	s.mux.HandleFunc("GET /api/skills", s.handleSkills)
	s.mux.Handle("/api/ws", cs)
	s.mux.Handle("/", s.staticHandler())
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	resp := map[string]string{
		"status":    "ok",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	err := enc.Encode(resp)
	_ = err // ResponseWriter is in-memory; encoding should not fail
}

func (s *Server) handleSkills(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	err := enc.Encode(s.sp.Descriptions())
	_ = err // ResponseWriter is in-memory; encoding should not fail
}

func (s *Server) ListenAndServe() error {
	s.log.Info("server listening on " + s.httpSrv.Addr)
	return s.httpSrv.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpSrv.Shutdown(ctx)
}
