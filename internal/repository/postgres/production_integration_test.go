package postgres

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/auth"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/file"
	m "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/material"
	pay "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/payment"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/platform/objectstore"
	p "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/production"
	q "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/quotation"
	req "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/request"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestProductionIntegration(t *testing.T) {
	dsn := os.Getenv("PHASE11_DATABASE_URL")
	if dsn == "" {
		t.Skip("set dedicated PHASE11_DATABASE_URL")
	}
	u, e := url.Parse(dsn)
	if e != nil || u.Path != "/sa_lathe_shop_phase11_test" {
		t.Fatal("dedicated fresh production DB required")
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
		t.Fatal("fresh DB required; existing data unchanged")
	}
	hasher := auth.NewArgonHasher()
	authentication, e := auth.NewService(db, hasher, time.Hour)
	check(e)
	admin, e := authentication.BootstrapAdmin(ctx, "uc8_admin", "Admin", password)
	check(e)
	worker, e := authentication.CreateWorker(ctx, admin, auth.WorkerInput{Username: "uc8_worker", Name: "Worker", Password: password, EmployeeNo: "UC8-W", Email: "worker@example.com", Phone: "+66812345678"})
	check(e)
	customers := auth.NewCustomerService(db, hasher)
	customer, e := customers.Register(ctx, auth.Registration{Username: "uc8_customer", Name: "Buyer", Password: password, CustomerType: "company", CompanyName: "UC8 Company"})
	check(e)
	profiles, e := customers.Profiles(ctx, customer)
	check(e)
	requests := req.NewService(db)
	quotes := q.NewService(db)
	decisions := q.NewDecisionService(db)
	payments := pay.NewService(db)
	store := objectstore.New(objectstore.Config{Endpoint: os.Getenv("TEST_S3_ENDPOINT"), Region: "us-east-1", Bucket: "sa-lathe-shop-private", AccessKey: os.Getenv("S3_ACCESS_KEY"), SecretKey: os.Getenv("S3_SECRET_KEY"), PathStyle: true})
	files := file.NewService(db, store, requests)
	price := func(s string) *q.Amount { v, e := q.ParseAmount(s); check(e); return &v }
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
			t.Fatal("private S3 upload failed")
		}
	}
	newJob := func() string {
		t.Helper()
		principal := req.Principal{AccountID: customer.ID, Role: customer.Role, Active: true}
		request, e := requests.Submit(ctx, principal, req.Input{CustomerID: profiles[0].ID, Description: "UC8 mould", Quantity: 1})
		check(e)
		draft, e := quotes.Create(ctx, worker.Account, request.ID, q.DraftInput{MaterialCost: price("0"), LaborCost: price("0"), FinalPrice: price("1000")})
		check(e)
		sent, e := quotes.Send(ctx, worker.Account, draft.ID, draft.LockVersion)
		check(e)
		_, e = decisions.Decide(ctx, customer, sent.ID, q.DecisionInput{Outcome: "accept", ExpectedLockVersion: sent.LockVersion})
		check(e)
		data := []byte("Company PO")
		link, e := files.Start(ctx, principal, request.ID, file.Input{Name: "po.txt", ContentType: "text/plain", Size: int64(len(data))})
		check(e)
		put(link, data)
		_, e = files.Complete(ctx, principal, link.AttachmentID)
		check(e)
		proof, e := payments.Submit(ctx, customer, sent.ID, pay.Submission{IdempotencyKey: uuid.NewString(), AttachmentID: link.AttachmentID, Kind: "po"})
		check(e)
		_, e = payments.Review(ctx, admin, proof.ID, pay.Review{Outcome: "approved", ExpectedVersion: 1})
		check(e)
		fulfilled, e := payments.Get(ctx, customer, sent.ID)
		check(e)
		if fulfilled.Job == nil {
			t.Fatal("job not opened")
		}
		return fulfilled.Job.ID
	}
	inventory := m.NewService(db)
	service := p.NewService(db)
	photos := p.NewPhotoService(db, store)
	unit, e := inventory.CreateUnit(ctx, admin, m.UnitInput{Name: "kg"})
	check(e)
	steel, e := inventory.Create(ctx, admin, m.MaterialInput{Name: "UC8 steel", UnitID: unit.ID})
	check(e)
	extra, e := inventory.Create(ctx, admin, m.MaterialInput{Name: "UC8 rubber", UnitID: unit.ID})
	check(e)
	for _, v := range []struct{ id, amount, cost string }{{steel.ID, "20", "10"}, {extra.ID, "5", "4"}} {
		_, e = inventory.Stock(ctx, admin, v.id, m.StockInput{IdempotencyKey: uuid.NewString(), ExpectedVersion: 1, Mode: "addition", Quantity: qty(v.amount), UnitCost: qty(v.cost), Reason: "Opening"})
		check(e)
	}
	cmd := func(version int64) p.Command {
		return p.Command{IdempotencyKey: uuid.NewString(), ExpectedVersion: version}
	}
	plan := func(job string, version int64, quantity string) m.Plan {
		t.Helper()
		v, e := inventory.Save(ctx, worker.Account, job, m.PlanInput{IdempotencyKey: uuid.NewString(), ExpectedVersion: version, Lines: []m.Requirement{{MaterialID: steel.ID, Quantity: qty(quantity)}}})
		check(e)
		return v
	}
	start := func(job string, planVersion int64) p.State {
		t.Helper()
		v, e := service.Start(ctx, worker.Account, job, p.StartInput{Command: cmd(0), ExpectedPlanVersion: &planVersion})
		check(e)
		return v
	}
	use := func(job string, version int64, material, quantity string) (p.State, error) {
		return service.Use(ctx, worker.Account, job, p.UsageInput{Command: cmd(version), Lines: []p.UsageLine{{MaterialID: material, Quantity: qty(quantity)}}})
	}
	stock := func() m.Material {
		t.Helper()
		v, e := inventory.Search(ctx, admin, "UC8 steel", 100, 0)
		check(e)
		if len(v) != 1 {
			t.Fatal("stock missing")
		}
		return v[0]
	}
	jobA, jobB, short := newJob(), newJob(), newJob()
	pa, pb := plan(jobA, 0, "5"), plan(jobB, 0, "10")
	missing := int64(1)
	if _, e = service.Start(ctx, worker.Account, short, p.StartInput{Command: cmd(0), ExpectedPlanVersion: &missing}); !errors.Is(e, p.ErrConflict) {
		t.Fatal("empty plan started", e)
	}
	ps := plan(short, 0, "10")
	if ps.Ready {
		t.Fatal("shortage fixture invalid")
	}
	if _, e = service.Start(ctx, worker.Account, short, p.StartInput{Command: cmd(0), ExpectedPlanVersion: &ps.Version}); !errors.Is(e, p.ErrConflict) {
		t.Fatal("partial plan started", e)
	}
	_, e = inventory.Save(ctx, worker.Account, short, m.PlanInput{IdempotencyKey: uuid.NewString(), ExpectedVersion: ps.Version, Lines: []m.Requirement{}})
	check(e)
	a, b := start(jobA, pa.Version), start(jobB, pb.Version)
	if stock().OnHand.String() != "20.000000" {
		t.Fatal("start consumed stock")
	}
	if _, e = inventory.Save(ctx, worker.Account, jobA, m.PlanInput{IdempotencyKey: uuid.NewString(), ExpectedVersion: pa.Version, Lines: []m.Requirement{}}); !errors.Is(e, m.ErrConflict) {
		t.Fatal("started plan edited", e)
	}
	a, e = use(jobA, a.Version, steel.ID, "7")
	check(e)
	if stock().OnHand.String() != "13.000000" || stock().Reserved.String() != "10.000000" {
		t.Fatal("own reservation/free usage mismatch")
	}
	if _, e = use(jobB, b.Version, steel.ID, "14"); !errors.Is(e, p.ErrConflict) {
		t.Fatal("used another job's reservation", e)
	}
	consumedPlan, e := inventory.Get(ctx, worker.Account, jobA)
	check(e)
	if consumedPlan.Ready || !consumedPlan.Lines[0].Shortage.IsZero() || consumedPlan.Lines[0].Consumed.String() != "5.000000" {
		t.Fatal("plan treats consumed stock as shortage")
	}
	a, e = use(jobA, a.Version, extra.ID, "2")
	check(e)
	a, e = service.Time(ctx, worker.Account, jobA, p.TimeInput{Command: cmd(a.Version), Minutes: 45, Note: "Actual machine work"})
	check(e)
	a, e = service.Complete(ctx, worker.Account, jobA, cmd(a.Version))
	check(e)
	if a.Status != "completed" || a.Finance != nil || a.LaborMinutes != 45 || a.CompletedAt == nil {
		t.Fatal("completion/time/redaction failed")
	}
	adminA, e := service.Labor(ctx, admin, jobA, p.LaborInput{Command: cmd(a.Version), Amount: price("120")})
	check(e)
	if adminA.Finance.MaterialCost.String() != "78.00" || adminA.Finance.TotalCost.String() != "198.00" {
		t.Fatal("actual cost mismatch")
	}
	adminA, e = service.Labor(ctx, admin, jobA, p.LaborInput{Command: cmd(adminA.Version), Amount: price("125")})
	check(e)
	if len(adminA.Events) != 7 || adminA.Finance.TotalCost.String() != "203.00" {
		t.Fatal("labor revision audit failed", len(adminA.Events))
	}
	b, e = use(jobB, b.Version, steel.ID, "2")
	check(e)
	before := stock().OnHand
	b, e = service.Complete(ctx, worker.Account, jobB, cmd(b.Version))
	check(e)
	if stock().OnHand != before || !stock().Reserved.IsZero() || b.Materials[0].Released.String() != "8.000000" {
		t.Fatal("unused reservation release manufactured stock")
	}
	// Concurrent same-key usage creates exactly one issue, even after later completion.
	jobC := newJob()
	pc := plan(jobC, 0, "1")
	c := start(jobC, pc.Version)
	same := p.UsageInput{Command: cmd(c.Version), Lines: []p.UsageLine{{MaterialID: steel.ID, Quantity: qty("2")}}}
	results := make(chan p.State, 2)
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() { defer wg.Done(); v, e := service.Use(ctx, worker.Account, jobC, same); results <- v; errs <- e }()
	}
	wg.Wait()
	close(results)
	close(errs)
	for e := range errs {
		check(e)
	}
	for v := range results {
		c = v
	}
	if len(c.Usage) != 1 || c.Version != 2 {
		t.Fatal("duplicate issue")
	}
	originalUseJSON, e := json.Marshal(c)
	check(e)
	c, e = service.Complete(ctx, worker.Account, jobC, cmd(c.Version))
	check(e)
	replay, e := service.Use(ctx, worker.Account, jobC, same)
	check(e)
	if replay.Status != "in_progress" || replay.Version != 2 {
		t.Fatal("retry did not replay original")
	}
	replayJSON, e := json.Marshal(replay)
	check(e)
	if !bytes.Equal(originalUseJSON, replayJSON) {
		t.Fatal("retry changed original full production response after completion")
	}
	same.Note = "changed"
	if _, e = service.Use(ctx, worker.Account, jobC, same); !errors.Is(e, p.ErrConflict) {
		t.Fatal("retry payload reused", e)
	}
	// Usage and completion at the same version have one winner.
	jobD := newJob()
	pd := plan(jobD, 0, "1")
	d := start(jobD, pd.Version)
	race := make(chan error, 2)
	wg.Add(2)
	go func() { defer wg.Done(); _, e := use(jobD, d.Version, steel.ID, "1"); race <- e }()
	go func() {
		defer wg.Done()
		_, e := service.Complete(ctx, worker.Account, jobD, cmd(d.Version))
		race <- e
	}()
	wg.Wait()
	close(race)
	wins, conflicts := 0, 0
	for e := range race {
		if e == nil {
			wins++
		} else if errors.Is(e, p.ErrConflict) {
			conflicts++
		} else {
			t.Fatal(e)
		}
	}
	if wins != 1 || conflicts != 1 {
		t.Fatal("completion/usage race")
	}
	d, e = service.Get(ctx, worker.Account, jobD)
	check(e)
	if d.Status == "in_progress" {
		_, e = service.Complete(ctx, worker.Account, jobD, cmd(d.Version))
		check(e)
	}
	// Different jobs compete for the same free stock but cannot steal the other's 1.
	jobE, jobF := newJob(), newJob()
	pe, pf := plan(jobE, 0, "1"), plan(jobF, 0, "1")
	stateE, stateF := start(jobE, pe.Version), start(jobF, pf.Version)
	amount, e := stock().OnHand.Subtract(*qty("1"))
	check(e)
	race = make(chan error, 2)
	for _, v := range []p.State{stateE, stateF} {
		wg.Add(1)
		go func(v p.State) {
			defer wg.Done()
			_, e := use(v.JobID, v.Version, steel.ID, amount.String())
			race <- e
		}(v)
	}
	wg.Wait()
	close(race)
	wins, conflicts = 0, 0
	for e := range race {
		if e == nil {
			wins++
		} else if errors.Is(e, p.ErrConflict) {
			conflicts++
		} else {
			t.Fatal(e)
		}
	}
	if wins != 1 || conflicts != 1 || stock().OnHand.String() != "1.000000" || stock().Reserved.String() != "1.000000" {
		t.Fatal("free-stock race overspent")
	}
	for _, job := range []string{jobE, jobF} {
		v, e := service.Get(ctx, worker.Account, job)
		check(e)
		_, e = service.Complete(ctx, worker.Account, job, cmd(v.Version))
		check(e)
	}
	// Force audit failure after reservation and physical issue: rollback everything.
	jobG := newJob()
	pg := plan(jobG, 0, "1")
	g := start(jobG, pg.Version)
	beforeStock := stock()
	_, e = db.pool.Exec(ctx, `CREATE FUNCTION uc8_fail_audit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.action='use' THEN RAISE EXCEPTION 'test failure';END IF;RETURN NEW;END $$;CREATE TRIGGER uc8_fail_audit BEFORE INSERT ON production_event FOR EACH ROW EXECUTE FUNCTION uc8_fail_audit()`)
	check(e)
	if _, e = use(jobG, g.Version, steel.ID, "1"); e == nil {
		t.Fatal("audit failure ignored")
	}
	_, e = db.pool.Exec(ctx, `DROP TRIGGER uc8_fail_audit ON production_event;DROP FUNCTION uc8_fail_audit()`)
	check(e)
	g, e = service.Get(ctx, worker.Account, jobG)
	check(e)
	afterStock := stock()
	if g.Version != 1 || len(g.Usage) != 0 || beforeStock.OnHand != afterStock.OnHand || beforeStock.Reserved != afterStock.Reserved || beforeStock.Version != afterStock.Version {
		t.Fatal("failed issue left stock/reservation/audit")
	}
	var imageData bytes.Buffer
	check(png.Encode(&imageData, image.NewRGBA(image.Rect(0, 0, 1, 1))))
	photo, e := photos.Start(ctx, worker.Account, jobG, file.Input{Name: "job.png", ContentType: "image/png", Size: int64(imageData.Len())})
	check(e)
	put(photo, imageData.Bytes())
	_, e = photos.Complete(ctx, worker.Account, photo.AttachmentID)
	check(e)
	link, e := photos.Download(ctx, admin, photo.AttachmentID)
	check(e)
	response, e := http.Get(link.URL)
	check(e)
	response.Body.Close()
	if response.StatusCode != 200 {
		t.Fatal("private photo download")
	}
	if _, e = photos.Download(ctx, customer, photo.AttachmentID); !errors.Is(e, req.ErrForbidden) {
		t.Fatal("customer saw production photo")
	}
	g, e = service.Complete(ctx, worker.Account, jobG, cmd(g.Version))
	check(e)
	if stock().OnHand.String() != "1.000000" || !stock().Reserved.IsZero() || len(g.Photos) != 1 {
		t.Fatal("photo completion/release mismatch")
	}
	// Both usage rows are written before batched ledger inserts. A ledger failure
	// must roll back both materials, their reservations, usage, versions and event.
	multiJob := newJob()
	multiPlan, e := inventory.Save(ctx, worker.Account, multiJob, m.PlanInput{IdempotencyKey: uuid.NewString(), Lines: []m.Requirement{{MaterialID: steel.ID, Quantity: qty("1")}, {MaterialID: extra.ID, Quantity: qty("1")}}})
	check(e)
	multi := start(multiJob, multiPlan.Version)
	beforeMultiStock := stock()
	_, e = db.pool.Exec(ctx, `CREATE FUNCTION uc8_fail_batch_ledger() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.mode='job_issue' THEN RAISE EXCEPTION 'test batch failure';END IF;RETURN NEW;END $$;CREATE TRIGGER uc8_fail_batch_ledger BEFORE INSERT ON stock_movement FOR EACH ROW EXECUTE FUNCTION uc8_fail_batch_ledger()`)
	check(e)
	_, e = service.Use(ctx, worker.Account, multiJob, p.UsageInput{Command: cmd(multi.Version), Lines: []p.UsageLine{{MaterialID: steel.ID, Quantity: qty("1")}, {MaterialID: extra.ID, Quantity: qty("1")}}})
	if e == nil {
		t.Fatal("batched ledger failure ignored")
	}
	_, e = db.pool.Exec(ctx, `DROP TRIGGER uc8_fail_batch_ledger ON stock_movement;DROP FUNCTION uc8_fail_batch_ledger()`)
	check(e)
	multiAfter, e := service.Get(ctx, worker.Account, multiJob)
	check(e)
	if multiAfter.Version != multi.Version || len(multiAfter.Usage) != 0 || stock().OnHand != beforeMultiStock.OnHand || stock().Reserved != beforeMultiStock.Reserved || stock().Version != beforeMultiStock.Version {
		t.Fatal("batch failure partially committed")
	}
	_, e = service.Complete(ctx, worker.Account, multiJob, cmd(multi.Version))
	check(e)
	// Snapshots are immutable; monetary overflow and populated rollback are rejected.
	if _, e = service.Labor(ctx, admin, jobA, p.LaborInput{Command: cmd(adminA.Version), Amount: price("9999999999999999.99")}); !errors.Is(e, req.ErrInvalidInput) {
		t.Fatal("actual cost overflow", e)
	}
	for _, sql := range []string{`UPDATE job_material_usage SET quantity=0 WHERE job_id=$1`, `UPDATE production_event SET action='labor' WHERE job_id=$1`, `UPDATE job_time_entry SET minutes=1 WHERE job_id=$1`} {
		if _, e = db.pool.Exec(ctx, sql, jobA); e == nil {
			t.Fatal("production audit edited")
		}
	}
	down, e := os.ReadFile("../../../migrations/000011_production.down.sql")
	check(e)
	tx, e := db.pool.Begin(ctx)
	check(e)
	_, e = tx.Exec(ctx, string(down))
	tx.Rollback(ctx)
	if e == nil {
		t.Fatal("populated production rollback allowed")
	}
	check(db.pool.QueryRow(ctx, `SELECT count(*) FROM material m WHERE m.qty_reserved<>(SELECT COALESCE(sum(qty_reserved),0) FROM material_reservation r WHERE r.material_id=m.id)`).Scan(&count))
	if count != 0 {
		t.Fatal("reserved balance drift")
	}
	single, e := Open(ctx, dsn, 1)
	check(e)
	readCtx, stop := context.WithTimeout(ctx, 2*time.Second)
	_, e = p.NewService(single).Get(readCtx, worker.Account, jobA)
	stop()
	single.Close()
	check(e)
	ah := auth.NewHandler(authentication)
	router := chi.NewRouter()
	p.NewHandler(service, photos).Routes(router, ah.Authenticate)
	workerLogin, e := authentication.Login(ctx, worker.Username, password)
	check(e)
	adminLogin, e := authentication.Login(ctx, admin.Username, password)
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
		w := httptest.NewRecorder()
		router.ServeHTTP(w, request)
		if w.Code != want {
			t.Fatalf("%s %s: %d want %d %s", method, path, w.Code, want, w.Body.String())
		}
		return w.Body.String()
	}
	call("GET", "/production/jobs", "", nil, 401)
	call("GET", "/production/jobs", customerLogin.AccessToken, nil, 403)
	data := call("GET", "/production/jobs/"+jobA, workerLogin.AccessToken, nil, 200)
	for _, field := range []string{"finance", "unit_cost", "total_amount", "labor_cost"} {
		if strings.Contains(data, `"`+field+`"`) {
			t.Fatal("Worker cost leak", field)
		}
	}
	call("GET", "/production/jobs/"+jobA, adminLogin.AccessToken, nil, 200)
	call("POST", "/production/jobs/"+jobA+"/complete", adminLogin.AccessToken, nil, 403)
	call("PUT", "/production/jobs/"+jobA+"/labor-cost", workerLogin.AccessToken, nil, 403)
	call("POST", "/production/jobs/"+jobA+"/usage", workerLogin.AccessToken, map[string]any{"unit_cost": "1"}, 400)
	call("POST", "/production/jobs/"+jobG+"/photos/uploads", adminLogin.AccessToken, nil, 403)
	call("GET", "/job-photos/"+photo.AttachmentID+"/download", customerLogin.AccessToken, nil, 403)
	_, e = db.pool.Exec(ctx, `UPDATE account SET is_active=false WHERE id=$1`, worker.ID)
	check(e)
	if _, e = service.Get(ctx, worker.Account, jobA); !errors.Is(e, req.ErrForbidden) {
		t.Fatal("stale worker read", e)
	}
	call("GET", "/production/jobs", workerLogin.AccessToken, nil, 401)
}
