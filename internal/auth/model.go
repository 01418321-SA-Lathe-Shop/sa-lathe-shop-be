package auth

import (
	"errors"
	"time"
)

const (
	RoleAdmin    = "admin"
	RoleWorker   = "worker"
	RoleCustomer = "customer"
)

var (
	ErrInvalidInput    = errors.New("invalid input")
	ErrUnauthenticated = errors.New("invalid credentials or session")
	ErrForbidden       = errors.New("permission denied")
	ErrNotFound        = errors.New("record not found")
	ErrConflict        = errors.New("record already exists")
	ErrAdminExists     = errors.New("initial admin already exists")
)

type Account struct {
	ID           string    `json:"id"`
	PersonID     string    `json:"person_id"`
	Username     string    `json:"username"`
	Name         string    `json:"name"`
	Role         string    `json:"role"`
	Active       bool      `json:"is_active"`
	SuperAdmin   bool      `json:"is_super_admin"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
}
type Worker struct {
	Account
	Email      *string `json:"email"`
	Phone      *string `json:"phone"`
	EmployeeNo string  `json:"employee_no"`
	Position   string  `json:"position"`
	HiredAt    string  `json:"hired_at,omitempty"`
}
type Admin struct {
	Account
	Email *string `json:"email"`
	Phone *string `json:"phone"`
}
type AdminInput struct {
	Email    string `json:"email"`
	Phone    string `json:"phone"`
	Username string `json:"username"`
	Password string `json:"password"`
	Name     string `json:"name"`
}
type Session struct {
	ID        string
	TokenHash []byte
	Account   Account
	CreatedAt time.Time
	ExpiresAt time.Time
	RevokedAt *time.Time
}
type LoginResult struct {
	AccessToken string    `json:"access_token"`
	TokenType   string    `json:"token_type"`
	ExpiresAt   time.Time `json:"expires_at"`
	Account     Account   `json:"account"`
}
type NewStaff struct {
	Account                       Account
	EmployeeNo, Position, HiredAt string
	Email, Phone                  string
}
type WorkerInput struct {
	Email      string `json:"email"`
	Phone      string `json:"phone"`
	Username   string `json:"username"`
	Password   string `json:"password"`
	Name       string `json:"name"`
	EmployeeNo string `json:"employee_no"`
	Position   string `json:"position"`
	HiredAt    string `json:"hired_at,omitempty"`
}
type WorkerUpdate struct {
	Email      *string `json:"email,omitempty"`
	Phone      *string `json:"phone,omitempty"`
	Username   *string `json:"username,omitempty"`
	Password   *string `json:"password,omitempty"`
	Name       *string `json:"name,omitempty"`
	EmployeeNo *string `json:"employee_no,omitempty"`
	Position   *string `json:"position,omitempty"`
	HiredAt    *string `json:"hired_at,omitempty"`
	Active     *bool   `json:"is_active,omitempty"`
}

// WorkerChanges is validated service input, not an HTTP DTO.
type WorkerChanges struct {
	Email, Phone                                                *string
	Username, Name, EmployeeNo, Position, HiredAt, PasswordHash *string
	Active                                                      *bool
}
