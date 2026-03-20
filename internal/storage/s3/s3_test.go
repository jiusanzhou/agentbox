//go:build integration

package s3

import (
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

// These integration tests require a running S3-compatible service (e.g. MinIO).
//
// Environment variables:
//   S3_ENDPOINT   – e.g. http://localhost:9000
//   S3_BUCKET     – e.g. agentbox-test
//   S3_REGION     – e.g. us-east-1  (defaults to us-east-1)
//   S3_ACCESS_KEY – access key
//   S3_SECRET_KEY – secret key
//
// Run: go test -tags integration -v ./internal/storage/s3/...

func testConfig(t *testing.T) Config {
	t.Helper()
	endpoint := os.Getenv("S3_ENDPOINT")
	bucket := os.Getenv("S3_BUCKET")
	accessKey := os.Getenv("S3_ACCESS_KEY")
	secretKey := os.Getenv("S3_SECRET_KEY")

	if endpoint == "" || bucket == "" || accessKey == "" || secretKey == "" {
		t.Skip("S3_ENDPOINT, S3_BUCKET, S3_ACCESS_KEY, and S3_SECRET_KEY must be set")
	}

	region := os.Getenv("S3_REGION")
	if region == "" {
		region = "us-east-1"
	}

	return Config{
		Endpoint:  endpoint,
		Bucket:    bucket,
		Region:    region,
		AccessKey: accessKey,
		SecretKey: secretKey,
	}
}

func TestNew(t *testing.T) {
	cfg := testConfig(t)
	store, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	if store == nil {
		t.Fatal("New() returned nil")
	}
}

func TestNew_MissingBucket(t *testing.T) {
	_, err := New(Config{Region: "us-east-1"})
	if err == nil {
		t.Fatal("expected error for missing bucket")
	}
}

func TestUploadDownload(t *testing.T) {
	cfg := testConfig(t)
	store, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	key := "test/upload-download-" + time.Now().Format("20060102T150405")
	content := "hello agentbox s3 storage"

	// Upload
	err = store.Upload(ctx, key, strings.NewReader(content))
	if err != nil {
		t.Fatalf("Upload() error: %v", err)
	}

	// Download
	rc, err := store.Download(ctx, key)
	if err != nil {
		t.Fatalf("Download() error: %v", err)
	}
	defer rc.Close()

	got, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll() error: %v", err)
	}
	if string(got) != content {
		t.Errorf("Download() = %q, want %q", got, content)
	}

	// Cleanup
	_ = store.Delete(ctx, key)
}

func TestUploadBinaryContentType(t *testing.T) {
	cfg := testConfig(t)
	store, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	key := "test/binary-" + time.Now().Format("20060102T150405") + ".png"

	// Minimal valid PNG (1x1 white pixel).
	pngData := []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
		0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53,
		0xde, 0x00, 0x00, 0x00, 0x0c, 0x49, 0x44, 0x41,
		0x54, 0x08, 0xd7, 0x63, 0xf8, 0xcf, 0xc0, 0x00,
		0x00, 0x00, 0x02, 0x00, 0x01, 0xe2, 0x21, 0xbc,
		0x33, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4e,
		0x44, 0xae, 0x42, 0x60, 0x82,
	}

	err = store.Upload(ctx, key, bytes.NewReader(pngData))
	if err != nil {
		t.Fatalf("Upload() error: %v", err)
	}

	// Download and verify round-trip.
	rc, err := store.Download(ctx, key)
	if err != nil {
		t.Fatalf("Download() error: %v", err)
	}
	defer rc.Close()

	got, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll() error: %v", err)
	}
	if !bytes.Equal(got, pngData) {
		t.Errorf("round-trip mismatch: got %d bytes, want %d", len(got), len(pngData))
	}

	_ = store.Delete(ctx, key)
}

func TestList(t *testing.T) {
	cfg := testConfig(t)
	store, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	prefix := "test/list-" + time.Now().Format("20060102T150405") + "/"
	keys := []string{
		prefix + "a.txt",
		prefix + "b.txt",
		prefix + "sub/c.txt",
	}

	for _, k := range keys {
		if err := store.Upload(ctx, k, strings.NewReader("data")); err != nil {
			t.Fatalf("Upload(%s) error: %v", k, err)
		}
	}

	listed, err := store.List(ctx, prefix)
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}

	if len(listed) != len(keys) {
		t.Errorf("List() returned %d keys, want %d", len(listed), len(keys))
	}

	listedSet := make(map[string]bool, len(listed))
	for _, k := range listed {
		listedSet[k] = true
	}
	for _, k := range keys {
		if !listedSet[k] {
			t.Errorf("List() missing key %q", k)
		}
	}

	// Cleanup
	for _, k := range keys {
		_ = store.Delete(ctx, k)
	}
}

func TestDelete(t *testing.T) {
	cfg := testConfig(t)
	store, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	key := "test/delete-" + time.Now().Format("20060102T150405")

	err = store.Upload(ctx, key, strings.NewReader("to-be-deleted"))
	if err != nil {
		t.Fatalf("Upload() error: %v", err)
	}

	err = store.Delete(ctx, key)
	if err != nil {
		t.Fatalf("Delete() error: %v", err)
	}

	// Verify the object is gone.
	_, err = store.Download(ctx, key)
	if err == nil {
		t.Error("Download() after Delete() should fail, but got nil error")
	}
}

func TestPresignedURL(t *testing.T) {
	cfg := testConfig(t)
	store, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	key := "test/presign-" + time.Now().Format("20060102T150405")

	err = store.Upload(ctx, key, strings.NewReader("presigned-content"))
	if err != nil {
		t.Fatalf("Upload() error: %v", err)
	}

	url, err := store.PresignedURL(ctx, key)
	if err != nil {
		t.Fatalf("PresignedURL() error: %v", err)
	}

	if url == "" {
		t.Error("PresignedURL() returned empty string")
	}

	if !strings.HasPrefix(url, "http") {
		t.Errorf("PresignedURL() = %q, expected http(s) URL", url)
	}

	// Cleanup
	_ = store.Delete(ctx, key)
}
