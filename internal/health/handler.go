package health

import (
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/platform/httpserver"
	"net/http"
)

func Live(w http.ResponseWriter, r *http.Request) {
	httpserver.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Service) Handler(w http.ResponseWriter, r *http.Request) {
	if err := s.Ready(r.Context()); err != nil {
		httpserver.LogInternalError(r, err)
		httpserver.WriteError(w, r, http.StatusServiceUnavailable, "database_unavailable", "Database is not ready")
		return
	}
	httpserver.JSON(w, http.StatusOK, map[string]string{"status": "ok", "database": "ok"})
}
