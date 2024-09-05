package xcron

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

func WithHighLevel() *scheduleOption {
	return newScheduleOption(func(opt *ScheduleOptions) {
		opt.jobWrapper = func(sc Schedule, p Picker, j Job) (Job, CancelHandler) {
			lp, ok := p.(*LevelPicker)
			if !ok {
				return j, func() {}
			}
			return newJobLevelWrapper(lp, lp.high, sc, j)
		}
	})
}

func WithLowLevel() *scheduleOption {
	return newScheduleOption(func(opt *ScheduleOptions) {
		opt.jobWrapper = lowJobWrapperHandler
	})
}

var lowJobWrapperHandler = func(sc Schedule, p Picker, j Job) (Job, CancelHandler) {
	lp, ok := p.(*LevelPicker)
	if !ok {
		return j, func() {}
	}
	return newJobLevelWrapper(lp, lp.low, sc, j)
}

type JobLevel interface {
	Pick() <-chan Job
	Remove(*JobLevelWrapper)
	Add(*JobLevelWrapper)
	Redo(*JobLevelWrapper)
}

type JobLevelWrapper struct {
	l JobLevel

	schedule Schedule

	next Job

	td *TimerData
}

func newJobLevelWrapper(lp *LevelPicker, l JobLevel, sc Schedule, j Job) (*JobLevelWrapper, CancelHandler) {
	w := &JobLevelWrapper{
		l:        l,
		schedule: sc,
		next:     j,
		td:       &TimerData{},
	}
	lp.Add(w, func() {
		w.l.Add(w)
	})
	return w, func() {
		w.l.Remove(w)
		lp.Del(w)
	}
}

func (w *JobLevelWrapper) Run() {
	w.l.Redo(w)
	w.next.Run()
}

type LevelPicker struct {
	sync.Mutex
	running          atomic.Bool
	waittingSchedule map[Job]func()

	low  JobLevel
	high JobLevel
}

func NewLevelPicker(loc *time.Location) *LevelPicker {
	return &LevelPicker{
		waittingSchedule: make(map[Job]func(), 0),
		low:              newLowLevel(loc),
		high:             newHighLevel(loc),
	}
}

func (lp *LevelPicker) PickSize(ctx context.Context, size int) ([]Job, int) {
	ctx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()

	jobs := make([]Job, 0, size)
	for i := 0; i < size; {
		select {
		case job := <-lp.high.Pick():
			if job != nil {
				jobs = append(jobs, job)
			}
		default:
			select {
			case job := <-lp.high.Pick():
				if job != nil {
					jobs = append(jobs, job)
				}
			case job := <-lp.low.Pick():
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
		}
		i++
	}
	return jobs, len(jobs)
}

func (lp *LevelPicker) Add(j Job, fn func()) error {
	if !lp.running.Load() {
		lp.Lock()
		defer lp.Unlock()
		lp.waittingSchedule[j] = fn
		return nil
	}
	fn()
	return nil
}

func (lp *LevelPicker) Del(j Job) {
	lp.Lock()
	defer lp.Unlock()
	delete(lp.waittingSchedule, j)
}

func (lp *LevelPicker) Start() error {
	if lp.running.CompareAndSwap(false, true) {
		lp.Lock()
		defer lp.Unlock()
		for _, fn := range lp.waittingSchedule {
			fn()
		}
	}
	return nil
}

func (lp *LevelPicker) Stop() error {
	if lp.running.CompareAndSwap(true, false) {
		lp.Lock()
		defer lp.Unlock()
		lp.waittingSchedule = make(map[Job]func())
	}
	return nil
}

type highLevel struct {
	timer    *Timer
	ready    chan Job
	location *time.Location
}

func newHighLevel(location *time.Location) *highLevel {
	ready := make(chan Job, 10)
	h := &highLevel{
		timer:    NewTimer(10, ready),
		ready:    ready,
		location: location,
	}
	return h
}

func (h *highLevel) now() time.Time {
	return time.Now().In(h.location)
}

func (h *highLevel) Remove(e *JobLevelWrapper) {
	h.timer.Del(e.td)
	e.td = nil
}

func (h *highLevel) Add(e *JobLevelWrapper) {
	now := h.now()
	next := e.schedule.Next(now)
	if next.Before(now) {
		return
	}
	e.td = h.timer.Add(next, e)
}

func (h *highLevel) Redo(e *JobLevelWrapper) {
	h.timer.Set(e.td, e.schedule.Next(h.now()))
}

func (h *highLevel) Pick() <-chan Job {
	return h.ready
}

type lowLevel struct {
	timer    *Timer
	ready    chan Job
	location *time.Location
}

func newLowLevel(location *time.Location) *lowLevel {
	ready := make(chan Job, 10)
	l := &lowLevel{
		timer:    NewTimer(10, ready),
		ready:    ready,
		location: location,
	}
	return l
}

func (l *lowLevel) now() time.Time {
	return time.Now().In(l.location)
}

func (l *lowLevel) Remove(e *JobLevelWrapper) {
	l.timer.Del(e.td)
	e.td = nil
}

func (l *lowLevel) Add(e *JobLevelWrapper) {
	now := l.now()
	next := e.schedule.Next(now)
	if next.Before(now) {
		return
	}
	e.td = l.timer.Add(next, e)
}

func (l *lowLevel) Redo(e *JobLevelWrapper) {
	l.timer.Set(e.td, e.schedule.Next(l.now()))
}

func (l *lowLevel) Pick() <-chan Job {
	return l.ready
}
