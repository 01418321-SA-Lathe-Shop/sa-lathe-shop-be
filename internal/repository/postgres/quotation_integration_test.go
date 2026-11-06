package postgres

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/auth"
	q "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/quotation"
	req "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/request"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func TestQuotationIntegration(t *testing.T) {
	dsn := os.Getenv("PHASE5_DATABASE_URL")
	if dsn == "" {
		t.Skip("set PHASE5_DATABASE_URL to a fresh migrated Phase 5 test database")
	}
	u, err := url.Parse(dsn)
	if err != nil || u.Path != "/sa_lathe_shop_phase5_test" {
		t.Fatal("dedicated Phase 5 test database required")
	}
	password := os.Getenv("TEST_ADMIN_PASSWORD")
	if len(password) < 12 {
		t.Fatal("private test password required")
	}
	ctx := context.Background()
	db, err := Open(ctx, dsn, 10)
	if err != nil {
		t.Fatal("cannot open test DB")
	}
	defer db.Close()
	var count int
	if err = db.pool.QueryRow(ctx, `SELECT count(*) FROM account`).Scan(&count); err != nil || count != 0 {
		t.Fatal("test database must be fresh; existing data is not overwritten")
	}
	hasher := auth.NewArgonHasher()
	authentication, err := auth.NewService(db, hasher, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	admin, err := authentication.BootstrapAdmin(ctx, "quote_admin", "Admin", password)
	if err != nil {
		t.Fatal(err)
	}
	w, err := authentication.CreateWorker(ctx, admin, auth.WorkerInput{Username: "quote_worker", Password: password, Name: "Worker", EmployeeNo: "QUOTE-W", Email: "worker@example.com", Phone: "+66812345678"})
	if err != nil {
		t.Fatal(err)
	}
	customers := auth.NewCustomerService(db, hasher)
	customer, err := customers.Register(ctx, auth.Registration{Username: "quote_customer", Password: password, Name: "Customer", CustomerType: "company", CompanyName: "Quote Company"})
	if err != nil {
		t.Fatal(err)
	}
	outside, err := customers.Register(ctx, auth.Registration{Username: "quote_outside", Password: password, Name: "Other", CustomerType: "individual"})
	if err != nil {
		t.Fatal(err)
	}
	profiles, err := customers.Profiles(ctx, customer)
	if err != nil || len(profiles) != 1 {
		t.Fatal("customer profile missing")
	}
	requests := req.NewService(db)
	quotes := q.NewService(db)
	createRequest := func() req.Request {
		t.Helper()
		v, e := requests.Submit(ctx, req.Principal{AccountID: customer.ID, Active: true, Role: auth.RoleCustomer}, req.Input{CustomerID: profiles[0].ID, Description: "Mould", Quantity: 7})
		if e != nil {
			t.Fatal(e)
		}
		return v
	}
	parse := func(value string) *q.Amount {
		t.Helper()
		v, e := q.ParseAmount(value)
		if e != nil {
			t.Fatal(e)
		}
		return &v
	}
	input := q.DraftInput{MaterialCost: parse("800"), LaborCost: parse("200"), FinalPrice: parse("1600")}
	job := createRequest()
	draft, err := quotes.Create(ctx, w.Account, job.ID, input)
	if err != nil {
		t.Fatal(err)
	}
	if draft.FinalPrice.String() != "1600.00" || draft.Version != 1 || draft.LockVersion != 1 {
		t.Fatal("wrong whole-request amount or version")
	}
	if _, err = quotes.Get(ctx, customer, draft.ID); !errors.Is(err, q.ErrNotFound) {
		t.Fatal("customer saw draft")
	}
	history, err := quotes.List(ctx, customer, job.ID, 50, 0)
	if err != nil || len(history) != 0 {
		t.Fatal("draft listed for customer")
	}
	if _, err = quotes.Create(ctx, w.Account, job.ID, input); !errors.Is(err, q.ErrConflict) {
		t.Fatal("duplicate active quotation accepted")
	}
	if _, err = quotes.Edit(ctx, w.Account, draft.ID, q.EditInput{DraftInput: input, ExpectedLockVersion: 2}); !errors.Is(err, q.ErrConflict) {
		t.Fatal("stale edit accepted")
	}
	edited, err := quotes.Edit(ctx, w.Account, draft.ID, q.EditInput{DraftInput: input, ExpectedLockVersion: 1})
	if err != nil || edited.LockVersion != 2 {
		t.Fatal("edit version not advanced")
	}
	// Two concurrent sends must have exactly one winner.
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() { defer wg.Done(); _, e := quotes.Send(ctx, w.Account, draft.ID, 2); results <- e }()
	}
	wg.Wait()
	close(results)
	success, conflict := 0, 0
	for e := range results {
		if e == nil {
			success++
		} else if errors.Is(e, q.ErrConflict) {
			conflict++
		} else {
			t.Fatal(e)
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatal("concurrent sends were not serialized")
	}
	sent, err := quotes.Get(ctx, customer, draft.ID)
	if err != nil || sent.Status != "sent" || sent.SentAt == nil || sent.Costs != nil {
		t.Fatal("sent quote not customer-readable")
	}
	encoded, _ := json.Marshal(sent)
	for _, secret := range []string{"material_cost", "labor_cost", "internal_costs", "minimum_price"} {
		if bytes.Contains(encoded, []byte(secret)) {
			t.Fatal("customer cost disclosure")
		}
	}
	if _, err = quotes.Get(ctx, outside, draft.ID); !errors.Is(err, q.ErrNotFound) {
		t.Fatal("cross-company quote disclosure")
	}
	if _, err = quotes.List(ctx, outside, job.ID, 50, 0); !errors.Is(err, q.ErrNotFound) {
		t.Fatal("cross-company history disclosure")
	}
	if _, err = quotes.Edit(ctx, w.Account, draft.ID, q.EditInput{DraftInput: input, ExpectedLockVersion: 3}); !errors.Is(err, q.ErrConflict) {
		t.Fatal("sent quote mutated")
	}
	// UC2 Q2.E1/Q2.E2 (M-02): staff may reject a request whose quotation is sent but not yet accepted.
	sentRequest := createRequest()
	sentQuote, err := quotes.Create(ctx, w.Account, sentRequest.ID, input)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = quotes.Send(ctx, w.Account, sentQuote.ID, 1); err != nil {
		t.Fatal(err)
	}
	if err = quotes.Reject(ctx, w.Account, sentRequest.ID, "no capacity"); err != nil {
		t.Fatal("sent but unaccepted request could not be rejected", err)
	}
	var status string
	db.pool.QueryRow(ctx, `SELECT status FROM job_request WHERE id=$1`, job.ID).Scan(&status)
	if status != "ongoing" {
		t.Fatal("request not updated atomically with send")
	}
	// DB minimum-price check must roll back the parent quotation insert too.
	invalidRequest := createRequest()
	invalidID := uuid.NewString()
	_, err = db.CreateDraft(ctx, w.ID, q.Draft{ID: invalidID, RequestID: invalidRequest.ID, MaterialCost: *parse("800"), LaborCost: *parse("200"), FinalPrice: *parse("1599.99")})
	if !errors.Is(err, q.ErrInvalidInput) {
		t.Fatal("DB minimum check missing")
	}
	db.pool.QueryRow(ctx, `SELECT count(*) FROM quotation WHERE id=$1`, invalidID).Scan(&count)
	if count != 0 {
		t.Fatal("orphan quotation after failed cost insert")
	}
	// Force the second half of send to fail in this disposable DB only.
	rollbackRequest := createRequest()
	rollbackQuote, err := quotes.Create(ctx, w.Account, rollbackRequest.ID, input)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.pool.Exec(ctx, fmt.Sprintf(`ALTER TABLE job_request ADD CONSTRAINT uc2_test_send_failure CHECK(id<>'%s'::uuid OR status<>'ongoing')`, rollbackRequest.ID))
	if err != nil {
		t.Fatal(err)
	}
	_, sendErr := quotes.Send(ctx, w.Account, rollbackQuote.ID, 1)
	_, dropErr := db.pool.Exec(ctx, `ALTER TABLE job_request DROP CONSTRAINT uc2_test_send_failure`)
	if dropErr != nil {
		t.Fatal(dropErr)
	}
	if sendErr == nil {
		t.Fatal("expected transaction failure")
	}
	unchanged, err := quotes.Get(ctx, w.Account, rollbackQuote.ID)
	if err != nil || unchanged.Status != "draft" || unchanged.LockVersion != 1 || unchanged.SentAt != nil {
		t.Fatal("partial send committed")
	}
	rejectRequest := createRequest()
	if err = quotes.Reject(ctx, w.Account, rejectRequest.ID, "Cannot machine this geometry"); err != nil {
		t.Fatal(err)
	}
	var reason string
	err = db.pool.QueryRow(ctx, `SELECT r.status,x.reason FROM job_request r JOIN request_rejection x ON x.request_id=r.id WHERE r.id=$1`, rejectRequest.ID).Scan(&status, &reason)
	if err != nil || status != "canceled" || reason == "" {
		t.Fatal("rejection not atomic/audited")
	}
	if _, err = quotes.Create(ctx, w.Account, rejectRequest.ID, input); !errors.Is(err, q.ErrConflict) {
		t.Fatal("canceled request reopened")
	}
	// Race request rejection against sending a draft: only one may win.
	raceRequest := createRequest()
	raceQuote, err := quotes.Create(ctx, w.Account, raceRequest.ID, input)
	if err != nil {
		t.Fatal(err)
	}
	raceSend := make(chan error, 1)
	raceReject := make(chan error, 1)
	wg.Add(2)
	go func() { defer wg.Done(); _, e := quotes.Send(ctx, w.Account, raceQuote.ID, 1); raceSend <- e }()
	go func() { defer wg.Done(); raceReject <- quotes.Reject(ctx, w.Account, raceRequest.ID, "No capacity") }()
	wg.Wait()
	// Rejection may now follow a send (M-02), so it always wins; a send that loses the race fails cleanly.
	if e := <-raceReject; e != nil {
		t.Fatal("rejection lost the race", e)
	}
	if e := <-raceSend; e != nil && !errors.Is(e, q.ErrConflict) && !errors.Is(e, q.ErrInvalidInput) {
		t.Fatal(e)
	}
	var raceRequestStatus, raceQuoteStatus string
	if err = db.pool.QueryRow(ctx, `SELECT r.status,q.status FROM job_request r JOIN quotation q ON q.request_id=r.id WHERE r.id=$1 AND q.id=$2`, raceRequest.ID, raceQuote.ID).Scan(&raceRequestStatus, &raceQuoteStatus); err != nil || raceRequestStatus != "canceled" || (raceQuoteStatus != "rejected" && raceQuoteStatus != "draft") {
		t.Fatal("send/reject race broke state", raceRequestStatus, raceQuoteStatus, err)
	}
	success, conflict = 0, 0
	// Same request row lock also protects simultaneous draft creation.
	newRequest := createRequest()
	race := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() { defer wg.Done(); _, e := quotes.Create(ctx, w.Account, newRequest.ID, input); race <- e }()
	}
	wg.Wait()
	close(race)
	success, conflict = 0, 0
	for e := range race {
		if e == nil {
			success++
		} else if errors.Is(e, q.ErrConflict) {
			conflict++
		} else {
			t.Fatal(e)
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatal("parallel drafts duplicated")
	}
	// Real HTTP auth, strict amount strings, conflict mapping and cost redaction.
	rejection, err := quotes.Rejection(ctx, customer, rejectRequest.ID)
	if err != nil || rejection.Reason != "Cannot machine this geometry" {
		t.Fatal("customer cannot read refusal reason")
	}
	if _, err = quotes.Rejection(ctx, outside, rejectRequest.ID); !errors.Is(err, q.ErrNotFound) {
		t.Fatal("cross-company refusal reason disclosure")
	}
	// Test-only fixture for a future UC3 decision; no customer-decision API exists.
	_, err = db.pool.Exec(ctx, `UPDATE quotation SET status='rejected' WHERE id=$1`, draft.ID)
	if err != nil {
		t.Fatal(err)
	}
	revision, err := quotes.Create(ctx, w.Account, job.ID, input)
	if err != nil || revision.Version != 2 || revision.PreviousID == nil || *revision.PreviousID != draft.ID {
		t.Fatal("revision chain not preserved")
	}
	h := auth.NewHandler(authentication)
	router := chi.NewRouter()
	h.Routes(router)
	q.NewHandler(quotes).Routes(router, h.Authenticate)
	call := func(method, path, token, body string, wanted int) []byte {
		t.Helper()
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		response := httptest.NewRecorder()
		router.ServeHTTP(response, r)
		if response.Code != wanted {
			t.Fatalf("%s %s expected %d, got %d", method, path, wanted, response.Code)
		}
		return response.Body.Bytes()
	}
	customerLogin, _ := authentication.Login(ctx, "quote_customer", password)
	workerLogin, _ := authentication.Login(ctx, "quote_worker", password)
	adminLogin, _ := authentication.Login(ctx, "quote_admin", password)
	call("GET", "/quotations/"+draft.ID, "", "", 401)
	body := string(call("GET", "/quotations/"+draft.ID, customerLogin.AccessToken, "", 200))
	if strings.Contains(body, "internal_costs") {
		t.Fatal("HTTP customer cost leak")
	}
	call("POST", "/quotations/estimate", customerLogin.AccessToken, `{"material_cost":"1","labor_cost":"1"}`, 403)
	call("POST", "/quotations/estimate", adminLogin.AccessToken, `{"material_cost":"1","labor_cost":"1"}`, 403)
	preview := call("POST", "/quotations/estimate", workerLogin.AccessToken, `{"material_cost":"0.01","labor_cost":"0.01"}`, 200)
	if !bytes.Contains(preview, []byte(`"minimum_price":"0.04"`)) {
		t.Fatal("incorrect estimate rounding")
	}
	call("POST", "/quotations/estimate", workerLogin.AccessToken, `{"material_cost":0.01,"labor_cost":"0.01"}`, 400)
	call("POST", "/quotations/"+draft.ID+"/send", workerLogin.AccessToken, `{"expected_lock_version":2}`, 409)
	// Inactive Workers cannot mutate even when a stale actor struct is supplied.
	_, err = authentication.UpdateWorker(ctx, admin, w.ID, auth.WorkerUpdate{Active: func() *bool { v := false; return &v }()})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = quotes.Create(ctx, w.Account, createRequest().ID, input); !errors.Is(err, q.ErrForbidden) {
		t.Fatal("inactive stale Worker mutated quotation")
	}
}
