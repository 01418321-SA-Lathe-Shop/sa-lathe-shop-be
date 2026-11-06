package delivery

import (
	"context"
	"errors"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/auth"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/file"
	req "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/request"
	"testing"
)

func TestEvidenceRoleBoundary(t *testing.T) {
	s := NewEvidenceService(nil, nil)
	a := auth.Account{Role: "worker", Active: true}
	if _, e := s.Start(context.Background(), a, "", file.Input{}); e == nil {
		t.Fatal("worker evidence upload")
	}
	// UC9: the Worker reviews transfers, so it may open proof (an invalid ID is still rejected).
	if _, e := s.Download(context.Background(), a, ""); !errors.Is(e, req.ErrInvalidInput) {
		t.Fatal("worker evidence download should pass the role gate", e)
	}
	if _, e := s.Complete(context.Background(), a, ""); e == nil {
		t.Fatal("worker completion")
	}
}
