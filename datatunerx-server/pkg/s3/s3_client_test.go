package s3_test

import (
	"bytes"
	"context"
	"datatunerx-server/pkg/s3"
	"io"
	"os"
	"testing"

	"github.com/minio/minio-go/v7"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testEndpoint  = ""
	testAccessKey = ""
	testSecretKey = ""
	testBucket    = ""
	testObject    = ""
)

func TestS3ClientUploadDownload(t *testing.T) {
	// 初始化S3客户端
	s3Client, err := s3.NewS3Client(testEndpoint, testAccessKey, testSecretKey, false)
	require.NoError(t, err, "Failed to create S3 client")

	// 创建测试文件内容
	testContent := []byte("Hello, S3!")

	// 上传文件
	_, err = s3Client.Client.PutObject(context.Background(), testBucket, testObject, bytes.NewReader(testContent), int64(len(testContent)), minio.PutObjectOptions{})
	require.NoError(t, err, "Failed to upload object to S3")

	// 下载文件
	downloadedObject, err := s3Client.Client.GetObject(context.Background(), testBucket, testObject, minio.GetObjectOptions{})
	require.NoError(t, err, "Failed to download object from S3")

	// 读取 MinIO 对象的内容
	downloadedContent, err := io.ReadAll(downloadedObject)
	if err != nil {
		// 处理错误
		t.Fatalf("Failed to read object content: %v", err)
		return
	}
	// 将字节片转换为字符串进行比较
	downloadedString := string(downloadedContent)
	expectedString := string(testContent)
	assert.Equal(t, expectedString, downloadedString, "Downloaded content does not match expected content")

	// 删除测试对象
	err = s3Client.Client.RemoveObject(context.Background(), testBucket, testObject, minio.RemoveObjectOptions{})
	require.NoError(t, err, "Failed to remove object from S3")
}

func TestMain(m *testing.M) {
	// 设置测试前的准备工作，例如创建测试桶等

	// 执行测试
	exitCode := m.Run()

	// 清理测试后的资源，例如删除测试桶等

	// 退出测试
	os.Exit(exitCode)
}
