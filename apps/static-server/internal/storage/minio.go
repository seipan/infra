package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/sirupsen/logrus"
)

// ErrNotFound reports that the object, or the bucket holding it, does not exist.
var ErrNotFound = errors.New("object not found")

type MinioStorage struct {
	core   *minio.Core
	logger *logrus.Logger
}

// Object is an open object: the metadata returned with the GET response, plus
// its body. Body is owned by the caller and must be closed.
type Object struct {
	Info minio.ObjectInfo
	Body io.ReadCloser
}

func NewMinioStorage(endpoint, accessKey, secretKey string, useSSL bool, logger *logrus.Logger) (*MinioStorage, error) {
	core, err := minio.NewCore(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize MinIO client: %w", err)
	}

	return &MinioStorage{
		core:   core,
		logger: logger,
	}, nil
}

// GetObject fetches an object in a single round trip, reporting ErrNotFound if
// it is missing.
//
// The Core client is used rather than the high level one on purpose: the latter
// returns a lazy handle, and asking that handle for metadata before reading
// issues a StatObject of its own, so serving one file would cost a HEAD plus a
// GET. Core issues the GET directly and hands back the metadata that came with
// the response.
func (m *MinioStorage) GetObject(ctx context.Context, bucket, objectName string) (*Object, error) {
	body, info, _, err := m.core.GetObject(ctx, bucket, objectName, minio.GetObjectOptions{})
	if err != nil {
		if minio.ToErrorResponse(err).StatusCode == http.StatusNotFound {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get object: %w", err)
	}
	return &Object{Info: info, Body: body}, nil
}
