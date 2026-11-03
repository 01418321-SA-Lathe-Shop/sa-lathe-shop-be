package auth

import (
	"context"
	"fmt"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/platform/validation"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Registration struct {
	Username     string `json:"username"`
	Password     string `json:"password"`
	Name         string `json:"name"`
	Phone        string `json:"phone,omitempty"`
	Email        string `json:"email,omitempty"`
	CustomerType string `json:"customer_type"`
	CompanyName  string `json:"company_name,omitempty"`
	TaxID        string `json:"tax_id,omitempty"`
	Invitation   string `json:"invitation,omitempty"`
	Address      string `json:"address,omitempty"`
	AddressLine1 string `json:"address_line1,omitempty"`
	AddressLine2 string `json:"address_line2,omitempty"`
	District     string `json:"district,omitempty"`
	Province     string `json:"province,omitempty"`
	PostalCode   string `json:"postal_code,omitempty"`
	IsDefault    *bool  `json:"is_default,omitempty"`
}
type CustomerProfile struct {
	ID    string `json:"id"`
	Type  string `json:"customer_type"`
	Name  string `json:"name"`
	Owner bool   `json:"is_owner"`
}
type CustomerRegistration struct {
	Account                                                            Account
	CustomerID, Type, CompanyName, Address                             string
	Phone, Email, TaxID                                                string
	AddressLine1, AddressLine2, District, Province, PostalCode         string
	IsDefault                                                          bool
	InvitationHash                                                     []byte
}
type CustomerAddress struct {
	ID           string    `json:"id"`
	Address      string    `json:"address"`
	AddressLine1 string    `json:"address_line1,omitempty"`
	AddressLine2 string    `json:"address_line2,omitempty"`
	District     string    `json:"district,omitempty"`
	Province     string    `json:"province,omitempty"`
	PostalCode   string    `json:"postal_code,omitempty"`
	IsDefault    bool      `json:"is_default"`
	CreatedAt    time.Time `json:"created_at"`
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

func normalizeStructuredAddress(address, line1, line2, district, province, postalCode string) (string, string, string, string, string, string, error) {
	address = strings.TrimSpace(address)
	line1 = strings.TrimSpace(line1)
	line2 = strings.TrimSpace(line2)
	district = strings.TrimSpace(district)
	province = strings.TrimSpace(province)
	postalCode = strings.TrimSpace(postalCode)
	hasStructured := line1 != "" || line2 != "" || district != "" || province != "" || postalCode != ""
	if hasStructured {
		if !validation.Text(line1, 1, 300) || !validation.Text(line2, 0, 300) ||
			!validation.Text(district, 1, 150) || !validation.Text(province, 1, 150) ||
			!postalCodePattern.MatchString(postalCode) {
			return "", "", "", "", "", "", fmt.Errorf("%w: structured address requires address_line1, district, province and 5-digit postal_code", ErrInvalidInput)
		}
		if address == "" {
			parts := []string{line1}
			if line2 != "" {
				parts = append(parts, line2)
			}
			parts = append(parts, district, province, postalCode)
			address = strings.Join(parts, " ")
		}
	}
	if address != "" {
		if err := validateAddress(address); err != nil {
			return "", "", "", "", "", "", err
		}
	}
	return address, line1, line2, district, province, postalCode, nil
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
	var phone, email string
	if strings.TrimSpace(in.Phone) != "" {
		phone, err = normalizePhone(in.Phone)
		if err != nil {
			return Account{}, err
		}
	}
	if strings.TrimSpace(in.Email) != "" {
		email, err = normalizeEmail(in.Email)
		if err != nil {
			return Account{}, err
		}
	}
	taxID := strings.TrimSpace(in.TaxID)
	if taxID != "" && !validation.Text(taxID, 1, 30) {
		return Account{}, ErrInvalidInput
	}
	// Invitation joins use the company's existing addresses.
	address, line1, line2, district, province, postalCode, err := normalizeStructuredAddress(in.Address, in.AddressLine1, in.AddressLine2, in.District, in.Province, in.PostalCode)
	if err != nil {
		return Account{}, err
	}
	if in.Invitation != "" && address != "" {
		return Account{}, ErrInvalidInput
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
	isDefault := true
	if in.IsDefault != nil {
		isDefault = *in.IsDefault
	}
	a, err := s.repo.RegisterCustomer(ctx, CustomerRegistration{
		Account:        Account{ID: uuid.NewString(), PersonID: uuid.NewString(), Username: username, Name: name, Role: RoleCustomer, Active: true, PasswordHash: hash},
		CustomerID:     uuid.NewString(),
		Type:           in.CustomerType,
		CompanyName:    company,
		Address:        address,
		Phone:          phone,
		Email:          email,
		TaxID:          taxID,
		AddressLine1:   line1,
		AddressLine2:   line2,
		District:       district,
		Province:       province,
		PostalCode:     postalCode,
		IsDefault:      isDefault,
		InvitationHash: invitationHash,
	})
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
	return s.AddStructuredAddress(ctx, a, customerID, CustomerAddress{Address: address})
}
func (s *CustomerService) AddStructuredAddress(ctx context.Context, a Account, customerID string, in CustomerAddress) (CustomerAddress, error) {
	if !a.Active || a.Role != RoleCustomer {
		return CustomerAddress{}, ErrForbidden
	}
	if err := validateID(customerID); err != nil {
		return CustomerAddress{}, err
	}
	address, line1, line2, district, province, postalCode, err := normalizeStructuredAddress(in.Address, in.AddressLine1, in.AddressLine2, in.District, in.Province, in.PostalCode)
	if err != nil || address == "" {
		if err != nil {
			return CustomerAddress{}, err
		}
		return CustomerAddress{}, ErrInvalidInput
	}
	return s.repo.AddCustomerAddress(ctx, a.ID, customerID, CustomerAddress{
		ID:           uuid.NewString(),
		Address:      address,
		AddressLine1: line1,
		AddressLine2: line2,
		District:     district,
		Province:     province,
		PostalCode:   postalCode,
		IsDefault:    in.IsDefault,
	})
}
