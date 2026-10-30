package postgres

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/auth"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/file"
	m "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/material"
	p "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/payment"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/platform/objectstore"
	production "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/production"
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

func TestMaterialIntegration(t *testing.T) {
	dsn := os.Getenv("PHASE8_DATABASE_URL")
	if dsn == "" {
		t.Skip("set PHASE8_DATABASE_URL for isolated UC5 integration")
	}
	parsed, err := url.Parse(dsn)
	if err != nil || parsed.Path != "/sa_lathe_shop_phase8_test" {
		t.Fatal("dedicated inventory database required")
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
		t.Fatal("fresh DB required; no existing data changed")
	}
	hasher := auth.NewArgonHasher()
	authentication, err := auth.NewService(db, hasher, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	admin, err := authentication.BootstrapAdmin(ctx, "uc5_admin", "Admin", password)
	if err != nil {
		t.Fatal(err)
	}
	worker, err := authentication.CreateWorker(ctx, admin, auth.WorkerInput{Username: "uc5_worker", Name: "Worker", EmployeeNo: "UC5-W", Password: password, Email: "worker@example.com", Phone: "+66812345678"})
	if err != nil {
		t.Fatal(err)
	}
	customers := auth.NewCustomerService(db, hasher)
	customer, err := customers.Register(ctx, auth.Registration{Username: "uc5_customer", Name: "Buyer", Password: password, CustomerType: "company", CompanyName: "UC5 Company"})
	if err != nil {
		t.Fatal(err)
	}
	profiles, err := customers.Profiles(ctx, customer)
	if err != nil || len(profiles) != 1 {
		t.Fatal("company missing")
	}
	requests := req.NewService(db)
	quotes := q.NewService(db)
	decisions := q.NewDecisionService(db)
	payments := p.NewService(db)
	store := objectstore.New(objectstore.Config{Endpoint: os.Getenv("TEST_S3_ENDPOINT"), Region: "us-east-1", Bucket: "sa-lathe-shop-private", AccessKey: os.Getenv("S3_ACCESS_KEY"), SecretKey: os.Getenv("S3_SECRET_KEY"), PathStyle: true})
	files := file.NewService(db, store, requests)
	price := func(s string) *q.Amount {
		v, e := q.ParseAmount(s)
		if e != nil {
			t.Fatal(e)
		}
		return &v
	}
	qty := func(s string) *m.Decimal {
		v, e := m.ParseDecimal(s)
		if e != nil {
			t.Fatal(e)
		}
		return &v
	}
	t.Run("unit conversions", func(t *testing.T) {
		service := m.NewService(db)
		base, e := service.CreateUnit(ctx, admin, m.UnitInput{Name: "conversion-kg"})
		if e != nil {
			t.Fatal(e)
		}
		sheet, e := service.CreateUnit(ctx, admin, m.UnitInput{Name: "conversion-sheet"})
		if e != nil {
			t.Fatal(e)
		}
		material, e := service.Create(ctx, admin, m.MaterialInput{Name: "Conversion steel", UnitID: base.ID})
		if e != nil {
			t.Fatal(e)
		}
		if _, e = service.CreateConversion(ctx, admin, material.ID, m.ConversionInput{UnitID: base.ID, Factor: qty("2")}); !errors.Is(e, req.ErrInvalidInput) {
			t.Fatal("base unit accepted", e)
		}
		if _, e = service.CreateConversion(ctx, admin, uuid.NewString(), m.ConversionInput{UnitID: sheet.ID, Factor: qty("2")}); !errors.Is(e, req.ErrNotFound) {
			t.Fatal("unknown material accepted", e)
		}
		first, e := service.CreateConversion(ctx, admin, material.ID, m.ConversionInput{UnitID: sheet.ID, Factor: qty("12.5")})
		if e != nil || first.UnitName != sheet.Name || first.Factor.String() != "12.500000" || !first.Active {
			t.Fatal(first, e)
		}
		if _, e = service.CreateConversion(ctx, admin, material.ID, m.ConversionInput{UnitID: sheet.ID, Factor: qty("13")}); !errors.Is(e, m.ErrConflict) {
			t.Fatal("second active conversion accepted", e)
		}
		if _, e = service.DeactivateConversion(ctx, admin, uuid.NewString(), first.ID); !errors.Is(e, req.ErrNotFound) {
			t.Fatal("conversion reached through another material", e)
		}
		gone, e := service.DeactivateConversion(ctx, admin, material.ID, first.ID)
		if e != nil || gone.Active || gone.DeactivatedAt == nil || gone.DeactivatedBy == nil || *gone.DeactivatedBy != admin.ID {
			t.Fatal(gone, e)
		}
		retried, e := service.DeactivateConversion(ctx, admin, material.ID, first.ID)
		if e != nil || !retried.DeactivatedAt.Equal(*gone.DeactivatedAt) {
			t.Fatal("deactivate retry changed the record", e)
		}
		if _, e = service.CreateConversion(ctx, admin, material.ID, m.ConversionInput{UnitID: sheet.ID, Factor: qty("13")}); e != nil {
			t.Fatal(e)
		}
		values, e := service.Conversions(ctx, worker.Account, material.ID)
		if e != nil || len(values) != 2 || values[0].ID != first.ID || values[0].Active || !values[1].Active {
			t.Fatal(values, e)
		}
		for _, statement := range []string{`UPDATE unit_conversion SET factor=99 WHERE id=$1`, `UPDATE unit_conversion SET is_active=true,deactivated_by=NULL,deactivated_at=NULL WHERE id=$1`, `DELETE FROM unit_conversion WHERE id=$1`} {
			if _, e = db.pool.Exec(ctx, statement, first.ID); e == nil {
				t.Fatal("retained conversion changed: " + statement)
			}
		}
	})
	job := func() string {
		t.Helper()
		r, e := requests.Submit(ctx, req.Principal{AccountID: customer.ID, Role: customer.Role, Active: true}, req.Input{CustomerID: profiles[0].ID, Description: "Inventory test mould", Quantity: 1})
		if e != nil {
			t.Fatal(e)
		}
		draft, e := quotes.Create(ctx, worker.Account, r.ID, q.DraftInput{MaterialCost: price("0"), LaborCost: price("0"), FinalPrice: price("1000")})
		if e != nil {
			t.Fatal(e)
		}
		sent, e := quotes.Send(ctx, worker.Account, draft.ID, draft.LockVersion)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = decisions.Decide(ctx, customer, sent.ID, q.DecisionInput{Outcome: "accept", ExpectedLockVersion: sent.LockVersion}); e != nil {
			t.Fatal(e)
		}
		principal := req.Principal{AccountID: customer.ID, Role: customer.Role, Active: true}
		data := []byte("Company PO")
		link, e := files.Start(ctx, principal, r.ID, file.Input{Name: "po.txt", ContentType: "text/plain", Size: int64(len(data))})
		if e != nil {
			t.Fatal(e)
		}
		upload, e := http.NewRequest("PUT", link.URL, bytes.NewReader(data))
		if e != nil {
			t.Fatal(e)
		}
		for k, v := range link.Headers {
			upload.Header.Set(k, v)
		}
		response, e := http.DefaultClient.Do(upload)
		if e != nil {
			t.Fatal(e)
		}
		response.Body.Close()
		if response.StatusCode != 200 {
			t.Fatal("PO upload failed")
		}
		if _, e = files.Complete(ctx, principal, link.AttachmentID); e != nil {
			t.Fatal(e)
		}
		evidence, e := payments.Submit(ctx, customer, sent.ID, p.Submission{IdempotencyKey: uuid.NewString(), AttachmentID: link.AttachmentID, Kind: "po"})
		if e != nil {
			t.Fatal(e)
		}
		if _, e = payments.Review(ctx, admin, evidence.ID, p.Review{Outcome: "approved", ExpectedVersion: 1}); e != nil {
			t.Fatal(e)
		}
		fulfillment, e := payments.Get(ctx, customer, sent.ID)
		if e != nil || fulfillment.Job == nil {
			t.Fatal("queued job missing", e)
		}
		return fulfillment.Job.ID
	}
	inventory := m.NewService(db)
	unit, err := inventory.CreateUnit(ctx, admin, m.UnitInput{Name: "kg"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = inventory.CreateUnit(ctx, admin, m.UnitInput{Name: "KG"}); !errors.Is(err, m.ErrConflict) {
		t.Fatal("unit duplicate allowed", err)
	}
	steel, err := inventory.Create(ctx, admin, m.MaterialInput{Name: "S45C steel", UnitID: unit.ID})
	if err != nil {
		t.Fatal(err)
	}
	rubber, err := inventory.Create(ctx, admin, m.MaterialInput{Name: "Rubber", UnitID: unit.ID})
	if err != nil {
		t.Fatal(err)
	}
	current := func(id string) m.Material {
		t.Helper()
		values, e := inventory.Search(ctx, worker.Account, "", 100, 0)
		if e != nil {
			t.Fatal(e)
		}
		for _, v := range values {
			if v.ID == id {
				return v
			}
		}
		t.Fatal("material missing")
		return m.Material{}
	}
	change := func(id, mode, value, cost string) m.Movement {
		t.Helper()
		material := current(id)
		in := m.StockInput{IdempotencyKey: uuid.NewString(), Mode: mode, Quantity: qty(value), ExpectedVersion: material.Version, Reason: "Verified count/opening test stock"}
		if mode == "addition" {
			in.UnitCost = qty(cost)
		}
		movement, e := inventory.Stock(ctx, admin, id, in)
		if e != nil {
			t.Fatal(e)
		}
		return movement
	}
	t.Run("entry in other units", func(t *testing.T) {
		base, e := inventory.CreateUnit(ctx, admin, m.UnitInput{Name: "entry-kg"})
		if e != nil {
			t.Fatal(e)
		}
		sheet, e := inventory.CreateUnit(ctx, admin, m.UnitInput{Name: "entry-sheet"})
		if e != nil {
			t.Fatal(e)
		}
		material, e := inventory.Create(ctx, admin, m.MaterialInput{Name: "Entry steel", UnitID: base.ID})
		if e != nil {
			t.Fatal(e)
		}
		factor := func(value string) m.Conversion {
			t.Helper()
			c, e := inventory.CreateConversion(ctx, admin, material.ID, m.ConversionInput{UnitID: sheet.ID, Factor: qty(value)})
			if e != nil {
				t.Fatal(e)
			}
			return c
		}
		stock := func(quantity, unit string) m.StockInput {
			return m.StockInput{IdempotencyKey: uuid.NewString(), Mode: "addition", Quantity: qty(quantity), UnitID: unit, UnitCost: qty("10"), Reason: "Entered in sheets", ExpectedVersion: current(material.ID).Version}
		}
		if _, e = inventory.Stock(ctx, admin, material.ID, stock("1", sheet.ID)); !errors.Is(e, req.ErrInvalidInput) {
			t.Fatal("unit without a conversion accepted", e)
		}
		inexact := factor("0.333333")
		if _, e = inventory.Stock(ctx, admin, material.ID, stock("0.5", sheet.ID)); !errors.Is(e, req.ErrInvalidInput) {
			t.Fatal("rounded conversion accepted", e)
		}
		if _, e = inventory.DeactivateConversion(ctx, admin, material.ID, inexact.ID); e != nil {
			t.Fatal(e)
		}
		first := factor("12.5")
		in := stock("2", sheet.ID)
		move, e := inventory.Stock(ctx, admin, material.ID, in)
		if e != nil || move.Quantity.String() != "25.000000" || move.AfterQuantity.String() != "25.000000" || move.EnteredQuantity.String() != "2.000000" || *move.EnteredUnit != sheet.Name || move.FactorUsed.String() != "12.500000" {
			t.Fatal(move, e)
		}
		if got := current(material.ID); got.OnHand.String() != "25.000000" || got.AverageCost.String() != "10.000000" {
			t.Fatal("stock must stay in the base unit", got)
		}
		if _, e = inventory.DeactivateConversion(ctx, admin, material.ID, first.ID); e != nil {
			t.Fatal(e)
		}
		// A new factor must not change the outcome of an exact retry.
		factor("13")
		retry, e := inventory.Stock(ctx, admin, material.ID, in)
		if e != nil || retry.ID != move.ID || retry.Quantity.String() != "25.000000" || retry.FactorUsed.String() != "12.500000" {
			t.Fatal("retry converted differently", retry, e)
		}
		different := in
		different.UnitID = ""
		if _, e = inventory.Stock(ctx, admin, material.ID, different); !errors.Is(e, m.ErrConflict) {
			t.Fatal("same key with a different unit accepted", e)
		}
		same, e := inventory.Stock(ctx, admin, material.ID, stock("1", base.ID))
		if e != nil || same.Quantity.String() != "1.000000" || same.EnteredQuantity != nil {
			t.Fatal("base unit should record nothing extra", same, e)
		}
		jobID := job()
		saved, e := inventory.Save(ctx, worker.Account, jobID, m.PlanInput{IdempotencyKey: uuid.NewString(), ExpectedVersion: 0, Lines: []m.Requirement{{MaterialID: material.ID, Quantity: qty("1"), UnitID: sheet.ID}}})
		if e != nil || saved.Lines[0].Required.String() != "13.000000" || saved.Lines[0].Reserved.String() != "13.000000" || saved.Lines[0].EnteredQuantity.String() != "1.000000" {
			t.Fatal(saved, e)
		}
		rechecked, e := inventory.Reserve(ctx, worker.Account, jobID, m.ReserveInput{IdempotencyKey: uuid.NewString(), ExpectedVersion: saved.Version})
		if e != nil || rechecked.Lines[0].EnteredQuantity == nil || rechecked.Lines[0].FactorUsed.String() != "13.000000" {
			t.Fatal("reservation lost the entered values", rechecked, e)
		}
		work := production.NewService(db)
		started, e := work.Start(ctx, worker.Account, jobID, production.StartInput{Command: production.Command{IdempotencyKey: uuid.NewString(), ExpectedVersion: 0}, ExpectedPlanVersion: &rechecked.Version})
		if e != nil {
			t.Fatal(e)
		}
		use := production.UsageInput{Command: production.Command{IdempotencyKey: uuid.NewString(), ExpectedVersion: started.Version}, Lines: []production.UsageLine{{MaterialID: material.ID, Quantity: qty("0.5"), UnitID: sheet.ID}}}
		used, e := work.Use(ctx, worker.Account, jobID, use)
		if e != nil || len(used.Usage) != 1 || used.Usage[0].Quantity.String() != "6.500000" || used.Usage[0].EnteredQuantity.String() != "0.500000" || *used.Usage[0].EnteredUnit != sheet.Name {
			t.Fatal(used, e)
		}
		repeat, e := work.Use(ctx, worker.Account, jobID, use)
		if e != nil || repeat.Usage[0].ID != used.Usage[0].ID || repeat.Usage[0].FactorUsed.String() != "13.000000" {
			t.Fatal("usage retry changed", repeat, e)
		}
		history, e := inventory.Movements(ctx, worker.Account, material.ID, 1, 0)
		if e != nil || history[0].Mode != "job_issue" || history[0].Quantity.String() != "6.500000" || history[0].EnteredQuantity.String() != "0.500000" {
			t.Fatal("ledger lost the entered values", history, e)
		}
		if got := current(material.ID); got.OnHand.String() != "19.500000" || got.Reserved.String() != "6.500000" {
			t.Fatal("wrong balance after usage", got)
		}
	})
	opening := m.StockInput{IdempotencyKey: uuid.NewString(), Mode: "addition", Quantity: qty("10"), UnitCost: qty("10"), ExpectedVersion: steel.Version, Reason: "Opening inventory"}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() { defer wg.Done(); _, e := inventory.Stock(ctx, admin, steel.ID, opening); results <- e }()
	}
	wg.Wait()
	close(results)
	for e := range results {
		if e != nil {
			t.Fatal("stock retry failed", e)
		}
	}
	if v := current(steel.ID); v.OnHand.String() != "10.000000" || v.AverageCost.String() != "10.000000" {
		t.Fatal("stock added twice or incorrectly parsed", v)
	}
	changed := opening
	changed.Reason = "Changed retry"
	if _, err = inventory.Stock(ctx, admin, steel.ID, changed); !errors.Is(err, m.ErrConflict) {
		t.Fatal("changed stock retry accepted", err)
	}
	change(steel.ID, "addition", "10", "30")
	if current(steel.ID).AverageCost.String() != "20.000000" {
		t.Fatal("weighted average incorrect")
	}
	change(rubber.ID, "addition", "10.5", "2")
	values, err := inventory.Search(ctx, worker.Account, "s45c", 100, 0)
	if err != nil || len(values) != 1 || values[0].ID != steel.ID {
		t.Fatal("material search failed", err)
	}
	literal, err := inventory.Search(ctx, worker.Account, "%", 100, 0)
	if err != nil || len(literal) != 0 {
		t.Fatal("search treated input as SQL wildcard")
	}
	job1, job2 := job(), job()
	planInput := func(version int64, lines ...m.Requirement) m.PlanInput {
		return m.PlanInput{IdempotencyKey: uuid.NewString(), ExpectedVersion: version, Lines: lines}
	}
	in1 := planInput(0, m.Requirement{MaterialID: steel.ID, Quantity: qty("15")})
	in2 := planInput(0, m.Requirement{MaterialID: steel.ID, Quantity: qty("15")})
	results = make(chan error, 2)
	wg.Add(2)
	go func() { defer wg.Done(); _, e := inventory.Save(ctx, worker.Account, job1, in1); results <- e }()
	go func() { defer wg.Done(); _, e := inventory.Save(ctx, worker.Account, job2, in2); results <- e }()
	wg.Wait()
	close(results)
	for e := range results {
		if e != nil {
			t.Fatal("competing jobs failed", e)
		}
	}
	p1, err := inventory.Get(ctx, worker.Account, job1)
	if err != nil {
		t.Fatal(err)
	}
	p2, err := inventory.Get(ctx, worker.Account, job2)
	if err != nil {
		t.Fatal(err)
	}
	total, e := p1.Lines[0].Reserved.Add(p2.Lines[0].Reserved)
	if e != nil || total.String() != "20.000000" {
		t.Fatal("over/under-reservation", total, e)
	}
	fullID, partialID := job1, job2
	if p2.Ready {
		fullID, partialID = job2, job1
	}
	if v := current(steel.ID); v.OnHand.String() != "20.000000" || v.Reserved.String() != "20.000000" || !v.Available.IsZero() {
		t.Fatal("reservation consumed physical stock", v)
	}
	reduced, err := inventory.Save(ctx, worker.Account, fullID, planInput(1, m.Requirement{MaterialID: steel.ID, Quantity: qty("4")}))
	if err != nil || reduced.Lines[0].Reserved.String() != "4.000000" {
		t.Fatal("reservation release failed", err)
	}
	reserve := m.ReserveInput{IdempotencyKey: uuid.NewString(), ExpectedVersion: 1}
	topped, err := inventory.Reserve(ctx, worker.Account, partialID, reserve)
	if err != nil || !topped.Ready || topped.Lines[0].Reserved.String() != "15.000000" {
		t.Fatal("partial top-up failed", err)
	}
	// Exact retry returns the same snapshot and cannot reserve more stock.
	replay, err := inventory.Reserve(ctx, worker.Account, partialID, reserve)
	if err != nil || replay.Version != topped.Version {
		t.Fatal("reservation retry changed result", err)
	}
	material := current(steel.ID)
	correction := m.StockInput{IdempotencyKey: uuid.NewString(), Mode: "correction", Quantity: qty("18"), Reason: "Physical count", ExpectedVersion: material.Version}
	if _, err = inventory.Stock(ctx, admin, steel.ID, correction); !errors.Is(err, m.ErrConflict) {
		t.Fatal("stock correction undercut reservations", err)
	}
	change(steel.ID, "correction", "19", "")
	if current(steel.ID).AverageCost.String() != "20.000000" {
		t.Fatal("count correction repriced stock")
	}
	change(steel.ID, "addition", "5", "10")
	if current(steel.ID).AverageCost.String() != "17.916667" {
		t.Fatal("fractional weighted average mismatch")
	}
	cleared, err := inventory.Save(ctx, worker.Account, fullID, m.PlanInput{IdempotencyKey: uuid.NewString(), ExpectedVersion: reduced.Version, Lines: []m.Requirement{}})
	if err != nil || len(cleared.Lines) != 0 {
		t.Fatal("explicit clear failed", err)
	}
	if current(steel.ID).Reserved.String() != "15.000000" {
		t.Fatal("released job still reserved")
	}
	expanded, err := inventory.Save(ctx, worker.Account, partialID, planInput(topped.Version, m.Requirement{MaterialID: steel.ID, Quantity: qty("30")}))
	if err != nil || expanded.Lines[0].Reserved.String() != "24.000000" || expanded.Lines[0].Shortage.String() != "6.000000" {
		t.Fatal("expanded plan shortage incorrect", err)
	}
	// Reordered multi-material requests lock rows in deterministic UUID order.
	job3, job4 := job(), job()
	results = make(chan error, 2)
	wg.Add(2)
	raceCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	go func() {
		defer wg.Done()
		_, e := inventory.Save(raceCtx, worker.Account, job3, planInput(0, m.Requirement{MaterialID: steel.ID, Quantity: qty("2")}, m.Requirement{MaterialID: rubber.ID, Quantity: qty("8")}))
		results <- e
	}()
	go func() {
		defer wg.Done()
		_, e := inventory.Save(raceCtx, worker.Account, job4, planInput(0, m.Requirement{MaterialID: rubber.ID, Quantity: qty("8")}, m.Requirement{MaterialID: steel.ID, Quantity: qty("2")}))
		results <- e
	}()
	wg.Wait()
	cancel()
	close(results)
	for e := range results {
		if e != nil {
			t.Fatal("multi-material deadlock/stock conflict", e)
		}
	}
	if current(rubber.ID).Reserved.String() != "10.500000" {
		t.Fatal("fractional stock over-reserved")
	}
	if _, err = inventory.Save(ctx, worker.Account, job3, m.PlanInput{IdempotencyKey: uuid.NewString(), ExpectedVersion: 0, Lines: []m.Requirement{}}); !errors.Is(err, m.ErrConflict) {
		t.Fatal("stale plan overwrote reservations", err)
	}
	// Force plan-command audit insertion failure after physical reservation updates.
	change(steel.ID, "addition", "3", "17.916667")
	job5 := job()
	beforeSteel, beforeRubber := current(steel.ID), current(rubber.ID)
	if _, err = db.pool.Exec(ctx, fmt.Sprintf(`ALTER TABLE job_material_command ADD CONSTRAINT uc5_plan_failure CHECK(job_id<>'%s'::uuid)`, job5)); err != nil {
		t.Fatal(err)
	}
	_, failed := inventory.Save(ctx, worker.Account, job5, planInput(0, m.Requirement{MaterialID: steel.ID, Quantity: qty("2")}, m.Requirement{MaterialID: rubber.ID, Quantity: qty("2")}))
	if _, err = db.pool.Exec(ctx, `ALTER TABLE job_material_command DROP CONSTRAINT uc5_plan_failure`); err != nil {
		t.Fatal(err)
	}
	if failed == nil {
		t.Fatal("forced plan audit failure ignored")
	}
	untouched, _ := inventory.Get(ctx, worker.Account, job5)
	if untouched.Version != 0 || len(untouched.Lines) != 0 || current(steel.ID).Reserved != beforeSteel.Reserved || current(rubber.ID).Reserved != beforeRubber.Reserved {
		t.Fatal("failed plan partially committed")
	}
	// Stock trigger updates and its ledger CHECK must roll back together.
	material = current(steel.ID)
	failureKey := uuid.NewString()
	if _, err = db.pool.Exec(ctx, fmt.Sprintf(`ALTER TABLE stock_movement ADD CONSTRAINT uc5_stock_failure CHECK(idempotency_key<>'%s'::uuid)`, failureKey)); err != nil {
		t.Fatal(err)
	}
	_, failed = inventory.Stock(ctx, admin, steel.ID, m.StockInput{IdempotencyKey: failureKey, Mode: "addition", Quantity: qty("3"), UnitCost: qty("99"), Reason: "Rollback probe", ExpectedVersion: material.Version})
	if _, err = db.pool.Exec(ctx, `ALTER TABLE stock_movement DROP CONSTRAINT uc5_stock_failure`); err != nil {
		t.Fatal(err)
	}
	if failed == nil || current(steel.ID).OnHand != material.OnHand || current(steel.ID).AverageCost != material.AverageCost {
		t.Fatal("stock/ledger rollback failed")
	}
	// UC8 now guards status writes. Reduce this fixture to its reserved rubber,
	// then start through the real fully-reserved production command, not raw SQL.
	beforeStart, err := inventory.Get(ctx, worker.Account, job3)
	if err != nil {
		t.Fatal(err)
	}
	var reservedRubber m.Decimal
	for _, line := range beforeStart.Lines {
		if line.MaterialID == rubber.ID {
			reservedRubber = line.Reserved
		}
	}
	readyPlan, err := inventory.Save(ctx, worker.Account, job3, planInput(beforeStart.Version, m.Requirement{MaterialID: rubber.ID, Quantity: &reservedRubber}))
	if err != nil || !readyPlan.Ready {
		t.Fatal("ready production fixture", err)
	}
	if _, err = production.NewService(db).Start(ctx, worker.Account, job3, production.StartInput{Command: production.Command{IdempotencyKey: uuid.NewString(), ExpectedVersion: 0}, ExpectedPlanVersion: &readyPlan.Version}); err != nil {
		t.Fatal(err)
	}
	currentPlan, _ := inventory.Get(ctx, worker.Account, job3)
	if _, err = inventory.Save(ctx, worker.Account, job3, m.PlanInput{IdempotencyKey: uuid.NewString(), ExpectedVersion: currentPlan.Version, Lines: []m.Requirement{}}); !errors.Is(err, m.ErrConflict) {
		t.Fatal("in-progress plan edited", err)
	}
	if _, err = inventory.Reserve(ctx, worker.Account, job3, m.ReserveInput{IdempotencyKey: uuid.NewString(), ExpectedVersion: currentPlan.Version}); !errors.Is(err, m.ErrConflict) {
		t.Fatal("in-progress job reserved", err)
	}
	// Inactive catalog entries can be released, never newly allocated/added.
	material = current(rubber.ID)
	inactive := false
	if _, err = inventory.Update(ctx, admin, rubber.ID, m.MaterialUpdate{Active: &inactive, ExpectedVersion: material.Version}); err != nil {
		t.Fatal(err)
	}
	if _, err = inventory.Save(ctx, worker.Account, job5, planInput(0, m.Requirement{MaterialID: rubber.ID, Quantity: qty("1")})); !errors.Is(err, m.ErrConflict) {
		t.Fatal("inactive material planned", err)
	}
	plan4, _ := inventory.Get(ctx, worker.Account, job4)
	if _, err = inventory.Save(ctx, worker.Account, job4, m.PlanInput{IdempotencyKey: uuid.NewString(), ExpectedVersion: plan4.Version, Lines: []m.Requirement{}}); err != nil {
		t.Fatal("inactive reservation could not release", err)
	}
	// Concurrent exact retries on the same job are one plan command/allocation.
	samePlan := planInput(0, m.Requirement{MaterialID: steel.ID, Quantity: qty("1")})
	results = make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() { defer wg.Done(); _, e := inventory.Save(ctx, worker.Account, job5, samePlan); results <- e }()
	}
	wg.Wait()
	close(results)
	for e := range results {
		if e != nil {
			t.Fatal("concurrent identical plan replay failed", e)
		}
	}
	once, _ := inventory.Get(ctx, worker.Account, job5)
	if once.Version != 1 || len(once.Lines) != 1 || once.Lines[0].Reserved.String() != "1.000000" {
		t.Fatal("same-key retry duplicated job reservation")
	}
	// DB guards and audit immutability, plus aggregate/cache reconciliation.
	if _, err = db.pool.Exec(ctx, `UPDATE material SET qty_on_hand=qty_on_hand+1 WHERE id=$1`, steel.ID); err == nil {
		t.Fatal("bare physical balance write allowed")
	}
	if _, err = db.pool.Exec(ctx, `UPDATE stock_movement SET reason='changed' WHERE material_id=$1`, steel.ID); err == nil {
		t.Fatal("movement audit was mutable")
	}
	if err = db.pool.QueryRow(ctx, `SELECT count(*) FROM material m WHERE m.qty_reserved<>(SELECT COALESCE(sum(qty_reserved),0) FROM material_reservation r WHERE r.material_id=m.id)`).Scan(&count); err != nil || count != 0 {
		t.Fatal("reserved cache drifted", err)
	}
	movements, err := inventory.Movements(ctx, admin, steel.ID, 100, 0)
	if err != nil || len(movements) != 5 {
		t.Fatal("stock replay or failed adjustment created extra movement", len(movements), err)
	}
	down, e := os.ReadFile("../../../migrations/000008_inventory.down.sql")
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
		t.Fatal("populated inventory rollback allowed")
	}
	single, e := Open(ctx, dsn, 1)
	if e != nil {
		t.Fatal(e)
	}
	readCtx, cancelRead := context.WithTimeout(ctx, 2*time.Second)
	_, e = m.NewService(single).Get(readCtx, worker.Account, job1)
	cancelRead()
	single.Close()
	if e != nil {
		t.Fatal("one-connection plan read deadlocked", e)
	}
	// Real HTTP sessions, permissions, strict bodies and search/plan read paths.
	handler := auth.NewHandler(authentication)
	router := chi.NewRouter()
	handler.Routes(router)
	m.NewHandler(inventory).Routes(router, handler.Authenticate)
	adminLogin, _ := authentication.Login(ctx, admin.Username, password)
	workerLogin, _ := authentication.Login(ctx, worker.Username, password)
	customerLogin, _ := authentication.Login(ctx, customer.Username, password)
	call := func(method, path, token string, body any, want int) {
		t.Helper()
		data, _ := json.Marshal(body)
		r := httptest.NewRequest(method, path, bytes.NewReader(data))
		r.Header.Set("Content-Type", "application/json")
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		if w.Code != want {
			t.Fatalf("%s %s got %d want %d", method, path, w.Code, want)
		}
	}
	call("GET", "/materials", "", nil, 401)
	call("GET", "/materials", customerLogin.AccessToken, nil, 403)
	call("GET", "/materials?search=S45C", workerLogin.AccessToken, nil, 200)
	call("POST", "/units", workerLogin.AccessToken, m.UnitInput{Name: "bar"}, 403)
	call("POST", "/materials/"+steel.ID+"/stock", workerLogin.AccessToken, opening, 403)
	call("PUT", "/jobs/"+job5+"/materials", adminLogin.AccessToken, m.PlanInput{IdempotencyKey: uuid.NewString(), Lines: []m.Requirement{}}, 403)
	call("PATCH", "/materials/"+steel.ID, adminLogin.AccessToken, map[string]any{"unit_id": unit.ID, "expected_version": current(steel.ID).Version}, 400)
	call("GET", "/jobs/"+job1+"/materials", workerLogin.AccessToken, nil, 200)
	if _, err = db.pool.Exec(ctx, `UPDATE account SET is_active=false WHERE id=$1`, worker.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = inventory.Reserve(ctx, worker.Account, job1, m.ReserveInput{IdempotencyKey: uuid.NewString(), ExpectedVersion: cleared.Version}); !errors.Is(err, req.ErrForbidden) {
		t.Fatal("stale inactive worker reserved", err)
	}
}
