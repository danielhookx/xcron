package xcron

import (
	"context"
	"time"
)

// PacingStrategy controls the trigger pace for task sampling
type PacingStrategy interface {
	// WaitForNext blocks until the next sample can be triggered.
	// Returns false if ctx is canceled, caller should exit.
	WaitForNext(ctx context.Context) bool
	// Stop releases resources held by the strategy (e.g. ticker)
	Stop()
}

// fullSpeedStrategy: full speed mode, returns immediately without waiting
type fullSpeedStrategy struct{}

func (s *fullSpeedStrategy) WaitForNext(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return false
	default:
		return true
	}
}

func (s *fullSpeedStrategy) Stop() {}

// fixedRateStrategy: fixed rate mode, waits at rate interval
type fixedRateStrategy struct {
	ticker *time.Ticker
}

func (s *fixedRateStrategy) WaitForNext(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return false
	case <-s.ticker.C:
		return true
	}
}

func (s *fixedRateStrategy) Stop() {
	if s.ticker != nil {
		s.ticker.Stop()
	}
}

// NewPacingStrategy creates a strategy based on rate: rate <= 0 for full speed, rate > 0 for fixed rate
func NewPacingStrategy(rate int) PacingStrategy {
	if rate > 0 {
		return &fixedRateStrategy{
			ticker: time.NewTicker(time.Second / time.Duration(rate)),
		}
	}
	return &fullSpeedStrategy{}
}
