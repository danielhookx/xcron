package xcron

import (
	"sync"
	"sync/atomic"
	"time"
)

// jobScheduler manages when jobs are due and pushes them to the ready channel.
// It holds the Timer, waiting schedule, and lifecycle (Start/Stop).
type jobScheduler struct {
	sync.Mutex
	running         atomic.Bool
	waitingSchedule map[Job]func()

	timer    *Timer
	ready    chan Job
	location *time.Location
}

func newJobScheduler(loc *time.Location) *jobScheduler {
	ready := make(chan Job, 10)
	return &jobScheduler{
		waitingSchedule: make(map[Job]func()),
		timer:           NewTimer(10, ready),
		ready:           ready,
		location:        loc,
	}
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
	s.timer.Set(j.td, j.schedule.Next(s.now()))
}

func (s *jobScheduler) now() time.Time {
	return time.Now().In(s.location)
}
