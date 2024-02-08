package handler

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"path"

	// "time"

	"datatunerx-server/pkg/s3"

	"github.com/DataTunerX/utility-server/logging"
	"github.com/gin-gonic/gin"
	"github.com/minio/minio-go/v7"
)

type UploadHandler struct {
	S3Client s3.S3Client
}

type ClientChan chan string

// NewUploadHandler creates a new instance of UploadHandler
func NewUploadHandler(s3Client s3.S3Client) *UploadHandler {
	return &UploadHandler{
		S3Client: s3Client,
	}
}

// UploadFile handles file upload and returns the S3 URL when successful
func (uh *UploadHandler) UploadFile(c *gin.Context) {
	// // Set up SSE headers
	// c.Header("Content-Type", "text/event-stream")
	// c.Header("Cache-Control", "no-cache")
	// c.Header("Connection", "keep-alive")
	// c.Header("Access-Control-Allow-Origin", "*")

	// // Set up the SSE data channel
	dataCh := make(chan string)
	defer close(dataCh)

	// // Start a goroutine to send SSE events
	// go uh.sendSSEEvents(c, dataCh)

	// Parse form
	err := c.Request.ParseMultipartForm(0) // No limit on file size
	if err != nil {
		logging.ZLogger.Errorf("Failed to parse form: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse form"})
		return
	}

	// Get file from form
	originalFile, header, err := c.Request.FormFile("file")
	if err != nil {
		logging.ZLogger.Errorf("Failed to get file from form: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to get file from form"})
		return
	}

	// Ensure the file is not nil
	if originalFile == nil {
		logging.ZLogger.Errorf("File is nil")
		c.JSON(http.StatusBadRequest, gin.H{"error": "File is nil"})
		return
	}

	// Clone the file
	file := originalFile
	// defer file.Close()

	// Set up the S3 bucket and object name
	bucketName := "datatunerx"
	objectName := "uploads/" + header.Filename // Use original filename

	// Check if file already exists in S3
	// currentHash, err := uh.compareFileInS3(bucketName, objectName, file)
	// if err != nil {
	// 	logging.ZLogger.Errorf("Failed to compare file in S3: %v", err)
	// 	c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to compare file in S3"})
	// 	return
	// }

	// // Reopen the file for uploading
	// file.Seek(0, io.SeekStart) // Seek to the beginning of the file

	// // Set up the context
	// ctx, cancel := context.WithCancel(context.Background())
	// defer cancel()

	// // Set up progress tracking
	// progressCh := make(chan int64)
	// go uh.trackUploadProgress(ctx, progressCh, file)

	// Upload the file to S3
	s3URL, err := uh.uploadToS3WithProgress(bucketName, objectName, file, header, dataCh)
	if err != nil {
		logging.ZLogger.Errorf("Failed to upload file to S3: %v", err)
		// // Send an error event through SSE
		// dataCh <- fmt.Sprintf("error: %v", err)
		return
	}

	// Format the S3 URL
	formattedS3URL, err := formatS3URL(s3URL)
	if err != nil {
		logging.ZLogger.Errorf("Failed to format S3 URL: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to format S3 URL"})
		return
	}

	// // Send completion event through SSE
	// dataCh <- "complete"

	// Return formatted S3 URL to the client
	c.JSON(http.StatusOK, gin.H{"url": formattedS3URL})
}

// formatS3URL formats the S3 URL to the desired format
func formatS3URL(s3URL string) (string, error) {
	parsedURL, err := url.Parse(s3URL)
	if err != nil {
		return "", err
	}

	// Extract the path and object name from the S3 URL
	objectName := path.Base(parsedURL.Path)

	// URL encode the object name
	encodedObjectName := url.PathEscape(objectName)

	// Reconstruct the path with encoded object name
	encodedPath := path.Join(path.Dir(parsedURL.Path), encodedObjectName)

	// Format the new S3 URL
	formattedS3URL := fmt.Sprintf("s3:/%s?endpoint_override=http://%s", encodedPath, parsedURL.Host)
	return formattedS3URL, nil
}

// compareFileInS3 compares file in S3 based on file content hash
func (uh *UploadHandler) compareFileInS3(bucketName, objectName string, file io.Reader) (string, error) {
	// Get current file content hash
	currentHash, err := uh.calculateContentHash(file)
	if err != nil {
		return "", err
	}

	// Get object info from S3
	info, err := uh.S3Client.Client.StatObject(context.Background(), bucketName, objectName, minio.StatObjectOptions{})
	if err != nil {
		// If the file doesn't exist, proceed with the upload
		if minio.ToErrorResponse(err).StatusCode == http.StatusNotFound {
			return currentHash, nil
		}
		return "", err
	}

	// Compare content hash
	if info.Metadata.Get("Content-MD5") != currentHash {
		return "", fmt.Errorf("file with the same name but different content already exists in S3")
	}

	return currentHash, nil
}

// calculateContentHash calculates MD5 hash of file content
func (uh *UploadHandler) calculateContentHash(file io.Reader) (string, error) {
	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// trackUploadProgress tracks the progress of file upload
func (uh *UploadHandler) trackUploadProgress(ctx context.Context, progressCh chan<- int64, file io.Reader) {
	// Wrap the file reader with TeeReader to track the progress
	teeReader := io.TeeReader(file, &progressReader{progressCh: progressCh})
	io.Copy(io.Discard, teeReader)
}

// progressReader is a custom io.Writer that tracks the progress of file upload
type progressReader struct {
	progressCh chan<- int64
	bytesRead  int64
}

// Write implements the io.Writer interface
func (pr *progressReader) Write(p []byte) (n int, err error) {
	n = len(p)
	pr.bytesRead += int64(n)
	select {
	case pr.progressCh <- pr.bytesRead:
	default:
	}
	return
}

// uploadToS3WithProgress uploads a file to S3 and reports progress through SSE
func (uh *UploadHandler) uploadToS3WithProgress(bucketName, objectName string, file io.Reader, header *multipart.FileHeader, dataCh chan<- string) (string, error) {
	// Set up the context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// // Create a pipe to stream file content
	// reader, _ := io.Pipe()

	// // Set up progress tracking
	// go uh.trackUploadProgress(ctx, progressCh, reader)

	// Upload the file to S3 using the reader end of the pipe
	_, err := uh.S3Client.Client.PutObject(
		ctx,
		bucketName,
		objectName,
		file,
		header.Size, // Specify the size of the file
		minio.PutObjectOptions{
			ContentType:        header.Header.Get("Content-Type"),
			ContentEncoding:    header.Header.Get("Content-Encoding"),
			ContentDisposition: header.Header.Get("Content-Disposition"),
			ContentLanguage:    header.Header.Get("Content-Language"),
		},
	)
	if err != nil {
		return "", err
	}

	// Generate the public URL for the uploaded file
	s3URL := uh.S3Client.Client.EndpointURL().String() + "/" + bucketName + "/" + objectName

	return s3URL, nil
}

// progressReaderWithCallback is a custom TeeReader with a Progress callback
type progressReaderWithCallback struct {
	Reader   io.Reader
	Progress func(bytesSent int64)
}

// Read implements the io.Reader interface
func (pr *progressReaderWithCallback) Read(p []byte) (n int, err error) {
	n, err = pr.Reader.Read(p)
	if n > 0 && pr.Progress != nil {
		pr.Progress(int64(n))
	}
	return
}

// // sendSSEEvents sends SSE events to the client
// func (uh *UploadHandler) sendSSEEvents(c *gin.Context, dataCh <-chan string) {
// 	for {
// 		select {
// 		case data, ok := <-dataCh:
// 			if !ok {
// 				// Channel closed, stop sending events
// 				return
// 			}
// 			if data != "" {
// 				// Send SSE event to the client only if data is not empty
// 				fmt.Printf("Sending SSE Event: %s\n", data)
// 				c.SSEvent("message", data)
// 				c.Writer.Flush()
// 			}
// 		case <-time.After(1 * time.Second):
// 			// Check periodically for connection close
// 			if c.Writer.Written() {
// 				// If data has been written, continue checking
// 				continue
// 			}
// 			// Otherwise, assume the client has closed the connection
// 			return
// 		}
// 	}
// }

// PrintFileContent prints the content of the file
func PrintFileContent(file io.Reader) {
	// Read the content of the file
	content, err := io.ReadAll(file)
	if err != nil {
		logging.ZLogger.Errorf("Failed to read file content: %v", err)
		return
	}

	// Print the content
	fmt.Println("File Content:")
	fmt.Println(string(content))
}
