package delivery

import (
	"context"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/auth"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/file"
	"testing"
)

func TestEvidenceRoleBoundary(t *testing.T) {
	s := NewEvidenceService(nil, nil)
	a := auth.Account{Role: "worker", Active: true}
	if _, e := s.Start(context.Background(), a, "", file.Input{}); e == nil {
		t.Fatal("worker evidence upload")
	}
	if _, e := s.Download(context.Background(), a, ""); e == nil {
		t.Fatal("worker private evidence")
	}
	if _, e := s.Complete(context.Background(), a, ""); e == nil {
		t.Fatal("worker completion")
	}
}
