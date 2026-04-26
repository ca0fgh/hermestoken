package crypto_payment

import (
	"context"
	"time"

	"github.com/go-redis/redis/v8"
)

type LockStore interface {
	SetNX(ctx context.Context, key string, value string, ttl time.Duration) (bool, error)
	Eval(ctx context.Context, script string, keys []string, args ...interface{}) (interface{}, error)
}

type ScannerLock struct {
	store LockStore
	key   string
	owner string
	ttl   time.Duration
}

type redisLockStore struct {
	client *redis.Client
}

func NewRedisLockStore(client *redis.Client) LockStore {
	return &redisLockStore{client: client}
}

func (s *redisLockStore) SetNX(ctx context.Context, key string, value string, ttl time.Duration) (bool, error) {
	return s.client.SetNX(ctx, key, value, ttl).Result()
}

func (s *redisLockStore) Eval(ctx context.Context, script string, keys []string, args ...interface{}) (interface{}, error) {
	return s.client.Eval(ctx, script, keys, args...).Result()
}

func NewScannerLock(store LockStore, key string, owner string, ttl time.Duration) *ScannerLock {
	return &ScannerLock{store: store, key: key, owner: owner, ttl: ttl}
}

func (l *ScannerLock) Acquire(ctx context.Context) (bool, error) {
	return l.store.SetNX(ctx, l.key, l.owner, l.ttl)
}

func (l *ScannerLock) Renew(ctx context.Context) (bool, error) {
	result, err := l.store.Eval(ctx, renewLockScript, []string{l.key}, l.owner, int(l.ttl.Milliseconds()))
	if err != nil {
		return false, err
	}
	switch value := result.(type) {
	case int64:
		return value == 1, nil
	case int:
		return value == 1, nil
	default:
		return false, nil
	}
}

const renewLockScript = `
if redis.call("GET", KEYS[1]) == ARGV[1] then
  redis.call("PEXPIRE", KEYS[1], ARGV[2])
  return 1
end
return 0
`
