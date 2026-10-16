package auth

import (
	"context"
	"errors"
	"testing"
)

func TestContactValidation(t *testing.T) {
	for _, value := range []string{"", " ", "worker", "a@", "Name <a@example.com>", "a@example.com,b@example.com", "a@example.com\r\nBcc: b@example.com"} {
		if _, err := normalizeEmail(value); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("invalid email accepted: %q", value)
		}
	}
	if got, err := normalizeEmail(" Worker.Name+shop@example.com "); err != nil || got != "Worker.Name+shop@example.com" {
		t.Fatal(got, err)
	}
	for _, value := range []string{"", " ", "123", "1234567890123456", "+66abc12345678", "66+812345678", "0812345678 ext 1", "++66812345678", "１２３４５６７８"} {
		if _, err := normalizePhone(value); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("invalid phone accepted: %q", value)
		}
	}
	for raw, want := range map[string]string{" +66 (81) 234-5678 ": "+66812345678", "081-234-5678": "0812345678", "+1 202 555 0123": "+12025550123"} {
		if got, err := normalizePhone(raw); err != nil || got != want {
			t.Fatal(raw, got, err)
		}
	}
}

func TestRequiredWorkerContactsAndPartialUpdates(t *testing.T) {
	s, _, admin := setupService(t)
	ctx := context.Background()
	in := WorkerInput{Username: "worker.one", Password: "Worker-password-1234", Name: "Worker", EmployeeNo: "EMP-1", Email: "worker@example.com", Phone: "+66812345678"}
	for _, field := range []string{"email", "phone"} {
		bad := in
		if field == "email" {
			bad.Email = ""
		} else {
			bad.Phone = ""
		}
		if _, err := s.CreateWorker(ctx, admin, bad); !errors.Is(err, ErrInvalidInput) {
			t.Fatal("missing", field, err)
		}
	}
	w, err := s.CreateWorker(ctx, admin, in)
	if err != nil {
		t.Fatal(err)
	}
	newEmail := "other@example.com"
	w, err = s.UpdateWorker(ctx, admin, w.ID, WorkerUpdate{Email: &newEmail})
	if err != nil || w.Email == nil || *w.Email != newEmail || w.Phone == nil || *w.Phone != in.Phone {
		t.Fatal("partial contact update failed", err)
	}
	blank := " "
	for _, update := range []WorkerUpdate{{Email: &blank}, {Phone: &blank}} {
		if _, err := s.UpdateWorker(ctx, admin, w.ID, update); !errors.Is(err, ErrInvalidInput) {
			t.Fatal("blank contact accepted", err)
		}
	}
	in.Username, in.EmployeeNo = "worker.two", "EMP-2"
	in.Email = newEmail
	if _, err = s.CreateWorker(ctx, admin, in); err != nil {
		t.Fatal("non-unique contacts rejected", err)
	}
	if _, err = s.UpdateWorker(ctx, Account{Role: RoleWorker, Active: true}, w.ID, WorkerUpdate{Email: &newEmail}); !errors.Is(err, ErrForbidden) {
		t.Fatal("worker edited contacts", err)
	}
}
