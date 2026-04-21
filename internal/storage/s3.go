package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type S3Storage struct {
	client *s3.Client
	tm     *transfermanager.Client
	bucket string
	prefix string
}

func NewS3Storage(client *s3.Client, bucket string, prefix string) *S3Storage {
	tm := transfermanager.New(client)
	return &S3Storage{
		client: client,
		tm:     tm,
		bucket: bucket,
		prefix: prefix,
	}
}

func (s *S3Storage) GetLocationType() string {
	return "s3"
}

func (s *S3Storage) getObjectKey(hash string) string {
	if s.prefix == "" {
		return hash
	}
	return fmt.Sprintf("%s/%s", s.prefix, hash)
}

func (s *S3Storage) Store(ctx context.Context, r io.Reader) (string, error) {
	// For S3 (CAS), we must know the hash to set the Object Key.
	// Since the hash is derived from content, we must read the content first.
	// We use a local temp file to buffer, compute hash, and then upload.
	
	tmpFile, err := os.CreateTemp("", "s3-upload-*")
	if err != nil {
		return "", fmt.Errorf("failed to create local temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	hasher := sha256.New()
	mw := io.MultiWriter(tmpFile, hasher)

	if _, err := io.Copy(mw, r); err != nil {
		return "", fmt.Errorf("failed to buffer to temp file: %w", err)
	}

	hash := hex.EncodeToString(hasher.Sum(nil))
	key := s.getObjectKey(hash)

	// Check if it already exists in S3 to avoid redundant upload
	exists, _ := s.Exists(ctx, hash)
	if exists {
		return hash, nil
	}

	// Seek back to beginning of temp file for upload
	if _, err := tmpFile.Seek(0, 0); err != nil {
		return "", fmt.Errorf("failed to seek temp file: %w", err)
	}

	_, err = s.tm.UploadObject(ctx, &transfermanager.UploadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
		Body:   tmpFile,
	})

	if err != nil {
		return "", fmt.Errorf("failed to upload to s3: %w", err)
	}

	return hash, nil
}

func (s *S3Storage) Retrieve(ctx context.Context, hash string) (io.ReadCloser, error) {
	key := s.getObjectKey(hash)
	
	output, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	
	if err != nil {
		return nil, fmt.Errorf("failed to get object from s3: %w", err)
	}
	
	return output.Body, nil
}

func (s *S3Storage) Exists(ctx context.Context, hash string) (bool, error) {
	key := s.getObjectKey(hash)
	_, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return false, nil
	}
	return true, nil
}

func (s *S3Storage) Delete(ctx context.Context, hash string) error {
	key := s.getObjectKey(hash)
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("failed to delete object from s3: %w", err)
	}
	return nil
}
