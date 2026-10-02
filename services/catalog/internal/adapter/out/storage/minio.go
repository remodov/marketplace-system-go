package storage

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/remodov/marketplace-system-go/services/catalog/internal/core/product/port/out"
)

type Settings struct {
	Endpoint  string
	Region    string
	AccessKey string
	SecretKey string
	Bucket    string
	UploadTTL time.Duration
}

type ImageStorage struct {
	client    *minio.Client
	bucket    string
	uploadTTL time.Duration
	clock     out.Clock
}

func NewImageStorage(settings Settings, clock out.Clock) (*ImageStorage, error) {
	endpoint, err := url.Parse(settings.Endpoint)
	if err != nil || endpoint.Host == "" {
		return nil, fmt.Errorf("images: адрес хранилища %q не разобран", settings.Endpoint)
	}
	client, err := minio.New(endpoint.Host, &minio.Options{
		Creds:  credentials.NewStaticV4(settings.AccessKey, settings.SecretKey, ""),
		Secure: endpoint.Scheme == "https",
		Region: settings.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("images: клиент хранилища: %w", err)
	}
	return &ImageStorage{client: client, bucket: settings.Bucket, uploadTTL: settings.UploadTTL, clock: clock}, nil
}

func (s *ImageStorage) PresignUpload(ctx context.Context, key, contentType string) (out.PresignedUpload, error) {
	headers := http.Header{}
	headers.Set("Content-Type", contentType)
	signed, err := s.client.PresignHeader(ctx, http.MethodPut, s.bucket, key, s.uploadTTL, url.Values{}, headers)
	if err != nil {
		return out.PresignedUpload{}, fmt.Errorf("images: подпись ссылки: %w", err)
	}
	return out.PresignedUpload{Key: key, URL: signed.String(), ExpiresAt: s.clock.Now().Add(s.uploadTTL)}, nil
}
