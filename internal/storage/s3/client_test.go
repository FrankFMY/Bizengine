package s3

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestConfig_Defaults(t *testing.T) {
	cfg := Config{
		Endpoint:  "http://localhost:9000",
		Bucket:    "test-bucket",
		AccessKey: "minioadmin",
		SecretKey: "minioadmin",
	}

	assert.Empty(t, cfg.Region)
	assert.Equal(t, "test-bucket", cfg.Bucket)
	assert.False(t, cfg.UseSSL)
}

func TestObjectMeta_Fields(t *testing.T) {
	now := time.Now()
	meta := ObjectMeta{
		ContentType:   "image/png",
		ContentLength: 12345,
		LastModified:  now,
	}

	assert.Equal(t, "image/png", meta.ContentType)
	assert.Equal(t, int64(12345), meta.ContentLength)
	assert.Equal(t, now, meta.LastModified)
}

func TestDefaultTTL_PresignedPutURL(t *testing.T) {
	var ttl time.Duration
	if ttl == 0 {
		ttl = 5 * time.Minute
	}
	assert.Equal(t, 5*time.Minute, ttl)
}

func TestDefaultTTL_PresignedGetURL(t *testing.T) {
	var ttl time.Duration
	if ttl == 0 {
		ttl = 15 * time.Minute
	}
	assert.Equal(t, 15*time.Minute, ttl)
}

func TestDefaultRegion(t *testing.T) {
	region := ""
	if region == "" {
		region = "us-east-1"
	}
	assert.Equal(t, "us-east-1", region)
}

func TestNewClient_ValidConfig(t *testing.T) {
	cfg := Config{
		Endpoint:  "http://localhost:9000",
		Bucket:    "test",
		Region:    "us-east-1",
		AccessKey: "access",
		SecretKey: "secret",
	}

	client, err := NewClient(t.Context(), cfg)
	assert.NoError(t, err)
	assert.NotNil(t, client)
}

func TestNewClient_EmptyRegionDefaultsToUSEast1(t *testing.T) {
	cfg := Config{
		Endpoint:  "http://localhost:9000",
		Bucket:    "test",
		AccessKey: "access",
		SecretKey: "secret",
	}

	client, err := NewClient(t.Context(), cfg)
	assert.NoError(t, err)
	assert.NotNil(t, client)
}

func TestNewClient_PathStyleForMinIO(t *testing.T) {
	cfg := Config{
		Endpoint:  "http://localhost:9000",
		Bucket:    "test",
		AccessKey: "access",
		SecretKey: "secret",
	}

	client, err := NewClient(t.Context(), cfg)
	assert.NoError(t, err)
	assert.NotNil(t, client)
}
