package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/DataTunerX/utility-server/config"
	"github.com/DataTunerX/utility-server/logging"
	"github.com/dustin/go-humanize"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/sourcegraph/conc/pool"
)

func main() {
	ctx := context.Background()
	bucketName := config.GetS3Bucket()
	endpoint := config.GetS3Endpoint()
	filePath := config.GetS3FilePath()
	baseDir := config.GetMountPath()
	logging.NewZapLogger(config.GetLevel())
	s3Client, err := minio.New(endpoint, &minio.Options{
		Creds: credentials.NewStaticV4(config.GetS3AccesskeyId(),
			config.GetS3ESecretAccessKey(), ""),
		Secure: config.GetSecure(),
	})
	if err != nil {
		logging.ZLogger.Errorf("Connect %s failed, err: %v", endpoint, err)
		panic(err)
	}
	opts := minio.ListObjectsOptions{
		Prefix:    filePath,
		Recursive: true,
	}

	p := pool.New().WithMaxGoroutines(10)
	for obj := range s3Client.ListObjects(ctx, bucketName, opts) {
		obj := obj
		if strings.HasSuffix(obj.Key, "/") {
			logging.ZLogger.Infof("Skipping directory: %s", obj.Key)
			continue
		}

		readableSize := humanize.Bytes(uint64(obj.Size))
		logging.ZLogger.Infof("File: %s, Size: %s", obj.Key, readableSize)
		p.Go(func() {
			localPath := filepath.Join(baseDir, obj.Key)
			if err := os.MkdirAll(filepath.Dir(localPath), os.ModePerm); err != nil {
				logging.ZLogger.Errorf("Failed to create directory for %s: %v", localPath, err)
				panic(err)
			}
			if err := s3Client.FGetObject(ctx,
				bucketName, obj.Key,
				localPath, minio.GetObjectOptions{}); err != nil {
				logging.ZLogger.Errorf("Get file: %s failed: %v", filePath, err)
				panic(err)
			}
		})
	}
	p.Wait()
	logging.ZLogger.Infof("Successful download s3 file")
}
