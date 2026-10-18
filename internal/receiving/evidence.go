package receiving

import (
	"context"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/auth"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/file"
	req "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/request"
	"github.com/google/uuid"
	"io"
	"os"
	"time"
)

type EvidenceService struct {
	repo  EvidenceRepository
	store file.Store
	slots chan struct{}
}

func NewEvidenceService(r EvidenceRepository, s file.Store) *EvidenceService {
	return &EvidenceService{r, s, make(chan struct{}, 2)}
}
func (s *EvidenceService) Start(ctx context.Context, a auth.Account, order string, in file.Input) (file.Link, error) {
	if err := permitted(a, true); err != nil {
		return file.Link{}, err
	}
	if !id(order) {
		return file.Link{}, req.ErrInvalidInput
	}
	if err := file.ValidateInput(in); err != nil {
		return file.Link{}, err
	}
	key := uuid.NewString()
	v := Evidence{ID: key, OrderID: order, UploadedBy: a.ID, Input: in, StagingKey: "staging/orders/" + key, ObjectKey: "orders/" + order + "/evidence/" + key, State: "pending", ExpiresAt: time.Now().UTC().Add(file.LinkTTL)}
	link, err := s.store.UploadLink(ctx, v.StagingKey, in.ContentType, in.Size)
	if err != nil {
		return link, err
	}
	if err = s.repo.CreateOrderEvidence(ctx, v); err != nil {
		return file.Link{}, err
	}
	link.AttachmentID = key
	return link, nil
}
func (s *EvidenceService) Complete(ctx context.Context, a auth.Account, key string) (Evidence, error) {
	if err := permitted(a, true); err != nil {
		return Evidence{}, err
	}
	if !id(key) {
		return Evidence{}, req.ErrInvalidInput
	}
	select {
	case s.slots <- struct{}{}:
		defer func() { <-s.slots }()
	case <-ctx.Done():
		return Evidence{}, ctx.Err()
	}
	v, err := s.repo.BeginOrderEvidence(ctx, a.ID, key)
	if err != nil {
		return v, err
	}
	success := false
	defer func() {
		if !success {
			cleanup, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			_ = s.repo.FinishOrderEvidence(cleanup, a.ID, key, false)
		}
	}()
	body, etag, err := s.store.Open(ctx, v.StagingKey)
	if err != nil {
		return v, err
	}
	defer body.Close()
	tmp, err := os.CreateTemp("", "lathe-supplier-evidence-*")
	if err != nil {
		return v, err
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()
	size, err := io.Copy(tmp, io.LimitReader(body, v.Size+1))
	if err != nil {
		return v, err
	}
	if err = file.ValidateContent(tmp, size, v.Input); err != nil {
		return v, err
	}
	if err = s.store.Freeze(ctx, v.StagingKey, v.ObjectKey, etag); err != nil {
		return v, err
	}
	if err = s.repo.FinishOrderEvidence(ctx, a.ID, key, true); err != nil {
		return v, err
	}
	success = true
	v.State = "ready"
	return v, nil
}
func (s *EvidenceService) Download(ctx context.Context, a auth.Account, key string) (file.Link, error) {
	if err := permitted(a, true); err != nil {
		return file.Link{}, err
	}
	if !id(key) {
		return file.Link{}, req.ErrInvalidInput
	}
	v, err := s.repo.ReadOrderEvidence(ctx, a.ID, key)
	if err != nil {
		return file.Link{}, err
	}
	return s.store.DownloadLink(ctx, v.ObjectKey, v.Name)
}
