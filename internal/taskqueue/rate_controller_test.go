package taskqueue

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestRateControllerBacksOffAndRecovers(t *testing.T) {
	pacer := NewUniformPacer(time.Second)
	c := NewRateController(pacer, 100, 20, 400)

	if c.Rate() != 100 {
		t.Fatalf("start rate %d", c.Rate())
	}

	c.RateLimited()
	if c.Rate() != 50 {
		t.Fatalf("expected halve to 50, got %d", c.Rate())
	}

	c.RateLimited()
	c.RateLimited()
	if c.Rate() != 50 {
		t.Fatalf("burst of 429s inside cooldown must cut once, got %d", c.Rate())
	}

	for i := 0; i < 1000; i++ {
		c.Success()
	}
	if c.Rate() != 50 {
		t.Fatalf("must not probe upward during probe backoff, got %d", c.Rate())
	}

	c.mutex.Lock()
	c.lastCut = time.Now().Add(-time.Minute)
	c.mutex.Unlock()
	for i := 0; i < 2000; i++ {
		c.Success()
	}
	if c.Rate() != 400 {
		t.Fatalf("expected recovery to max 400, got %d", c.Rate())
	}
}

func TestRateControllerFloorAndCeiling(t *testing.T) {
	c := NewRateController(NewUniformPacer(time.Second), 30, 25, 40)
	for i := 0; i < 5; i++ {
		c.RateLimited()
		c.mutex.Lock()
		c.lastCut = time.Now().Add(-time.Minute)
		c.mutex.Unlock()
	}
	if c.Rate() != 25 {
		t.Fatalf("floor broken: %d", c.Rate())
	}
}

func TestAdaptiveBeatsFixedAgainstHiddenLimit(t *testing.T) {
	const hiddenLimitPerSec = 40.0
	const total = 500

	run := func(queue *SmoothQueue[int], controller *RateController) (time.Duration, int64) {
		var mu sync.Mutex
		var tokens = hiddenLimitPerSec
		last := time.Now()
		var limited atomic.Int64

		hit := func() bool {
			mu.Lock()
			defer mu.Unlock()
			now := time.Now()
			tokens = min(hiddenLimitPerSec, tokens+now.Sub(last).Seconds()*hiddenLimitPerSec)
			last = now
			if tokens < 1 {
				return false
			}
			tokens--
			return true
		}

		start := time.Now()
		var wg sync.WaitGroup
		for i := 0; i < total; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for {
					res := <-queue.QueueTask(func() (int, error) {
						time.Sleep(5 * time.Millisecond)
						if hit() {
							return 1, nil
						}
						return 0, errLimited
					})
					if res.Error == nil {
						if controller != nil {
							controller.Success()
						}
						return
					}
					limited.Add(1)
					if controller != nil {
						controller.RateLimited()
					}
					time.Sleep(100 * time.Millisecond)
				}
			}()
		}
		wg.Wait()
		return time.Since(start), limited.Load()
	}

	fixedQueue := NewSmoothQueue[int](64, 1500)
	fixedTime, _ := run(fixedQueue, nil)

	adaptiveQueue := NewSmoothQueue[int](64, 1500)
	controller := NewRateController(adaptiveQueue.Limiter, 1500, 300, 12000)
	adaptiveTime, limited := run(adaptiveQueue, controller)

	t.Logf("fixed 1500/min: %v, adaptive: %v (rate-limited %d times, final rate %d/min, cuts %d)", fixedTime, adaptiveTime, limited, controller.Rate(), controller.Cuts())

	if adaptiveTime >= fixedTime {
		t.Fatalf("adaptive (%v) should finish before fixed (%v)", adaptiveTime, fixedTime)
	}
}

type limitedError struct{}

func (limitedError) Error() string { return "limited" }

var errLimited = limitedError{}
