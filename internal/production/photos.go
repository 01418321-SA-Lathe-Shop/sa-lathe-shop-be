package production

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

type PhotoService struct {
	repo  PhotoRepository
	store file.Store
	slots chan struct{}
}

func NewPhotoService(r PhotoRepository, s file.Store) *PhotoService {
	return &PhotoService{r, s, make(chan struct{}, 2)}
}
func (s *PhotoService) Start(ctx context.Context, a auth.Account, job string, in file.Input) (file.Link, error) {
	if e := permitted(a, auth.RoleWorker); e != nil {
		return file.Link{}, e
	}
	if !validID(job) {
		return file.Link{}, req.ErrInvalidInput
	}
	if e := file.ValidateInput(in); e != nil {
		return file.Link{}, e
	}
	if in.ContentType != "image/jpeg" && in.ContentType != "image/png" {
		return file.Link{}, req.ErrInvalidInput
	}
	key := uuid.NewString()
	v := Photo{ID: key, JobID: job, UploadedBy: a.ID, Input: in, StagingKey: "staging/job-photos/" + key, ObjectKey: "jobs/" + job + "/photos/" + key, State: "pending", ExpiresAt: time.Now().UTC().Add(file.LinkTTL)}
	link, e := s.store.UploadLink(ctx, v.StagingKey, in.ContentType, in.Size)
	if e != nil {
		return link, e
	}
	if e = s.repo.CreateJobPhoto(ctx, v); e != nil {
		return file.Link{}, e
	}
	link.AttachmentID = key
	return link, nil
}
func (s *PhotoService) Complete(ctx context.Context, a auth.Account, key string) (Photo, error) {
	if e := permitted(a, auth.RoleWorker); e != nil {
		return Photo{}, e
	}
	if !validID(key) {
		return Photo{}, req.ErrInvalidInput
	}
	select {
	case s.slots <- struct{}{}:
		defer func() { <-s.slots }()
	case <-ctx.Done():
		return Photo{}, ctx.Err()
	}
	v, e := s.repo.BeginJobPhoto(ctx, a.ID, key)
	if e != nil {
		return v, e
	}
	success := false
	defer func() {
		if !success {
			cleanup, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			_ = s.repo.FinishJobPhoto(cleanup, a.ID, key, false)
		}
	}()
	body, etag, e := s.store.Open(ctx, v.StagingKey)
	if e != nil {
		return v, e
	}
	defer body.Close()
	tmp, e := os.CreateTemp("", "lathe-job-photo-*")
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
	if e = s.repo.FinishJobPhoto(ctx, a.ID, key, true); e != nil {
		return v, e
	}
	success = true
	v.State = "ready"
	return v, nil
}
// Download serves staff and the job's own customer members (the repository checks membership).
func (s *PhotoService) Download(ctx context.Context, a auth.Account, key string) (file.Link, error) {
	if e := permitted(a, "staff"); e != nil && (!a.Active || a.Role != auth.RoleCustomer) {
		return file.Link{}, e
	}
	if !validID(key) {
		return file.Link{}, req.ErrInvalidInput
	}
	v, e := s.repo.ReadJobPhoto(ctx, a.ID, key)
	if e != nil {
		return file.Link{}, e
	}
	return s.store.DownloadLink(ctx, v.ObjectKey, v.Name)
}
