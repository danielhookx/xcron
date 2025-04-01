package xcron

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

type Picker interface {
	PickSize(ctx context.Context, size int) ([]Job, int)
}

func warpJob(picker Picker) func(Schedule, Job) (Job, CancelHandler) {
	return func(schedule Schedule, j Job) (Job, CancelHandler) {
		p, ok := picker.(*defaultPicker)
		if !ok {
			return j, func() {}
		}

		wj := &job{
			picker:   p,
			next:     j,
			schedule: schedule,
			td:       &TimerData{},
		}
		p.add(wj)
		return wj, func() {
			p.del(wj)
		}
	}
}

type job struct {
	picker   *defaultPicker
	next     Job
	td       *TimerData
	schedule Schedule
}

func (j *job) Run() {
	j.picker.redo(j)
	j.next.Run()
}

func (j *job) add() {
	now := j.picker.now()
	next := j.schedule.Next(now)
	if next.Before(now) {
		return
	}
	j.td = j.picker.timer.Add(next, j)
}

func (j *job) remove() {
	j.picker.timer.Del(j.td)
	j.td = nil
}

type defaultPicker struct {
	sync.Mutex
	running          atomic.Bool
	waittingSchedule map[Job]func()

	timer    *Timer
	ready    chan Job
	location *time.Location
}

func newPicker(loc *time.Location) *defaultPicker {
	ready := make(chan Job, 10)
	return &defaultPicker{
		waittingSchedule: make(map[Job]func()),
		timer:            NewTimer(10, ready),
		ready:            ready,
		location:         loc,
	}
}

func (s *defaultPicker) PickSize(ctx context.Context, size int) ([]Job, int) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Millisecond)
	defer cancel()

	jobs := make([]Job, 0, size)
	for i := 0; i < size; {
		select {
		case job := <-s.ready:
			if job != nil {
				jobs = append(jobs, job)
			}
		case <-ctx.Done():
			if ctx.Err() == context.DeadlineExceeded {
				if len(jobs) == 0 {
					continue
				}
			}
			if ctx.Err() == context.Canceled {
				return nil, -1
			}
			break // ctx.Err() == context.Canceled
		}
		i++
	}
	return jobs, len(jobs)
}

func (s *defaultPicker) Start() error {
	if s.running.CompareAndSwap(false, true) {
		s.Lock()
		defer s.Unlock()
		for _, fn := range s.waittingSchedule {
			fn()
		}
	}
	return nil
}

func (s *defaultPicker) Stop() error {
	if s.running.CompareAndSwap(true, false) {
		s.Lock()
		defer s.Unlock()
		s.waittingSchedule = make(map[Job]func())
	}
	return nil
}

func (s *defaultPicker) add(j *job) error {
	if !s.running.Load() {
		s.Lock()
		defer s.Unlock()
		s.waittingSchedule[j] = func() {
			j.add()
		}
		return nil
	}
	j.add()
	return nil
}

func (s *defaultPicker) del(j *job) {
	s.Lock()
	delete(s.waittingSchedule, j)
	s.Unlock()

	j.remove()
}

func (s *defaultPicker) redo(j *job) {
	s.timer.Set(j.td, j.schedule.Next(s.now()))
}

func (s *defaultPicker) now() time.Time {
	return time.Now().In(s.location)
}
