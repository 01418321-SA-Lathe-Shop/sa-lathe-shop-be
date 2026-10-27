package postgres

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/auth"
	f "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/delivery"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/file"
	m "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/material"
	pay "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/payment"
	proc "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/procurement"
	p "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/production"
	q "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/quotation"
	req "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/request"
	"github.com/google/uuid"
)

type performanceFixture struct {
	admin       auth.Account
	workers     []auth.Account
	customers   []auth.Account
	longJobIDs  []string
	accounts    []auth.Account
	materialIDs []string
	seedElapsed time.Duration
}

// Seed through the real services/repositories and private S3 lifecycle. Never
// disable triggers or fabricate approved documents to make fixtures easier.
func seedPerformanceFixture(t *testing.T, ctx context.Context, db *Database, authentication *auth.Service, store file.Store, password string) performanceFixture {
	t.Helper()
	start := time.Now()
	check := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	amount := func(text string) *q.Amount { v, err := q.ParseAmount(text); check(err); return &v }
	quantity := func(text string) *m.Decimal { v, err := m.ParseDecimal(text); check(err); return &v }
	fixture := performanceFixture{}
	admin, err := authentication.BootstrapAdmin(ctx, "bench_admin", "Benchmark Admin", password)
	check(err)
	fixture.admin = admin
	fixture.accounts = append(fixture.accounts, admin)
	for i := 0; i < 9; i++ {
		worker, err := authentication.CreateWorker(ctx, admin, auth.WorkerInput{Username: fmt.Sprintf("bench_worker_%02d", i), Name: "Benchmark Worker", Password: password, EmployeeNo: fmt.Sprintf("BENCH-%02d", i), Email: "fixture@example.com", Phone: "+66812345678"})
		check(err)
		fixture.workers = append(fixture.workers, worker.Account)
		fixture.accounts = append(fixture.accounts, worker.Account)
	}
	customers := auth.NewCustomerService(db, auth.NewArgonHasher())
	profiles := make([]string, 90)
	for i := 0; i < 90; i++ {
		customer, err := customers.Register(ctx, auth.Registration{Username: fmt.Sprintf("bench_customer_%02d", i), Password: password, Name: "Benchmark Customer", CustomerType: "company", CompanyName: fmt.Sprintf("Benchmark Company %02d", i)})
		check(err)
		values, err := customers.Profiles(ctx, customer)
		check(err)
		if len(values) != 1 {
			t.Fatal("benchmark customer profile missing")
		}
		profiles[i] = values[0].ID
		fixture.customers = append(fixture.customers, customer)
		fixture.accounts = append(fixture.accounts, customer)
	}
	inventory := m.NewService(db)
	unit, err := inventory.CreateUnit(ctx, admin, m.UnitInput{Name: "kg"})
	check(err)
	var steel m.Material
	for i := 0; i < 1000; i++ {
		value, err := inventory.Create(ctx, admin, m.MaterialInput{Name: fmt.Sprintf("Benchmark steel %04d", i), UnitID: unit.ID})
		check(err)
		if i == 0 {
			steel = value
		}
		if i < 100 {
			fixture.materialIDs = append(fixture.materialIDs, value.ID)
			_, err = inventory.Stock(ctx, admin, value.ID, m.StockInput{IdempotencyKey: uuid.NewString(), ExpectedVersion: value.Version, Mode: "addition", Quantity: quantity("100000"), UnitCost: quantity("10"), Reason: "Synthetic benchmark opening stock"})
			check(err)
		}
	}
	procurement := proc.NewService(db)
	for i := 0; i < 100; i++ {
		_, err := procurement.CreateSupplier(ctx, admin, proc.SupplierInput{Name: fmt.Sprintf("Benchmark supplier %03d", i), Email: "supplier@example.com", Phone: "+66812345678"})
		check(err)
	}
	requests := req.NewService(db)
	files := file.NewService(db, store, requests)
	quotes, decisions := q.NewService(db), q.NewDecisionService(db)
	deposits := pay.NewService(db)
	production, delivery := p.NewService(db), f.NewService(db)
	var inProgress []string
	for i := 0; i < 1000; i++ {
		customer := fixture.customers[i%90]
		worker := fixture.workers[i%9]
		principal := req.Principal{AccountID: customer.ID, Role: customer.Role, Active: true}
		request, err := requests.Submit(ctx, principal, req.Input{CustomerID: profiles[i%90], Description: fmt.Sprintf("Synthetic mould job %04d", i), Quantity: 1, MaterialSpec: "Steel"})
		check(err)
		draft, err := quotes.Create(ctx, worker, request.ID, q.DraftInput{MaterialCost: amount("10"), LaborCost: amount("10"), FinalPrice: amount("100")})
		check(err)
		sent, err := quotes.Send(ctx, worker, draft.ID, draft.LockVersion)
		check(err)
		_, err = decisions.Decide(ctx, customer, sent.ID, q.DecisionInput{Outcome: "accept", ExpectedLockVersion: sent.LockVersion})
		check(err)
		data := []byte("Synthetic benchmark company PO")
		link, err := files.Start(ctx, principal, request.ID, file.Input{Name: "po.txt", ContentType: "text/plain", Size: int64(len(data))})
		check(err)
		upload, err := http.NewRequestWithContext(ctx, http.MethodPut, link.URL, bytes.NewReader(data))
		check(err)
		for key, value := range link.Headers {
			upload.Header.Set(key, value)
		}
		response, err := http.DefaultClient.Do(upload)
		if err != nil {
			t.Fatal("benchmark private PO upload failed; signed URL withheld")
		}
		response.Body.Close()
		if response.StatusCode != http.StatusOK {
			t.Fatal("benchmark private PO upload failed; signed URL withheld")
		}
		_, err = files.Complete(ctx, principal, link.AttachmentID)
		check(err)
		evidence, err := deposits.Submit(ctx, customer, sent.ID, pay.Submission{IdempotencyKey: uuid.NewString(), AttachmentID: link.AttachmentID, Kind: "po"})
		check(err)
		_, err = deposits.Review(ctx, worker, evidence.ID, pay.Review{ExpectedVersion: 1, Outcome: "approved"})
		check(err)
		fulfilled, err := deposits.Get(ctx, customer, sent.ID)
		check(err)
		if fulfilled.Job == nil {
			t.Fatal("benchmark job did not open through approved PO")
		}
		job := fulfilled.Job.ID
		plan, err := inventory.Save(ctx, worker, job, m.PlanInput{IdempotencyKey: uuid.NewString(), Lines: []m.Requirement{{MaterialID: steel.ID, Quantity: quantity("1")}}})
		check(err)
		if i%5 == 0 {
			continue // 200 queued jobs.
		}
		state, err := production.Start(ctx, worker, job, p.StartInput{Command: p.Command{IdempotencyKey: uuid.NewString()}, ExpectedPlanVersion: &plan.Version})
		check(err)
		state, err = production.Use(ctx, worker, job, p.UsageInput{Command: p.Command{IdempotencyKey: uuid.NewString(), ExpectedVersion: state.Version}, Lines: []p.UsageLine{{MaterialID: steel.ID, Quantity: quantity("1")}}})
		check(err)
		state, err = production.Time(ctx, worker, job, p.TimeInput{Command: p.Command{IdempotencyKey: uuid.NewString(), ExpectedVersion: state.Version}, Minutes: 10, Note: "Synthetic machine time"})
		check(err)
		if i%5 == 1 {
			inProgress = append(inProgress, job)
			continue
		}
		_, err = production.Complete(ctx, worker, job, p.Command{IdempotencyKey: uuid.NewString(), ExpectedVersion: state.Version})
		check(err)
		if i%5 == 2 {
			continue // 200 completed, uninvoiced jobs.
		}
		days := 14
		final, err := delivery.Terms(ctx, admin, job, f.TermsInput{Command: f.Command{IdempotencyKey: uuid.NewString()}, CreditDays: &days})
		check(err)
		final, err = delivery.Issue(ctx, worker, job, f.Command{IdempotencyKey: uuid.NewString(), ExpectedVersion: final.Version})
		check(err)
		final, err = delivery.Ship(ctx, worker, job, f.ShipInput{Command: f.Command{IdempotencyKey: uuid.NewString(), ExpectedVersion: final.Version}, Method: "self", ReceiverName: "Benchmark Customer"})
		check(err)
		if i%5 == 3 {
			_, err = delivery.Inspect(ctx, customer, job, f.InspectInput{Command: f.Command{IdempotencyKey: uuid.NewString(), ExpectedVersion: final.Version}, Outcome: "return", Reason: "Synthetic adjustment"})
			check(err)
			continue // 200 jobs in their free-rework cycle.
		}
		final, err = delivery.Inspect(ctx, customer, job, f.InspectInput{Command: f.Command{IdempotencyKey: uuid.NewString(), ExpectedVersion: final.Version}, Outcome: "accept"})
		check(err)
		paidAt := time.Now().UTC().Add(-time.Second)
		_, err = delivery.Pay(ctx, admin, job, f.PaymentInput{Command: f.Command{IdempotencyKey: uuid.NewString(), ExpectedVersion: final.Version}, Amount: amount("100"), PaidAt: &paidAt}, true)
		check(err) // 200 accepted and fully settled jobs.
		if i%100 == 99 {
			t.Logf("benchmark fixture: %d/1000 jobs seeded", i+1)
		}
	}
	for i, entries := range []int{10, 100, 1000} {
		job := inProgress[i]
		state, err := production.Get(ctx, fixture.workers[0], job)
		check(err)
		for len(state.TimeEntries) < entries {
			state, err = production.Time(ctx, fixture.workers[0], job, p.TimeInput{Command: p.Command{IdempotencyKey: uuid.NewString(), ExpectedVersion: state.Version}, Minutes: 1, Note: "Long-history stress fixture"})
			check(err)
		}
		fixture.longJobIDs = append(fixture.longJobIDs, job)
	}
	fixture.seedElapsed = time.Since(start)
	return fixture
}
