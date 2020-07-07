package main

import (
	"fmt"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"gopkg.in/src-d/go-git.v4"
	"log"
	"mime"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func hugobuild() (string, error) {
	gitRepo := os.Getenv("GIT_REPO")
	s3Bucket := os.Getenv("S3_BUCKET")
	s3Region := os.Getenv("S3_REGION")

	blogTmpPath := "/tmp/blog"
	blogOutputPath := blogTmpPath + "/public"

	// Cleanup any leftover Git pulls.
	fmt.Printf("Cleanup old git pulls at %s\n", blogTmpPath)
	rmArgs := []string{"-rf", blogTmpPath}
	if rmErr := exec.Command("rm", rmArgs...).Run(); rmErr != nil {
		fmt.Fprintln(os.Stderr, rmErr)
		os.Exit(1)
	}
	// Clone the Git repro into blogTmpPath
	fmt.Printf("git clone %s to %s\n", gitRepo, blogTmpPath)
	if _, gitErr := git.PlainClone(blogTmpPath, false, &git.CloneOptions{
		URL:   gitRepo,
		Depth: 1, // only need the latest version
	}); gitErr != nil {
		log.Fatalf("Git clone failed with %s\n", gitErr)
		return "", gitErr
	}

	// Run hugo to build the site
	// This assumes the 'hugo' executable was included in your zip,
	// which will place it at /var/task/hugo
	// Download the Linux 64-bit binary from here:
	// https://github.com/gohugoio/hugo/releases
	fmt.Printf("Running hugo at path %s\n", blogTmpPath)
	hugoArgs := []string{"-s", blogTmpPath, "--minify"}
	if hugoErr := exec.Command("/var/task/hugo", hugoArgs...).Run(); hugoErr != nil {
		log.Fatalf("Hugo run failed with %s\n", hugoErr)
		return "", hugoErr
	}

	// Sync to S3
	fmt.Printf("S3 Sync from %s to %s\n", blogOutputPath, s3Bucket)
	if s3Err := s3Sync(s3Region, blogOutputPath, s3Bucket); s3Err != nil {
		log.Fatalf("S3 sync failed with %s\n", s3Err)
		return "", s3Err
	}

	return "SUCCESS", nil
}

func s3Sync(s3Region string, blogOutputPath string, s3Bucket string) error {
	// Take the built artifacts from Hugo and sync them to the S3 bucket.
	// This code was taken from this example:
	// https://github.com/aws/aws-sdk-go/tree/master/example/service/s3/sync
	sess := session.New(&aws.Config{
		Region: &s3Region,
	})
	uploader := s3manager.NewUploader(sess)
	iter := NewSyncFolderIterator(blogOutputPath, s3Bucket)
	if err := uploader.UploadWithIterator(aws.BackgroundContext(), iter); err != nil {
		fmt.Fprintf(os.Stderr, "unexpected error has occured: %v", err)
		return err
	}
	if err := iter.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "unexpected error occured during file walking: %v", err)
		return err
	}

	return nil
}

// SyncFolderIterator is used to upload a given folder
// to Amazon S3.
type SyncFolderIterator struct {
	bucket    string
	fileInfos []fileInfo
	err       error
}

type fileInfo struct {
	key      string
	fullpath string
}

// NewSyncFolderIterator will walk the path, and store the key and full path
// of the object to be uploaded. This will return a new SyncFolderIterator
// with the data provided from walking the path.
func NewSyncFolderIterator(path, bucket string) *SyncFolderIterator {
	metadata := []fileInfo{}
	filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			key := strings.TrimPrefix(p, path)
			metadata = append(metadata, fileInfo{key, p})
		}

		return nil
	})

	return &SyncFolderIterator{
		bucket,
		metadata,
		nil,
	}
}

// Next will determine whether or not there is any remaining files to
// be uploaded.
func (iter *SyncFolderIterator) Next() bool {
	return len(iter.fileInfos) > 0
}

// Err returns any error when os.Open is called.
func (iter *SyncFolderIterator) Err() error {
	return iter.err
}

// UploadObject will prep the new upload object by open that file and constructing a new
// s3manager.UploadInput.
func (iter *SyncFolderIterator) UploadObject() s3manager.BatchUploadObject {
	fi := iter.fileInfos[0]
	iter.fileInfos = iter.fileInfos[1:]
	body, err := os.Open(fi.fullpath)
	if err != nil {
		iter.err = err
	}

	extension := filepath.Ext(fi.key)
	mimeType := mime.TypeByExtension(extension)

	if mimeType == "" {
		mimeType = "binary/octet-stream"
	}

	cacheControl := "no-cache"

	if strings.Contains(mimeType, "image/") || strings.Contains(mimeType, "binary/octet-stream") {
		cacheControl = "max-age=86400"
	} else if strings.Contains(mimeType, "text/css") || strings.Contains(mimeType, "application/x-javascript") {
		cacheControl = "max-age=31536000"
	}

	input := s3manager.UploadInput{
		Bucket:       &iter.bucket,
		Key:          &fi.key,
		Body:         body,
		ContentType:  &mimeType,
		CacheControl: &cacheControl,
	}

	return s3manager.BatchUploadObject{
		&input,
		nil,
	}
}

func main() {
	// Make the handler available for Remote Procedure Call by AWS Lambda
	lambda.Start(hugobuild)
}
