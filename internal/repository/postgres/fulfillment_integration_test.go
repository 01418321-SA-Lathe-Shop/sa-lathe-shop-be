package postgres

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/auth"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/file"
	p "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/payment"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/platform/objectstore"
	q "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/quotation"
	req "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/request"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"
)

func TestFulfillmentIntegration(t *testing.T) {
	dsn := os.Getenv("PHASE7_DATABASE_URL")
	if dsn == "" {
		t.Skip("set PHASE7_DATABASE_URL for a fresh isolated UC4 DB")
	}
	u, err := url.Parse(dsn)
	if err != nil || u.Path != "/sa_lathe_shop_phase7_test" {
		t.Fatal("dedicated UC4 test database required")
	}
	password := os.Getenv("TEST_ADMIN_PASSWORD")
	if len(password) < 12 || os.Getenv("TEST_S3_ENDPOINT") == "" {
		t.Fatal("private test configuration required")
	}
	ctx := context.Background()
	db, err := Open(ctx, dsn, 10)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var count int
	if err = db.pool.QueryRow(ctx, `SELECT count(*) FROM account`).Scan(&count); err != nil || count != 0 {
		t.Fatal("fresh test DB required; no data overwritten")
	}
	hasher := auth.NewArgonHasher()
	authentication, err := auth.NewService(db, hasher, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	admin, err := authentication.BootstrapAdmin(ctx, "uc4_admin", "Admin", password)
	if err != nil {
		t.Fatal(err)
	}
	worker, err := authentication.CreateWorker(ctx, admin, auth.WorkerInput{Username: "uc4_worker", Name: "Worker", EmployeeNo: "UC4-W", Password: password, Email: "worker@example.com", Phone: "+66812345678"})
	if err != nil {
		t.Fatal(err)
	}
	customers := auth.NewCustomerService(db, hasher)
	owner, err := customers.Register(ctx, auth.Registration{Username: "uc4_owner", Name: "Owner", Password: password, CustomerType: "company", CompanyName: "UC4 Company"})
	if err != nil {
		t.Fatal(err)
	}
	profiles, err := customers.Profiles(ctx, owner)
	if err != nil || len(profiles) != 1 {
		t.Fatal("company missing")
	}
	company := profiles[0].ID
	invitation, err := customers.Invite(ctx, owner, company)
	if err != nil {
		t.Fatal(err)
	}
	member, err := customers.Register(ctx, auth.Registration{Username: "uc4_member", Name: "Member", Password: password, CustomerType: "company", Invitation: invitation})
	if err != nil {
		t.Fatal(err)
	}
	outsider, err := customers.Register(ctx, auth.Registration{Username: "uc4_outsider", Name: "Individual", Password: password, CustomerType: "individual"})
	if err != nil {
		t.Fatal(err)
	}
	outsideProfiles, _ := customers.Profiles(ctx, outsider)
	requests := req.NewService(db)
	quotes := q.NewService(db)
	decisions := q.NewDecisionService(db)
	payments := p.NewService(db)
	amount := func(s string) *q.Amount {
		v, e := q.ParseAmount(s)
		if e != nil {
			t.Fatal(e)
		}
		return &v
	}
	// Historical fixtures represent contracts accepted 15 days ago; no production
	// clock override or mutable acceptance-term API is introduced for testing.
	contract := func(actor auth.Account, party, price string, old bool) (string, q.Terms) {
		t.Helper()
		r, e := requests.Submit(ctx, req.Principal{AccountID: actor.ID, Role: actor.Role, Active: true}, req.Input{CustomerID: party, Description: "UC4 mould", Quantity: 1})
		if e != nil {
			t.Fatal(e)
		}
		draft, e := quotes.Create(ctx, worker.Account, r.ID, q.DraftInput{MaterialCost: amount("0"), LaborCost: amount("0"), FinalPrice: amount(price)})
		if e != nil {
			t.Fatal(e)
		}
		sent, e := quotes.Send(ctx, worker.Account, draft.ID, draft.LockVersion)
		if e != nil {
			t.Fatal(e)
		}
		if !old {
			decision, e := decisions.Decide(ctx, actor, sent.ID, q.DecisionInput{Outcome: "accept", ExpectedLockVersion: sent.LockVersion})
			if e != nil {
				t.Fatal(e)
			}
			return sent.ID, *decision.Terms
		}
		when := time.Now().UTC().Add(-15 * 24 * time.Hour).Truncate(time.Microsecond)
		kind := "company"
		if party != company {
			kind = "individual"
		}
		terms, e := q.AcceptanceTerms(kind, *amount(price), when)
		if e != nil {
			t.Fatal(e)
		}
		tx, e := db.pool.Begin(ctx)
		if e != nil {
			t.Fatal(e)
		}
		defer tx.Rollback(ctx)
		_, e = tx.Exec(ctx, `UPDATE quotation SET status='approved',sent_at=$2,edited_at=$2,created_at=$2,lock_version=lock_version+1 WHERE id=$1`, sent.ID, when)
		if e != nil {
			t.Fatal(e)
		}
		_, e = tx.Exec(ctx, `INSERT INTO quotation_decision(quotation_id,request_id,actor_id,outcome,reason,expected_lock_version,decided_at,rejection_count,request_status) VALUES($1,$2,$3,'accept','',$4,$5,0,'ongoing')`, sent.ID, r.ID, actor.ID, sent.LockVersion, when)
		if e != nil {
			t.Fatal(e)
		}
		_, e = tx.Exec(ctx, `INSERT INTO quotation_acceptance_terms(quotation_id,customer_type,final_price,deposit_required,po_required,deposit_amount,accepted_at,documents_due_at) VALUES($1,$2,$3::numeric,$4,$5,$6::numeric,$7,$8)`, sent.ID, kind, terms.FinalPrice.String(), terms.DepositRequired, terms.PORequired, terms.DepositAmount.String(), when, terms.DocumentsDueAt)
		if e != nil {
			t.Fatal(e)
		}
		if e = tx.Commit(ctx); e != nil {
			t.Fatal(e)
		}
		return sent.ID, terms
	}
	store := objectstore.New(objectstore.Config{Endpoint: os.Getenv("TEST_S3_ENDPOINT"), Region: "us-east-1", Bucket: "sa-lathe-shop-private", AccessKey: os.Getenv("S3_ACCESS_KEY"), SecretKey: os.Getenv("S3_SECRET_KEY"), PathStyle: true})
	files := file.NewService(db, store, requests)
	attachment := func(actor auth.Account, quote string) string {
		t.Helper()
		var requestID string
		if e := db.pool.QueryRow(ctx, `SELECT request_id::text FROM quotation WHERE id=$1`, quote).Scan(&requestID); e != nil {
			t.Fatal(e)
		}
		data := []byte("Private UC4 evidence")
		principal := req.Principal{AccountID: actor.ID, Role: actor.Role, Active: true}
		link, e := files.Start(ctx, principal, requestID, file.Input{Name: "proof.txt", ContentType: "text/plain", Size: int64(len(data))})
		if e != nil {
			t.Fatal(e)
		}
		r, e := http.NewRequest("PUT", link.URL, bytes.NewReader(data))
		if e != nil {
			t.Fatal(e)
		}
		for k, v := range link.Headers {
			r.Header.Set(k, v)
		}
		response, e := http.DefaultClient.Do(r)
		if e != nil {
			t.Fatal(e)
		}
		response.Body.Close()
		if response.StatusCode != 200 {
			t.Fatal("private S3 upload failed", response.StatusCode)
		}
		if _, e = files.Complete(ctx, principal, link.AttachmentID); e != nil {
			t.Fatal(e)
		}
		return link.AttachmentID
	}
	submission := func(actor auth.Account, quote, kind, value string) p.Submission {
		in := p.Submission{IdempotencyKey: uuid.NewString(), AttachmentID: attachment(actor, quote), Kind: kind}
		if kind == "deposit" {
			when := time.Now().UTC().Add(-time.Minute).Truncate(time.Microsecond)
			in.Amount = amount(value)
			in.PaidAt = &when
		}
		return in
	}
	approve := p.Review{Outcome: "approved", ExpectedVersion: 1}
	quote, terms := contract(owner, company, "10000.01", false)
	depositInput := submission(member, quote, "deposit", "7000.00")
	// Ready private files are not payments and cannot open a job on their own.
	f, err := payments.Get(ctx, member, quote)
	if err != nil || f.Job != nil {
		t.Fatal("upload opened job", err)
	}
	if _, err = payments.Submit(ctx, outsider, quote, depositInput); !errors.Is(err, req.ErrNotFound) {
		t.Fatal("outsider submitted", err)
	}
	if _, err = payments.Submit(ctx, owner, quote, depositInput); !errors.Is(err, req.ErrNotFound) {
		t.Fatal("another user's upload was reused", err)
	}
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() { defer wg.Done(); _, e := payments.Submit(ctx, member, quote, depositInput); results <- e }()
	}
	wg.Wait()
	close(results)
	for e := range results {
		if e != nil {
			t.Fatal("exact concurrent submit replay failed", e)
		}
	}
	f, err = payments.Get(ctx, member, quote)
	if err != nil || len(f.Evidence) != 1 || f.Job != nil {
		t.Fatal("duplicate submission or premature job", err)
	}
	deposit := f.Evidence[0]
	changed := depositInput
	changed.Amount = amount("8000.00")
	if _, err = payments.Submit(ctx, member, quote, changed); !errors.Is(err, p.ErrConflict) {
		t.Fatal("changed retry accepted", err)
	}
	if _, err = payments.Review(ctx, member, deposit.ID, approve); !errors.Is(err, req.ErrForbidden) {
		t.Fatal("customer reviewed own deposit", err)
	}
	if _, err = payments.Review(ctx, admin, deposit.ID, approve); err != nil {
		t.Fatal(err)
	}
	f, _ = payments.Get(ctx, owner, quote)
	if f.Job != nil {
		t.Fatal("missing PO still opened job")
	}
	po, err := payments.Submit(ctx, member, quote, submission(member, quote, "po", ""))
	if err != nil {
		t.Fatal(err)
	}
	results = make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() { defer wg.Done(); _, e := payments.Review(ctx, worker.Account, po.ID, approve); results <- e }()
	}
	wg.Wait()
	close(results)
	for e := range results {
		if e != nil {
			t.Fatal("concurrent review replay failed", e)
		}
	}
	f, err = payments.Get(ctx, owner, quote)
	if err != nil || f.Job == nil || f.Job.Status != "queued" || f.Job.DepositAmount.String() != "7000.00" || f.Job.Balance.String() != "3000.01" {
		t.Fatal("actual deposit/job snapshot incorrect", err)
	}
	if f.Terms.DepositAmount.String() != terms.DepositAmount.String() {
		t.Fatal("actual payment mutated accepted minimum")
	}
	if _, err = payments.Review(ctx, admin, po.ID, approve); !errors.Is(err, p.ErrConflict) {
		t.Fatal("another reviewer rewrote audit", err)
	}
	if err = db.pool.QueryRow(ctx, `SELECT count(*) FROM job WHERE quotation_id=$1`, quote).Scan(&count); err != nil || count != 1 {
		t.Fatal("duplicate queued jobs")
	}
	// Signed evidence remains private and accessible only within the owning company.
	principal := req.Principal{AccountID: owner.ID, Role: owner.Role, Active: true}
	link, err := files.Download(ctx, principal, deposit.AttachmentID)
	if err != nil {
		t.Fatal(err)
	}
	response, err := http.Get(link.URL)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := io.ReadAll(response.Body)
	response.Body.Close()
	if response.StatusCode != 200 || string(data) != "Private UC4 evidence" {
		t.Fatal("private proof download failed")
	}
	unsigned, _ := url.Parse(link.URL)
	unsigned.RawQuery = ""
	response, err = http.Get(unsigned.String())
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 403 {
		t.Fatal("proof is public")
	}
	if _, err = files.Download(ctx, req.Principal{AccountID: outsider.ID, Role: outsider.Role, Active: true}, deposit.AttachmentID); !errors.Is(err, req.ErrNotFound) {
		t.Fatal("outsider read proof")
	}
	// Late missing evidence cancels, but timely complete pending evidence is safe.
	expired, expiredTerms := contract(owner, company, "10000.01", true)
	if _, err = payments.Submit(ctx, member, expired, submission(member, expired, "po", "")); !errors.Is(err, p.ErrConflict) {
		t.Fatal("late submission accepted", err)
	}
	pending, pendingTerms := contract(owner, company, "10000.00", true)
	pastFile := attachment(member, pending)
	past := pendingTerms.DocumentsDueAt.Add(-time.Hour)
	if _, err = db.pool.Exec(ctx, `UPDATE request_attachment SET completed_at=$2 WHERE id=$1`, pastFile, past.Add(-time.Second)); err != nil {
		t.Fatal(err)
	}
	pendingID := uuid.NewString()
	if _, err = db.pool.Exec(ctx, `INSERT INTO purchase_order(id,quotation_id,attachment_id,submitted_by,idempotency_key,kind,submitted_at) VALUES($1,$2,$3,$4,$5,'po',$6)`, pendingID, pending, pastFile, member.ID, uuid.NewString(), past); err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExpireUnfulfilled(ctx); err != nil {
		t.Fatal(err)
	}
	expiredState, _ := payments.Get(ctx, owner, expired)
	if expiredState.RequestStatus != "canceled" || expiredState.CanceledAt == nil || expiredState.Job != nil {
		t.Fatal("missing evidence not canceled")
	}
	pendingState, _ := payments.Get(ctx, member, pending)
	if pendingState.RequestStatus != "ongoing" || pendingState.CanceledAt != nil {
		t.Fatal("timely pending evidence expired")
	}
	if _, err = payments.Review(ctx, admin, pendingID, approve); err != nil {
		t.Fatal("late staff review rejected", err)
	}
	pendingState, _ = payments.Get(ctx, owner, pending)
	if pendingState.Job == nil {
		t.Fatal("timely evidence could not open job after deadline")
	}
	if expiredTerms.DocumentsDueAt.After(time.Now()) {
		t.Fatal("fixture not expired")
	}
	// Reject a timely pending historical PO: it is no longer a complete valid set.
	rejected, rt := contract(owner, company, "10000", true)
	rf := attachment(member, rejected)
	before := rt.DocumentsDueAt.Add(-time.Hour)
	db.pool.Exec(ctx, `UPDATE request_attachment SET completed_at=$2 WHERE id=$1`, rf, before.Add(-time.Second))
	rejectedID := uuid.NewString()
	if _, err = db.pool.Exec(ctx, `INSERT INTO purchase_order(id,quotation_id,attachment_id,submitted_by,idempotency_key,kind,submitted_at) VALUES($1,$2,$3,$4,$5,'po',$6)`, rejectedID, rejected, rf, member.ID, uuid.NewString(), before); err != nil {
		t.Fatal(err)
	}
	if _, err = payments.Review(ctx, worker.Account, rejectedID, p.Review{Outcome: "rejected", Reason: "Not a company PO", ExpectedVersion: 1}); err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExpireUnfulfilled(ctx); err != nil {
		t.Fatal(err)
	}
	rejectedState, _ := payments.Get(ctx, owner, rejected)
	if rejectedState.RequestStatus != "canceled" {
		t.Fatal("rejected missing evidence remained active")
	}
	// Rollback final approval when job insertion fails: no approved-but-jobless partial commit.
	rollback, _ := contract(outsider, outsideProfiles[0].ID, "1600.01", false)
	re, err := payments.Submit(ctx, outsider, rollback, submission(outsider, rollback, "deposit", "1600.01"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.pool.Exec(ctx, fmt.Sprintf(`ALTER TABLE job ADD CONSTRAINT uc4_forced_failure CHECK(quotation_id<>'%s'::uuid)`, rollback))
	if err != nil {
		t.Fatal(err)
	}
	_, failed := payments.Review(ctx, admin, re.ID, approve)
	if _, err = db.pool.Exec(ctx, `ALTER TABLE job DROP CONSTRAINT uc4_forced_failure`); err != nil {
		t.Fatal(err)
	}
	if failed == nil {
		t.Fatal("job failure not propagated")
	}
	state, _ := payments.Get(ctx, outsider, rollback)
	if state.Job != nil || state.Evidence[0].Status != "pending" {
		t.Fatal("review partially committed")
	}
	if _, err = payments.Review(ctx, admin, re.ID, approve); err != nil {
		t.Fatal(err)
	}
	state, _ = payments.Get(ctx, outsider, rollback)
	if state.Job == nil || state.Job.Balance.String() != "0.00" {
		t.Fatal("full deposit balance incorrect")
	}
	// Rejected evidence may be replaced before due, while preserving both records.
	replacement, _ := contract(owner, company, "10000", false)
	badPO, err := payments.Submit(ctx, member, replacement, submission(member, replacement, "po", ""))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = payments.Review(ctx, worker.Account, badPO.ID, p.Review{Outcome: "rejected", Reason: "Wrong document", ExpectedVersion: 1}); err != nil {
		t.Fatal(err)
	}
	goodPO, err := payments.Submit(ctx, member, replacement, submission(member, replacement, "po", ""))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = payments.Review(ctx, admin, goodPO.ID, approve); err != nil {
		t.Fatal(err)
	}
	replacementState, _ := payments.Get(ctx, owner, replacement)
	if len(replacementState.Evidence) != 2 || replacementState.Job == nil {
		t.Fatal("replacement lost audit or job")
	}
	// Opposite staff reviews race: only one outcome/reviewer can win.
	raceQuote, _ := contract(owner, company, "10000", false)
	raceEvidence, err := payments.Submit(ctx, member, raceQuote, submission(member, raceQuote, "po", ""))
	if err != nil {
		t.Fatal(err)
	}
	results = make(chan error, 2)
	wg.Add(2)
	go func() { defer wg.Done(); _, e := payments.Review(ctx, admin, raceEvidence.ID, approve); results <- e }()
	go func() {
		defer wg.Done()
		_, e := payments.Review(ctx, worker.Account, raceEvidence.ID, p.Review{Outcome: "rejected", Reason: "Wrong PO", ExpectedVersion: 1})
		results <- e
	}()
	wg.Wait()
	close(results)
	wins, conflicts := 0, 0
	for e := range results {
		if e == nil {
			wins++
		} else if errors.Is(e, p.ErrConflict) {
			conflicts++
		} else {
			t.Fatal(e)
		}
	}
	if wins != 1 || conflicts != 1 {
		t.Fatal("opposite reviews were not serialized")
	}
	// Expiry and approval serialize on the request and protect a timely complete set.
	racing, rr := contract(owner, company, "10000", true)
	racingFile := attachment(member, racing)
	racingAt := rr.DocumentsDueAt.Add(-time.Hour)
	if _, err = db.pool.Exec(ctx, `UPDATE request_attachment SET completed_at=$2 WHERE id=$1`, racingFile, racingAt.Add(-time.Second)); err != nil {
		t.Fatal(err)
	}
	racingID := uuid.NewString()
	if _, err = db.pool.Exec(ctx, `INSERT INTO purchase_order(id,quotation_id,attachment_id,submitted_by,idempotency_key,kind,submitted_at) VALUES($1,$2,$3,$4,$5,'po',$6)`, racingID, racing, racingFile, member.ID, uuid.NewString(), racingAt); err != nil {
		t.Fatal(err)
	}
	results = make(chan error, 2)
	wg.Add(2)
	go func() { defer wg.Done(); _, e := db.ExpireUnfulfilled(ctx); results <- e }()
	go func() { defer wg.Done(); _, e := payments.Review(ctx, admin, racingID, approve); results <- e }()
	wg.Wait()
	close(results)
	for e := range results {
		if e != nil {
			t.Fatal(e)
		}
	}
	racingState, _ := payments.Get(ctx, owner, racing)
	if racingState.Job == nil || racingState.RequestStatus != "ongoing" || racingState.CanceledAt != nil {
		t.Fatal("expiry raced past timely evidence")
	}
	// An audit failure must roll back cancellation, not leave an unaudited cancel.
	expiryRollback, _ := contract(owner, company, "10000", true)
	if _, err = db.pool.Exec(ctx, fmt.Sprintf(`ALTER TABLE uc4_cancellation ADD CONSTRAINT uc4_audit_failure CHECK(quotation_id<>'%s'::uuid)`, expiryRollback)); err != nil {
		t.Fatal(err)
	}
	_, failed = db.ExpireUnfulfilled(ctx)
	if _, err = db.pool.Exec(ctx, `ALTER TABLE uc4_cancellation DROP CONSTRAINT uc4_audit_failure`); err != nil {
		t.Fatal(err)
	}
	if failed == nil {
		t.Fatal("forced cancellation audit failure ignored")
	}
	rollbackState, _ := payments.Get(ctx, owner, expiryRollback)
	if rollbackState.RequestStatus != "ongoing" || rollbackState.CanceledAt != nil {
		t.Fatal("cancellation partially committed")
	}
	if _, err = db.ExpireUnfulfilled(ctx); err != nil {
		t.Fatal(err)
	}
	single, err := Open(ctx, dsn, 1)
	if err != nil {
		t.Fatal(err)
	}
	readCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	_, err = p.NewService(single).Get(readCtx, owner, quote)
	cancel()
	single.Close()
	if err != nil {
		t.Fatal("one-connection status read deadlocked", err)
	}
	if _, err = db.pool.Exec(ctx, `UPDATE deposit SET reason='changed' WHERE id=$1`, deposit.ID); err == nil {
		t.Fatal("reviewed evidence was mutable")
	}
	down, e := os.ReadFile("../../../migrations/000007_fulfillment.down.sql")
	if e != nil {
		t.Fatal(e)
	}
	tx, e := db.pool.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	_, e = tx.Exec(ctx, string(down))
	tx.Rollback(ctx)
	if e == nil {
		t.Fatal("populated UC4 rollback was allowed")
	}
	// HTTP session/role/isolation and strict body contracts against the real DB.
	handler := auth.NewHandler(authentication)
	router := chi.NewRouter()
	handler.Routes(router)
	p.NewHandler(payments).Routes(router, handler.Authenticate)
	memberLogin, _ := authentication.Login(ctx, member.Username, password)
	outsideLogin, _ := authentication.Login(ctx, outsider.Username, password)
	call := func(method, path, token string, body any, want int) {
		t.Helper()
		encoded, _ := json.Marshal(body)
		r := httptest.NewRequest(method, path, bytes.NewReader(encoded))
		r.Header.Set("Content-Type", "application/json")
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		if w.Code != want {
			t.Fatalf("%s %s: got %d want %d", method, path, w.Code, want)
		}
	}
	call("GET", "/quotations/"+quote+"/fulfillment", "", nil, 401)
	call("GET", "/quotations/"+quote+"/fulfillment", outsideLogin.AccessToken, nil, 404)
	call("GET", "/quotations/"+quote+"/fulfillment", memberLogin.AccessToken, nil, 200)
	call("POST", "/evidence/"+po.ID+"/review", memberLogin.AccessToken, approve, 403)
	call("POST", "/quotations/"+quote+"/evidence", memberLogin.AccessToken, map[string]any{"kind": "po", "submitted_by": admin.ID}, 400)
	// Stale activity/membership must not bypass repository authorization.
	if _, err = db.pool.Exec(ctx, `UPDATE account SET is_active=false WHERE id=$1`, worker.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = payments.Review(ctx, worker.Account, po.ID, approve); !errors.Is(err, req.ErrNotFound) {
		t.Fatal("inactive worker replay accepted", err)
	}
	if _, err = db.pool.Exec(ctx, `DELETE FROM customer_member WHERE customer_id=$1 AND account_id=$2`, company, member.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = payments.Get(ctx, member, quote); !errors.Is(err, req.ErrNotFound) {
		t.Fatal("removed member read fulfillment", err)
	}
}
