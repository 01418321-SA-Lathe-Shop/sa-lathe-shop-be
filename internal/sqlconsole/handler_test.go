package sqlconsole

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/auth"
	"github.com/go-chi/chi/v5"
)

type fakeRepo struct {
	ran int
	err error
}

func (f *fakeRepo) ExecuteSQL(context.Context, string, time.Duration, int) (Result, error) {
	f.ran++
	return Result{Command: "SELECT 1"}, f.err
}
func (f *fakeRepo) Schema(context.Context) ([]SchemaTable, error) { return nil, nil }

func call(t *testing.T, repo *fakeRepo, acct auth.Account, method, path, body string) *httptest.ResponseRecorder {
	h := NewHandler(repo)
	h.current = func(*http.Request) auth.Account { return acct }
	r := chi.NewRouter()
	h.Routes(r, func(next http.Handler) http.Handler { return next })
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestPermissions(t *testing.T) {
	sup := auth.Account{Active: true, Role: auth.RoleAdmin, SuperAdmin: true}
	for name, a := range map[string]auth.Account{
		"admin":     {Active: true, Role: auth.RoleAdmin},
		"worker":    {Active: true, Role: auth.RoleWorker},
		"customer":  {Active: true, Role: "customer"},
		"inactive":  {Role: auth.RoleAdmin, SuperAdmin: true},
		"anonymous": {},
	} {
		repo := &fakeRepo{}
		if w := call(t, repo, a, "POST", "/admin/sql/execute", `{"sql":"select 1"}`); w.Code != 403 || repo.ran != 0 {
			t.Errorf("%s: code %d ran %d", name, w.Code, repo.ran)
		}
		if w := call(t, repo, a, "GET", "/admin/sql/schema", ""); w.Code != 403 {
			t.Errorf("%s schema: %d", name, w.Code)
		}
	}
	if w := call(t, &fakeRepo{}, sup, "POST", "/admin/sql/execute", `{"sql":"select 1"}`); w.Code != 200 {
		t.Errorf("super admin: %d", w.Code)
	}
}

func TestDestructiveNeedsConfirmation(t *testing.T) {
	sup := auth.Account{Active: true, Role: auth.RoleAdmin, SuperAdmin: true}
	repo := &fakeRepo{}
	for _, sql := range []string{"drop table t", "truncate t", "delete from t", "update t set a=1"} {
		if w := call(t, repo, sup, "POST", "/admin/sql/execute", `{"sql":"`+sql+`"}`); w.Code != 409 || !strings.Contains(w.Body.String(), "confirmation_required") {
			t.Errorf("%s: %d %s", sql, w.Code, w.Body)
		}
	}
	if repo.ran != 0 {
		t.Fatal("destructive statement ran without confirmation")
	}
	if w := call(t, repo, sup, "POST", "/admin/sql/execute", `{"sql":"delete from t","confirm":true}`); w.Code != 200 || repo.ran != 1 {
		t.Errorf("confirmed: %d ran %d", w.Code, repo.ran)
	}
}

func TestSQLErrorIsReportedNot500(t *testing.T) {
	sup := auth.Account{Active: true, Role: auth.RoleAdmin, SuperAdmin: true}
	w := call(t, &fakeRepo{err: &DBError{Message: `syntax error at or near "selec"`, SQLState: "42601", Position: 1}}, sup, "POST", "/admin/sql/execute", `{"sql":"selec 1"}`)
	if w.Code != 422 || !strings.Contains(w.Body.String(), "42601") {
		t.Errorf("%d %s", w.Code, w.Body)
	}
}
