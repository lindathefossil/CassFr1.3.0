package taskqueue

import (
	"sync"
	"time"
)

const (
	rateIncreaseFactor = 1.12
	rateDecreaseFactor = 0.5
	rateCutCooldown    = 3 * time.Second
	rateProbeBackoff   = 8 * time.Second
)

type RateController struct {
	mutex      sync.Mutex
	pacer      *UniformPacer
	rate       float64
	minRate    float64
	maxRate    float64
	probeEvery int
	streak     int
	lastCut    time.Time
	cuts       int
}

func NewRateController(pacer *UniformPacer, start, minRate, maxRate int) *RateController {
	if minRate < 1 {
		minRate = 1
	}
	if maxRate < minRate {
		maxRate = minRate
	}
	start = min(max(start, minRate), maxRate)

	c := &RateController{
		pacer:      pacer,
		rate:       float64(start),
		minRate:    float64(minRate),
		maxRate:    float64(maxRate),
		probeEvery: 20,
	}
	c.apply()
	return c
}

func (c *RateController) apply() {
	c.pacer.SetGap(time.Duration(float64(time.Minute) / c.rate))
}

func (c *RateController) Success() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if !c.lastCut.IsZero() && time.Since(c.lastCut) < rateProbeBackoff {
		return
	}

	c.streak++
	if c.streak < c.probeEvery {
		return
	}
	c.streak = 0

	next := min(c.rate*rateIncreaseFactor+1, c.maxRate)
	if next == c.rate {
		return
	}
	c.rate = next
	c.apply()
}

func (c *RateController) RateLimited() bool {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.streak = 0
	if !c.lastCut.IsZero() && time.Since(c.lastCut) < rateCutCooldown {
		return false
	}

	c.lastCut = time.Now()
	c.cuts++
	c.rate = max(c.rate*rateDecreaseFactor, c.minRate)
	c.apply()
	return true
}

func (c *RateController) Rate() int {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	return int(c.rate)
}

func (c *RateController) Cuts() int {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	return c.cuts
}
