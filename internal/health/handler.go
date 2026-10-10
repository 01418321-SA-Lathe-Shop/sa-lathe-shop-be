package health

import (
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/platform/httpserver"
	"net/http"
)

func Live(w http.ResponseWriter, r *http.Request) {
	httpserver.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
