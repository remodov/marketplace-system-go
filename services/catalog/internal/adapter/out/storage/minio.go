package storage

import (
	"context"
	"errors"
	"fmt"
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
	// TODO шаг 12: подписанная ссылка на PUT объекта.
	// Тип содержимого должен войти в подпись, срок жизни берётся из настроек,
	// expiresAt считается от часов сервиса. Клиент уже знает регион и в сеть не ходит.
	return out.PresignedUpload{}, errors.New("шаг 12: ссылка на загрузку не реализована")
}
