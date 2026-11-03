package auth

import "context"

type Repository interface {
	FindAccount(context.Context, string) (Account, error)
	CreateSession(context.Context, Session, string) error
	FindSession(context.Context, []byte) (Session, error)
	RevokeSession(context.Context, []byte) error
	BootstrapAdmin(context.Context, NewStaff) (Account, error)
	CreateWorker(context.Context, NewStaff) (Worker, error)
	ListWorkers(context.Context, int, int) ([]Worker, error)
	GetWorker(context.Context, string) (Worker, error)
	UpdateWorker(context.Context, string, WorkerChanges) (Worker, error)
	ListAdmins(context.Context) ([]Admin, error)
	CreateAdmin(context.Context, NewStaff) (Admin, error)
	SetAdminActive(ctx context.Context, id string, active bool) (Admin, error)
	// ChangePassword(accountID, expectedHash, newHash, keepTokenHash) revokes all other sessions.
	ChangePassword(context.Context, string, string, string, []byte) error
}
type PasswordHasher interface {
	Hash(context.Context, string) (string, error)
	Verify(context.Context, string, string) (bool, error)
}
