package document

import (
	"context"
	"errors"
	"testing"

	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/auth"
)

const testID = "57039d8b-c58e-4ee1-8943-539dc6203626"

type fakeRepo struct {
	calls int
	shop  ShopInput
	bill  BillingInput
	po    POInput
}

func (f *fakeRepo) GetShopProfile(context.Context) (ShopProfile, error) {
	f.calls++
	return ShopProfile{}, nil
}
func (f *fakeRepo) SaveShopProfile(_ context.Context, _ string, in ShopInput) (ShopProfile, error) {
	f.calls++
	f.shop = in
	return in.ShopProfile, nil
}
func (f *fakeRepo) GetBilling(context.Context, string, string) (Billing, error) {
	f.calls++
	return Billing{}, nil
}
func (f *fakeRepo) SaveBilling(_ context.Context, _, _ string, in BillingInput) (Billing, error) {
	f.calls++
	f.bill = in
	return Billing{}, nil
}
func (f *fakeRepo) GetCustomerPO(context.Context, string, string) (CustomerPO, error) {
	f.calls++
	return CustomerPO{}, nil
}
func (f *fakeRepo) SaveCustomerPO(_ context.Context, _, _ string, in POInput) (CustomerPO, error) {
	f.calls++
	f.po = in
	return CustomerPO{}, nil
}
func account(role string) auth.Account { return auth.Account{ID: testID, Active: true, Role: role} }

func TestShopProfileOnlyAdminWritesAndInputIsCleaned(t *testing.T) {
	repo := &fakeRepo{}
	s := NewService(repo)
	in := ShopInput{ShopProfile: ShopProfile{Name: "  ร้านกลึง  ", Address: "บรรทัด 1\nบรรทัด 2", Email: "shop@example.com"}, ExpectedLockVersion: 1}
	for _, role := range []string{auth.RoleWorker, auth.RoleCustomer} {
		if _, err := s.SaveShopProfile(context.Background(), account(role), in); !errors.Is(err, ErrForbidden) {
			t.Fatalf("%s save = %v", role, err)
		}
	}
	if _, err := s.SaveShopProfile(context.Background(), account(auth.RoleAdmin), in); err != nil || repo.shop.Name != "ร้านกลึง" {
		t.Fatalf("admin save = %v, name %q", err, repo.shop.Name)
	}
	bad := []ShopInput{
		{ShopProfile: ShopProfile{Email: "not an email"}, ExpectedLockVersion: 1},
		{ShopProfile: ShopProfile{Name: "a\nb"}, ExpectedLockVersion: 1},
		{ShopProfile: ShopProfile{Name: "ok"}, ExpectedLockVersion: -1},
	}
	for _, in := range bad {
		if _, err := s.SaveShopProfile(context.Background(), account(auth.RoleAdmin), in); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("bad input %+v = %v", in, err)
		}
	}
	if _, err := s.ShopProfile(context.Background(), auth.Account{ID: testID, Role: auth.RoleWorker}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("inactive read = %v", err)
	}
}

func TestBillingWrittenByCustomersOnly(t *testing.T) {
	repo := &fakeRepo{}
	s := NewService(repo)
	in := BillingInput{Address: " 1 ถนน ", TaxID: "0105555555555", ExpectedLockVersion: 1}
	if _, err := s.SaveBilling(context.Background(), account(auth.RoleAdmin), testID, in); !errors.Is(err, ErrForbidden) {
		t.Fatalf("admin billing save = %v", err)
	}
	if _, err := s.SaveBilling(context.Background(), account(auth.RoleCustomer), "bad", in); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("bad id = %v", err)
	}
	if _, err := s.SaveBilling(context.Background(), account(auth.RoleCustomer), testID, in); err != nil || repo.bill.Address != "1 ถนน" {
		t.Fatalf("customer save = %v, %q", err, repo.bill.Address)
	}
	if _, err := s.Billing(context.Background(), account(auth.RoleWorker), testID); err != nil {
		t.Fatalf("staff read = %v", err)
	}
}

func TestCustomerPOValidation(t *testing.T) {
	repo := &fakeRepo{}
	s := NewService(repo)
	ok := POInput{PONo: " PO-1 ", IssuedOn: "2026-11-03"}
	if _, err := s.SaveCustomerPO(context.Background(), account(auth.RoleWorker), testID, ok); !errors.Is(err, ErrForbidden) {
		t.Fatalf("worker PO = %v", err)
	}
	for _, in := range []POInput{{PONo: "", IssuedOn: "2026-11-03"}, {PONo: "PO", IssuedOn: "03/11/2026"}, {PONo: "PO", IssuedOn: "2026-11-03", ExpectedLockVersion: -1}} {
		if _, err := s.SaveCustomerPO(context.Background(), account(auth.RoleCustomer), testID, in); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("bad PO %+v = %v", in, err)
		}
	}
	if _, err := s.SaveCustomerPO(context.Background(), account(auth.RoleCustomer), testID, ok); err != nil || repo.po.PONo != "PO-1" {
		t.Fatalf("customer PO = %v, %q", err, repo.po.PONo)
	}
	if repo.calls != 1 {
		t.Fatalf("repo calls = %d, want only the valid save", repo.calls)
	}
}
