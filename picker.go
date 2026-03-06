package xcron

import (
	"context"
	"time"
)

// Picker selects ready jobs from the scheduler.
// Similar to io.Reader: pull-based, may return fewer than requested.
type Picker interface {
	// Pick returns up to size ready jobs. Blocks until at least one is available
	// or ctx is done. May return fewer than size if deadline exceeded.
	// Returns (nil, ctx.Err()) when ctx is canceled.
	Pick(ctx context.Context, size int) ([]Job, error)
}

func warpJob(picker Picker) func(Schedule, Job) (Job, CancelHandler) {
	return func(schedule Schedule, j Job) (Job, CancelHandler) {
		p, ok := picker.(*defaultPicker)
		if !ok {
			return j, func() {}
		}

		wj := &job{
			scheduler: p.scheduler,
			next:      j,
			schedule:  schedule,
			td:        &TimerData{},
		}
		p.scheduler.add(wj)
		return wj, func() {
			p.scheduler.del(wj)
		}
	}
}

type job struct {
	scheduler *jobScheduler
	next      Job
	td        *TimerData
	schedule  Schedule
}

func (j *job) Run() {
	j.scheduler.redo(j)
	j.next.Run()
}

func (j *job) add() {
	now := j.scheduler.now()
	next := j.schedule.Next(now)
	if next.Before(now) {
		return
	}
	j.td = j.scheduler.timer.Add(next, j)
}

func (j *job) remove() {
	j.scheduler.timer.Del(j.td)
	j.td = nil
}

const defaultPickTimeout = 5 * time.Millisecond

// defaultPicker implements Picker by reading from the scheduler's ready channel.
// It delegates scheduling (when to trigger) to jobScheduler and only handles
// selection (which/how many jobs to return).
type defaultPicker struct {
	scheduler   *jobScheduler
	pickTimeout time.Duration
}

func newPicker(loc *time.Location) *defaultPicker {
	return newPickerWithTimeout(loc, 0)
}

func newPickerWithTimeout(loc *time.Location, timeout time.Duration) *defaultPicker {
	if timeout <= 0 {
		timeout = defaultPickTimeout
	}
	return &defaultPicker{
		scheduler:   newJobScheduler(loc),
		pickTimeout: timeout,
	}
}

func (p *defaultPicker) Pick(ctx context.Context, size int) ([]Job, error) {
	jobs := make([]Job, 0, size)
	for len(jobs) < size {
		pickCtx, cancel := context.WithTimeout(ctx, p.pickTimeout)
		select {
		case j := <-p.scheduler.ready:
			cancel()
			if j != nil {
				jobs = append(jobs, j)
			}
		case <-pickCtx.Done():
			cancel()
			if len(jobs) > 0 {
				return jobs, nil
			}
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			// pickCtx deadline exceeded, parent ctx still valid: retry
			continue
		}
	}
	return jobs, nil
}

func (p *defaultPicker) Start() error {
	return p.scheduler.Start()
}

func (p *defaultPicker) Stop() error {
	return p.scheduler.Stop()
}
