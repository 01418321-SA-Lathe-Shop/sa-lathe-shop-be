package receiving

import (
	"context"
	"errors"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/auth"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/file"
	req "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/request"
	"testing"
)

func TestEvidenceAdminOnly(t *testing.T) {
	s := NewEvidenceService(nil, nil)
	ctx := context.Background()
	for _, role := range []string{auth.RoleWorker, auth.RoleCustomer} {
		a := auth.Account{Role: role, Active: true}
		if _, e := s.Start(ctx, a, "", file.Input{}); !errors.Is(e, req.ErrForbidden) {
			t.Fatal("upload permitted")
		}
		if _, e := s.Complete(ctx, a, ""); !errors.Is(e, req.ErrForbidden) {
			t.Fatal("completion permitted")
		}
		if _, e := s.Download(ctx, a, ""); !errors.Is(e, req.ErrForbidden) {
			t.Fatal("download permitted")
		}
	}
}
