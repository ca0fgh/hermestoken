package channelhealth

import (
	"sync"
	"testing"
	"time"
)

// resetForTest clears global state and installs a controllable clock.
func resetForTest(t *testing.T, base time.Time) *time.Time {
	t.Helper()
	mu.Lock()
	states = make(map[int]*breakerState)
	cfg = config{}
	mu.Unlock()
	clock := base
	nowFn = func() time.Time { return clock }
	t.Cleanup(func() {
		nowFn = time.Now
		mu.Lock()
		states = make(map[int]*breakerState)
		cfg = config{}
		mu.Unlock()
	})
	return &clock
}

func TestDisabledByDefault(t *testing.T) {
	resetForTest(t, time.Unix(1000, 0))
	// threshold 0 => disabled: failures are no-ops, never opens.
	for i := 0; i < 10; i++ {
		RecordFailure(7)
	}
	if IsOpen(7) {
		t.Fatal("breaker should never open when disabled (threshold<=0)")
	}
}

func TestTripsAfterThresholdWithinWindow(t *testing.T) {
	clock := resetForTest(t, time.Unix(1000, 0))
	Configure(3, 30*time.Second, 30*time.Second)

	RecordFailure(1)
	RecordFailure(1)
	if IsOpen(1) {
		t.Fatal("should not be open before reaching threshold")
	}
	RecordFailure(1) // 3rd within window -> trips
	if !IsOpen(1) {
		t.Fatal("should be open after threshold failures within window")
	}

	// A different channel is unaffected.
	if IsOpen(2) {
		t.Fatal("unrelated channel must not be open")
	}

	// After cooldown elapses, half-open (allows a trial).
	*clock = clock.Add(31 * time.Second)
	if IsOpen(1) {
		t.Fatal("should be half-open (false) after cooldown")
	}
}

func TestStaleFailuresAgeOut(t *testing.T) {
	clock := resetForTest(t, time.Unix(1000, 0))
	Configure(3, 30*time.Second, 30*time.Second)

	RecordFailure(1)
	RecordFailure(1)
	// Two failures, then jump past the window so they age out.
	*clock = clock.Add(31 * time.Second)
	RecordFailure(1) // only 1 failure inside the window now
	if IsOpen(1) {
		t.Fatal("stale failures should age out of the window, not trip the breaker")
	}
}

func TestSuccessClearsBreaker(t *testing.T) {
	resetForTest(t, time.Unix(1000, 0))
	Configure(2, 30*time.Second, 30*time.Second)

	RecordFailure(1)
	RecordFailure(1)
	if !IsOpen(1) {
		t.Fatal("expected open after 2 failures")
	}
	RecordSuccess(1)
	if IsOpen(1) {
		t.Fatal("success must close the breaker immediately")
	}
}

func TestConcurrentAccessIsRaceFree(t *testing.T) {
	resetForTest(t, time.Unix(1000, 0))
	Configure(5, time.Second, time.Second)
	var wg sync.WaitGroup
	for g := 0; g < 16; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				RecordFailure(id % 4)
				_ = IsOpen(id % 4)
				if i%10 == 0 {
					RecordSuccess(id % 4)
				}
			}
		}(g)
	}
	wg.Wait()
}
