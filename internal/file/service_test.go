package file

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	req "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/request"
)

type uploadFake struct {
	a                Attachment
	begins, finishes int
	success          bool
}

func (f *uploadFake) CreateUpload(_ context.Context, a Attachment) error { f.a = a; return nil }
func (f *uploadFake) BeginCompletion(_ context.Context, _, _ string) (Attachment, error) {
	f.begins++
	return f.a, nil
}
func (f *uploadFake) FinishCompletion(_ context.Context, _ string, success bool) error {
	f.finishes++
	f.success = success
	return nil
}
func (f *uploadFake) GetAttachment(context.Context, string, string) (Attachment, error) {
	return f.a, nil
}
func (f *uploadFake) ListAttachments(context.Context, string, string) ([]Attachment, error) {
	return nil, nil
}

type storeFake struct {
	data   []byte
	frozen bool
}

func (*storeFake) UploadLink(context.Context, string, string, int64) (Link, error) {
	return Link{URL: "private-signed-link"}, nil
}
func (f *storeFake) Open(context.Context, string) (io.ReadCloser, string, error) {
	return io.NopCloser(bytes.NewReader(f.data)), "etag", nil
}
func (f *storeFake) Freeze(context.Context, string, string, string) error {
	f.frozen = true
	return nil
}
func (*storeFake) DownloadLink(context.Context, string, string) (Link, error) {
	return Link{URL: "private-signed-download"}, nil
}

type requestReader struct{ err error }

func (f requestReader) Get(context.Context, req.Principal, string) (req.Request, error) {
	return req.Request{}, f.err
}
func TestUploadValidationBeforeFreeze(t *testing.T) {
	for _, valid := range []bool{false, true} {
		body := []byte("details")
		name, mime := "notes.txt", "text/plain"
		if !valid {
			name, mime = "drawing.pdf", "application/pdf"
		}
		repo := &uploadFake{}
		store := &storeFake{data: body}
		service := NewService(repo, store, requestReader{})
		p := req.Principal{AccountID: "57039d8b-c58e-4ee1-8943-539dc6203626", Role: "customer", Active: true}
		link, err := service.Start(context.Background(), p, "request", Input{Name: name, ContentType: mime, Size: int64(len(body))})
		if err != nil {
			t.Fatal(err)
		}
		_, err = service.Complete(context.Background(), p, link.AttachmentID)
		if valid && (err != nil || !store.frozen || !repo.success) {
			t.Fatal("valid upload not finalized")
		}
		if !valid && (!errors.Is(err, req.ErrInvalidInput) || store.frozen || repo.success) {
			t.Fatal("invalid upload became available")
		}
	}
}
func TestUploadOwnershipBeforeSigning(t *testing.T) {
	repo := &uploadFake{}
	service := NewService(repo, &storeFake{}, requestReader{err: req.ErrNotFound})
	_, err := service.Start(context.Background(), req.Principal{Role: "customer", Active: true}, "other-request", Input{Name: "notes.txt", ContentType: "text/plain", Size: 5})
	if !errors.Is(err, req.ErrNotFound) || repo.a.ID != "" {
		t.Fatal("unowned request created an upload")
	}
}
