package postgres

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/auth"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/file"
	m "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/material"
	payment "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/payment"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/platform/objectstore"
	p "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/procurement"
	q "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/quotation"
	req "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/request"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"
)

func TestProcurementIntegration(t *testing.T) {
	dsn := os.Getenv("PHASE9_DATABASE_URL")
	if dsn == "" {
		t.Skip("set PHASE9_DATABASE_URL for isolated UC6 integration")
	}
	parsed, err := url.Parse(dsn)
	if err != nil || parsed.Path != "/sa_lathe_shop_phase9_test" {
		t.Fatal("dedicated fresh UC6 database required")
	}
	password := os.Getenv("TEST_ADMIN_PASSWORD")
	if len(password) < 12 || os.Getenv("TEST_S3_ENDPOINT") == "" {
		t.Fatal("private test environment required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	db, err := Open(ctx, dsn, 10)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var count int
	if err = db.pool.QueryRow(ctx, `SELECT count(*) FROM account`).Scan(&count); err != nil || count != 0 {
		t.Fatal("fresh database required; existing data never modified")
	}
	check := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	hasher := auth.NewArgonHasher()
	authentication, err := auth.NewService(db, hasher, time.Hour)
	check(err)
	admin, err := authentication.BootstrapAdmin(ctx, "uc6_admin", "Admin", password)
	check(err)
	worker, err := authentication.CreateWorker(ctx, admin, auth.WorkerInput{Username: "uc6_worker", Name: "Worker", EmployeeNo: "UC6-W", Password: password, Email: "worker@example.com", Phone: "+66812345678"})
	check(err)
	customers := auth.NewCustomerService(db, hasher)
	customer, err := customers.Register(ctx, auth.Registration{Username: "uc6_customer", Name: "Buyer", Password: password, CustomerType: "company", CompanyName: "UC6 Company"})
	check(err)
	profiles, err := customers.Profiles(ctx, customer)
	check(err)
	requests := req.NewService(db)
	quotes := q.NewService(db)
	decisions := q.NewDecisionService(db)
	payments := payment.NewService(db)
	store := objectstore.New(objectstore.Config{Endpoint: os.Getenv("TEST_S3_ENDPOINT"), Region: "us-east-1", Bucket: "sa-lathe-shop-private", AccessKey: os.Getenv("S3_ACCESS_KEY"), SecretKey: os.Getenv("S3_SECRET_KEY"), PathStyle: true})
	files := file.NewService(db, store, requests)
	price := func(s string) *q.Amount { v, e := q.ParseAmount(s); check(e); return &v }
	qty := func(s string) *m.Decimal { v, e := m.ParseDecimal(s); check(e); return &v }
	// Create valid jobs through UC1-UC4, including actual private S3 evidence.
	newJob := func() string {
		t.Helper()
		principal := req.Principal{AccountID: customer.ID, Role: customer.Role, Active: true}
		r, e := requests.Submit(ctx, principal, req.Input{CustomerID: profiles[0].ID, Description: "UC6 mould", Quantity: 1})
		check(e)
		draft, e := quotes.Create(ctx, worker.Account, r.ID, q.DraftInput{MaterialCost: price("1"), LaborCost: price("1"), FinalPrice: price("1000")})
		check(e)
		sent, e := quotes.Send(ctx, worker.Account, draft.ID, draft.LockVersion)
		check(e)
		_, e = decisions.Decide(ctx, customer, sent.ID, q.DecisionInput{Outcome: "accept", ExpectedLockVersion: sent.LockVersion})
		check(e)
		data := []byte("Company PO")
		link, e := files.Start(ctx, principal, r.ID, file.Input{Name: "po.txt", ContentType: "text/plain", Size: int64(len(data))})
		check(e)
		upload, e := http.NewRequestWithContext(ctx, "PUT", link.URL, bytes.NewReader(data))
		check(e)
		for k, v := range link.Headers {
			upload.Header.Set(k, v)
		}
		response, e := http.DefaultClient.Do(upload)
		check(e)
		response.Body.Close()
		if response.StatusCode != 200 {
			t.Fatal("PO upload failed")
		}
		_, e = files.Complete(ctx, principal, link.AttachmentID)
		check(e)
		evidence, e := payments.Submit(ctx, customer, sent.ID, payment.Submission{IdempotencyKey: uuid.NewString(), AttachmentID: link.AttachmentID, Kind: "po"})
		check(e)
		_, e = payments.Review(ctx, admin, evidence.ID, payment.Review{Outcome: "approved", ExpectedVersion: 1})
		check(e)
		fulfilled, e := payments.Get(ctx, customer, sent.ID)
		check(e)
		if fulfilled.Job == nil {
			t.Fatal("job not opened")
		}
		return fulfilled.Job.ID
	}
	job1, job2 := newJob(), newJob()
	inventory := m.NewService(db)
	service := p.NewService(db)
	unit, err := inventory.CreateUnit(ctx, admin, m.UnitInput{Name: "kg"})
	check(err)
	steel, err := inventory.Create(ctx, admin, m.MaterialInput{Name: "UC6 S45C", UnitID: unit.ID})
	check(err)
	other, err := inventory.Create(ctx, admin, m.MaterialInput{Name: "UC6 Steel B", UnitID: unit.ID})
	check(err)
	_, err = inventory.Stock(ctx, admin, steel.ID, m.StockInput{IdempotencyKey: uuid.NewString(), ExpectedVersion: steel.Version, Mode: "addition", Quantity: qty("10"), UnitCost: qty("20"), Reason: "Opening"})
	check(err)
	supplier, err := service.CreateSupplier(ctx, admin, p.SupplierInput{Name: "UC6 Supplier", Email: "supplier@example.com", Phone: "+66812345678"})
	check(err)
	active := true
	input := p.OrderInput{IdempotencyKey: uuid.NewString(), SupplierID: supplier.ID, JobIDs: []string{job2, job1}, Lines: []p.LineInput{{MaterialID: steel.ID, Quantity: qty("25"), UnitCost: qty("0.335")}, {MaterialID: other.ID, Quantity: qty("3"), UnitCost: qty("0.335")}}}
	// Two same-key concurrent placements produce exactly one immutable commitment.
	results := make(chan p.Order, 2)
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() { defer wg.Done(); v, e := service.Place(ctx, admin, input); results <- v; errs <- e }()
	}
	wg.Wait()
	close(results)
	close(errs)
	for e := range errs {
		check(e)
	}
	var order p.Order
	for v := range results {
		if order.ID != "" && order.ID != v.ID {
			t.Fatal("retry duplicated order")
		}
		order = v
	}
	if len(order.JobIDs) != 2 || len(order.Lines) != 2 || order.Total.String() != "9.39" || len(order.Events) != 1 {
		t.Fatal("incorrect consolidated rounded order", order.Total.String())
	}
	stockUnchanged := func() {
		t.Helper()
		var hand, reserved, cost string
		check(db.pool.QueryRow(ctx, `SELECT qty_on_hand::text,qty_reserved::text,avg_cost::text FROM material WHERE id=$1`, steel.ID).Scan(&hand, &reserved, &cost))
		if hand != "10.000000" || reserved != "0.000000" || cost != "20.000000" {
			t.Fatal("procurement changed stock", hand, reserved, cost)
		}
	}
	stockUnchanged()
	changed := input
	changed.Lines = append([]p.LineInput{}, input.Lines...)
	changed.Lines[0].Quantity = qty("26")
	if _, err = service.Place(ctx, admin, changed); !errors.Is(err, p.ErrConflict) {
		t.Fatal("different payload reused key", err)
	}
	// Snapshot master changes must not rewrite placed-order names or contacts.
	supplier, err = service.UpdateSupplier(ctx, admin, supplier.ID, p.SupplierUpdate{SupplierInput: p.SupplierInput{Name: "Renamed supplier"}, Active: &active, ExpectedVersion: 1})
	check(err)
	_, err = inventory.Update(ctx, admin, steel.ID, m.MaterialUpdate{Name: func() *string { s := "Renamed steel"; return &s }(), ExpectedVersion: 2})
	check(err)
	current, err := service.Get(ctx, admin, order.ID)
	check(err)
	if current.Supplier.Name != "UC6 Supplier" || current.Supplier.Email != "supplier@example.com" {
		t.Fatal("supplier snapshot changed")
	}
	for _, line := range current.Lines {
		if line.MaterialID == steel.ID && line.MaterialName != "UC6 S45C" {
			t.Fatal("material snapshot changed")
		}
	}
	// Plans remain flexible even with a placed order exceeding all requirements.
	plan, err := inventory.Save(ctx, worker.Account, job1, m.PlanInput{IdempotencyKey: uuid.NewString(), ExpectedVersion: 0, Lines: []m.Requirement{{MaterialID: steel.ID, Quantity: qty("1")}}})
	check(err)
	_, err = inventory.Save(ctx, worker.Account, job1, m.PlanInput{IdempotencyKey: uuid.NewString(), ExpectedVersion: plan.Version, Lines: []m.Requirement{}})
	check(err)
	stockUnchanged()
	action := p.ActionInput{IdempotencyKey: uuid.NewString(), ExpectedVersion: 1, Action: "accept"}
	accepted, err := service.Act(ctx, admin, order.ID, action)
	check(err)
	if accepted.Status != "ordered" || accepted.SupplierResponse != "accepted" || accepted.Version != 2 {
		t.Fatal("acceptance state")
	}
	replay, err := service.Place(ctx, admin, input)
	check(err)
	if replay.Version != 1 || replay.SupplierResponse != "pending" {
		t.Fatal("placement replay is not original snapshot")
	}
	_, err = service.Act(ctx, admin, order.ID, p.ActionInput{IdempotencyKey: uuid.NewString(), ExpectedVersion: 2, Action: "reject", Reason: "Late rejection"})
	if !errors.Is(err, p.ErrConflict) {
		t.Fatal("supplier response changed twice", err)
	}
	cancelled, err := service.Act(ctx, admin, order.ID, p.ActionInput{IdempotencyKey: uuid.NewString(), ExpectedVersion: 2, Action: "cancel", Reason: "Reorder new specification"})
	check(err)
	if cancelled.Status != "cancelled" || len(cancelled.Events) != 3 {
		t.Fatal("missing cancel audit")
	}
	original, err := service.Act(ctx, admin, order.ID, action)
	check(err)
	if original.Version != 2 {
		t.Fatal("action retry not original result")
	}
	stockUnchanged()
	all, err := service.Orders(ctx, admin, job2, 100, 0)
	check(err)
	if len(all) != 1 || all[0].ID != order.ID {
		t.Fatal("many-job order read missing")
	}
	// New keys intentionally allow excess/general replenishment. No shortage cap.
	input.IdempotencyKey = uuid.NewString()
	input.JobIDs = nil
	general, err := service.Place(ctx, admin, input)
	check(err)
	rejected, err := service.Act(ctx, admin, general.ID, p.ActionInput{IdempotencyKey: uuid.NewString(), ExpectedVersion: 1, Action: "reject", Reason: "Supplier unavailable"})
	check(err)
	if rejected.Status != "cancelled" || rejected.SupplierResponse != "rejected" {
		t.Fatal("rejection did not cancel")
	}
	// Opposite responses racing at the same version: one winner, one conflict.
	input.IdempotencyKey = uuid.NewString()
	racing, err := service.Place(ctx, admin, input)
	check(err)
	raceErrs := make(chan error, 2)
	for _, action := range []string{"accept", "reject"} {
		wg.Add(1)
		go func(action string) {
			defer wg.Done()
			_, e := service.Act(ctx, admin, racing.ID, p.ActionInput{IdempotencyKey: uuid.NewString(), ExpectedVersion: 1, Action: action, Reason: "Race response"})
			raceErrs <- e
		}(action)
	}
	wg.Wait()
	close(raceErrs)
	wins, conflicts := 0, 0
	for e := range raceErrs {
		if e == nil {
			wins++
		} else if errors.Is(e, p.ErrConflict) {
			conflicts++
		} else {
			t.Fatal(e)
		}
	}
	if wins != 1 || conflicts != 1 {
		t.Fatal("opposite responses both applied")
	}
	raceState, err := service.Get(ctx, admin, racing.ID)
	check(err)
	if raceState.Version != 2 || len(raceState.Events) != 2 {
		t.Fatal("race produced extra audit")
	}
	// Failed placement rolls back previously inserted links/items and does not consume its retry key.
	invalid := input
	invalid.IdempotencyKey = uuid.NewString()
	invalid.JobIDs = []string{uuid.NewString()}
	var before int
	check(db.pool.QueryRow(ctx, `SELECT count(*) FROM material_order_event`).Scan(&before))
	if _, err = service.Place(ctx, admin, invalid); err == nil {
		t.Fatal("nonexistent job allowed")
	}
	invalid.JobIDs = nil
	_, err = service.Place(ctx, admin, invalid)
	check(err)
	check(db.pool.QueryRow(ctx, `SELECT count(*) FROM material_order_event`).Scan(&count))
	if count != before+1 {
		t.Fatal("failed placement left audit")
	}
	// Inactive supplier/material block new orders, but exact retries still work.
	active = false
	supplier, err = service.UpdateSupplier(ctx, admin, supplier.ID, p.SupplierUpdate{SupplierInput: supplier.SupplierInput, Active: &active, ExpectedVersion: supplier.Version})
	check(err)
	input.IdempotencyKey = uuid.NewString()
	if _, err = service.Place(ctx, admin, input); !errors.Is(err, p.ErrConflict) {
		t.Fatal("inactive supplier accepted", err)
	}
	_, err = service.Get(ctx, admin, order.ID)
	check(err)
	active = true
	supplier, err = service.UpdateSupplier(ctx, admin, supplier.ID, p.SupplierUpdate{SupplierInput: supplier.SupplierInput, Active: &active, ExpectedVersion: supplier.Version})
	check(err)
	_, err = inventory.Update(ctx, admin, other.ID, m.MaterialUpdate{Active: func() *bool { b := false; return &b }(), ExpectedVersion: 1})
	check(err)
	if _, err = service.Place(ctx, admin, input); !errors.Is(err, p.ErrConflict) {
		t.Fatal("inactive material accepted", err)
	}
	// SQL integrity: no edits, append-after-place, forged totals, audit mutation or data-losing rollback.
	for _, sql := range []string{
		`UPDATE material_order SET total_amount=0 WHERE id=$1`,
		`UPDATE material_order_item SET unit_cost=0 WHERE order_id=$1`,
		`DELETE FROM material_order_job WHERE order_id=$1`,
		`UPDATE material_order_event SET reason='rewrite' WHERE order_id=$1`,
		`INSERT INTO material_order_job(order_id,job_id) VALUES($1,'` + job1 + `')`,
	} {
		if _, err = db.pool.Exec(ctx, sql, order.ID); err == nil {
			t.Fatal("immutable order/history write accepted", sql)
		}
	}
	// A forged header total must fail at COMMIT, even with an otherwise valid event.
	forgedID := uuid.NewString()
	forgedTx, err := db.pool.Begin(ctx)
	check(err)
	_, err = forgedTx.Exec(ctx, `INSERT INTO material_order_item(order_id,material_id,material_name,unit_id,unit_name,qty_ordered,unit_cost) VALUES($1,$2,'snapshot',$3,'kg',1,1)`, forgedID, steel.ID, unit.ID)
	check(err)
	_, err = forgedTx.Exec(ctx, `INSERT INTO material_order(id,order_no,supplier_id,supplier_snapshot,total_amount,created_by) SELECT $1::uuid,$1::text,supplier_id,supplier_snapshot,0,created_by FROM material_order WHERE id=$2`, forgedID, order.ID)
	check(err)
	_, err = forgedTx.Exec(ctx, `INSERT INTO material_order_event(id,order_id,actor_id,action,version,command,result) VALUES($1,$2,$3,'place',1,'{}','{}')`, uuid.NewString(), forgedID, admin.ID)
	check(err)
	if err = forgedTx.Commit(ctx); err == nil {
		t.Fatal("forged sum committed")
	}
	check(db.pool.QueryRow(ctx, `SELECT count(*) FROM material_order_item WHERE order_id=$1`, forgedID).Scan(&count))
	if count != 0 {
		t.Fatal("failed commit left orphan lines")
	}
	// Force audit insertion failure after UPDATE; transaction must restore header state.
	_, err = db.pool.Exec(ctx, `CREATE FUNCTION uc6_test_fail_event() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.reason='force audit failure' THEN RAISE EXCEPTION 'test audit failure'; END IF; RETURN NEW; END $$; CREATE TRIGGER uc6_test_fail_event BEFORE INSERT ON material_order_event FOR EACH ROW EXECUTE FUNCTION uc6_test_fail_event()`)
	check(err)
	if raceState.Status == "ordered" {
		_, err = service.Act(ctx, admin, racing.ID, p.ActionInput{IdempotencyKey: uuid.NewString(), ExpectedVersion: 2, Action: "cancel", Reason: "force audit failure"})
	} else {
		// Create an eligible order using only the active material.
		eligible := input
		eligible.IdempotencyKey = uuid.NewString()
		eligible.Lines = []p.LineInput{{MaterialID: steel.ID, Quantity: qty("1"), UnitCost: qty("1")}}
		racing, err = service.Place(ctx, admin, eligible)
		check(err)
		raceState = racing
		_, err = service.Act(ctx, admin, racing.ID, p.ActionInput{IdempotencyKey: uuid.NewString(), ExpectedVersion: 1, Action: "cancel", Reason: "force audit failure"})
	}
	if err == nil {
		t.Fatal("injected failure ignored")
	}
	afterFailure, e := service.Get(ctx, admin, racing.ID)
	check(e)
	if afterFailure.Version != raceState.Version || afterFailure.Status != raceState.Status {
		t.Fatal("audit failure left partial update")
	}
	_, err = db.pool.Exec(ctx, `DROP TRIGGER uc6_test_fail_event ON material_order_event; DROP FUNCTION uc6_test_fail_event()`)
	check(err)
	down, err := os.ReadFile("../../../migrations/000009_procurement.down.sql")
	check(err)
	tx, err := db.pool.Begin(ctx)
	check(err)
	_, err = tx.Exec(ctx, string(down))
	tx.Rollback(ctx)
	if err == nil {
		t.Fatal("populated procurement rollback allowed")
	}
	single, err := Open(ctx, dsn, 1)
	check(err)
	readCtx, stop := context.WithTimeout(ctx, 2*time.Second)
	_, err = p.NewService(single).Orders(readCtx, admin, "", 100, 0)
	stop()
	single.Close()
	check(err)
	// Real sessions: all procurement operations including reads are Admin-only.
	handler := auth.NewHandler(authentication)
	router := chi.NewRouter()
	p.NewHandler(service).Routes(router, handler.Authenticate)
	adminLogin, err := authentication.Login(ctx, admin.Username, password)
	check(err)
	workerLogin, err := authentication.Login(ctx, worker.Username, password)
	check(err)
	customerLogin, err := authentication.Login(ctx, customer.Username, password)
	check(err)
	call := func(method, path, token string, body any, want int) {
		t.Helper()
		data, e := json.Marshal(body)
		check(e)
		r := httptest.NewRequest(method, path, bytes.NewReader(data))
		r.Header.Set("Content-Type", "application/json")
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		if w.Code != want {
			t.Fatalf("%s %s: %d want %d: %s", method, path, w.Code, want, w.Body.String())
		}
	}
	call("GET", "/material-orders", "", nil, 401)
	for _, token := range []string{workerLogin.AccessToken, customerLogin.AccessToken} {
		for _, route := range []struct{ method, path string }{{"GET", "/suppliers"}, {"POST", "/suppliers"}, {"PUT", "/suppliers/" + supplier.ID}, {"GET", "/material-orders"}, {"POST", "/material-orders"}, {"GET", "/material-orders/" + order.ID}, {"POST", "/material-orders/" + order.ID + "/actions"}} {
			call(route.method, route.path, token, nil, 403)
		}
	}
	call("GET", "/suppliers?search=Renamed", adminLogin.AccessToken, nil, 200)
	call("PUT", "/suppliers/"+supplier.ID, adminLogin.AccessToken, p.SupplierUpdate{SupplierInput: supplier.SupplierInput, Active: &active, ExpectedVersion: 1}, 409)
	call("POST", "/suppliers", adminLogin.AccessToken, p.SupplierInput{Name: "Bad contact", Email: "invalid"}, 400)
	call("GET", "/material-orders?job_id="+job1, adminLogin.AccessToken, nil, 200)
	call("GET", "/material-orders/"+order.ID, adminLogin.AccessToken, nil, 200)
	call("GET", "/material-orders?limit=0", adminLogin.AccessToken, nil, 400)
	call("POST", "/material-orders", adminLogin.AccessToken, map[string]any{"unknown_field": true}, 400)
	call("POST", "/material-orders/"+order.ID+"/actions", adminLogin.AccessToken, p.ActionInput{IdempotencyKey: uuid.NewString(), ExpectedVersion: 3, Action: "cancel", Reason: "Already cancelled"}, 409)
	stockUnchanged()
	_, err = db.pool.Exec(ctx, `UPDATE account SET is_active=false WHERE id=$1`, admin.ID)
	check(err)
	if _, err = service.Get(ctx, admin, order.ID); !errors.Is(err, req.ErrForbidden) {
		t.Fatal("stale disabled admin read procurement", err)
	}
	call("GET", "/material-orders", adminLogin.AccessToken, nil, 401)
}
