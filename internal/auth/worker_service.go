package auth

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"strings"
)

func requireAdmin(actor Account) error {
	if !actor.Active || actor.Role != RoleAdmin {
		return ErrForbidden
	}
	return nil
}
func (s *Service) CreateWorker(ctx context.Context, actor Account, input WorkerInput) (Worker, error) {
	if err := requireAdmin(actor); err != nil {
		return Worker{}, err
	}
	username, err := normalizeUsername(input.Username)
	if err != nil {
		return Worker{}, err
	}
	name := strings.TrimSpace(input.Name)
	if err = validateName(name); err != nil {
		return Worker{}, err
	}
	employee, err := normalizeEmployee(input.EmployeeNo)
	if err != nil {
		return Worker{}, err
	}
	if err = validatePosition(input.Position); err != nil {
		return Worker{}, err
	}
	if err = validatePassword(input.Password); err != nil {
		return Worker{}, err
	}
	hash, err := s.hasher.Hash(ctx, input.Password)
	if err != nil {
		return Worker{}, err
	}
	return s.repo.CreateWorker(ctx, NewStaff{Account: Account{ID: uuid.NewString(), PersonID: uuid.NewString(), Username: username, Name: name, PasswordHash: hash, Role: RoleWorker, Active: true}, EmployeeNo: employee, Position: input.Position})
}
func (s *Service) ListWorkers(ctx context.Context, actor Account, limit, offset int) ([]Worker, error) {
	if err := requireAdmin(actor); err != nil {
		return nil, err
	}
	if limit < 1 || limit > 100 || offset < 0 {
		return nil, fmt.Errorf("%w: limit 1-100 and offset >= 0 required", ErrInvalidInput)
	}
	return s.repo.ListWorkers(ctx, limit, offset)
}
func (s *Service) GetWorker(ctx context.Context, actor Account, id string) (Worker, error) {
	if err := requireAdmin(actor); err != nil {
		return Worker{}, err
	}
	if err := validateID(id); err != nil {
		return Worker{}, err
	}
	return s.repo.GetWorker(ctx, id)
}
func (s *Service) UpdateWorker(ctx context.Context, actor Account, id string, input WorkerUpdate) (Worker, error) {
	if err := requireAdmin(actor); err != nil {
		return Worker{}, err
	}
	if err := validateID(id); err != nil {
		return Worker{}, err
	}
	changes := WorkerChanges{Active: input.Active}
	if input.Username != nil {
		v, err := normalizeUsername(*input.Username)
		if err != nil {
			return Worker{}, err
		}
		changes.Username = &v
	}
	if input.Name != nil {
		v := strings.TrimSpace(*input.Name)
		if err := validateName(v); err != nil {
			return Worker{}, err
		}
		changes.Name = &v
	}
	if input.EmployeeNo != nil {
		v, err := normalizeEmployee(*input.EmployeeNo)
		if err != nil {
			return Worker{}, err
		}
		changes.EmployeeNo = &v
	}
	if input.Position != nil {
		if err := validatePosition(*input.Position); err != nil {
			return Worker{}, err
		}
		changes.Position = input.Position
	}
	if input.Password != nil {
		if err := validatePassword(*input.Password); err != nil {
			return Worker{}, err
		}
		v, err := s.hasher.Hash(ctx, *input.Password)
		if err != nil {
			return Worker{}, err
		}
		changes.PasswordHash = &v
	}
	if changes.Active == nil && changes.Username == nil && changes.Name == nil && changes.EmployeeNo == nil && changes.Position == nil && changes.PasswordHash == nil {
		return Worker{}, fmt.Errorf("%w: at least one field required", ErrInvalidInput)
	}
	return s.repo.UpdateWorker(ctx, id, changes)
}
