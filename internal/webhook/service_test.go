package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestComputeHMAC(t *testing.T) {
	payload := []byte(`{"event":"test"}`)
	secret := "my-secret"

	sig := computeHMAC(payload, secret)

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	expected := hex.EncodeToString(mac.Sum(nil))

	assert.Equal(t, expected, sig)
}

func TestComputeHMAC_DifferentSecrets(t *testing.T) {
	payload := []byte(`{"event":"test"}`)

	sig1 := computeHMAC(payload, "secret-1")
	sig2 := computeHMAC(payload, "secret-2")

	assert.NotEqual(t, sig1, sig2)
}
