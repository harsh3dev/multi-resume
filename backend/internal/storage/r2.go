// Package storage is the Cloudflare R2 (S3-compatible) client. Objects are
// private; access is via presigned URLs with 60s expiry.
package storage

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// Client wraps S3 operations against one R2 bucket.
type Client struct {
	s3       *s3.Client
	presign  *s3.PresignClient
	bucket   string
}

// Config carries R2 credentials (from env; never logged).
type Config struct {
	AccountID       string
	AccessKeyID     string
	SecretAccessKey string
	Bucket          string
}

// New builds a client. It performs no network calls.
func New(cfg Config) (*Client, error) {
	if cfg.AccountID == "" || cfg.AccessKeyID == "" || cfg.SecretAccessKey == "" || cfg.Bucket == "" {
		return nil, fmt.Errorf("r2 configuration incomplete")
	}
	awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(),
		awsconfig.WithRegion("auto"),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, "")),
	)
	if err != nil {
		return nil, fmt.Errorf("r2 client init failed")
	}
	s3c := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(fmt.Sprintf("https://%s.r2.cloudflarestorage.com", cfg.AccountID))
	})
	return &Client{s3: s3c, presign: s3.NewPresignClient(s3c), bucket: cfg.Bucket}, nil
}

// PresignGet returns a 60-second presigned GET URL for key.
func (c *Client) PresignGet(ctx context.Context, key string) (string, error) {
	out, err := c.presign.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(c.bucket), Key: aws.String(key),
	}, s3.WithPresignExpires(60*time.Second))
	if err != nil {
		return "", fmt.Errorf("presign failed")
	}
	return out.URL, nil
}

// PresignPut returns a 60-second presigned PUT URL for key.
func (c *Client) PresignPut(ctx context.Context, key, contentType string) (string, error) {
	out, err := c.presign.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(c.bucket), Key: aws.String(key), ContentType: aws.String(contentType),
	}, s3.WithPresignExpires(60*time.Second))
	if err != nil {
		return "", fmt.Errorf("presign failed")
	}
	return out.URL, nil
}

// PutObject stores data at key.
func (c *Client) PutObject(ctx context.Context, key, contentType string, data []byte) error {
	_, err := c.s3.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(c.bucket), Key: aws.String(key),
		ContentType: aws.String(contentType), Body: strings.NewReader(string(data)),
	})
	if err != nil {
		return fmt.Errorf("upload failed")
	}
	return nil
}

// GetObject fetches key's contents.
func (c *Client) GetObject(ctx context.Context, key string) ([]byte, error) {
	out, err := c.s3.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(c.bucket), Key: aws.String(key)})
	if err != nil {
		return nil, fmt.Errorf("fetch failed")
	}
	defer out.Body.Close()
	var b strings.Builder
	buf := make([]byte, 32*1024)
	for {
		n, err := out.Body.Read(buf)
		b.Write(buf[:n])
		if err != nil {
			break
		}
	}
	return []byte(b.String()), nil
}

// DeleteByPrefix deletes every object under prefix (e.g. "resume/<id>/").
func (c *Client) DeleteByPrefix(ctx context.Context, prefix string) error {
	var token *string
	for {
		out, err := c.s3.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
			Bucket: aws.String(c.bucket), Prefix: aws.String(prefix), ContinuationToken: token,
		})
		if err != nil {
			return fmt.Errorf("list failed")
		}
		for _, obj := range out.Contents {
			if _, err := c.s3.DeleteObject(ctx, &s3.DeleteObjectInput{
				Bucket: aws.String(c.bucket), Key: obj.Key,
			}); err != nil {
				return fmt.Errorf("delete failed")
			}
		}
		if out.IsTruncated == nil || !*out.IsTruncated {
			return nil
		}
		token = out.NextContinuationToken
	}
}
