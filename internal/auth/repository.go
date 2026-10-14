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
}
type PasswordHasher interface {
	Hash(context.Context, string) (string, error)
	Verify(context.Context, string, string) (bool, error)
}
