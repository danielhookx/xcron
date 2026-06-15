package xcron

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

const defaultPickTimeout = 5 * time.Millisecond

type job struct {
	mu        sync.Mutex // protects td
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
	td := j.scheduler.timer.Add(next, j)
	j.mu.Lock()
	j.td = td
	j.mu.Unlock()
}

func (j *job) remove() {
	j.mu.Lock()
	td := j.td
	j.td = nil
	j.mu.Unlock()
	if td != nil {
		j.scheduler.timer.Del(td)
	}
}

var _ Scheduler = (*jobScheduler)(nil)

type JobSchedulerOptions struct {
	timeout time.Duration // pick timeout, 0 means use default
}

type JobSchedulerOption interface {
	apply(*JobSchedulerOptions)
}

type jobSchedulerOption struct {
	f func(opts *JobSchedulerOptions)
}

func (o *jobSchedulerOption) apply(opts *JobSchedulerOptions) {
	o.f(opts)
}

func newJobSchedulerOption(f func(*JobSchedulerOptions)) *jobSchedulerOption {
	return &jobSchedulerOption{
		f: f,
	}
}

func WithPickTimeout(timeout time.Duration) *jobSchedulerOption {
	return newJobSchedulerOption(func(opt *JobSchedulerOptions) {
		opt.timeout = timeout
	})
}

// jobScheduler manages when jobs are due and pushes them to the ready channel.
// It holds the Timer, waiting schedule, and lifecycle (Start/Stop).
type jobScheduler struct {
	sync.Mutex
	running         atomic.Bool
	waitingSchedule map[Job]func()

	timer    *Timer
	ready    chan Job
	location *time.Location
	timeout  time.Duration
}

func NewJobScheduler(loc *time.Location, opt ...JobSchedulerOption) *jobScheduler {
	opts := JobSchedulerOptions{
		timeout: defaultPickTimeout,
	}
	for _, o := range opt {
		o.apply(&opts)
	}
	ready := make(chan Job, 10)
	return &jobScheduler{
		waitingSchedule: make(map[Job]func()),
		timer:           NewTimer(10, ready),
		ready:           ready,
		location:        loc,
		timeout:         opts.timeout,
	}
}

func (s *jobScheduler) JobWrapper() JobWrapper {
	return func(schedule Schedule, j Job) (Job, CancelHandler) {
		wj := &job{
			scheduler: s,
			next:      j,
			schedule:  schedule,
			td:        &TimerData{},
		}
		s.add(wj)
		return wj, func() {
			s.del(wj)
		}
	}
}

// Pick implements Picker by reading from the scheduler's ready channel.
// It delegates scheduling (when to trigger) to jobScheduler and only handles
// selection (which/how many jobs to return).
func (s *jobScheduler) Pick(ctx context.Context, size int) ([]Job, error) {
	jobs := make([]Job, 0, size)
	for len(jobs) < size {
		pickCtx, cancel := context.WithTimeout(ctx, s.timeout)
		select {
		case j := <-s.ready:
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

func (s *jobScheduler) Start() error {
	if s.running.CompareAndSwap(false, true) {
		s.Lock()
		defer s.Unlock()
		for _, fn := range s.waitingSchedule {
			fn()
		}
	}
	return nil
}

func (s *jobScheduler) Stop() error {
	if s.running.CompareAndSwap(true, false) {
		s.Lock()
		defer s.Unlock()
		s.waitingSchedule = make(map[Job]func())
	}
	return nil
}

func (s *jobScheduler) add(j *job) error {
	if !s.running.Load() {
		s.Lock()
		defer s.Unlock()
		s.waitingSchedule[j] = func() {
			j.add()
		}
		return nil
	}
	j.add()
	return nil
}

func (s *jobScheduler) del(j *job) {
	s.Lock()
	delete(s.waitingSchedule, j)
	s.Unlock()

	j.remove()
}

func (s *jobScheduler) redo(j *job) {
	j.mu.Lock()
	td := j.td
	j.mu.Unlock()
	if td != nil {
		s.timer.Set(td, j.schedule.Next(s.now()))
	}
}

func (s *jobScheduler) now() time.Time {
	return time.Now().In(s.location)
}
