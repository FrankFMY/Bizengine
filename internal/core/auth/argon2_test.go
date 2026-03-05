package auth

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHashSecret_Format(t *testing.T) {
	hash, err := HashSecret("testpassword")
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(hash, "$argon2id$"), "hash should start with $argon2id$")
	parts := strings.Split(hash, "$")
	assert.Equal(t, 6, len(parts), "hash should have 6 parts separated by $")
}

func TestVerifySecret_Correct(t *testing.T) {
	hash, err := HashSecret("correct-password")
	require.NoError(t, err)

	ok, err := VerifySecret("correct-password", hash)
	require.NoError(t, err)
	assert.True(t, ok, "correct password should verify")
}

func TestVerifySecret_Wrong(t *testing.T) {
	hash, err := HashSecret("correct-password")
	require.NoError(t, err)

	ok, err := VerifySecret("wrong-password", hash)
	require.NoError(t, err)
	assert.False(t, ok, "wrong password should not verify")
}

func TestVerifySecret_InvalidFormat(t *testing.T) {
	_, err := VerifySecret("anything", "not-a-valid-hash")
	assert.Error(t, err)
}

func TestHashSecret_UniquePerCall(t *testing.T) {
	h1, err := HashSecret("same-password")
	require.NoError(t, err)
	h2, err := HashSecret("same-password")
	require.NoError(t, err)
	assert.NotEqual(t, h1, h2, "each hash should use a unique salt")
}
