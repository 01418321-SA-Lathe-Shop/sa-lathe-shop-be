package health

import (
	"context"
	"time"
)

type Repository interface{ Check(context.Context) error }
type Service struct {
	repository Repository
	timeout    time.Duration
}

func NewService(repository Repository, timeout time.Duration) *Service {
	return &Service{repository: repository, timeout: timeout}
}
func (s *Service) Ready(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	return s.repository.Check(ctx)
}
