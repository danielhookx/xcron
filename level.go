package xcron

import (
	"context"
	"time"
)

type JobLevel interface {
	Pick() <-chan Job
	Remove(*Entry)
	Add(*Entry)
	Redo(*Entry)
}

func WithHighLevel() *scheduleOption {
	return newScheduleOption(func(opt *ScheduleOptions) {
		opt.jobWrapper = func(p Picker, j Job) Job {
			lp, ok := p.(*LevelPicker)
			if !ok {
				return j
			}
			return newJobLevelWrapper(lp.high, j)
		}
	})
}

func WithLowLevel() *scheduleOption {
	return newScheduleOption(func(opt *ScheduleOptions) {
		opt.jobWrapper = lowJobWrapperHandler
	})
}

var lowJobWrapperHandler = func(p Picker, j Job) Job {
	lp, ok := p.(*LevelPicker)
	if !ok {
		return j
	}
	return newJobLevelWrapper(lp.low, j)
}

type JobLevelWrapper struct {
	l    JobLevel
	next Job
}

func newJobLevelWrapper(jobLevel JobLevel, j Job) *JobLevelWrapper {
	return &JobLevelWrapper{
		l:    jobLevel,
		next: j,
	}
}

func (w *JobLevelWrapper) Run() {
	w.next.Run()
	w.l.Redo(e)
}

func (w *JobLevelWrapper) Distory() {
	w.l.Remove(e)
	w.next.Distory()
}

type LevelPicker struct {
	low  JobLevel
	high JobLevel
}

func NewLevelPicker() *LevelPicker {
	lp := &LevelPicker{}
	lp.low = NewLowLevel(c.location)
	lp.high = NewHighLevel(c.location)
	return lp
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

func (h *highLevel) Remove(e *Entry) {
	h.timer.Del(e.td)
	e.td = nil
}

func (h *highLevel) Add(e *Entry) {
	e.td = h.timer.Add(e.Schedule.Next(h.now()), e)
}

func (h *highLevel) Redo(e *Entry) {
	h.timer.Set(e.td, e.Schedule.Next(h.now()))
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

func (l *lowLevel) Remove(e *Entry) {
	l.timer.Del(e.td)
	e.td = nil
}

func (l *lowLevel) Add(e *Entry) {
	e.td = l.timer.Add(e.Schedule.Next(l.now()), e)
}

func (l *lowLevel) Redo(e *Entry) {
	l.timer.Set(e.td, e.Schedule.Next(l.now()))
}

func (l *lowLevel) Pick() <-chan Job {
	return l.ready
}
