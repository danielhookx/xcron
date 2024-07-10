package xcron

import (
	"time"
)

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
