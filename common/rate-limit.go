package common

import (
	"sync"
	"time"
)

type InMemoryRateLimiter struct {
	store              map[string]*[]int64
	mutex              sync.Mutex
	expirationDuration time.Duration
	lastCleanup        int64
}

func (l *InMemoryRateLimiter) Init(expirationDuration time.Duration) {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	if l.store == nil {
		l.store = make(map[string]*[]int64)
	}
	l.expirationDuration = expirationDuration
}

func (l *InMemoryRateLimiter) deleteExpiredItemsLocked(expirationDuration time.Duration) {
	if expirationDuration <= 0 || l.store == nil {
		return
	}
	now := time.Now().Unix()
	if l.lastCleanup > 0 && now-l.lastCleanup < int64(expirationDuration.Seconds()) {
		return
	}
	l.lastCleanup = now
	for key := range l.store {
		queue := l.store[key]
		size := len(*queue)
		if size == 0 || now-(*queue)[size-1] > int64(expirationDuration.Seconds()) {
			delete(l.store, key)
		}
	}
}

// Request parameter duration's unit is seconds
func (l *InMemoryRateLimiter) Request(key string, maxRequestNum int, duration int64) bool {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	if l.store == nil {
		l.store = make(map[string]*[]int64)
	}
	l.deleteExpiredItemsLocked(l.expirationDuration)
	// [old <-- new]
	queue, ok := l.store[key]
	now := time.Now().Unix()
	if ok {
		if len(*queue) < maxRequestNum {
			*queue = append(*queue, now)
			return true
		} else {
			if now-(*queue)[0] >= duration {
				*queue = (*queue)[1:]
				*queue = append(*queue, now)
				return true
			} else {
				return false
			}
		}
	} else {
		s := make([]int64, 0, maxRequestNum)
		l.store[key] = &s
		*(l.store[key]) = append(*(l.store[key]), now)
	}
	return true
}
