package file

import (
	"context"
	"io"
	"time"

	req "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/request"
)

const MaxBytes int64 = 100000000
const LinkTTL = 10 * time.Minute

type Input struct {
	Name        string `json:"file_name"`
	ContentType string `json:"content_type"`
	Size        int64  `json:"byte_size"`
}
type Attachment struct {
	ID         string `json:"id"`
	RequestID  string `json:"request_id"`
	UploadedBy string `json:"uploaded_by"`
	Input
	StagingKey string    `json:"-"`
	ObjectKey  string    `json:"-"`
	State      string    `json:"state"`
	ExpiresAt  time.Time `json:"expires_at"`
}
type Link struct {
	URL          string            `json:"url"`
	Headers      map[string]string `json:"headers,omitempty"`
	ExpiresAt    time.Time         `json:"expires_at"`
	AttachmentID string            `json:"attachment_id,omitempty"`
}
type Repository interface {
	CreateUpload(context.Context, Attachment) error
	BeginCompletion(context.Context, string, string) (Attachment, error)
	FinishCompletion(context.Context, string, bool) error
	GetAttachment(context.Context, string, string) (Attachment, error)
	ListAttachments(context.Context, string, string) ([]Attachment, error)
}
type Store interface {
	UploadLink(context.Context, string, string, int64) (Link, error)
	Open(context.Context, string) (io.ReadCloser, string, error)
	Freeze(context.Context, string, string, string) error
	DownloadLink(context.Context, string, string) (Link, error)
}
type RequestReader interface {
	Get(context.Context, req.Principal, string) (req.Request, error)
}
