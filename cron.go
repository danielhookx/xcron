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

func (f FuncJob) Distory() {}

// Schedule describes a job's duty cycle.
type Schedule interface {
	// Next returns the next activation time, later than the given time.
	// Next is invoked initially, and then each time the job is run.
	Next(time.Time) time.Time
}

type EngineCreator func(Picker) Engine
type PickerCreator func() Picker

var defaultPickerCreatorHandler = func(loc *time.Location) PickerCreator {
	return func() Picker {
		return NewLevelPicker(loc)
	}
}

var defaultEngineCreator EngineCreator = func(p Picker) Engine {
	return NewEngine(1, p)
}

// EntryID identifies an entry within a Cron instance
type EntryID string

type Cron struct {
	lock     sync.Mutex
	entrys   map[EntryID]Job
	engine   Engine
	picker   Picker
	location *time.Location
	index    int
}

func NewCron(opt ...CronOption) *Cron {
	opts := CronOptions{
		loc:           time.Local,
		engineCreator: defaultEngineCreator,
	}
	for _, o := range opt {
		o.apply(&opts)
	}

	if opts.pickerCreator == nil {
		opts.pickerCreator = defaultPickerCreatorHandler(opts.loc)
	}
	c := &Cron{
		entrys:   make(map[EntryID]Job),
		location: opts.loc,
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
	if opts.id == "" {
		opts.id = EntryID(strconv.Itoa(c.index))
	}
	c.entrys[opts.id] = opts.jobWrapper(schedule, c.picker, cmd)
	c.lock.Unlock()
	return opts.id
}

func (c *Cron) Remove(id EntryID) {
	c.lock.Lock()
	e := c.entrys[id]
	delete(c.entrys, id)
	c.lock.Unlock()
	if e != nil {
		e.Distory()
	}
}
