package server

import (
	"net/http"
	"os"
	"path/filepath"
)

func (s *Server) staticHandler() http.Handler {
	absPath, err := filepath.Abs("../frontend/dist")
	if err != nil {
		s.log.Warn("could not resolve static directory path", "error", err)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.NotFound(w, r)
		})
	}

	_, err = os.Stat(absPath)
	if err != nil {
		s.log.Warn("static directory does not exist", "path", absPath)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.NotFound(w, r)
		})
	}

	s.log.Info("serving static files", "path", absPath)
	return http.FileServer(http.Dir(absPath))
}
