package production

import (
	"context"
	"errors"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/auth"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/file"
	req "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/request"
	"github.com/google/uuid"
	"testing"
)

func TestPhotoPermissionsAndTypes(t *testing.T) {
	s := NewPhotoService(nil, nil)
	ctx := context.Background()
	a := auth.Account{Active: true, Role: auth.RoleAdmin}
	if _, e := s.Start(ctx, a, uuid.NewString(), file.Input{}); !errors.Is(e, req.ErrForbidden) {
		t.Fatal("Admin photo mutation")
	}
	a.Role = auth.RoleCustomer
	if _, e := s.Download(ctx, a, uuid.NewString()); !errors.Is(e, req.ErrForbidden) {
		t.Fatal("customer photo read")
	}
	a.Role = auth.RoleWorker
	if _, e := s.Start(ctx, a, uuid.NewString(), file.Input{Name: "doc.txt", ContentType: "text/plain", Size: 1}); !errors.Is(e, req.ErrInvalidInput) {
		t.Fatal("non-photo accepted")
	}
}
