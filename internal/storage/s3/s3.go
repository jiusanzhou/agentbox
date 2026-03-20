package s3

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"go.zoe.im/agentbox/internal/storage"
	"go.zoe.im/x"
)

// Config for S3-compatible object storage.
type Config struct {
	Endpoint  string `json:"endpoint" yaml:"endpoint"`
	Bucket    string `json:"bucket" yaml:"bucket"`
	Region    string `json:"region" yaml:"region"`
	AccessKey string `json:"access_key" yaml:"access_key"`
	SecretKey string `json:"secret_key" yaml:"secret_key"`
}

func init() {
	storage.Register("s3", func(cfg x.TypedLazyConfig, opts ...any) (storage.Storage, error) {
		var c Config
		if err := cfg.Unmarshal(&c); err != nil {
			return nil, err
		}
		return New(c)
	})
}

type s3Storage struct {
	client        *s3.Client
	presignClient *s3.PresignClient
	bucket        string
}

// New creates an S3-backed storage using aws-sdk-go-v2.
func New(cfg Config) (storage.Storage, error) {
	if cfg.Bucket == "" {
		return nil, fmt.Errorf("s3: bucket is required")
	}
	if cfg.Region == "" {
		cfg.Region = "us-east-1"
	}

	var optFns []func(*awsconfig.LoadOptions) error

	optFns = append(optFns, awsconfig.WithRegion(cfg.Region))

	if cfg.AccessKey != "" && cfg.SecretKey != "" {
		optFns = append(optFns, awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, ""),
		))
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(), optFns...)
	if err != nil {
		return nil, fmt.Errorf("s3: failed to load AWS config: %w", err)
	}

	var s3OptFns []func(*s3.Options)
	if cfg.Endpoint != "" {
		s3OptFns = append(s3OptFns, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
			o.UsePathStyle = true // Required for MinIO / R2 / local S3-compatible stores.
		})
	}

	client := s3.NewFromConfig(awsCfg, s3OptFns...)
	presignClient := s3.NewPresignClient(client)

	return &s3Storage{
		client:        client,
		presignClient: presignClient,
		bucket:        cfg.Bucket,
	}, nil
}

// Upload writes the contents of reader to the given key.
// It detects the content type by reading the first 512 bytes.
func (s *s3Storage) Upload(ctx context.Context, key string, reader io.Reader) error {
	// Read the first 512 bytes for content-type detection.
	buf := make([]byte, 512)
	n, err := io.ReadAtLeast(reader, buf, 1)
	if err != nil && err != io.ErrUnexpectedEOF {
		return fmt.Errorf("s3: failed to read for content detection: %w", err)
	}
	buf = buf[:n]

	contentType := http.DetectContentType(buf)

	// Recombine the peeked bytes with the remaining reader.
	body := io.MultiReader(bytes.NewReader(buf), reader)

	_, err = s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		Body:        body,
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return fmt.Errorf("s3: upload %q failed: %w", key, err)
	}
	return nil
}

// Download returns a ReadCloser for the object at key.
func (s *s3Storage) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("s3: download %q failed: %w", key, err)
	}
	return out.Body, nil
}

// PresignedURL returns a pre-signed GET URL valid for 15 minutes.
func (s *s3Storage) PresignedURL(ctx context.Context, key string) (string, error) {
	out, err := s.presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(15*time.Minute))
	if err != nil {
		return "", fmt.Errorf("s3: presign %q failed: %w", key, err)
	}
	return out.URL, nil
}

// Delete removes the object at key.
func (s *s3Storage) Delete(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("s3: delete %q failed: %w", key, err)
	}
	return nil
}

// List returns all object keys under the given prefix.
func (s *s3Storage) List(ctx context.Context, prefix string) ([]string, error) {
	var keys []string
	paginator := s3.NewListObjectsV2Paginator(s.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
		Prefix: aws.String(prefix),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("s3: list prefix %q failed: %w", prefix, err)
		}
		for _, obj := range page.Contents {
			keys = append(keys, aws.ToString(obj.Key))
		}
	}
	return keys, nil
}
