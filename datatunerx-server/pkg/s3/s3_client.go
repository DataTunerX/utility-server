package s3

import (
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// S3Client contains the MinIO client
type S3Client struct {
	Client *minio.Client
}

// NewS3Client initializes the MinIO client for S3
func NewS3Client(endpoint, accessKey, secretKey string, useSSL bool) (S3Client, error) {
	minioClient, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return S3Client{}, err
	}

	return S3Client{
		Client: minioClient,
	}, nil
}
