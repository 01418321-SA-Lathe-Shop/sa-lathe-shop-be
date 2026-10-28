// Explicit operator-only local/test command. Never imported by api or worker.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/auth"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/demoseed"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/repository/postgres"
)

func main() {
	if e := run(); e != nil {
		message := "Demo seed operation failed; no credentials or database details are logged."
		for _, safe := range []error{demoseed.ErrGuard, demoseed.ErrConflict, demoseed.ErrDependency, demoseed.ErrVerification, auth.ErrInvalidInput, auth.ErrAdminExists, auth.ErrConflict} {
			if errors.Is(e, safe) {
				message = safe.Error()
				break
			}
		}
		fmt.Fprintln(os.Stderr, message)
		os.Exit(1)
	}
}
func run() error {
	if len(os.Args) < 2 {
		return fmt.Errorf("usage: seed-demo plan|apply|status --profile accounts")
	}
	action := os.Args[1]
	flags := flag.NewFlagSet("seed-demo", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	profile := flags.String("profile", "accounts", "Only accounts is supported; no workflow fixtures")
	execute := flags.Bool("execute", false, "Explicitly execute cleanup or retirement")
	dryRun := flags.Bool("dry-run", false, "Simulate cleanup and roll back all changes")
	apiBase := flags.String("api-base", os.Getenv("SEED_API_BASE_URL"), "Loopback API origin for real verification")
	if e := flags.Parse(os.Args[2:]); e != nil {
		return e
	}
	if *profile != "accounts" || flags.NArg() != 0 || (*execute && *dryRun) || (*execute && action != "cleanup" && action != "retire") || (*dryRun && action != "cleanup") || (action != "plan" && action != "apply" && action != "status" && action != "cleanup" && action != "retire" && action != "verify") {
		return demoseed.ErrGuard
	}
	cfg, e := demoseed.LoadConfig(os.Getenv)
	if e != nil {
		return e
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	db, e := postgres.Open(ctx, cfg.DatabaseURL, 2)
	if e != nil {
		return e
	}
	defer db.Close()
	service := demoseed.New(postgres.NewDemoSeedRepository(db), auth.NewArgonHasher(), cfg)
	var status demoseed.Status
	switch action {
	case "apply":
		status, e = service.Apply(ctx, os.Getenv)
	case "cleanup":
		status, e = service.Cleanup(ctx, *execute)
	case "retire":
		if !*execute {
			return demoseed.ErrGuard
		}
		status, e = service.Retire(ctx)
	case "verify":
		e = service.VerifyHTTP(ctx, *apiBase, os.Getenv)
		if e == nil {
			status.State = "real_api_verified"
		}
	default:
		status, e = service.Status(ctx)
	}
	if e != nil {
		return e
	}
	return json.NewEncoder(os.Stdout).Encode(struct {
		Action   string                 `json:"action"`
		Status   demoseed.Status        `json:"status"`
		Accounts []demoseed.AccountSpec `json:"manifest"`
	}{action, status, demoseed.Accounts()})
}
