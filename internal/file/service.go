package file

import (
	"context"
	"io"
	"os"
	"time"

	req "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/request"
	"github.com/google/uuid"
)

type Service struct {
	repo     Repository
	store    Store
	requests RequestReader
	slots    chan struct{}
}

func NewService(repo Repository, store Store, requests RequestReader) *Service {
	return &Service{repo: repo, store: store, requests: requests, slots: make(chan struct{}, 2)}
}
func (s *Service) Start(ctx context.Context, p req.Principal, requestID string, in Input) (Link, error) {
	if !p.Active || p.Role != "customer" {
		return Link{}, req.ErrForbidden
	}
	if err := ValidateInput(in); err != nil {
		return Link{}, err
	}
	if _, err := s.requests.Get(ctx, p, requestID); err != nil {
		return Link{}, err
	}
	id := uuid.NewString()
	a := Attachment{ID: id, RequestID: requestID, UploadedBy: p.AccountID, Input: in, StagingKey: "staging/" + id, ObjectKey: "requests/" + requestID + "/" + id, State: "pending", ExpiresAt: time.Now().UTC().Add(LinkTTL)}
	link, err := s.store.UploadLink(ctx, a.StagingKey, in.ContentType, in.Size)
	if err != nil {
		return Link{}, err
	}
	if err = s.repo.CreateUpload(ctx, a); err != nil {
		return Link{}, err
	}
	link.AttachmentID = id
	return link, nil
}
func (s *Service) Complete(ctx context.Context, p req.Principal, id string) (Attachment, error) {
	if !p.Active || p.Role != "customer" {
		return Attachment{}, req.ErrForbidden
	}
	if _, err := uuid.Parse(id); err != nil || len(id) != 36 {
		return Attachment{}, req.ErrInvalidInput
	}
	select {
	case s.slots <- struct{}{}:
		defer func() { <-s.slots }()
	case <-ctx.Done():
		return Attachment{}, ctx.Err()
	}
	a, err := s.repo.BeginCompletion(ctx, p.AccountID, id)
	if err != nil {
		return Attachment{}, err
	}
	// Failed completion is terminal. Retry means creating a fresh upload intent.
	success := false
	defer func() {
		if !success {
			cleanup, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			_ = s.repo.FinishCompletion(cleanup, id, false)
		}
	}()
	body, etag, err := s.store.Open(ctx, a.StagingKey)
	if err != nil {
		return Attachment{}, err
	}
	defer body.Close()
	tmp, err := os.CreateTemp("", "lathe-upload-*")
	if err != nil {
		return Attachment{}, err
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()
	size, err := io.Copy(tmp, io.LimitReader(body, a.Size+1))
	if err != nil {
		return Attachment{}, err
	}
	if err = ValidateContent(tmp, size, a.Input); err != nil {
		return Attachment{}, err
	}
	// CopySourceIfMatch prevents replacement between inspection and finalization.
	if err = s.store.Freeze(ctx, a.StagingKey, a.ObjectKey, etag); err != nil {
		return Attachment{}, err
	}
	if err = s.repo.FinishCompletion(ctx, id, true); err != nil {
		return Attachment{}, err
	}
	success = true
	a.State = "ready"
	return a, nil
}
func (s *Service) Download(ctx context.Context, p req.Principal, id string) (Link, error) {
	if !p.Active {
		return Link{}, req.ErrForbidden
	}
	if _, err := uuid.Parse(id); err != nil || len(id) != 36 {
		return Link{}, req.ErrInvalidInput
	}
	a, err := s.repo.GetAttachment(ctx, p.AccountID, id)
	if err != nil {
		return Link{}, err
	}
	return s.store.DownloadLink(ctx, a.ObjectKey, a.Name)
}
func (s *Service) List(ctx context.Context, p req.Principal, requestID string) ([]Attachment, error) {
	if _, err := s.requests.Get(ctx, p, requestID); err != nil {
		return nil, err
	}
	return s.repo.ListAttachments(ctx, p.AccountID, requestID)
}
