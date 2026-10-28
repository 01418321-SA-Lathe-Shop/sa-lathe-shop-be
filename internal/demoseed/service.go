package demoseed

import (
	"context"
	"time"

	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/auth"
)

type Status struct {
	State    string `json:"state"`
	Accounts int    `json:"accounts"`
	Records  int    `json:"owned_records"`
	Reused   bool   `json:"reused"`
}

// The repository owns SQL, target verification, locking and the outer transaction.
type Store interface {
	auth.Repository
	auth.CustomerRepository
	Inspect(context.Context) (Status, error)
	Initialize(context.Context) error
	Capture(context.Context) error
}
type Repository interface {
	Within(context.Context, Config, func(Store) error) error
}
type Service struct {
	repo   Repository
	hasher auth.PasswordHasher
	cfg    Config
}

func New(repo Repository, hasher auth.PasswordHasher, cfg Config) *Service {
	return &Service{repo, hasher, cfg}
}

func (s *Service) Status(ctx context.Context) (Status, error) {
	var status Status
	e := s.repo.Within(ctx, s.cfg, func(store Store) error { var e error; status, e = store.Inspect(ctx); return e })
	return status, e
}

func (s *Service) Apply(ctx context.Context, get func(string) string) (Status, error) {
	var status Status
	e := s.repo.Within(ctx, s.cfg, func(store Store) error {
		var e error
		status, e = store.Inspect(ctx)
		if e != nil {
			return e
		}
		if status.State == "ready" {
			status.Reused = true
			return nil
		}
		if status.State != "uninitialized" {
			return ErrConflict
		}
		if e = store.Initialize(ctx); e != nil {
			return e
		}
		staff, e := auth.NewService(store, s.hasher, time.Hour)
		if e != nil {
			return e
		}
		customers := auth.NewCustomerService(store, s.hasher)
		var admin, owner auth.Account
		var companyID string
		for _, spec := range Accounts() {
			password := get(spec.PasswordEnv)
			switch spec.Role {
			case "admin":
				admin, e = staff.BootstrapAdmin(ctx, spec.Username, spec.Name, password)
			case "worker":
				_, e = staff.CreateWorker(ctx, admin, auth.WorkerInput{Username: spec.Username, Name: spec.Name, Password: password, EmployeeNo: spec.EmployeeNo, Position: spec.Position, Email: spec.Email, Phone: spec.Phone})
			case "customer":
				in := auth.Registration{Username: spec.Username, Name: spec.Name, Password: password, CustomerType: "individual"}
				if spec.Username == "nida.company" {
					in.CustomerType = "company"
					in.CompanyName = CompanyName
				}
				if spec.Username == "ploy.company" {
					in.CustomerType = "company"
					in.Invitation, e = customers.Invite(ctx, owner, companyID)
					if e != nil {
						return e
					}
				}
				var account auth.Account
				account, e = customers.Register(ctx, in)
				if e == nil && spec.Username == "nida.company" {
					owner = account
					var profiles []auth.CustomerProfile
					profiles, e = customers.Profiles(ctx, owner)
					if e == nil && len(profiles) != 1 {
						return ErrConflict
					}
					if e == nil {
						companyID = profiles[0].ID
					}
				}
			}
			if e != nil {
				return e
			}
		}
		if e = store.Capture(ctx); e != nil {
			return e
		}
		status, e = store.Inspect(ctx)
		return e
	})
	return status, e
}
