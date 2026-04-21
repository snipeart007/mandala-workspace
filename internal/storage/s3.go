// Package storage provides the Content-Addressable Storage (CAS) interfaces and implementations.
// This file implements an Amazon S3-compatible storage backend for the Mandala project.
package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
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
	slog.Info("Initializing S3 storage", "bucket", bucket, "prefix", prefix)
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
	slog.Debug("Storing content to S3", "bucket", s.bucket)
	
	tmpFile, err := os.CreateTemp("", "s3-upload-*")
	if err != nil {
		slog.Error("Failed to create local temp file for S3 upload", "error", err)
		return "", fmt.Errorf("failed to create local temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	hasher := sha256.New()
	mw := io.MultiWriter(tmpFile, hasher)

	if _, err := io.Copy(mw, r); err != nil {
		slog.Error("Failed to buffer content to temp file for S3 upload", "error", err)
		return "", fmt.Errorf("failed to buffer to temp file: %w", err)
	}

	hash := hex.EncodeToString(hasher.Sum(nil))
	key := s.getObjectKey(hash)

	// Check if it already exists in S3 to avoid redundant upload
	exists, _ := s.Exists(ctx, hash)
	if exists {
		slog.Debug("Content already exists in S3, skipping upload", "hash", hash, "key", key)
		return hash, nil
	}

	// Seek back to beginning of temp file for upload
	if _, err := tmpFile.Seek(0, 0); err != nil {
		slog.Error("Failed to seek temp file for S3 upload", "error", err)
		return "", fmt.Errorf("failed to seek temp file: %w", err)
	}

	_, err = s.tm.UploadObject(ctx, &transfermanager.UploadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
		Body:   tmpFile,
	})

	if err != nil {
		slog.Error("Failed to upload object to S3", "bucket", s.bucket, "key", key, "error", err)
		return "", fmt.Errorf("failed to upload to s3: %w", err)
	}

	slog.Info("Content stored in S3", "hash", hash, "bucket", s.bucket, "key", key)
	return hash, nil
}

func (s *S3Storage) Retrieve(ctx context.Context, hash string) (io.ReadCloser, error) {
	key := s.getObjectKey(hash)
	slog.Debug("Retrieving content from S3", "bucket", s.bucket, "key", key)
	
	output, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	
	if err != nil {
		slog.Error("Failed to get object from S3", "bucket", s.bucket, "key", key, "error", err)
		return nil, fmt.Errorf("failed to get object from s3: %w", err)
	}
	
	return output.Body, nil
}

func (s *S3Storage) Exists(ctx context.Context, hash string) (bool, error) {
	key := s.getObjectKey(hash)
	slog.Debug("Checking if content exists in S3", "bucket", s.bucket, "key", key)
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
	slog.Info("Deleting content from S3", "bucket", s.bucket, "key", key)
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		slog.Error("Failed to delete object from S3", "bucket", s.bucket, "key", key, "error", err)
		return fmt.Errorf("failed to delete object from s3: %w", err)
	}
	return nil
}
