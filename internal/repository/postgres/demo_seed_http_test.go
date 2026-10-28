package postgres

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/auth"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/demoseed"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/health"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/material"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/notification"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/platform/httpserver"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/procurement"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/production"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/quotation"
	req "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/request"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func demoTestServer(t *testing.T, db *Database) *httptest.Server {
	t.Helper()
	a, e := auth.NewService(db, auth.NewArgonHasher(), time.Hour)
	if e != nil {
		t.Fatal(e)
	}
	h := auth.NewHandler(a)
	router := httpserver.RouterWithRoutes(slog.New(slog.NewTextHandler(io.Discard, nil)), 10*time.Second, []string{"http://localhost:3000"}, health.Live, health.NewService(db, time.Second).Handler, func(r chi.Router) {
		h.Routes(r)
		h.CustomerRoutes(r, auth.NewCustomerService(db, auth.NewArgonHasher()))
		req.NewHandler(req.NewService(db)).Routes(r, h.Authenticate)
		material.NewHandler(material.NewService(db)).Routes(r, h.Authenticate)
		procurement.NewHandler(procurement.NewService(db)).Routes(r, h.Authenticate)
		production.NewHandler(production.NewService(db), production.NewPhotoService(db, nil)).Routes(r, h.Authenticate)
		quotation.NewHandler(quotation.NewService(db)).Routes(r, h.Authenticate)
		notification.Routes(r, notification.NewInbox(db), h.Authenticate)
	})
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)
	return server
}

func verifyDemoAccountHTTP(t *testing.T, db *Database, cfg demoseed.Config, password string) {
	t.Helper()
	ctx := context.Background()
	seed := demoseed.New(NewDemoSeedRepository(db), auth.NewArgonHasher(), cfg)
	server := demoTestServer(t, db)
	if e := seed.VerifyHTTP(ctx, server.URL, func(string) string { return password }); e != nil {
		t.Fatal(e)
	}
	server.Close()
	// A separate fresh server fixture has its own unmodified production limiter.
	// Each fixture performs six logins, below the existing ten-per-minute limit.
	server = demoTestServer(t, db)
	call := func(method, path, token string, body any, want int, out any) {
		t.Helper()
		var data []byte
		if body != nil {
			data, _ = json.Marshal(body)
		}
		r, e := http.NewRequest(method, server.URL+path, bytes.NewReader(data))
		if e != nil {
			t.Fatal("request construction failed")
		}
		if body != nil {
			r.Header.Set("Content-Type", "application/json")
		}
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		response, e := server.Client().Do(r)
		if e != nil {
			t.Fatal("real HTTP transport failed")
		}
		defer response.Body.Close()
		if response.StatusCode != want {
			t.Fatalf("%s %s returned %d, want %d (body withheld)", method, path, response.StatusCode, want)
		}
		if out != nil {
			if e = json.NewDecoder(io.LimitReader(response.Body, 128*1024)).Decode(out); e != nil {
				t.Fatal("invalid response JSON")
			}
		}
	}
	customers := auth.NewCustomerService(db, auth.NewArgonHasher())
	other, e := customers.Register(ctx, auth.Registration{Username: "outside.company", Name: "Outside Test Customer", Password: password, CustomerType: "company", CompanyName: "Outside Test Company"})
	if e != nil {
		t.Fatal(e)
	}
	tokens := map[string]string{}
	for _, username := range []string{"nida.company", "ploy.company", "somchai", "anan.staff", "pim.admin", other.Username} {
		var login auth.LoginResult
		call("POST", "/auth/login", "", map[string]string{"username": username, "password": password}, 200, &login)
		tokens[username] = login.AccessToken
	}
	defer func() {
		for _, token := range tokens {
			call("POST", "/auth/logout", token, nil, 204, nil)
		}
	}()
	profiles := map[string]string{}
	for _, username := range []string{"nida.company", "ploy.company", "somchai", other.Username} {
		var out struct {
			Items []auth.CustomerProfile `json:"items"`
		}
		call("GET", "/customers", tokens[username], nil, 200, &out)
		if len(out.Items) != 1 {
			t.Fatal("unexpected company memberships")
		}
		profiles[username] = out.Items[0].ID
	}
	requestIDs := map[string]string{}
	for _, username := range []string{"nida.company", "somchai", other.Username} {
		var out req.Request
		call("POST", "/requests", tokens[username], map[string]any{"customer_id": profiles[username], "description": "Disposable seed isolation check", "qty": 1, "material_spec": "S45C"}, 201, &out)
		requestIDs[username] = out.ID
	}
	call("GET", "/requests/"+requestIDs["nida.company"], tokens["ploy.company"], nil, 200, nil)
	for _, pair := range [][2]string{{"somchai", "nida.company"}, {"nida.company", "somchai"}, {other.Username, "nida.company"}, {"ploy.company", other.Username}} {
		call("GET", "/requests/"+requestIDs[pair[1]], tokens[pair[0]], nil, 404, nil)
	}
	for _, username := range []string{"anan.staff", "pim.admin"} {
		call("GET", "/requests/"+requestIDs[other.Username], tokens[username], nil, 200, nil)
	}
	for _, username := range []string{"ploy.company", "somchai"} {
		call("POST", "/customers/"+profiles[username]+"/invitations", tokens[username], nil, 403, nil)
	}
	call("POST", "/customers/"+profiles["nida.company"]+"/invitations", tokens["nida.company"], nil, 201, nil)
	call("POST", "/units", tokens["anan.staff"], map[string]string{"name": "denied"}, 403, nil)
	call("POST", "/suppliers", tokens["anan.staff"], map[string]string{"name": "denied"}, 403, nil)
	call("POST", "/quotations/estimate", tokens["pim.admin"], map[string]string{"material_cost": "1.00", "labor_cost": "1.00"}, 403, nil)
	call("POST", "/production/jobs/"+uuid.NewString()+"/start", tokens["pim.admin"], map[string]any{"idempotency_key": uuid.NewString(), "expected_version": 0, "expected_plan_version": 1}, 403, nil)
	call("POST", "/requests", tokens["somchai"], map[string]any{"customer_id": profiles["nida.company"], "description": "Denied cross-company create", "qty": 1}, 403, nil)
	// Outbox rows come from real request transactions, not mock notifications.
	var count int
	if e = db.pool.QueryRow(ctx, `SELECT count(*) FROM outbox`).Scan(&count); e != nil || count != 6 {
		t.Fatal("real request/outbox transaction missing")
	}
}
