package crypto_payment

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeLockStore struct {
	owner string
}

func (s *fakeLockStore) SetNX(ctx context.Context, key string, value string, ttl time.Duration) (bool, error) {
	if s.owner != "" {
		return false, nil
	}
	s.owner = value
	return true, nil
}

func (s *fakeLockStore) Eval(ctx context.Context, script string, keys []string, args ...interface{}) (interface{}, error) {
	if len(args) >= 1 && s.owner == args[0].(string) {
		return int64(1), nil
	}
	return int64(0), nil
}

func TestScannerLockAcquireAndRenew(t *testing.T) {
	store := &fakeLockStore{}
	lock := NewScannerLock(store, "crypto:scanner:test", "owner-a", 30*time.Second)
	acquired, err := lock.Acquire(context.Background())
	require.NoError(t, err)
	assert.True(t, acquired)
	renewed, err := lock.Renew(context.Background())
	require.NoError(t, err)
	assert.True(t, renewed)
}

func TestScannerLockRejectsSecondOwner(t *testing.T) {
	store := &fakeLockStore{}
	first := NewScannerLock(store, "crypto:scanner:test", "owner-a", 30*time.Second)
	second := NewScannerLock(store, "crypto:scanner:test", "owner-b", 30*time.Second)
	acquired, err := first.Acquire(context.Background())
	require.NoError(t, err)
	assert.True(t, acquired)
	acquired, err = second.Acquire(context.Background())
	require.NoError(t, err)
	assert.False(t, acquired)
}
