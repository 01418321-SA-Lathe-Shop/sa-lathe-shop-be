package delivery

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
func evidencePermission(a auth.Account) error {
	if !a.Active || (a.Role != "admin" && a.Role != "customer") {
		return req.ErrForbidden
	}
	return nil
}
func (s *EvidenceService) Start(ctx context.Context, a auth.Account, job string, in file.Input) (file.Link, error) {
	if e := evidencePermission(a); e != nil {
		return file.Link{}, e
	}
	if !validID(job) {
		return file.Link{}, req.ErrInvalidInput
	}
	if e := file.ValidateInput(in); e != nil {
		return file.Link{}, e
	}
	id := uuid.NewString()
	v := Evidence{ID: id, JobID: job, UploadedBy: a.ID, Input: in, StagingKey: "staging/customer-payments/" + id, ObjectKey: "jobs/" + job + "/customer-payments/" + id, State: "pending", ExpiresAt: time.Now().UTC().Add(file.LinkTTL)}
	link, e := s.store.UploadLink(ctx, v.StagingKey, in.ContentType, in.Size)
	if e != nil {
		return link, e
	}
	if e = s.repo.CreateCustomerEvidence(ctx, v); e != nil {
		return file.Link{}, e
	}
	link.AttachmentID = id
	return link, nil
}
func (s *EvidenceService) Complete(ctx context.Context, a auth.Account, id string) (Evidence, error) {
	if e := evidencePermission(a); e != nil {
		return Evidence{}, e
	}
	if !validID(id) {
		return Evidence{}, req.ErrInvalidInput
	}
	select {
	case s.slots <- struct{}{}:
		defer func() { <-s.slots }()
	case <-ctx.Done():
		return Evidence{}, ctx.Err()
	}
	v, e := s.repo.BeginCustomerEvidence(ctx, a.ID, id)
	if e != nil {
		return v, e
	}
	success := false
	defer func() {
		if !success {
			cleanup, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			_ = s.repo.FinishCustomerEvidence(cleanup, a.ID, id, false)
		}
	}()
	body, etag, e := s.store.Open(ctx, v.StagingKey)
	if e != nil {
		return v, e
	}
	defer body.Close()
	tmp, e := os.CreateTemp("", "lathe-customer-proof-*")
	if e != nil {
		return v, e
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()
	size, e := io.Copy(tmp, io.LimitReader(body, v.Size+1))
	if e != nil {
		return v, e
	}
	if e = file.ValidateContent(tmp, size, v.Input); e != nil {
		return v, e
	}
	if e = s.store.Freeze(ctx, v.StagingKey, v.ObjectKey, etag); e != nil {
		return v, e
	}
	if e = s.repo.FinishCustomerEvidence(ctx, a.ID, id, true); e != nil {
		return v, e
	}
	success = true
	v.State = "ready"
	return v, nil
}
func (s *EvidenceService) Download(ctx context.Context, a auth.Account, id string) (file.Link, error) {
	if e := evidencePermission(a); e != nil {
		return file.Link{}, e
	}
	if !validID(id) {
		return file.Link{}, req.ErrInvalidInput
	}
	v, e := s.repo.ReadCustomerEvidence(ctx, a.ID, id)
	if e != nil {
		return file.Link{}, e
	}
	return s.store.DownloadLink(ctx, v.ObjectKey, v.Name)
}
