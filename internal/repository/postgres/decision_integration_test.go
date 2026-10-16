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
)

func TestDecisionIntegration(t *testing.T) {
	dsn := os.Getenv("PHASE6_DATABASE_URL")
	if dsn == "" {
		t.Skip("set PHASE6_DATABASE_URL to a fresh migrated Phase 6 test database")
	}
	u, err := url.Parse(dsn)
	if err != nil || u.Path != "/sa_lathe_shop_phase6_test" {
		t.Fatal("dedicated Phase 6 test DB required")
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
		t.Fatal("test DB must be fresh; no existing accounts overwritten")
	}
	hasher := auth.NewArgonHasher()
	authentication, err := auth.NewService(db, hasher, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	admin, err := authentication.BootstrapAdmin(ctx, "decision_admin", "Admin", password)
	if err != nil {
		t.Fatal(err)
	}
	worker, err := authentication.CreateWorker(ctx, admin, auth.WorkerInput{Username: "decision_worker", Password: password, Name: "Worker", EmployeeNo: "DECISION-W", Email: "worker@example.com", Phone: "+66812345678"})
	if err != nil {
		t.Fatal(err)
	}
	customers := auth.NewCustomerService(db, hasher)
	owner, err := customers.Register(ctx, auth.Registration{Username: "decision_owner", Password: password, Name: "Owner", CustomerType: "company", CompanyName: "Decision Company"})
	if err != nil {
		t.Fatal(err)
	}
	profiles, err := customers.Profiles(ctx, owner)
	if err != nil || len(profiles) != 1 {
		t.Fatal("company missing")
	}
	companyID := profiles[0].ID
	token, err := customers.Invite(ctx, owner, companyID)
	if err != nil {
		t.Fatal(err)
	}
	member, err := customers.Register(ctx, auth.Registration{Username: "decision_member", Password: password, Name: "Member", CustomerType: "company", Invitation: token})
	if err != nil {
		t.Fatal(err)
	}
	memberProfiles, err := customers.Profiles(ctx, member)
	if err != nil || len(memberProfiles) != 1 || memberProfiles[0].Owner {
		t.Fatal("test member must not be company owner")
	}
	outsider, err := customers.Register(ctx, auth.Registration{Username: "decision_outsider", Password: password, Name: "Outside", CustomerType: "individual"})
	if err != nil {
		t.Fatal(err)
	}
	outsideProfiles, err := customers.Profiles(ctx, outsider)
	if err != nil || len(outsideProfiles) != 1 {
		t.Fatal("individual profile missing")
	}
	requests := req.NewService(db)
	quotes := q.NewService(db)
	decisions := q.NewDecisionService(db)
	parse := func(value string) *q.Amount {
		t.Helper()
		v, e := q.ParseAmount(value)
		if e != nil {
			t.Fatal(e)
		}
		return &v
	}
	newRequest := func(a auth.Account, customerID string) req.Request {
		t.Helper()
		r, e := requests.Submit(ctx, req.Principal{AccountID: a.ID, Role: a.Role, Active: a.Active}, req.Input{CustomerID: customerID, Description: "Mould", Quantity: 3})
		if e != nil {
			t.Fatal(e)
		}
		return r
	}
	sentQuote := func(requestID, price string) q.Quotation {
		t.Helper()
		draft, e := quotes.Create(ctx, worker.Account, requestID, q.DraftInput{MaterialCost: parse("0"), LaborCost: parse("0"), FinalPrice: parse(price)})
		if e != nil {
			t.Fatal(e)
		}
		sent, e := quotes.Send(ctx, worker.Account, draft.ID, draft.LockVersion)
		if e != nil {
			t.Fatal(e)
		}
		return sent
	}
	h := auth.NewHandler(authentication)
	router := chi.NewRouter()
	h.Routes(router)
	q.NewDecisionHandler(decisions).Routes(router, h.Authenticate)
	call := func(method, path, token string, body any, wanted int) []byte {
		t.Helper()
		var encoded []byte
		if body != nil {
			encoded, _ = json.Marshal(body)
		}
		r := httptest.NewRequest(method, path, bytes.NewReader(encoded))
		r.Header.Set("Content-Type", "application/json")
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		response := httptest.NewRecorder()
		router.ServeHTTP(response, r)
		if response.Code != wanted {
			t.Fatalf("%s %s expected %d got %d", method, path, wanted, response.Code)
		}
		return response.Body.Bytes()
	}
	memberLogin, err := authentication.Login(ctx, "decision_member", password)
	if err != nil {
		t.Fatal(err)
	}
	outsideLogin, _ := authentication.Login(ctx, "decision_outsider", password)
	workerLogin, _ := authentication.Login(ctx, "decision_worker", password)
	request := newRequest(owner, companyID)
	sent := sentQuote(request.ID, "10000.01")
	accept := q.DecisionInput{Outcome: "accept", ExpectedLockVersion: sent.LockVersion}
	call("POST", "/quotations/"+sent.ID+"/decision", "", accept, 401)
	call("POST", "/quotations/"+sent.ID+"/decision", workerLogin.AccessToken, accept, 403)
	call("POST", "/quotations/"+sent.ID+"/decision", outsideLogin.AccessToken, accept, 404)
	var accepted q.Decision
	if err = json.Unmarshal(call("POST", "/quotations/"+sent.ID+"/decision", memberLogin.AccessToken, accept, 200), &accepted); err != nil {
		t.Fatal(err)
	}
	if accepted.ActorID != member.ID || accepted.Terms == nil || accepted.Terms.DepositAmount.String() != "5000.01" || !accepted.Terms.PORequired || accepted.Terms.DocumentsDueAt.Sub(accepted.Terms.AcceptedAt) != 336*time.Hour {
		t.Fatal("non-owner acceptance or exact terms failed")
	}
	var replay q.Decision
	json.Unmarshal(call("POST", "/quotations/"+sent.ID+"/decision", memberLogin.AccessToken, accept, 200), &replay)
	if !replay.DecidedAt.Equal(accepted.DecidedAt) || !replay.Terms.DocumentsDueAt.Equal(accepted.Terms.DocumentsDueAt) {
		t.Fatal("retry reset acceptance/deadline")
	}
	if _, err = decisions.Decide(ctx, owner, sent.ID, accept); !errors.Is(err, q.ErrConflict) {
		t.Fatal("different member overwrote decision")
	}
	if _, err = decisions.Decide(ctx, member, sent.ID, q.DecisionInput{Outcome: "reject", ExpectedLockVersion: sent.LockVersion}); !errors.Is(err, q.ErrConflict) {
		t.Fatal("conflicting decision accepted")
	}
	call("GET", "/quotations/"+sent.ID+"/decision", outsideLogin.AccessToken, nil, 404)
	body := call("GET", "/quotations/"+sent.ID+"/decision", memberLogin.AccessToken, nil, 200)
	if strings.Contains(string(body), "internal_costs") || strings.Contains(string(body), "material_cost") {
		t.Fatal("decision cost disclosure")
	}
	quote, err := quotes.Get(ctx, owner, sent.ID)
	if err != nil || quote.Status != "approved" || quote.LockVersion != sent.LockVersion+1 {
		t.Fatal("quotation not atomically approved")
	}
	// Historical terms use stored amounts even if a profile's current type changes.
	_, err = db.pool.Exec(ctx, `UPDATE customer SET customer_type='individual' WHERE id=$1`, companyID)
	if err != nil {
		t.Fatal(err)
	}
	historical, err := decisions.Get(ctx, owner, sent.ID)
	if err != nil || historical.Terms.CustomerType != "company" || historical.Terms.DepositAmount.String() != "5000.01" {
		t.Fatal("terms recalculated from current profile")
	}
	db.pool.Exec(ctx, `UPDATE customer SET customer_type='company' WHERE id=$1`, companyID)
	// A one-connection pool must not deadlock while reading history plus terms.
	single, err := Open(ctx, dsn, 1)
	if err != nil {
		t.Fatal(err)
	}
	short, cancel := context.WithTimeout(ctx, 3*time.Second)
	history, err := q.NewDecisionService(single).List(short, owner, request.ID, 50, 0)
	cancel()
	single.Close()
	if err != nil || len(history) != 1 || history[0].Terms == nil {
		t.Fatal("single-connection history failed")
	}
	// Full accept/reject race across members: exactly one state transition.
	racedRequest := newRequest(owner, companyID)
	racedQuote := sentQuote(racedRequest.ID, "1600.01")
	race := make(chan error, 2)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, e := decisions.Decide(ctx, owner, racedQuote.ID, q.DecisionInput{Outcome: "accept", ExpectedLockVersion: racedQuote.LockVersion})
		race <- e
	}()
	go func() {
		defer wg.Done()
		_, e := decisions.Decide(ctx, member, racedQuote.ID, q.DecisionInput{Outcome: "reject", ExpectedLockVersion: racedQuote.LockVersion})
		race <- e
	}()
	wg.Wait()
	close(race)
	successes, conflicts := 0, 0
	for e := range race {
		if e == nil {
			successes++
		} else if errors.Is(e, q.ErrConflict) {
			conflicts++
		} else {
			t.Fatal(e)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatal("accept/reject race not serialized")
	}
	// Six distinct rejections, each duplicated concurrently, still count once.
	repeatedRequest := newRequest(owner, companyID)
	for ordinal := 1; ordinal <= 6; ordinal++ {
		v := sentQuote(repeatedRequest.ID, "1600.01")
		actor := owner
		if ordinal%2 == 0 {
			actor = member
		}
		input := q.DecisionInput{Outcome: "reject", Reason: "Price", ExpectedLockVersion: v.LockVersion}
		outcomes := make(chan q.Decision, 2)
		failures := make(chan error, 2)
		for range 2 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				result, e := decisions.Decide(ctx, actor, v.ID, input)
				outcomes <- result
				failures <- e
			}()
		}
		wg.Wait()
		close(outcomes)
		close(failures)
		for e := range failures {
			if e != nil {
				t.Fatal(e)
			}
		}
		for result := range outcomes {
			expected := "ongoing"
			if ordinal == 6 {
				expected = "canceled"
			}
			if result.RejectionCount != ordinal || result.RequestStatus != expected || result.Terms != nil {
				t.Fatal("retry inflated rejection count or wrong threshold")
			}
		}
		if ordinal == 1 {
			if _, err = decisions.Decide(ctx, actor, v.ID, q.DecisionInput{Outcome: "reject", Reason: "Changed", ExpectedLockVersion: v.LockVersion}); !errors.Is(err, q.ErrConflict) {
				t.Fatal("retry rewrote reason")
			}
		}
	}
	rejectionHistory, err := decisions.List(ctx, member, repeatedRequest.ID, 50, 0)
	if err != nil || len(rejectionHistory) != 6 {
		t.Fatal("incorrect rejection audit history")
	}
	if _, err = quotes.Create(ctx, worker.Account, repeatedRequest.ID, q.DraftInput{MaterialCost: parse("0"), LaborCost: parse("0"), FinalPrice: parse("1600")}); !errors.Is(err, q.ErrConflict) {
		t.Fatal("new revision after sixth rejection")
	}
	// Sixth quotation revision can be accepted after only five rejections.
	afterFive := newRequest(owner, companyID)
	for range 5 {
		v := sentQuote(afterFive.ID, "1600.01")
		if _, err = decisions.Decide(ctx, member, v.ID, q.DecisionInput{Outcome: "reject", ExpectedLockVersion: v.LockVersion}); err != nil {
			t.Fatal(err)
		}
	}
	sixth := sentQuote(afterFive.ID, "1600.01")
	after, err := decisions.Decide(ctx, member, sixth.ID, q.DecisionInput{Outcome: "accept", ExpectedLockVersion: sixth.LockVersion})
	if err != nil || after.RejectionCount != 5 || after.RequestStatus != "ongoing" {
		t.Fatal("revision count confused with rejection count")
	}
	// Individual odd-satang and company 10,000 threshold boundary snapshots.
	for _, test := range []struct {
		actor                 auth.Account
		party, price, deposit string
		po                    bool
	}{{outsider, outsideProfiles[0].ID, "1600.01", "800.01", false}, {owner, companyID, "10000.00", "0.00", true}} {
		r := newRequest(test.actor, test.party)
		v := sentQuote(r.ID, test.price)
		result, e := decisions.Decide(ctx, test.actor, v.ID, q.DecisionInput{Outcome: "accept", ExpectedLockVersion: v.LockVersion})
		if e != nil || result.Terms.DepositAmount.String() != test.deposit || result.Terms.PORequired != test.po {
			t.Fatal("boundary acceptance terms failed")
		}
	}
	// A terms failure must roll back approval, audit insertion and request update.
	rollbackRequest := newRequest(owner, companyID)
	rollbackQuote := sentQuote(rollbackRequest.ID, "10000.01")
	_, err = db.pool.Exec(ctx, fmt.Sprintf(`ALTER TABLE quotation_acceptance_terms ADD CONSTRAINT uc3_test_failure CHECK(quotation_id<>'%s'::uuid)`, rollbackQuote.ID))
	if err != nil {
		t.Fatal(err)
	}
	_, failed := decisions.Decide(ctx, member, rollbackQuote.ID, q.DecisionInput{Outcome: "accept", ExpectedLockVersion: rollbackQuote.LockVersion})
	_, err = db.pool.Exec(ctx, `ALTER TABLE quotation_acceptance_terms DROP CONSTRAINT uc3_test_failure`)
	if err != nil {
		t.Fatal(err)
	}
	if failed == nil {
		t.Fatal("expected forced terms failure")
	}
	unchanged, err := quotes.Get(ctx, owner, rollbackQuote.ID)
	if err != nil || unchanged.Status != "sent" || unchanged.LockVersion != rollbackQuote.LockVersion {
		t.Fatal("partial decision committed")
	}
	db.pool.QueryRow(ctx, `SELECT count(*) FROM quotation_decision WHERE quotation_id=$1`, rollbackQuote.ID).Scan(&count)
	if count != 0 {
		t.Fatal("orphan audit after failed terms insert")
	}
	// Current activity and membership are rechecked, not trusted from stale structs.
	db.pool.Exec(ctx, `UPDATE account SET is_active=false WHERE id=$1`, member.ID)
	if _, err = decisions.Decide(ctx, member, rollbackQuote.ID, q.DecisionInput{Outcome: "accept", ExpectedLockVersion: rollbackQuote.LockVersion}); !errors.Is(err, q.ErrNotFound) {
		t.Fatal("inactive stale member accepted")
	}
	db.pool.Exec(ctx, `UPDATE account SET is_active=true WHERE id=$1`, member.ID)
	db.pool.Exec(ctx, `DELETE FROM customer_member WHERE account_id=$1 AND customer_id=$2`, member.ID, companyID)
	if _, err = decisions.Get(ctx, member, sent.ID); !errors.Is(err, q.ErrNotFound) {
		t.Fatal("removed member read decision")
	}
	if _, err = decisions.Decide(ctx, member, rollbackQuote.ID, q.DecisionInput{Outcome: "accept", ExpectedLockVersion: rollbackQuote.LockVersion}); !errors.Is(err, q.ErrNotFound) {
		t.Fatal("removed member accepted")
	}
}
