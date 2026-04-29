package service

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

const MarketplaceCredentialSecretEnv = "MARKETPLACE_CREDENTIAL_SECRET"

var errMarketplaceCredentialSecretRequired = errors.New("marketplace credential secret is required")

func GetMarketplaceCredentialSecret() (string, error) {
	secret := strings.TrimSpace(os.Getenv(MarketplaceCredentialSecretEnv))
	if secret == "" {
		return "", errMarketplaceCredentialSecretRequired
	}
	return secret, nil
}

func EncryptMarketplaceAPIKey(plaintext string, secret string) (string, error) {
	plaintext = strings.TrimSpace(plaintext)
	if plaintext == "" {
		return "", errors.New("marketplace api key is required")
	}
	key, err := marketplaceCryptoKey(secret, "marketplace-api-key-encryption")
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := aead.Seal(nil, nonce, []byte(plaintext), nil)
	payload := append(nonce, ciphertext...)
	return "v1:" + base64.RawURLEncoding.EncodeToString(payload), nil
}

func DecryptMarketplaceAPIKey(encrypted string, secret string) (string, error) {
	if !strings.HasPrefix(encrypted, "v1:") {
		return "", errors.New("unsupported marketplace api key ciphertext version")
	}
	key, err := marketplaceCryptoKey(secret, "marketplace-api-key-encryption")
	if err != nil {
		return "", err
	}

	payload, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(encrypted, "v1:"))
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(payload) <= aead.NonceSize() {
		return "", errors.New("invalid marketplace api key ciphertext")
	}

	nonce := payload[:aead.NonceSize()]
	ciphertext := payload[aead.NonceSize():]
	plaintext, err := aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

func FingerprintMarketplaceAPIKey(plaintext string, secret string) (string, error) {
	plaintext = strings.TrimSpace(plaintext)
	if plaintext == "" {
		return "", errors.New("marketplace api key is required")
	}
	key, err := marketplaceCryptoKey(secret, "marketplace-api-key-fingerprint")
	if err != nil {
		return "", err
	}

	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(plaintext))
	return "hmac-sha256:" + hex.EncodeToString(mac.Sum(nil)), nil
}

func marketplaceCryptoKey(secret string, purpose string) ([]byte, error) {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return nil, errMarketplaceCredentialSecretRequired
	}
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s\x00%s", purpose, secret)))
	return sum[:], nil
}
