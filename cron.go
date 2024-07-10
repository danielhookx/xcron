package xcron

import (
	"context"
	"strconv"
	"sync"
	"time"
)

// FuncJob is a wrapper that turns a func() into a cron.Job
type FuncJob func()

func (f FuncJob) Run() { f() }

// Schedule describes a job's duty cycle.
type Schedule interface {
	// Next returns the next activation time, later than the given time.
	// Next is invoked initially, and then each time the job is run.
	Next(time.Time) time.Time
}

type JobLevel interface {
	Pick() <-chan Job
	Remove(*Entry)
	Add(*Entry)
	Redo(*Entry)
}

type Level int

const (
	LowLevel Level = iota
	HighLevel
)

// EntryID identifies an entry within a Cron instance
type EntryID string

// Entry consists of a schedule and the func to execute on that schedule.
type Entry struct {
	// ID is the cron-assigned ID of this entry, which may be used to look up a
	// snapshot or remove it.
	ID EntryID

	// Schedule on which this job should be run.
	Schedule Schedule

	l    JobLevel
	td   *TimerData
	next Job
}

func (e *Entry) Run() {
	e.next.Run()
	e.l.Redo(e)
}

type ScheduleOptions struct {
	id    EntryID
	level Level
}

type ScheduleOption interface {
	apply(*ScheduleOptions)
}

type scheduleOption struct {
	f func(opts *ScheduleOptions)
}

func (o *scheduleOption) apply(opts *ScheduleOptions) {
	o.f(opts)
}

func newScheduleOption(f func(*ScheduleOptions)) *scheduleOption {
	return &scheduleOption{
		f: f,
	}
}

func WithLevel(level Level) *scheduleOption {
	return newScheduleOption(func(opt *ScheduleOptions) {
		opt.level = level
	})
}

func WithID(id EntryID) *scheduleOption {
	return newScheduleOption(func(opt *ScheduleOptions) {
		opt.id = id
	})
}

type Cron struct {
	lock     sync.Mutex
	entrys   map[EntryID]*Entry
	engine   *engine
	low      JobLevel
	high     JobLevel
	location *time.Location
	index    int
}

func NewCron(numWorkers int) *Cron {
	tm := &Cron{
		entrys:   make(map[EntryID]*Entry),
		location: time.Local,
	}
	tm.low = NewLowLevel(tm.location)
	tm.high = NewHighLevel(tm.location)
	tm.engine = NewEngine(numWorkers, tm)
	return tm
}

func (m *Cron) Start(ctx context.Context) error {
	return m.engine.Start(ctx)
}

func (m *Cron) Stop(ctx context.Context) error {
	return m.engine.Stop(ctx)
}

func (c *Cron) Schedule(schedule Schedule, cmd Job, opt ...ScheduleOption) EntryID {
	opts := ScheduleOptions{
		level: LowLevel,
	}
	for _, o := range opt {
		o.apply(&opts)
	}

	l := c.jobLevel(opts.level)
	c.lock.Lock()
	c.index++
	e := &Entry{
		ID:       EntryID(strconv.Itoa(c.index)),
		Schedule: schedule,
		next:     cmd,
		l:        l,
	}
	c.entrys[e.ID] = e
	c.lock.Unlock()
	l.Add(e)
	return e.ID
}

func (c *Cron) Remove(id EntryID) {
	c.lock.Lock()
	e := c.entrys[id]
	delete(c.entrys, id)
	c.lock.Unlock()
	if e != nil {
		e.l.Remove(e)
	}
}

func (c *Cron) jobLevel(l Level) JobLevel {
	switch l {
	case LowLevel:
		return c.low
	case HighLevel:
		return c.high
	}
	return c.low
}

func (m *Cron) PickSize(ctx context.Context, size int) ([]Job, int) {
	ctx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()

	jobs := make([]Job, 0, size)
	for i := 0; i < size; {
		select {
		case job := <-m.high.Pick():
			if job != nil {
				jobs = append(jobs, job)
			}
		default:
			select {
			case job := <-m.high.Pick():
				if job != nil {
					jobs = append(jobs, job)
				}
			case job := <-m.low.Pick():
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
