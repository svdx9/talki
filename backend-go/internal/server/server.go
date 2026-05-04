package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/svdx9/talki/backend-go/internal/config"
	"github.com/svdx9/talki/backend-go/internal/skills"
)

type Server struct {
	cfg     config.Config
	sk      []skills.Skill
	log     *slog.Logger
	mux     *http.ServeMux
	httpSrv *http.Server
}

func New(cfg config.Config, sk []skills.Skill, log *slog.Logger) *Server {
	s := &Server{
		cfg:     cfg,
		sk:      sk,
		log:     log,
		mux:     http.NewServeMux(),
		httpSrv: nil,
	}

	s.registerRoutes()

	//exhaustruct:ignore
	s.httpSrv = &http.Server{
		Addr:         ":" + strconv.Itoa(cfg.Port),
		Handler:      s.mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return s
}

func (s *Server) registerRoutes() {
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
	s.mux.HandleFunc("GET /api/skills", s.handleSkills)
	s.mux.Handle("/api/ws", s.newWSServer())
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
	type skillSubset struct {
		ID    string `json:"id"`
		Title string `json:"title"`
		Level string `json:"level"`
	}
	out := make([]skillSubset, len(s.sk))
	for i, sk := range s.sk {
		out[i] = skillSubset{
			ID:    sk.ID,
			Title: sk.Title,
			Level: sk.Level,
		}
	}
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	err := enc.Encode(out)
	_ = err // ResponseWriter is in-memory; encoding should not fail
}

func (s *Server) ListenAndServe() error {
	s.log.Info("server listening on " + s.httpSrv.Addr)
	return s.httpSrv.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpSrv.Shutdown(ctx)
}
