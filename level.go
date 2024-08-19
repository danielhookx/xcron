package xcron

import (
	"context"
	"time"
)

func WithHighLevel() *scheduleOption {
	return newScheduleOption(func(opt *ScheduleOptions) {
		opt.jobWrapper = func(sc Schedule, p Picker, j Job) Job {
			lp, ok := p.(*LevelPicker)
			if !ok {
				return j
			}
			return newJobLevelWrapper(sc, lp.high, j)
		}
	})
}

func WithLowLevel() *scheduleOption {
	return newScheduleOption(func(opt *ScheduleOptions) {
		opt.jobWrapper = lowJobWrapperHandler
	})
}

var lowJobWrapperHandler = func(sc Schedule, p Picker, j Job) Job {
	lp, ok := p.(*LevelPicker)
	if !ok {
		return j
	}
	return newJobLevelWrapper(sc, lp.low, j)
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

func newJobLevelWrapper(sc Schedule, jobLevel JobLevel, j Job) *JobLevelWrapper {
	w := &JobLevelWrapper{
		l:        jobLevel,
		schedule: sc,
		next:     j,
	}
	jobLevel.Add(w)
	return w
}

func (w *JobLevelWrapper) Run() {
	w.next.Run()
	w.l.Redo(w)
}

func (w *JobLevelWrapper) Distory() {
	w.l.Remove(w)
	w.next.Distory()
}

type LevelPicker struct {
	low  JobLevel
	high JobLevel
}

func NewLevelPicker(loc *time.Location) *LevelPicker {
	return &LevelPicker{
		low:  NewLowLevel(loc),
		high: NewHighLevel(loc),
	}
}

func (lp *LevelPicker) PickSize(ctx context.Context, size int) ([]Job, int) {
	ctx, cancel := context.WithTimeout(ctx, 1*time.Second)
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
				break // ctx.Err() == context.Canceled
			}
		}
		i++
	}
	return jobs, len(jobs)
}

type highLevel struct {
	timer    *Timer
	ready    chan Job
	location *time.Location
}

func NewHighLevel(location *time.Location) *highLevel {
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
	e.td = h.timer.Add(e.schedule.Next(h.now()), e)
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

func NewLowLevel(location *time.Location) *lowLevel {
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
	e.td = l.timer.Add(e.schedule.Next(l.now()), e)
}

func (l *lowLevel) Redo(e *JobLevelWrapper) {
	l.timer.Set(e.td, e.schedule.Next(l.now()))
}

func (l *lowLevel) Pick() <-chan Job {
	return l.ready
}
