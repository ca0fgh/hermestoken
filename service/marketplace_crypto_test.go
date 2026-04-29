package service

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMarketplaceAPIKeyEncryptionRoundTrip(t *testing.T) {
	secret := "0123456789abcdef0123456789abcdef"
	plaintext := "sk-test-marketplace-secret"

	encrypted, err := EncryptMarketplaceAPIKey(plaintext, secret)
	require.NoError(t, err)
	require.NotEmpty(t, encrypted)
	assert.True(t, strings.HasPrefix(encrypted, "v1:"))
	assert.NotContains(t, encrypted, plaintext)

	decrypted, err := DecryptMarketplaceAPIKey(encrypted, secret)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)
}

func TestMarketplaceAPIKeyEncryptionUsesRandomNonce(t *testing.T) {
	secret := "0123456789abcdef0123456789abcdef"
	plaintext := "sk-test-marketplace-secret"

	first, err := EncryptMarketplaceAPIKey(plaintext, secret)
	require.NoError(t, err)
	second, err := EncryptMarketplaceAPIKey(plaintext, secret)
	require.NoError(t, err)

	assert.NotEqual(t, first, second)
}

func TestMarketplaceAPIKeyDecryptRejectsWrongSecret(t *testing.T) {
	encrypted, err := EncryptMarketplaceAPIKey("sk-test-marketplace-secret", "correct-secret")
	require.NoError(t, err)

	_, err = DecryptMarketplaceAPIKey(encrypted, "wrong-secret")
	require.Error(t, err)
}

func TestMarketplaceAPIKeyFingerprintIsKeyedAndStable(t *testing.T) {
	plaintext := "sk-test-marketplace-secret"

	first, err := FingerprintMarketplaceAPIKey(plaintext, "first-secret")
	require.NoError(t, err)
	second, err := FingerprintMarketplaceAPIKey(plaintext, "first-secret")
	require.NoError(t, err)
	third, err := FingerprintMarketplaceAPIKey(plaintext, "second-secret")
	require.NoError(t, err)

	assert.Equal(t, first, second)
	assert.NotEqual(t, first, third)
	assert.NotContains(t, first, plaintext)
	assert.True(t, strings.HasPrefix(first, "hmac-sha256:"))
}

func TestMarketplaceAPIKeyCryptoRejectsMissingInputs(t *testing.T) {
	_, err := EncryptMarketplaceAPIKey("", "secret")
	require.Error(t, err)
	_, err = EncryptMarketplaceAPIKey("sk-test", "")
	require.Error(t, err)
	_, err = FingerprintMarketplaceAPIKey("sk-test", "")
	require.Error(t, err)
}
