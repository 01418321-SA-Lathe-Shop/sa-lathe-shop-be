package objectstore

import (
	"context"
	"fmt"
	"io"
	"mime"
	"net/url"
	"time"

	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/file"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type Config struct {
	Endpoint, PublicEndpoint, Region, Bucket, AccessKey, SecretKey string
	PathStyle                                                      bool
}
type S3 struct {
	client *s3.Client
	signer *s3.PresignClient
	bucket string
}

func New(c Config) *S3 {
	config := aws.Config{Region: c.Region, Credentials: aws.NewCredentialsCache(credentials.NewStaticCredentialsProvider(c.AccessKey, c.SecretKey, ""))}
	create := func(endpoint string) *s3.Client {
		return s3.NewFromConfig(config, func(o *s3.Options) {
			o.UsePathStyle = c.PathStyle
			if endpoint != "" {
				o.BaseEndpoint = aws.String(endpoint)
			}
			o.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
			o.ResponseChecksumValidation = aws.ResponseChecksumValidationWhenRequired
		})
	}
	public := c.PublicEndpoint
	if public == "" {
		public = c.Endpoint
	}
	return &S3{client: create(c.Endpoint), signer: s3.NewPresignClient(create(public)), bucket: c.Bucket}
}
func (s *S3) UploadLink(ctx context.Context, key, contentType string, size int64) (file.Link, error) {
	signed, err := s.signer.PresignPutObject(ctx, &s3.PutObjectInput{Bucket: &s.bucket, Key: &key, ContentType: &contentType, ContentLength: &size}, func(o *s3.PresignOptions) { o.Expires = file.LinkTTL })
	if err != nil {
		return file.Link{}, err
	}
	headers := map[string]string{"Content-Type": contentType}
	for k, v := range signed.SignedHeader {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}
	return file.Link{URL: signed.URL, Headers: headers, ExpiresAt: time.Now().UTC().Add(file.LinkTTL)}, nil
}
func (s *S3) Open(ctx context.Context, key string) (io.ReadCloser, string, error) {
	object, err := s.client.GetObject(ctx, &s3.GetObjectInput{Bucket: &s.bucket, Key: &key})
	if err != nil {
		return nil, "", err
	}
	if object.ETag == nil {
		object.Body.Close()
		return nil, "", fmt.Errorf("object has no version validator")
	}
	return object.Body, *object.ETag, nil
}
func (s *S3) Freeze(ctx context.Context, source, destination, etag string) error {
	copySource := url.PathEscape(s.bucket + "/" + source)
	_, err := s.client.CopyObject(ctx, &s3.CopyObjectInput{Bucket: &s.bucket, Key: &destination, CopySource: &copySource, CopySourceIfMatch: &etag})
	return err
}
func (s *S3) DownloadLink(ctx context.Context, key, name string) (file.Link, error) {
	disposition := mime.FormatMediaType("attachment", map[string]string{"filename": name})
	contentType := "application/octet-stream"
	signed, err := s.signer.PresignGetObject(ctx, &s3.GetObjectInput{Bucket: &s.bucket, Key: &key, ResponseContentDisposition: &disposition, ResponseContentType: &contentType}, func(o *s3.PresignOptions) { o.Expires = file.LinkTTL })
	if err != nil {
		return file.Link{}, err
	}
	return file.Link{URL: signed.URL, ExpiresAt: time.Now().UTC().Add(file.LinkTTL)}, nil
}
