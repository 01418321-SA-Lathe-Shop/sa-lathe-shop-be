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
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
}
type Worker struct {
	Account
	EmployeeNo string `json:"employee_no"`
	Position   string `json:"position"`
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
	Account              Account
	EmployeeNo, Position string
}
type WorkerInput struct {
	Username   string `json:"username"`
	Password   string `json:"password"`
	Name       string `json:"name"`
	EmployeeNo string `json:"employee_no"`
	Position   string `json:"position"`
}
type WorkerUpdate struct {
	Username   *string `json:"username,omitempty"`
	Password   *string `json:"password,omitempty"`
	Name       *string `json:"name,omitempty"`
	EmployeeNo *string `json:"employee_no,omitempty"`
	Position   *string `json:"position,omitempty"`
	Active     *bool   `json:"is_active,omitempty"`
}

// WorkerChanges is validated service input, not an HTTP DTO.
type WorkerChanges struct {
	Username, Name, EmployeeNo, Position, PasswordHash *string
	Active                                             *bool
}
