package rest

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRateLimiter_AllowsUnderLimit(t *testing.T) {
	rl := newRateLimiter(5, time.Minute)

	for i := 0; i < 5; i++ {
		assert.True(t, rl.allow("192.168.1.1"))
	}
}

func TestRateLimiter_BlocksOverLimit(t *testing.T) {
	rl := newRateLimiter(3, time.Minute)

	for i := 0; i < 3; i++ {
		assert.True(t, rl.allow("192.168.1.1"))
	}
	assert.False(t, rl.allow("192.168.1.1"))
}

func TestRateLimiter_DifferentKeys(t *testing.T) {
	rl := newRateLimiter(2, time.Minute)

	assert.True(t, rl.allow("a"))
	assert.True(t, rl.allow("a"))
	assert.False(t, rl.allow("a"))

	assert.True(t, rl.allow("b"))
	assert.True(t, rl.allow("b"))
}

func TestRateLimiter_ResetsAfterWindow(t *testing.T) {
	rl := newRateLimiter(1, 50*time.Millisecond)

	assert.True(t, rl.allow("key"))
	assert.False(t, rl.allow("key"))

	time.Sleep(60 * time.Millisecond)
	assert.True(t, rl.allow("key"))
}
