package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/auth"
	f "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/delivery"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/health"
	m "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/material"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/platform/httpserver"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/platform/objectstore"
	proc "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/procurement"
	p "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/production"
	req "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/request"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type performanceTrace struct{ count, nanos atomic.Int64 }
type performanceTraceStart struct{}

func (s *performanceTrace) TraceQueryStart(ctx context.Context, _ *pgx.Conn, _ pgx.TraceQueryStartData) context.Context {
	s.count.Add(1)
	return context.WithValue(ctx, performanceTraceStart{}, time.Now())
}
func (s *performanceTrace) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, _ pgx.TraceQueryEndData) {
	if start, ok := ctx.Value(performanceTraceStart{}).(time.Time); ok {
		s.nanos.Add(time.Since(start).Nanoseconds())
	}
}
func (s *performanceTrace) TraceBatchStart(ctx context.Context, _ *pgx.Conn, _ pgx.TraceBatchStartData) context.Context {
	return ctx
}
func (s *performanceTrace) TraceBatchQuery(context.Context, *pgx.Conn, pgx.TraceBatchQueryData) {
	s.count.Add(1)
}
func (s *performanceTrace) TraceBatchEnd(context.Context, *pgx.Conn, pgx.TraceBatchEndData) {}

type performanceSample struct {
	ms, headers float64
	bytes       int64
	status      int
	failed      bool
}
type performanceResult struct {
	Name                  string
	Repeat, Requests      int
	Concurrency, Failures int
	P50Ms, P95Ms, P99Ms   float64
	HeadersP95Ms          float64
	Bytes                 int64
	ElapsedSeconds        float64
	RequestsPerSecond     float64
	SQLStatements         int64
	SQLClientWallMs       float64
	PoolWaitMs            float64
	ProcessHeapAllocBytes uint64
	StatusCounts          map[int]int
}

func performancePercentile(values []float64, percentile float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sort.Float64s(values)
	return values[int(math.Ceil(float64(len(values))*percentile))-1]
}

// Request timing ends only after body drain. Each worker starts the next request
// immediately; think time is used only for the separately labelled user mix.
func performanceLoad(t *testing.T, db *Database, trace *performanceTrace, client *http.Client, base, name string, tokens, paths []string, requests, concurrency int, think time.Duration) performanceResult {
	t.Helper()
	beforeCount, beforeNanos := trace.count.Load(), trace.nanos.Load()
	beforeWait := db.pool.Stat().EmptyAcquireWaitTime()
	results := make(chan performanceSample, requests)
	var next atomic.Int64
	var group sync.WaitGroup
	start := time.Now()
	for worker := 0; worker < concurrency; worker++ {
		group.Add(1)
		go func(worker int) {
			defer group.Done()
			for {
				index := int(next.Add(1) - 1)
				if index >= requests {
					return
				}
				// A virtual user keeps its own identity. Fixed-read tests use one token.
				identity := worker % len(tokens)
				path := paths[identity%len(paths)]
				request, err := http.NewRequest(http.MethodGet, base+path, nil)
				if err != nil {
					results <- performanceSample{failed: true}
					continue
				}
				if tokens[identity] != "" {
					request.Header.Set("Authorization", "Bearer "+tokens[identity])
				}
				timer := time.Now()
				response, err := client.Do(request)
				sample := performanceSample{failed: err != nil, headers: float64(time.Since(timer).Microseconds()) / 1000}
				if err == nil {
					sample.status = response.StatusCode
					sample.bytes, err = io.Copy(io.Discard, response.Body)
					response.Body.Close()
					sample.failed = err != nil || response.StatusCode != http.StatusOK
				}
				sample.ms = float64(time.Since(timer).Microseconds()) / 1000
				results <- sample
				if think != 0 {
					time.Sleep(think)
				}
			}
		}(worker)
	}
	group.Wait()
	close(results)
	result := performanceResult{Name: name, Requests: requests, Concurrency: concurrency, ElapsedSeconds: time.Since(start).Seconds(), StatusCounts: map[int]int{}}
	var full, headers []float64
	for sample := range results {
		result.StatusCounts[sample.status]++
		result.Bytes += sample.bytes
		if sample.failed {
			result.Failures++
		} else {
			full = append(full, sample.ms)
			headers = append(headers, sample.headers)
		}
	}
	result.P50Ms = performancePercentile(full, .5)
	result.P95Ms = performancePercentile(full, .95)
	result.P99Ms = performancePercentile(full, .99)
	result.HeadersP95Ms = performancePercentile(headers, .95)
	result.RequestsPerSecond = float64(requests) / result.ElapsedSeconds
	result.SQLStatements = trace.count.Load() - beforeCount
	result.SQLClientWallMs = float64(trace.nanos.Load()-beforeNanos) / 1e6
	result.PoolWaitMs = float64((db.pool.Stat().EmptyAcquireWaitTime() - beforeWait).Microseconds()) / 1000
	var memory runtime.MemStats
	runtime.ReadMemStats(&memory)
	result.ProcessHeapAllocBytes = memory.HeapAlloc
	if result.Failures != 0 {
		t.Errorf("%s: %d/%d failed requests; no response/token bodies logged", name, result.Failures, requests)
	}
	return result
}

func TestPerformanceBaseline(t *testing.T) {
	dsn := os.Getenv("PERFORMANCE_DATABASE_URL")
	if dsn == "" {
		t.Skip("run scripts/measure-performance.ps1 for fresh synthetic fixtures")
	}
	parsed, err := url.Parse(dsn)
	if err != nil || parsed.Path != "/sa_lathe_shop_performance_test" || os.Getenv("PERFORMANCE_CONFIRM_DISPOSABLE") != "yes" {
		t.Fatal("explicit fresh dedicated performance database required")
	}
	check := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Minute)
	defer cancel()
	db, err := Open(ctx, dsn, 10)
	check(err)
	defer db.Close()
	var count int
	check(db.pool.QueryRow(ctx, `SELECT count(*) FROM account`).Scan(&count))
	if count != 0 {
		t.Fatal("performance DB must be fresh; existing accounts are never overwritten")
	}
	trace := &performanceTrace{}
	poolConfig := db.pool.Config()
	poolConfig.ConnConfig.Tracer = trace
	db.pool.Close()
	db.pool, err = pgxpool.NewWithConfig(ctx, poolConfig)
	check(err)
	password := os.Getenv("TEST_ADMIN_PASSWORD")
	if len(password) < 12 || os.Getenv("TEST_S3_ENDPOINT") == "" {
		t.Fatal("private disposable S3 configuration and test password required")
	}
	authentication, err := auth.NewService(db, auth.NewArgonHasher(), 2*time.Hour)
	check(err)
	store := objectstore.New(objectstore.Config{Endpoint: os.Getenv("TEST_S3_ENDPOINT"), Region: "us-east-1", Bucket: "sa-lathe-shop-private", AccessKey: os.Getenv("S3_ACCESS_KEY"), SecretKey: os.Getenv("S3_SECRET_KEY"), PathStyle: true})
	fixture := seedPerformanceFixture(t, ctx, db, authentication, store, password)
	_, err = db.pool.Exec(ctx, `ANALYZE`)
	check(err)
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	authHandler := auth.NewHandler(authentication)
	router := httpserver.RouterWithRoutes(logger, 30*time.Second, []string{"http://localhost:3000"}, health.Live, health.NewService(db, 3*time.Second).Handler, func(router chi.Router) {
		authHandler.Routes(router)
		req.NewHandler(req.NewService(db)).Routes(router, authHandler.Authenticate)
		m.NewHandler(m.NewService(db)).Routes(router, authHandler.Authenticate)
		proc.NewHandler(proc.NewService(db)).Routes(router, authHandler.Authenticate)
		p.NewHandler(p.NewService(db), p.NewPhotoService(db, store)).Routes(router, authHandler.Authenticate)
		f.NewHandler(f.NewService(db), f.NewEvidenceService(db, store)).Routes(router, authHandler.Authenticate)
	})
	server := httptest.NewServer(router)
	defer server.Close()
	client := &http.Client{Timeout: 35 * time.Second, Transport: &http.Transport{MaxIdleConns: 100, MaxIdleConnsPerHost: 100, MaxConnsPerHost: 100}}
	defer client.CloseIdleConnections()
	var tokens, mixPaths []string
	var loginDurations []float64
	for _, account := range fixture.accounts {
		start := time.Now()
		login, err := authentication.Login(ctx, account.Username, password)
		check(err)
		loginDurations = append(loginDurations, float64(time.Since(start).Microseconds())/1000)
		tokens = append(tokens, login.AccessToken)
		path := "/fulfillment/jobs?limit=25&offset=0"
		if account.Role == auth.RoleWorker {
			path = "/production/jobs?limit=25&offset=0"
		} else if account.Role == auth.RoleAdmin {
			path = "/materials?search=steel&limit=25&offset=0"
		}
		mixPaths = append(mixPaths, path)
	}
	var measurements []performanceResult
	paths := []string{"/health", "/requests?limit=25&offset=0", "/fulfillment/jobs?limit=25&offset=0", "/production/jobs?limit=25&offset=0", "/materials?search=steel&limit=25&offset=0", "/suppliers?search=supplier&limit=25&offset=0"}
	for _, job := range fixture.longJobIDs {
		paths = append(paths, "/production/jobs/"+job)
	}
	for _, path := range paths {
		performanceLoad(t, db, trace, client, server.URL, "warmup", tokens[:1], []string{path}, 10, 1, 0)
		for _, concurrency := range []int{1, 10, 25} {
			for repeat := 1; repeat <= 3; repeat++ {
				result := performanceLoad(t, db, trace, client, server.URL, path, tokens[:1], []string{path}, 100, concurrency, 0)
				result.Repeat = repeat
				measurements = append(measurements, result)
			}
		}
		t.Logf("benchmark measured %s", strings.Split(path, "?")[0])
	}
	for repeat := 1; repeat <= 3; repeat++ {
		result := performanceLoad(t, db, trace, client, server.URL, "100 virtual users; fixed identities; 500ms think time", tokens, mixPaths, 300, 100, 500*time.Millisecond)
		result.Repeat = repeat
		measurements = append(measurements, result)
	}
	// Write measurements are separately labelled direct service calls. They keep
	// real transactions/triggers/valuation and avoid mixing HTTP/read timings.
	production := p.NewService(db)
	for _, lineCount := range []int{1, 10, 100} {
		for repeat := 1; repeat <= 3; repeat++ {
			state, err := production.Get(ctx, fixture.workers[0], fixture.longJobIDs[0])
			check(err)
			quantity, err := m.ParseDecimal("0.001")
			check(err)
			lines := make([]p.UsageLine, lineCount)
			for i := range lines {
				lines[i] = p.UsageLine{MaterialID: fixture.materialIDs[i], Quantity: &quantity}
			}
			beforeCount, beforeNanos := trace.count.Load(), trace.nanos.Load()
			start := time.Now()
			_, err = production.Use(ctx, fixture.workers[0], state.JobID, p.UsageInput{Command: p.Command{IdempotencyKey: uuid.NewString(), ExpectedVersion: state.Version}, Lines: lines, Note: "Synthetic write measurement"})
			check(err)
			elapsed := time.Since(start)
			measurements = append(measurements, performanceResult{Name: fmt.Sprintf("service production usage: %d material lines (not HTTP)", lineCount), Repeat: repeat, Requests: 1, Concurrency: 1, P50Ms: float64(elapsed.Microseconds()) / 1000, P95Ms: float64(elapsed.Microseconds()) / 1000, P99Ms: float64(elapsed.Microseconds()) / 1000, ElapsedSeconds: elapsed.Seconds(), SQLStatements: trace.count.Load() - beforeCount, SQLClientWallMs: float64(trace.nanos.Load()-beforeNanos) / 1e6})
		}
	}
	plans := map[string]json.RawMessage{}
	for _, query := range []struct {
		name, sql string
		args      []any
	}{
		{"production_list", `SELECT ` + productionSummary + ` FROM job j WHERE status<>'cancelled' ORDER BY created_at DESC,id DESC LIMIT 25`, nil},
		{"material_search", `SELECT ` + materialFields + ` FROM material m JOIN unit u ON u.id=m.unit_id WHERE strpos(lower(m.name),lower($1))>0 ORDER BY lower(m.name),m.id LIMIT 25`, []any{"steel"}},
		{"supplier_search", `SELECT ` + supplierFields + ` FROM supplier WHERE strpos(lower(name),lower($1))>0 ORDER BY lower(name),id LIMIT 25`, []any{"supplier"}},
		{"production_history", `SELECT ` + productionDocument + ` FROM job j WHERE j.id=$1`, []any{fixture.longJobIDs[2], true}},
	} {
		tx, err := db.pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
		check(err)
		var plan []byte
		err = tx.QueryRow(ctx, "EXPLAIN (ANALYZE, BUFFERS, SETTINGS, FORMAT JSON) "+query.sql, query.args...).Scan(&plan)
		_ = tx.Rollback(ctx)
		check(err)
		plans[query.name] = plan
	}
	var eventBytes int64
	check(db.pool.QueryRow(ctx, `SELECT COALESCE(sum(pg_column_size(result)),0) FROM production_event`).Scan(&eventBytes))
	historyStorage := []map[string]any{}
	for _, id := range fixture.longJobIDs {
		var entries, events, bytes int64
		check(db.pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM job_time_entry WHERE job_id=$1),count(*),COALESCE(sum(pg_column_size(result)),0) FROM production_event WHERE job_id=$1`, id).Scan(&entries, &events, &bytes))
		historyStorage = append(historyStorage, map[string]any{"time_entries": entries, "events": events, "stored_result_bytes": bytes})
	}
	report := map[string]any{
		"go": runtime.Version(), "os": runtime.GOOS, "arch": runtime.GOARCH, "gomaxprocs": runtime.GOMAXPROCS(0),
		"accounts": 100, "jobs": 1000, "materials": 1000, "suppliers": 100, "history_time_entries": []int{10, 100, 1000},
		"seed_seconds": fixture.seedElapsed.Seconds(), "pool_max": 10, "production_result_storage_bytes": eventBytes,
		"history_storage":      historyStorage,
		"service_login_p95_ms": performancePercentile(loginDurations, .95), "measurements": measurements, "plans": plans,
		"limitations": "Loopback real HTTP handlers/middleware/auth/PostgreSQL, not deployed API/network. Fixture private S3 is real. Logging discarded. No external LINE/background worker load. SQL wall time includes client row consumption; batched-query timing is not included. Percentiles are successful full responses only. Login and usage-write metrics are service calls, not HTTP throughput. Heap snapshots cover the combined test server/client process, not peak API-only RSS. Not a production SLA.",
	}
	data, err := json.Marshal(report)
	check(err)
	t.Logf("PERFORMANCE_BASELINE_JSON=%s", data)
}
