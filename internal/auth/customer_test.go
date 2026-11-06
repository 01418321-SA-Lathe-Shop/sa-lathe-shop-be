package auth

import (
	"context"
	"errors"
	"testing"
)

type customerFake struct {
	calls      int
	input      CustomerRegistration
	invitation Invitation
	err        error
}

func (f *customerFake) RegisterCustomer(_ context.Context, in CustomerRegistration) (Account, error) {
	f.calls++
	f.input = in
	return in.Account, f.err
}
func (f *customerFake) CustomerProfiles(context.Context, string) ([]CustomerProfile, error) {
	f.calls++
	return nil, f.err
}
func (f *customerFake) SaveInvitation(_ context.Context, in Invitation) error {
	f.calls++
	f.invitation = in
	return f.err
}
func (f *customerFake) AcceptInvitation(context.Context, string, []byte) error {
	f.calls++
	return f.err
}

func (f *customerFake) CustomerAddresses(context.Context, string, string) ([]CustomerAddress, error) {
	f.calls++
	return nil, f.err
}
func (f *customerFake) AddCustomerAddress(_ context.Context, _, _ string, in CustomerAddress) (CustomerAddress, error) {
	f.calls++
	return in, f.err
}

type customerHasher struct{}

func (customerHasher) Hash(context.Context, string) (string, error)         { return "private-hash", nil }
func (customerHasher) Verify(context.Context, string, string) (bool, error) { return true, nil }
func TestCustomerRegistration(t *testing.T) {
	repo := &customerFake{}
	s := NewCustomerService(repo, customerHasher{})
	a, err := s.Register(context.Background(), Registration{Username: " User.Name ", Password: "a-long-test-password", Name: " ลูกค้า ", CustomerType: "company", CompanyName: " Example Company ", ContactName: " ฝ่ายรับสินค้า ", Address: " 1 ถนนสุขุมวิท\nกรุงเทพฯ "})
	if err != nil || a.Role != RoleCustomer || a.PasswordHash != "" || repo.input.Account.PasswordHash == "" || repo.input.CompanyName != "Example Company" || repo.input.ContactName != "ฝ่ายรับสินค้า" || a.Username != "user.name" || repo.input.Address != "1 ถนนสุขุมวิท\nกรุงเทพฯ" {
		t.Fatalf("registration failed: %v", err)
	}
}
func TestCustomerRegistrationRejectsAmbiguousMembership(t *testing.T) {
	for _, in := range []Registration{
		{Username: "newuser", Password: "a-long-test-password", Name: "Name", CustomerType: "admin"},
		{Username: "newuser", Password: "a-long-test-password", Name: "Name", CustomerType: "company"},
		{Username: "newuser", Password: "a-long-test-password", Name: "Name", CustomerType: "company", CompanyName: "Company", Invitation: "token"},
		{Username: "newuser", Password: "a-long-test-password", Name: "Name", CustomerType: "individual", CompanyName: "Company"},
		{Username: "newuser", Password: "a-long-test-password", Name: "Name", CustomerType: "company", Invitation: "token", Address: "1 Road"},
		{Username: "newuser", Password: "a-long-test-password", Name: "Name", CustomerType: "individual", Address: "bad\x00"},
	} {
		repo := &customerFake{}
		_, err := NewCustomerService(repo, customerHasher{}).Register(context.Background(), in)
		if !errors.Is(err, ErrInvalidInput) || repo.calls != 0 {
			t.Fatalf("accepted ambiguous account: %v", err)
		}
	}
}
func TestInvitationStoresDigestOnly(t *testing.T) {
	repo := &customerFake{}
	s := NewCustomerService(repo, customerHasher{})
	token, err := s.Invite(context.Background(), Account{ID: "57039d8b-c58e-4ee1-8943-539dc6203626", Active: true, Role: RoleCustomer}, "57039d8b-c58e-4ee1-8943-539dc6203626")
	if err != nil || !validToken(token) || len(repo.invitation.Hash) != 32 || string(repo.invitation.Hash) == token {
		t.Fatal("unsafe invitation")
	}
	_, err = s.Invite(context.Background(), Account{Active: true, Role: RoleWorker}, "57039d8b-c58e-4ee1-8943-539dc6203626")
	if !errors.Is(err, ErrForbidden) {
		t.Fatal(err)
	}
}
func TestAddAddressValidatesInput(t *testing.T) {
	id := "57039d8b-c58e-4ee1-8943-539dc6203626"
	customer := Account{ID: id, Active: true, Role: RoleCustomer}
	repo := &customerFake{}
	s := NewCustomerService(repo, customerHasher{})
	for _, address := range []string{"   ", string(make([]rune, 501))} {
		if _, err := s.AddAddress(context.Background(), customer, id, address); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("accepted address %q: %v", address, err)
		}
	}
	if _, err := s.AddAddress(context.Background(), Account{ID: id, Active: true, Role: RoleWorker}, id, "1 Road"); !errors.Is(err, ErrForbidden) {
		t.Fatal(err)
	}
	if a, err := s.AddStructuredAddress(context.Background(), customer, id, CustomerAddress{ContactName: " ผู้รับสินค้า ", Address: " 1 Road "}); err != nil || a.Address != "1 Road" || a.ContactName != "ผู้รับสินค้า" || repo.calls != 1 {
		t.Fatalf("address not saved: %v", err)
	}
}
