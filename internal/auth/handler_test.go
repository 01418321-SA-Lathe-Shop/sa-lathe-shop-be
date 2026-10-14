package auth

import (
	"encoding/json"
	"github.com/go-chi/chi/v5"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHTTPAuthAndRoles(t *testing.T) {
	s, _, _ := setupService(t)
	h := NewHandler(s)
	r := chi.NewRouter()
	h.Routes(r)
	request := func(method, path, body, token string, want int) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != want {
			t.Fatalf("%s %s: %d %s", method, path, w.Code, w.Body.String())
		}
		if strings.Contains(w.Body.String(), "password_hash") {
			t.Fatal("hash field exposed")
		}
		return w
	}
	request("GET", "/auth/me", "", "", 401)
	w := request("POST", "/auth/login", `{"username":"admin.one","password":"Test-password-1234"}`, "", 200)
	var adminLogin LoginResult
	if err := json.Unmarshal(w.Body.Bytes(), &adminLogin); err != nil {
		t.Fatal(err)
	}
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("token response cacheable")
	}
	request("POST", "/admin/workers", `{"username":"worker.one","password":"Worker-password-1234","name":"Worker","employee_no":"EMP-1","role":"admin"}`, adminLogin.AccessToken, 400)
	request("POST", "/admin/workers", `{"username":"worker.one","password":"Worker-password-1234","name":"Worker","employee_no":"EMP-1"}`, adminLogin.AccessToken, 201)
	w = request("POST", "/auth/login", `{"username":"worker.one","password":"Worker-password-1234"}`, "", 200)
	var workerLogin LoginResult
	_ = json.Unmarshal(w.Body.Bytes(), &workerLogin)
	request("GET", "/auth/me", "", workerLogin.AccessToken, 200)
	request("GET", "/admin/workers", "", workerLogin.AccessToken, 403)
	request("GET", "/admin/workers?limit=101", "", adminLogin.AccessToken, 400)
	request("GET", "/admin/workers?limit=bad", "", adminLogin.AccessToken, 400)
	request("POST", "/auth/logout", "", workerLogin.AccessToken, 204)
	request("GET", "/auth/me", "", workerLogin.AccessToken, 401)
	request("POST", "/auth/login", `{"username":"admin.one","password":"Test-password-1234"} {}`, "", 400)
	request("POST", "/auth/login", `{"username":"admin.one","password":"`+strings.Repeat("x", 9000)+`"}`, "", 400)
}
func TestBearerOnly(t *testing.T) {
	s, _, _ := setupService(t)
	h := NewHandler(s)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/auth/me?token=secret", nil)
	h.Authenticate(http.HandlerFunc(h.me)).ServeHTTP(w, r)
	if w.Code != 401 {
		t.Fatal("URL credential accepted")
	}
}
