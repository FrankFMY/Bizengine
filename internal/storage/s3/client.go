package s3

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// Client wraps the AWS S3 SDK for file operations.
type Client struct {
	s3     *s3.Client
	presig *s3.PresignClient
	bucket string
}

// Config holds S3 connection parameters.
type Config struct {
	Endpoint  string // e.g. "http://localhost:9000" for MinIO
	Bucket    string
	Region    string // default: "us-east-1"
	AccessKey string
	SecretKey string
	UseSSL    bool
}

// NewClient creates a new S3 client.
func NewClient(ctx context.Context, cfg Config) (*Client, error) {
	region := cfg.Region
	if region == "" {
		region = "us-east-1"
	}

	resolver := aws.EndpointResolverWithOptionsFunc(
		func(service, resolvedRegion string, options ...interface{}) (aws.Endpoint, error) {
			if cfg.Endpoint != "" {
				return aws.Endpoint{
					URL:               cfg.Endpoint,
					HostnameImmutable: true,
				}, nil
			}
			return aws.Endpoint{}, &aws.EndpointNotFoundError{}
		},
	)

	awsCfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(region),
		config.WithEndpointResolverWithOptions(resolver),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			cfg.AccessKey, cfg.SecretKey, "",
		)),
	)
	if err != nil {
		return nil, fmt.Errorf("s3: load config: %w", err)
	}

	s3Client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.UsePathStyle = cfg.Endpoint != "" // MinIO requires path-style
	})

	return &Client{
		s3:     s3Client,
		presig: s3.NewPresignClient(s3Client),
		bucket: cfg.Bucket,
	}, nil
}

// PresignedPutURL generates a presigned URL for uploading a file.
func (c *Client) PresignedPutURL(ctx context.Context, key, contentType string, ttl time.Duration) (string, error) {
	if ttl == 0 {
		ttl = 5 * time.Minute
	}

	input := &s3.PutObjectInput{
		Bucket:      &c.bucket,
		Key:         &key,
		ContentType: &contentType,
	}

	resp, err := c.presig.PresignPutObject(ctx, input, s3.WithPresignExpires(ttl))
	if err != nil {
		return "", fmt.Errorf("s3: presign PUT: %w", err)
	}

	return resp.URL, nil
}

// PresignedGetURL generates a presigned URL for downloading a file.
func (c *Client) PresignedGetURL(ctx context.Context, key string, ttl time.Duration) (string, error) {
	if ttl == 0 {
		ttl = 15 * time.Minute
	}

	input := &s3.GetObjectInput{
		Bucket: &c.bucket,
		Key:    &key,
	}

	resp, err := c.presig.PresignGetObject(ctx, input, s3.WithPresignExpires(ttl))
	if err != nil {
		return "", fmt.Errorf("s3: presign GET: %w", err)
	}

	return resp.URL, nil
}

// Delete removes an object from the bucket.
func (c *Client) Delete(ctx context.Context, key string) error {
	_, err := c.s3.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: &c.bucket,
		Key:    &key,
	})
	if err != nil {
		return fmt.Errorf("s3: delete %s: %w", key, err)
	}
	return nil
}

// HeadObject checks if an object exists and returns its metadata.
func (c *Client) HeadObject(ctx context.Context, key string) (*ObjectMeta, error) {
	resp, err := c.s3.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: &c.bucket,
		Key:    &key,
	})
	if err != nil {
		return nil, fmt.Errorf("s3: head %s: %w", key, err)
	}
	return &ObjectMeta{
		ContentType:   aws.ToString(resp.ContentType),
		ContentLength: aws.ToInt64(resp.ContentLength),
		LastModified:  aws.ToTime(resp.LastModified),
	}, nil
}

// Upload uploads data directly (for server-side uploads, not presigned).
func (c *Client) Upload(ctx context.Context, key, contentType string, body io.Reader) error {
	_, err := c.s3.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      &c.bucket,
		Key:         &key,
		ContentType: &contentType,
		Body:        body,
	})
	if err != nil {
		return fmt.Errorf("s3: upload %s: %w", key, err)
	}
	return nil
}

// EnsureBucket creates the bucket if it doesn't exist.
func (c *Client) EnsureBucket(ctx context.Context) error {
	_, err := c.s3.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: &c.bucket})
	if err != nil {
		_, createErr := c.s3.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: &c.bucket})
		if createErr != nil {
			return fmt.Errorf("s3: create bucket %s: %w", c.bucket, createErr)
		}
	}
	return nil
}

// ObjectMeta holds metadata about an S3 object.
type ObjectMeta struct {
	ContentType   string
	ContentLength int64
	LastModified  time.Time
}

// HealthCheck verifies connectivity to S3.
func (c *Client) HealthCheck(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, err := c.s3.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: &c.bucket})
	return err
}
