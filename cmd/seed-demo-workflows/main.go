// Operator-only fixture command. Never composed into the API/worker startup.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	wf "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/demoworkflow"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/repository/postgres"
)

func main() {
	if e := run(); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
func run() error {
	if len(os.Args) != 2 || (os.Args[1] != "plan" && os.Args[1] != "apply" && os.Args[1] != "verify") {
		return fmt.Errorf("usage: seed-demo-workflows plan|apply|verify")
	}
	cfg, e := wf.LoadConfig(os.Getenv)
	if e != nil {
		return e
	}
	if os.Args[1] == "plan" {
		fmt.Println("workflows-v2: adds 11 realistic requests, 20 private item-specific PDF documents, quotations/revision, PO/deposit evidence, 6 jobs, 13 Thai stockist materials with per-material unit conversions, 3 supplier orders, receipts/claim, 2 invoices and pending/settled customer payments; completed reruns preserve tester edits")
		return nil
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	db, e := postgres.Open(ctx, cfg.Seed.DatabaseURL, 4)
	if e != nil {
		return fmt.Errorf("demo connection failed; private details withheld")
	}
	defer db.Close()
	report, e := wf.Apply(ctx, postgres.NewWorkflowRepository(db), cfg, os.Args[1] == "verify", func(key string) { fmt.Println("Completed demo step:", key) })
	if e != nil {
		var operation *wf.OperationError
		if errors.As(e, &operation) {
			return operation
		}
		// Domain/HTTP errors carry only fixed keys/statuses. SQL and connection
		// diagnostics may include credentials, so never forward arbitrary errors.
		return fmt.Errorf("workflow seed failed safely; inspect journal for pending step; no reset or automatic adoption (private diagnostics withheld)")
	}
	return json.NewEncoder(os.Stdout).Encode(report)
}
