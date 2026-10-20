package postgres

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/auth"
	f "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/delivery"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/file"
	m "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/material"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/notification"
	pay "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/payment"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/platform/objectstore"
	p "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/production"
	q "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/quotation"
	req "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/request"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestCustomerFulfillmentIntegration(t *testing.T) {
	dsn := os.Getenv("PHASE12_DATABASE_URL")
	if dsn == "" {
		t.Skip("set dedicated PHASE12_DATABASE_URL")
	}
	u, e := url.Parse(dsn)
	if e != nil || u.Path != "/sa_lathe_shop_phase12_test" {
		t.Fatal("fresh dedicated UC9 DB required")
	}
	password := os.Getenv("TEST_ADMIN_PASSWORD")
	if len(password) < 12 || os.Getenv("TEST_S3_ENDPOINT") == "" {
		t.Fatal("private test environment required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	check := func(e error) {
		t.Helper()
		if e != nil {
			t.Fatal(e)
		}
	}
	db, e := Open(ctx, dsn, 12)
	check(e)
	defer db.Close()
	var count int
	check(db.pool.QueryRow(ctx, `SELECT count(*) FROM account`).Scan(&count))
	if count != 0 {
		t.Fatal("fresh database only; existing data untouched")
	}
	hasher := auth.NewArgonHasher()
	authentication, e := auth.NewService(db, hasher, time.Hour)
	check(e)
	admin, e := authentication.BootstrapAdmin(ctx, "uc9_admin", "Admin", password)
	check(e)
	worker, e := authentication.CreateWorker(ctx, admin, auth.WorkerInput{Username: "uc9_worker", Name: "Worker", Password: password, EmployeeNo: "UC9-W", Email: "worker@example.com", Phone: "+66812345678"})
	check(e)
	customers := auth.NewCustomerService(db, hasher)
	customer, e := customers.Register(ctx, auth.Registration{Username: "uc9_customer", Name: "Buyer", Password: password, CustomerType: "company", CompanyName: "UC9 Company"})
	check(e)
	profiles, e := customers.Profiles(ctx, customer)
	check(e)
	stranger, e := customers.Register(ctx, auth.Registration{Username: "uc9_stranger", Name: "Other", Password: password, CustomerType: "individual"})
	check(e)
	requests := req.NewService(db)
	quotes := q.NewService(db)
	decisions := q.NewDecisionService(db)
	deposits := pay.NewService(db)
	inventory := m.NewService(db)
	production := p.NewService(db)
	service := f.NewService(db)
	store := objectstore.New(objectstore.Config{Endpoint: os.Getenv("TEST_S3_ENDPOINT"), Region: "us-east-1", Bucket: "sa-lathe-shop-private", AccessKey: os.Getenv("S3_ACCESS_KEY"), SecretKey: os.Getenv("S3_SECRET_KEY"), PathStyle: true})
	files := file.NewService(db, store, requests)
	evidence := f.NewEvidenceService(db, store)
	amount := func(s string) *q.Amount { v, e := q.ParseAmount(s); check(e); return &v }
	qty := func(s string) *m.Decimal { v, e := m.ParseDecimal(s); check(e); return &v }
	put := func(link file.Link, data []byte) {
		t.Helper()
		request, e := http.NewRequestWithContext(ctx, "PUT", link.URL, bytes.NewReader(data))
		check(e)
		for k, v := range link.Headers {
			request.Header.Set(k, v)
		}
		response, e := http.DefaultClient.Do(request)
		check(e)
		response.Body.Close()
		if response.StatusCode != 200 {
			t.Fatal("private S3 upload", response.StatusCode)
		}
	}
	unit, e := inventory.CreateUnit(ctx, admin, m.UnitInput{Name: "kg"})
	check(e)
	steel, e := inventory.Create(ctx, admin, m.MaterialInput{Name: "UC9 steel", UnitID: unit.ID})
	check(e)
	_, e = inventory.Stock(ctx, admin, steel.ID, m.StockInput{IdempotencyKey: uuid.NewString(), ExpectedVersion: 1, Mode: "addition", Quantity: qty("20"), UnitCost: qty("10"), Reason: "Opening"})
	check(e)
	principal := req.Principal{AccountID: customer.ID, Role: customer.Role, Active: true}
	newJob := func(price string) string {
		t.Helper()
		r, e := requests.Submit(ctx, principal, req.Input{CustomerID: profiles[0].ID, Description: "UC9 mould", Quantity: 1})
		check(e)
		draft, e := quotes.Create(ctx, worker.Account, r.ID, q.DraftInput{MaterialCost: amount("0"), LaborCost: amount("0"), FinalPrice: amount(price)})
		check(e)
		sent, e := quotes.Send(ctx, worker.Account, draft.ID, draft.LockVersion)
		check(e)
		_, e = decisions.Decide(ctx, customer, sent.ID, q.DecisionInput{Outcome: "accept", ExpectedLockVersion: sent.LockVersion})
		check(e)
		data := []byte("Company PO")
		link, e := files.Start(ctx, principal, r.ID, file.Input{Name: "po.txt", ContentType: "text/plain", Size: int64(len(data))})
		check(e)
		put(link, data)
		_, e = files.Complete(ctx, principal, link.AttachmentID)
		check(e)
		proof, e := deposits.Submit(ctx, customer, sent.ID, pay.Submission{IdempotencyKey: uuid.NewString(), AttachmentID: link.AttachmentID, Kind: "po"})
		check(e)
		_, e = deposits.Review(ctx, admin, proof.ID, pay.Review{Outcome: "approved", ExpectedVersion: 1})
		check(e)
		fulfilled, e := deposits.Get(ctx, customer, sent.ID)
		check(e)
		if fulfilled.Job == nil {
			t.Fatal("missing job")
		}
		id := fulfilled.Job.ID
		plan, e := inventory.Save(ctx, worker.Account, id, m.PlanInput{IdempotencyKey: uuid.NewString(), ExpectedVersion: 0, Lines: []m.Requirement{{MaterialID: steel.ID, Quantity: qty("1")}}})
		check(e)
		state, e := production.Start(ctx, worker.Account, id, p.StartInput{Command: p.Command{IdempotencyKey: uuid.NewString(), ExpectedVersion: 0}, ExpectedPlanVersion: &plan.Version})
		check(e)
		_, e = production.Complete(ctx, worker.Account, id, p.Command{IdempotencyKey: uuid.NewString(), ExpectedVersion: state.Version})
		check(e)
		return id
	}
	cmd := func(v int64) f.Command { return f.Command{IdempotencyKey: uuid.NewString(), ExpectedVersion: v} }
	prepare := func(job string) f.State {
		t.Helper()
		days := 7
		v, e := service.Terms(ctx, admin, job, f.TermsInput{Command: cmd(0), CreditDays: &days})
		check(e)
		v, e = service.Issue(ctx, worker.Account, job, cmd(v.Version))
		check(e)
		if v.Invoice == nil || v.Invoice.DueAt.Sub(v.Invoice.IssuedAt) != 7*24*time.Hour {
			t.Fatal("invoice terms")
		}
		return v
	}
	proof := func(job string) string {
		t.Helper()
		data := []byte("Bank proof")
		link, e := evidence.Start(ctx, customer, job, file.Input{Name: "bank.txt", ContentType: "text/plain", Size: int64(len(data))})
		check(e)
		put(link, data)
		v, e := evidence.Complete(ctx, customer, link.AttachmentID)
		check(e)
		if v.State != "ready" {
			t.Fatal("not ready")
		}
		return v.ID
	}
	now := func() *time.Time { v := time.Now().UTC().Add(-time.Second); return &v }
	job := newJob("1000")
	v := prepare(job)
	if _, e = service.Issue(ctx, worker.Account, job, cmd(v.Version)); e == nil {
		t.Fatal("duplicate invoice")
	}
	if _, e = service.Get(ctx, stranger, job); e == nil {
		t.Fatal("cross company read")
	}
	v, e = service.Ship(ctx, worker.Account, job, f.ShipInput{Command: cmd(v.Version), Method: "self", ReceiverName: "Buyer"})
	check(e)
	// Concurrent return/accept is serialized by request and job; one decision wins.
	// Main case returns before acceptance, then uses actual free stock without the old plan.
	returned := f.InspectInput{Command: cmd(v.Version), Outcome: "return", Reason: "Fit adjustment"}
	v, e = service.Inspect(ctx, customer, job, returned)
	check(e)
	if v.JobStatus != "in_progress" || v.Cycle != 2 || len(v.ReworkHistory) != 1 {
		t.Fatal("return cycle", v)
	}
	replay, e := service.Inspect(ctx, customer, job, returned)
	check(e)
	if replay.Version != v.Version {
		t.Fatal("return replay")
	}
	if _, e = service.Ship(ctx, worker.Account, job, f.ShipInput{Command: cmd(v.Version), Method: "self", ReceiverName: "Buyer"}); e == nil {
		t.Fatal("redelivery before rework completed")
	}
	ps, e := production.Get(ctx, worker.Account, job)
	check(e)
	if ps.Cycle != 2 {
		t.Fatal("production cycle hidden")
	}
	ps, e = production.Use(ctx, worker.Account, job, p.UsageInput{Command: p.Command{IdempotencyKey: uuid.NewString(), ExpectedVersion: ps.Version}, Lines: []p.UsageLine{{MaterialID: steel.ID, Quantity: qty("0.5")}}, Note: "Free rework"})
	check(e)
	ps, e = production.Time(ctx, worker.Account, job, p.TimeInput{Command: p.Command{IdempotencyKey: uuid.NewString(), ExpectedVersion: ps.Version}, Minutes: 15, Note: "Adjustment"})
	check(e)
	_, e = production.Complete(ctx, worker.Account, job, p.Command{IdempotencyKey: uuid.NewString(), ExpectedVersion: ps.Version})
	check(e)
	v, e = service.Get(ctx, customer, job)
	check(e)
	v, e = service.Ship(ctx, admin, job, f.ShipInput{Command: cmd(v.Version), Method: "third_party", ReceiverName: "Buyer"})
	check(e)
	v, e = service.Inspect(ctx, customer, job, f.InspectInput{Command: cmd(v.Version), Outcome: "accept"})
	check(e)
	if v.RequestStatus != "ongoing" {
		t.Fatal("acceptance closed unpaid request")
	}
	evidenceID := proof(job)
	if _, e = evidence.Download(ctx, worker.Account, evidenceID); e == nil {
		t.Fatal("worker proof leak")
	}
	if _, e = evidence.Download(ctx, stranger, evidenceID); e == nil {
		t.Fatal("stranger proof leak")
	}
	link, e := evidence.Download(ctx, customer, evidenceID)
	check(e)
	parsed, e := url.Parse(link.URL)
	check(e)
	parsed.RawQuery = ""
	response, e := http.Get(parsed.String())
	check(e)
	response.Body.Close()
	if response.StatusCode != 403 {
		t.Fatal("public proof")
	}
	input := f.PaymentInput{Command: cmd(v.Version), Amount: amount("400"), PaidAt: now(), EvidenceID: evidenceID}
	v, e = service.Pay(ctx, customer, job, input, false)
	check(e)
	if v.Invoice.Balance.String() != "1000.00" {
		t.Fatal("unapproved proof reduced balance")
	}
	pid := input.IdempotencyKey
	v, e = service.Review(ctx, admin, job, f.ReviewInput{Command: cmd(v.Version), PaymentID: pid, Outcome: "approved"})
	check(e)
	if v.Invoice.Balance.String() != "600.00" || v.Invoice.Status != "partial" {
		t.Fatal("installment balance")
	}
	// Same-key concurrent cash retries are one receipt/allocation; replay stays original.
	cash := f.PaymentInput{Command: cmd(v.Version), Amount: amount("600"), PaidAt: now()}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() { defer wg.Done(); _, e := service.Pay(ctx, admin, job, cash, true); results <- e }()
	}
	wg.Wait()
	close(results)
	for e := range results {
		check(e)
	}
	v, e = service.Get(ctx, customer, job)
	check(e)
	if v.RequestStatus != "completed" || v.Invoice.Status != "paid" || len(v.Payments) != 2 {
		t.Fatal("closure/payment replay", v)
	}
	if _, e = service.Inspect(ctx, customer, job, f.InspectInput{Command: cmd(v.Version), Outcome: "return", Reason: "Late warranty"}); e == nil {
		t.Fatal("post acceptance return")
	}
	if _, e = service.Pay(ctx, admin, job, f.PaymentInput{Command: cmd(v.Version), Amount: amount("0.01"), PaidAt: now()}, true); e == nil {
		t.Fatal("overpayment")
	}
	// Zero balance closes on acceptance without inventing a payment.
	zero := newJob("0")
	z := prepare(zero)
	z, e = service.Ship(ctx, worker.Account, zero, f.ShipInput{Command: cmd(z.Version), Method: "self", ReceiverName: "Buyer"})
	check(e)
	z, e = service.Inspect(ctx, customer, zero, f.InspectInput{Command: cmd(z.Version), Outcome: "accept"})
	check(e)
	if z.RequestStatus != "completed" || z.Invoice.Balance.String() != "0.00" || len(z.Payments) != 0 {
		t.Fatal("zero balance fabricated payment")
	}
	// Approval race cannot overpay, even with two pending proofs for the same balance.
	race := newJob("1000")
	r := prepare(race)
	proofIDs := []string{proof(race), proof(race)}
	paymentIDs := []string{}
	for _, id := range proofIDs {
		in := f.PaymentInput{Command: cmd(r.Version), Amount: amount("700"), PaidAt: now(), EvidenceID: id}
		r, e = service.Pay(ctx, customer, race, in, false)
		check(e)
		paymentIDs = append(paymentIDs, in.IdempotencyKey)
	}
	results = make(chan error, 2)
	for _, id := range paymentIDs {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			_, e := service.Review(ctx, admin, race, f.ReviewInput{Command: cmd(r.Version), PaymentID: id, Outcome: "approved"})
			results <- e
		}(id)
	}
	wg.Wait()
	close(results)
	wins := 0
	for e := range results {
		if e == nil {
			wins++
		}
	}
	if wins != 1 {
		t.Fatal("approval race", wins)
	}
	r, e = service.Get(ctx, admin, race)
	check(e)
	if r.Invoice.Balance.String() != "300.00" {
		t.Fatal("race overpaid")
	}
	// Forced event failure must roll back invoice/version and any related effects.
	fault := newJob("1000")
	days := 0
	fv, e := service.Terms(ctx, admin, fault, f.TermsInput{Command: cmd(0), CreditDays: &days})
	check(e)
	_, e = db.pool.Exec(ctx, `CREATE FUNCTION uc9_test_fail() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'forced audit failure'; END $$; CREATE TRIGGER uc9_test_fail BEFORE INSERT ON fulfillment_event FOR EACH ROW EXECUTE FUNCTION uc9_test_fail()`)
	check(e)
	_, e = service.Issue(ctx, admin, fault, cmd(fv.Version))
	if e == nil {
		t.Fatal("audit failure ignored")
	}
	_, e = db.pool.Exec(ctx, `DROP TRIGGER uc9_test_fail ON fulfillment_event; DROP FUNCTION uc9_test_fail()`)
	check(e)
	fv, e = service.Get(ctx, admin, fault)
	check(e)
	if fv.Version != 1 || fv.Invoice != nil {
		t.Fatal("invoice rollback")
	}
	for _, sql := range []string{`UPDATE invoice SET amount=amount+1`, `UPDATE payment_allocation SET amount=amount+1`, `DELETE FROM fulfillment_event`, `UPDATE job SET balance=balance+1 WHERE id='` + job + `'`, `UPDATE job_request SET status='completed' WHERE id=(SELECT request_id FROM job WHERE id='` + race + `')`} {
		if _, e = db.pool.Exec(ctx, sql); e == nil {
			t.Fatal("audit/closure mutation allowed", sql)
		}
	}
	// Scoped dispatch: UC9 must not send new LINE or leak Admin notices to Workers.
	dispatcher := notification.NewDispatcher(db, notification.NewLINE(false, "", ""), slog.New(slog.NewTextHandler(io.Discard, nil)))
	for range 40 {
		e = dispatcher.Once(ctx)
		if errors.Is(e, notification.ErrEmpty) {
			break
		}
		check(e)
	}
	var leaks int
	check(db.pool.QueryRow(ctx, `SELECT count(*) FROM notification n JOIN outbox o ON o.id=n.event_id WHERE o.kind<>'request' AND n.account_id=$1`, worker.ID).Scan(&leaks))
	if leaks != 0 {
		t.Fatal("worker notification privacy")
	}
	check(db.pool.QueryRow(ctx, `SELECT count(*) FROM outbox WHERE kind<>'request' AND channel<>'in_system'`).Scan(&leaks))
	if leaks != 0 {
		t.Fatal("unexpected LINE")
	}
	// Real authenticated HTTP guards, strict JSON and no supplier/internal cost leakage.
	authHandler := auth.NewHandler(authentication)
	router := chi.NewRouter()
	f.NewHandler(service, evidence).Routes(router, authHandler.Authenticate)
	login, e := authentication.Login(ctx, "uc9_worker", password)
	check(e)
	request := httptest.NewRequest("GET", "/fulfillment/jobs/"+job, nil)
	request.Header.Set("Authorization", "Bearer "+login.AccessToken)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, request)
	if w.Code != 200 || strings.Contains(w.Body.String(), "evidence_id") || strings.Contains(w.Body.String(), "unit_cost") {
		t.Fatal("worker HTTP privacy", w.Code, w.Body.String())
	}
	request = httptest.NewRequest("POST", "/fulfillment/jobs/"+job+"/invoice", strings.NewReader(`{"idempotency_key":"`+uuid.NewString()+`","expected_version":0,"amount":"0"}`))
	request.Header.Set("Authorization", "Bearer "+login.AccessToken)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, request)
	if w.Code != 400 {
		t.Fatal("unknown JSON field", w.Code)
	}
	// Single-connection reads do not nest pool acquisitions.
	single, e := Open(ctx, dsn, 1)
	check(e)
	_, e = single.FulfillmentState(ctx, customer.ID, job)
	single.Close()
	check(e)
	rollback, e := os.ReadFile("../../../migrations/000012_fulfillment.down.sql")
	check(e)
	tx, e := db.pool.Begin(ctx)
	check(e)
	_, e = tx.Exec(ctx, string(rollback))
	if e == nil {
		t.Fatal("populated rollback allowed")
	}
	_ = tx.Rollback(ctx)
	// State remains serializable with all public keys and exact money strings.
	data, e := json.Marshal(v)
	check(e)
	if !bytes.Contains(data, []byte(`"balance":"0.00"`)) {
		t.Fatal("money JSON")
	}
}
