package auth

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Registration struct {
	Username     string `json:"username"`
	Password     string `json:"password"`
	Name         string `json:"name"`
	CustomerType string `json:"customer_type"`
	CompanyName  string `json:"company_name,omitempty"`
	Invitation   string `json:"invitation,omitempty"`
	Address      string `json:"address,omitempty"`
}
type CustomerProfile struct {
	ID    string `json:"id"`
	Type  string `json:"customer_type"`
	Name  string `json:"name"`
	Owner bool   `json:"is_owner"`
}
type CustomerRegistration struct {
	Account                                Account
	CustomerID, Type, CompanyName, Address string
	InvitationHash                         []byte
}
type CustomerAddress struct {
	ID        string    `json:"id"`
	Address   string    `json:"address"`
	CreatedAt time.Time `json:"created_at"`
}
type Invitation struct {
	ID, CustomerID, CreatedBy string
	Hash                      []byte
	ExpiresAt                 time.Time
}
type CustomerRepository interface {
	RegisterCustomer(context.Context, CustomerRegistration) (Account, error)
	CustomerProfiles(context.Context, string) ([]CustomerProfile, error)
	SaveInvitation(context.Context, Invitation) error
	AcceptInvitation(context.Context, string, []byte) error
	CustomerAddresses(ctx context.Context, accountID, customerID string) ([]CustomerAddress, error)
	AddCustomerAddress(ctx context.Context, accountID, customerID string, in CustomerAddress) (CustomerAddress, error)
}

// Separate interface keeps staff mocks and repository consumers unchanged.
type CustomerService struct {
	repo   CustomerRepository
	hasher PasswordHasher
}

func NewCustomerService(repo CustomerRepository, hasher PasswordHasher) *CustomerService {
	return &CustomerService{repo: repo, hasher: hasher}
}
func (s *CustomerService) Register(ctx context.Context, in Registration) (Account, error) {
	username, err := normalizeUsername(in.Username)
	if err != nil {
		return Account{}, err
	}
	name := strings.TrimSpace(in.Name)
	if err = validateName(name); err != nil {
		return Account{}, err
	}
	if err = validatePassword(in.Password); err != nil {
		return Account{}, err
	}
	if in.CustomerType != "individual" && in.CustomerType != "company" {
		return Account{}, ErrInvalidInput
	}
	company := strings.TrimSpace(in.CompanyName)
	if in.CustomerType == "individual" && (company != "" || in.Invitation != "") {
		return Account{}, ErrInvalidInput
	}
	if in.CustomerType == "company" && ((company == "") == (in.Invitation == "")) {
		return Account{}, ErrInvalidInput
	}
	if company != "" {
		if err = validateName(company); err != nil {
			return Account{}, err
		}
	}
	// Invitation joins use the company's existing addresses.
	address := strings.TrimSpace(in.Address)
	if in.Invitation != "" && address != "" {
		return Account{}, ErrInvalidInput
	}
	if address != "" {
		if err = validateAddress(address); err != nil {
			return Account{}, err
		}
	}
	var invitationHash []byte
	if in.Invitation != "" {
		if !validToken(in.Invitation) {
			return Account{}, ErrInvalidInput
		}
		invitationHash = digest(in.Invitation)
	}
	hash, err := s.hasher.Hash(ctx, in.Password)
	if err != nil {
		return Account{}, err
	}
	a, err := s.repo.RegisterCustomer(ctx, CustomerRegistration{Account: Account{ID: uuid.NewString(), PersonID: uuid.NewString(), Username: username, Name: name, Role: RoleCustomer, Active: true, PasswordHash: hash}, CustomerID: uuid.NewString(), Type: in.CustomerType, CompanyName: company, Address: address, InvitationHash: invitationHash})
	a.PasswordHash = ""
	return a, err
}
func (s *CustomerService) Profiles(ctx context.Context, a Account) ([]CustomerProfile, error) {
	if !a.Active || a.Role != RoleCustomer {
		return nil, ErrForbidden
	}
	return s.repo.CustomerProfiles(ctx, a.ID)
}
func (s *CustomerService) Invite(ctx context.Context, a Account, customerID string) (string, error) {
	if !a.Active || a.Role != RoleCustomer {
		return "", ErrForbidden
	}
	if err := validateID(customerID); err != nil {
		return "", err
	}
	token, hash, err := newToken()
	if err != nil {
		return "", err
	}
	// One-use bearer capability, owner-only issuance, short configurable UI flow.
	err = s.repo.SaveInvitation(ctx, Invitation{ID: uuid.NewString(), CustomerID: customerID, CreatedBy: a.ID, Hash: hash, ExpiresAt: time.Now().UTC().Add(24 * time.Hour)})
	if err != nil {
		return "", err
	}
	return token, nil
}
func (s *CustomerService) Join(ctx context.Context, a Account, token string) error {
	if !a.Active || a.Role != RoleCustomer {
		return ErrForbidden
	}
	if !validToken(token) {
		return ErrInvalidInput
	}
	return s.repo.AcceptInvitation(ctx, a.ID, digest(token))
}

// Addresses are readable by the customer's members; only the owner adds them.
func (s *CustomerService) Addresses(ctx context.Context, a Account, customerID string) ([]CustomerAddress, error) {
	if !a.Active || a.Role != RoleCustomer {
		return nil, ErrForbidden
	}
	if err := validateID(customerID); err != nil {
		return nil, err
	}
	return s.repo.CustomerAddresses(ctx, a.ID, customerID)
}
func (s *CustomerService) AddAddress(ctx context.Context, a Account, customerID, address string) (CustomerAddress, error) {
	if !a.Active || a.Role != RoleCustomer {
		return CustomerAddress{}, ErrForbidden
	}
	if err := validateID(customerID); err != nil {
		return CustomerAddress{}, err
	}
	address = strings.TrimSpace(address)
	if err := validateAddress(address); err != nil {
		return CustomerAddress{}, err
	}
	return s.repo.AddCustomerAddress(ctx, a.ID, customerID, CustomerAddress{ID: uuid.NewString(), Address: address})
}
