package xcron

import (
	"context"
	"strconv"
	"sync"
	"time"
)

// FuncJob is a wrapper that turns a func() into a Job
type FuncJob func()

func (f FuncJob) Run() { f() }

// Schedule describes a job's duty cycle.
type Schedule interface {
	// Next returns the next activation time, later than the given time.
	// Next is invoked initially, and then each time the job is run.
	Next(time.Time) time.Time
}

type EngineCreator func(Picker) Engine
type PickerCreator func() Picker

var defaultPickerCreator PickerCreator = func() Picker {
	return NewEngine(1, p)
}

var defaultEngineCreator EngineCreator = func(p Picker) Engine {
	return NewEngine(1, p)
}

// EntryID identifies an entry within a Cron instance
type EntryID string

// Entry consists of a schedule and the func to execute on that schedule.
type Entry struct {
	// ID is the cron-assigned ID of this entry, which may be used to look up a
	// snapshot or remove it.
	ID EntryID

	// Schedule on which this job should be run.
	Schedule Schedule

	td   *TimerData
	next Job
}

func (e *Entry) Run() {
	e.next.Run()
}

type Cron struct {
	lock     sync.Mutex
	entrys   map[EntryID]*Entry
	engine   Engine
	picker   Picker
	location *time.Location
	index    int
}

func NewCron(opt ...CronOption) *Cron {
	opts := CronOptions{
		engineCreator: defaultEngineCreator,
	}
	for _, o := range opt {
		o.apply(&opts)
	}

	c := &Cron{
		entrys:   make(map[EntryID]*Entry),
		location: time.Local,
	}
	c.picker = opts.pickerCreator()
	c.engine = opts.engineCreator(c.picker)
	return c
}

func (m *Cron) Start(ctx context.Context) error {
	return m.engine.Start(ctx)
}

func (m *Cron) Stop(ctx context.Context) error {
	return m.engine.Stop(ctx)
}

func (c *Cron) Schedule(schedule Schedule, cmd Job, opt ...ScheduleOption) EntryID {
	opts := ScheduleOptions{
		jobWrapper: lowJobWrapperHandler,
	}
	for _, o := range opt {
		o.apply(&opts)
	}

	c.lock.Lock()
	c.index++
	e := &Entry{
		ID:       EntryID(strconv.Itoa(c.index)),
		Schedule: schedule,
		next:     opts.jobWrapper(c.picker, cmd),
	}
	c.entrys[e.ID] = e
	c.lock.Unlock()
	return e.ID
}

func (c *Cron) Remove(id EntryID) {
	c.lock.Lock()
	e := c.entrys[id]
	delete(c.entrys, id)
	c.lock.Unlock()
	if e != nil {
		e.next.Distory()
	}
}
