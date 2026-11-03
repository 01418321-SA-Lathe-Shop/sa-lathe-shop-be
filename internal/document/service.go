package document

import (
	"context"

	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/auth"
)

type Service struct{ repo Repository }

func NewService(repo Repository) *Service { return &Service{repo: repo} }

func signedIn(a auth.Account) bool {
	return a.Active && validID(a.ID) && (a.Role == auth.RoleAdmin || a.Role == auth.RoleWorker || a.Role == auth.RoleCustomer)
}

func (s *Service) ShopProfile(ctx context.Context, a auth.Account) (ShopProfile, error) {
	if !signedIn(a) {
		return ShopProfile{}, ErrForbidden
	}
	return s.repo.GetShopProfile(ctx)
}
func (s *Service) SaveShopProfile(ctx context.Context, a auth.Account, in ShopInput) (ShopProfile, error) {
	if !signedIn(a) || a.Role != auth.RoleAdmin {
		return ShopProfile{}, ErrForbidden
	}
	p := &in.ShopProfile
	err := check(text(&p.Name, 200, false), text(&p.Address, 500, true), text(&p.TaxID, 30, false), text(&p.Phone, 64, false), email(&p.Email),
		text(&p.BankName, 200, false), text(&p.BankAccountName, 200, false), text(&p.BankAccountNo, 64, false))
	if err != nil || in.ExpectedLockVersion < 0 {
		return ShopProfile{}, ErrInvalidInput
	}
	return s.repo.SaveShopProfile(ctx, a.ID, in)
}

// Billing is readable by the customer's own members and by staff; only the owner edits it.
func (s *Service) Billing(ctx context.Context, a auth.Account, customerID string) (Billing, error) {
	if !signedIn(a) {
		return Billing{}, ErrForbidden
	}
	if !validID(customerID) {
		return Billing{}, ErrInvalidInput
	}
	return s.repo.GetBilling(ctx, a.ID, customerID)
}
func (s *Service) SaveBilling(ctx context.Context, a auth.Account, customerID string, in BillingInput) (Billing, error) {
	if !signedIn(a) || a.Role != auth.RoleCustomer {
		return Billing{}, ErrForbidden
	}
	err := check(text(&in.Address, 500, true), text(&in.TaxID, 30, false), text(&in.Phone, 64, false), email(&in.Email))
	if err != nil || !validID(customerID) || in.ExpectedLockVersion < 1 {
		return Billing{}, ErrInvalidInput
	}
	return s.repo.SaveBilling(ctx, a.ID, customerID, in)
}

func (s *Service) CustomerPO(ctx context.Context, a auth.Account, quotationID string) (CustomerPO, error) {
	if !signedIn(a) {
		return CustomerPO{}, ErrForbidden
	}
	if !validID(quotationID) {
		return CustomerPO{}, ErrInvalidInput
	}
	return s.repo.GetCustomerPO(ctx, a.ID, quotationID)
}

// SaveCustomerPO lets the customer write their own PO while the accepted terms need one.
func (s *Service) SaveCustomerPO(ctx context.Context, a auth.Account, quotationID string, in POInput) (CustomerPO, error) {
	if !signedIn(a) || a.Role != auth.RoleCustomer {
		return CustomerPO{}, ErrForbidden
	}
	err := check(text(&in.PONo, 64, false), date(&in.IssuedOn), text(&in.Remarks, 2000, true))
	if err != nil || in.PONo == "" || !validID(quotationID) || in.ExpectedLockVersion < 0 {
		return CustomerPO{}, ErrInvalidInput
	}
	return s.repo.SaveCustomerPO(ctx, a.ID, quotationID, in)
}
