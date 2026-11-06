package postgres

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/auth"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/file"
	m "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/material"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/platform/objectstore"
	p "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/procurement"
	r "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/receiving"
	req "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/request"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestReceivingIntegration(t *testing.T) {
	dsn := os.Getenv("PHASE10_DATABASE_URL")
	if dsn == "" {
		t.Skip("set dedicated PHASE10_DATABASE_URL")
	}
	u, e := url.Parse(dsn)
	if e != nil || u.Path != "/sa_lathe_shop_phase10_test" {
		t.Fatal("dedicated fresh receiving DB required")
	}
	password := os.Getenv("TEST_ADMIN_PASSWORD")
	if len(password) < 12 || os.Getenv("TEST_S3_ENDPOINT") == "" {
		t.Fatal("private test environment required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	check := func(e error) {
		t.Helper()
		if e != nil {
			t.Fatal(e)
		}
	}
	db, e := Open(ctx, dsn, 10)
	check(e)
	defer db.Close()
	var count int
	check(db.pool.QueryRow(ctx, `SELECT count(*) FROM account`).Scan(&count))
	if count != 0 {
		t.Fatal("fresh DB required")
	}
	hasher := auth.NewArgonHasher()
	authentication, e := auth.NewService(db, hasher, time.Hour)
	check(e)
	admin, e := authentication.BootstrapAdmin(ctx, "uc7_admin", "Admin", password)
	check(e)
	worker, e := authentication.CreateWorker(ctx, admin, auth.WorkerInput{Username: "uc7_worker", Name: "Worker", Password: password, EmployeeNo: "UC7-W", Email: "worker@example.com", Phone: "+66812345678"})
	check(e)
	customer, e := auth.NewCustomerService(db, hasher).Register(ctx, auth.Registration{Username: "uc7_customer", Name: "Customer", Password: password, CustomerType: "individual"})
	check(e)
	inventory := m.NewService(db)
	procurement := p.NewService(db)
	service := r.NewService(db)
	qty := func(s string) *m.Decimal { v, e := m.ParseDecimal(s); check(e); return &v }
	unit, e := inventory.CreateUnit(ctx, admin, m.UnitInput{Name: "kg"})
	check(e)
	steel, e := inventory.Create(ctx, admin, m.MaterialInput{Name: "UC7 steel", UnitID: unit.ID})
	check(e)
	_, e = inventory.Stock(ctx, admin, steel.ID, m.StockInput{IdempotencyKey: uuid.NewString(), ExpectedVersion: 1, Mode: "addition", Quantity: qty("10"), UnitCost: qty("10"), Reason: "Opening"})
	check(e)
	supplier, e := procurement.CreateSupplier(ctx, admin, p.SupplierInput{Name: "UC7 supplier"})
	check(e)
	newOrder := func(quantity, cost string) p.Order {
		t.Helper()
		v, e := procurement.Place(ctx, admin, p.OrderInput{IdempotencyKey: uuid.NewString(), SupplierID: supplier.ID, Lines: []p.LineInput{{MaterialID: steel.ID, Quantity: qty(quantity), UnitCost: qty(cost)}}})
		check(e)
		v, e = procurement.Act(ctx, admin, v.ID, p.ActionInput{IdempotencyKey: uuid.NewString(), ExpectedVersion: 1, Action: "accept"})
		check(e)
		return v
	}
	cmd := func(version int64) r.Command {
		return r.Command{IdempotencyKey: uuid.NewString(), ExpectedVersion: version}
	}
	receipt := func(version int64, good, bad, claim string) r.ReceiptInput {
		return r.ReceiptInput{Command: cmd(version), Lines: []r.ReceiptLine{{MaterialID: steel.ID, Good: qty(good), Damaged: qty(bad), ReplacementClaimID: claim}}}
	}
	order := newOrder("10", "30")
	days := 30
	v, e := service.Credit(ctx, admin, order.ID, r.CreditInput{Command: cmd(order.Version), Days: &days})
	check(e)
	if _, e = service.Receive(ctx, worker.Account, order.ID, receipt(v.Version, "11", "0", "")); !errors.Is(e, req.ErrInvalidInput) {
		t.Fatal("excess shipment accepted", e)
	}
	if _, e = service.Settle(ctx, admin, order.ID, r.SettlementInput{Command: cmd(v.Version), Method: "cash", PaidAt: time.Now()}); !errors.Is(e, r.ErrConflict) {
		t.Fatal("paid before full receipt", e)
	}
	in := receipt(v.Version, "4", "2", "")
	// Same-key concurrent receipts insert only one shipment, ledger and defect.
	values := make(chan r.State, 2)
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s, e := service.Receive(ctx, worker.Account, order.ID, in)
			values <- s
			errs <- e
		}()
	}
	wg.Wait()
	close(values)
	close(errs)
	for e := range errs {
		check(e)
	}
	for state := range values {
		v = state
	}
	if v.Finance != nil || v.Status != "partially_received" || len(v.Receipts) != 1 || len(v.Claims) != 1 || v.Lines[0].Good.String() != "4.000000" {
		t.Fatal("receipt replay/worker redaction failed")
	}
	claim := v.Claims[0]
	if claim.Status != "pending_claim" || claim.Quantity.String() != "2.000000" {
		t.Fatal("defect not logged")
	}
	if _, e = procurement.Act(ctx, admin, order.ID, p.ActionInput{IdempotencyKey: uuid.NewString(), ExpectedVersion: v.Version, Action: "cancel", Reason: "Already received"}); !errors.Is(e, p.ErrConflict) {
		t.Fatal("cancel after receipt allowed", e)
	}
	if _, e = service.Receive(ctx, worker.Account, order.ID, receipt(v.Version, "2", "0", claim.ID)); e == nil {
		t.Fatal("replacement before claim accepted")
	}
	v, e = service.Claim(ctx, worker.Account, order.ID, claim.ID, r.ClaimInput{Command: cmd(v.Version), Note: "Supplier contacted for replacement"})
	check(e)
	v, e = service.Receive(ctx, worker.Account, order.ID, receipt(v.Version, "0", "1", claim.ID))
	check(e)
	if len(v.Claims) != 1 || v.Claims[0].Replaced.String() != "0.000000" {
		t.Fatal("bad replacement duplicated defect liability")
	}
	v, e = service.Receive(ctx, admin, order.ID, receipt(v.Version, "4", "0", ""))
	check(e)
	// docs/02-frontend.md UC7: a fully accounted-for order (everything arrived) may still include damaged units.
	if v.Status != "received" || v.FullReceivedAt == nil {
		t.Fatal("fully accounted-for order not received", v.Status)
	}
	v, e = service.Receive(ctx, worker.Account, order.ID, receipt(v.Version, "2", "0", claim.ID))
	check(e)
	if v.Status != "received" || v.FullReceivedAt == nil || v.Claims[0].Status != "resolved" {
		t.Fatal("replacement did not complete order")
	}
	completedAt := *v.FullReceivedAt
	state, e := service.Get(ctx, admin, order.ID)
	check(e)
	if state.Finance.Total.String() != "300.00" || state.Finance.DueAt == nil || !state.Finance.DueAt.Equal(completedAt.Add(30*24*time.Hour)) {
		t.Fatal("credit due/amount mismatch")
	}
	var onHand, avg string
	check(db.pool.QueryRow(ctx, `SELECT qty_on_hand::text,avg_cost::text FROM material WHERE id=$1`, steel.ID).Scan(&onHand, &avg))
	if onHand != "20.000000" || avg != "20.000000" {
		t.Fatal("good-only average mismatch", onHand, avg)
	}
	replay, e := service.Receive(ctx, worker.Account, order.ID, in)
	check(e)
	if replay.Status != "partially_received" || len(replay.Receipts) != 1 {
		t.Fatal("receipt retry changed original result")
	}
	// A different payload using that key cannot change the original shipment.
	changed := in
	changed.Note = "changed"
	if _, e = service.Receive(ctx, worker.Account, order.ID, changed); !errors.Is(e, r.ErrConflict) {
		t.Fatal("retry payload overwrite", e)
	}
	store := objectstore.New(objectstore.Config{Endpoint: os.Getenv("TEST_S3_ENDPOINT"), Region: "us-east-1", Bucket: "sa-lathe-shop-private", AccessKey: os.Getenv("S3_ACCESS_KEY"), SecretKey: os.Getenv("S3_SECRET_KEY"), PathStyle: true})
	evidence := r.NewEvidenceService(db, store)
	upload := func(orderID, name, mime string, data []byte) (string, error) {
		t.Helper()
		link, e := evidence.Start(ctx, admin, orderID, file.Input{Name: name, ContentType: mime, Size: int64(len(data))})
		if e != nil {
			return "", e
		}
		request, e := http.NewRequestWithContext(ctx, "PUT", link.URL, bytes.NewReader(data))
		if e != nil {
			return "", e
		}
		for k, v := range link.Headers {
			request.Header.Set(k, v)
		}
		response, e := http.DefaultClient.Do(request)
		if e != nil {
			return "", e
		}
		response.Body.Close()
		if response.StatusCode != 200 {
			t.Fatal("S3 upload failed")
		}
		_, e = evidence.Complete(ctx, admin, link.AttachmentID)
		return link.AttachmentID, e
	}
	failedID, e := upload(order.ID, "fake.png", "image/png", []byte("not a png"))
	if !errors.Is(e, req.ErrInvalidInput) {
		t.Fatal("invalid evidence content accepted", e)
	}
	if _, e = evidence.Download(ctx, admin, failedID); !errors.Is(e, req.ErrNotFound) {
		t.Fatal("failed evidence downloadable", e)
	}
	proof, e := upload(order.ID, "slip.txt", "text/plain", []byte("private bank slip"))
	check(e)
	link, e := evidence.Download(ctx, admin, proof)
	check(e)
	response, e := http.Get(link.URL)
	check(e)
	response.Body.Close()
	if response.StatusCode != 200 {
		t.Fatal("private signed evidence download failed")
	}
	for _, a := range []auth.Account{worker.Account, customer} {
		if _, e = evidence.Download(ctx, a, proof); !errors.Is(e, req.ErrForbidden) {
			t.Fatal("evidence privacy failed")
		}
	}
	payment := r.SettlementInput{Command: cmd(state.Version), Method: "bank", PaidAt: time.Now().UTC(), EvidenceID: proof}
	paid, e := service.Settle(ctx, admin, order.ID, payment)
	check(e)
	if paid.Finance.Settlement.Amount.String() != "300.00" || paid.Finance.PaymentStatus != "paid" || !paid.FullReceivedAt.Equal(completedAt) {
		t.Fatal("settlement/full receipt changed")
	}
	_, e = service.Settle(ctx, admin, order.ID, payment)
	check(e)
	if _, e = service.Settle(ctx, admin, order.ID, r.SettlementInput{Command: cmd(paid.Version), Method: "cash", PaidAt: time.Now()}); !errors.Is(e, r.ErrConflict) {
		t.Fatal("duplicate full settlement allowed", e)
	}
	// One full cash settlement wins a concurrent race. Cash photo is optional.
	cashOrder := newOrder("1", "5")
	zero := 0
	cashState, e := service.Credit(ctx, admin, cashOrder.ID, r.CreditInput{Command: cmd(cashOrder.Version), Days: &zero})
	check(e)
	cashState, e = service.Receive(ctx, worker.Account, cashOrder.ID, receipt(cashState.Version, "1", "0", ""))
	check(e)
	if _, e = service.Settle(ctx, admin, cashOrder.ID, r.SettlementInput{Command: cmd(cashState.Version), Method: "bank", PaidAt: time.Now(), EvidenceID: proof}); !errors.Is(e, req.ErrInvalidInput) {
		t.Fatal("borrowed order evidence allowed", e)
	}
	errorsPay := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, e := service.Settle(ctx, admin, cashOrder.ID, r.SettlementInput{Command: cmd(cashState.Version), Method: "cash", PaidAt: time.Now()})
			errorsPay <- e
		}()
	}
	wg.Wait()
	close(errorsPay)
	wins, conflicts := 0, 0
	for e := range errorsPay {
		if e == nil {
			wins++
		} else if errors.Is(e, r.ErrConflict) {
			conflicts++
		} else {
			t.Fatal(e)
		}
	}
	if wins != 1 || conflicts != 1 {
		t.Fatal("payment race failed")
	}
	// Receipt-versus-cancel uses the same order lock, with one winner.
	raceOrder := newOrder("1", "1")
	raceErrors := make(chan error, 2)
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, e := service.Receive(ctx, worker.Account, raceOrder.ID, receipt(raceOrder.Version, "1", "0", ""))
		raceErrors <- e
	}()
	go func() {
		defer wg.Done()
		_, e := procurement.Act(ctx, admin, raceOrder.ID, p.ActionInput{IdempotencyKey: uuid.NewString(), ExpectedVersion: raceOrder.Version, Action: "cancel", Reason: "Race"})
		raceErrors <- e
	}()
	wg.Wait()
	close(raceErrors)
	wins, conflicts = 0, 0
	for e := range raceErrors {
		if e == nil {
			wins++
		} else if errors.Is(e, r.ErrConflict) {
			conflicts++
		} else {
			t.Fatal(e)
		}
	}
	if wins != 1 || conflicts != 1 {
		t.Fatal("receipt/cancel race failed")
	}
	// Inject event failure AFTER stock/claim writes: every effect must roll back.
	failedOrder := newOrder("2", "5")
	var beforeStock, beforeCost string
	check(db.pool.QueryRow(ctx, `SELECT qty_on_hand::text,avg_cost::text FROM material WHERE id=$1`, steel.ID).Scan(&beforeStock, &beforeCost))
	_, e = db.pool.Exec(ctx, `CREATE FUNCTION uc7_fail_audit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.action='receive' AND NEW.command->'input'->>'note'='force audit failure' THEN RAISE EXCEPTION 'test audit failure'; END IF;RETURN NEW;END $$;CREATE TRIGGER uc7_fail_audit BEFORE INSERT ON material_order_event FOR EACH ROW EXECUTE FUNCTION uc7_fail_audit()`)
	check(e)
	failing := receipt(failedOrder.Version, "1", "1", "")
	failing.Note = "force audit failure"
	if _, e = service.Receive(ctx, worker.Account, failedOrder.ID, failing); e == nil {
		t.Fatal("forced failure ignored")
	}
	_, e = db.pool.Exec(ctx, `DROP TRIGGER uc7_fail_audit ON material_order_event;DROP FUNCTION uc7_fail_audit()`)
	check(e)
	var afterStock, afterCost string
	check(db.pool.QueryRow(ctx, `SELECT qty_on_hand::text,avg_cost::text FROM material WHERE id=$1`, steel.ID).Scan(&afterStock, &afterCost))
	failedState, e := service.Get(ctx, admin, failedOrder.ID)
	check(e)
	if beforeStock != afterStock || beforeCost != afterCost || len(failedState.Receipts) != 0 || len(failedState.Claims) != 0 || failedState.Version != failedOrder.Version {
		t.Fatal("receipt failure left partial stock/audit")
	}
	// Append-only data and populated rollback guard.
	for _, sql := range []string{`UPDATE material_receipt SET note='changed' WHERE order_id=$1`, `UPDATE material_receipt_line SET good=0 WHERE order_id=$1`, `DELETE FROM supplier_settlement WHERE order_id=$1`} {
		if _, e = db.pool.Exec(ctx, sql, order.ID); e == nil {
			t.Fatal("immutable financial/receipt history changed")
		}
	}
	down, e := os.ReadFile("../../../migrations/000010_receiving.down.sql")
	check(e)
	tx, e := db.pool.Begin(ctx)
	check(e)
	_, e = tx.Exec(ctx, string(down))
	tx.Rollback(ctx)
	if e == nil {
		t.Fatal("data-losing rollback accepted")
	}
	single, e := Open(ctx, dsn, 1)
	check(e)
	readCtx, stop := context.WithTimeout(ctx, 2*time.Second)
	_, e = r.NewService(single).Get(readCtx, worker.Account, order.ID)
	stop()
	single.Close()
	check(e)
	// HTTP redaction includes legacy inventory endpoints and financial route denials.
	authHandler := auth.NewHandler(authentication)
	router := chi.NewRouter()
	r.NewHandler(service, evidence).Routes(router, authHandler.Authenticate)
	m.NewHandler(inventory).Routes(router, authHandler.Authenticate)
	adminLogin, e := authentication.Login(ctx, admin.Username, password)
	check(e)
	workerLogin, e := authentication.Login(ctx, worker.Username, password)
	check(e)
	customerLogin, e := authentication.Login(ctx, customer.Username, password)
	check(e)
	call := func(method, path, token string, body any, want int) string {
		t.Helper()
		data, e := json.Marshal(body)
		check(e)
		request := httptest.NewRequest(method, path, bytes.NewReader(data))
		request.Header.Set("Content-Type", "application/json")
		if token != "" {
			request.Header.Set("Authorization", "Bearer "+token)
		}
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != want {
			t.Fatalf("%s %s =%d want %d %s", method, path, response.Code, want, response.Body.String())
		}
		return response.Body.String()
	}
	call("GET", "/receiving/orders", "", nil, 401)
	call("GET", "/receiving/orders", customerLogin.AccessToken, nil, 403)
	for _, path := range []string{"/receiving/orders/" + order.ID, "/materials", "/materials/" + steel.ID + "/movements"} {
		data := call("GET", path, workerLogin.AccessToken, nil, 200)
		for _, field := range []string{"finance", "avg_cost", "unit_cost", "before_cost", "after_cost", "evidence_id", "payment_due_at"} {
			if strings.Contains(data, `"`+field+`"`) {
				t.Fatal("worker HTTP financial leak", field)
			}
		}
	}
	call("GET", "/order-evidence/"+proof+"/download", workerLogin.AccessToken, nil, 403)
	call("GET", "/order-evidence/"+proof+"/download", customerLogin.AccessToken, nil, 403)
	call("POST", "/receiving/orders/"+order.ID+"/settlement", workerLogin.AccessToken, nil, 403)
	call("POST", "/receiving/orders/"+order.ID+"/credit-terms", workerLogin.AccessToken, nil, 403)
	call("POST", "/material-orders/"+order.ID+"/evidence/uploads", workerLogin.AccessToken, nil, 403)
	call("GET", "/receiving/orders/"+order.ID, adminLogin.AccessToken, nil, 200)
	call("POST", "/receiving/orders/"+failedOrder.ID+"/receipts", workerLogin.AccessToken, map[string]any{"unit_cost": "1"}, 400)
	_, e = db.pool.Exec(ctx, `UPDATE account SET is_active=false WHERE id=$1`, worker.ID)
	check(e)
	if _, e = service.Receive(ctx, worker.Account, failedOrder.ID, receipt(failedOrder.Version, "1", "0", "")); !errors.Is(e, req.ErrForbidden) {
		t.Fatal("disabled worker received", e)
	}
	call("GET", "/receiving/orders", workerLogin.AccessToken, nil, 401)
}
