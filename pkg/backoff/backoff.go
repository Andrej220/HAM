package backoff

import (
    "math/rand"
    "time"
)
const (
	MaxElapsed     = 5 * time.Minute
	InitialBackoff = 1 * time.Second
	MaxBackoff     = 30 * time.Second
	DialTimeout    = 4 * time.Second
)
type Backoff struct {
    current time.Duration
    max     time.Duration
    rng     *rand.Rand
}

func New(initial, max time.Duration, seed int64) *Backoff {
    return &Backoff{
        current: initial,
        max:     max,
        rng:     rand.New(rand.NewSource(seed)),
    }
}

func (b *Backoff) Next() time.Duration {
    sleep := b.current/2 + time.Duration(b.rng.Int63n(int64(b.current/2)))
    b.current *= 2
    if b.current > b.max {
        b.current = b.max
    }
    return sleep
}

func (b *Backoff) Reset(initial time.Duration) {
    b.current = initial
}

