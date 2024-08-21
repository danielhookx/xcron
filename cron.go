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

type Job interface {
	Run()
}

// ScheduleParser is an interface for schedule spec parsers that return a Schedule
type ScheduleParser interface {
	Parse(spec string) (Schedule, error)
}

// Schedule describes a job's duty cycle.
type Schedule interface {
	// Next returns the next activation time, later than the given time.
	// Next is invoked initially, and then each time the job is run.
	Next(time.Time) time.Time
}

type JobPicker interface {
	Picker
	Start() error
	Stop() error
}

type EngineCreator func(Picker) Engine
type PickerCreator func() JobPicker

var defaultPickerCreatorHandler = func(loc *time.Location) PickerCreator {
	return func() JobPicker {
		return NewLevelPicker(loc)
	}
}

var defaultEngineCreator EngineCreator = func(p Picker) Engine {
	return NewEngine(0, 0, p)
}

type CancelHandler func()

// EntryID identifies an entry within a Cron instance
type EntryID string

type Entry struct {
	Job
	cancel CancelHandler
}

type Cron struct {
	lock     sync.Mutex
	entries  map[EntryID]*Entry
	engine   Engine
	picker   JobPicker
	parser   ScheduleParser
	location *time.Location
	index    int
}

func NewCron(opt ...CronOption) *Cron {
	opts := CronOptions{
		loc:            time.Local,
		engineCreator:  defaultEngineCreator,
		scheduleParser: standardParser,
	}
	for _, o := range opt {
		o.apply(&opts)
	}

	if opts.pickerCreator == nil {
		opts.pickerCreator = defaultPickerCreatorHandler(opts.loc)
	}
	c := &Cron{
		entries:  make(map[EntryID]*Entry),
		location: opts.loc,
		parser:   opts.scheduleParser,
	}
	c.picker = opts.pickerCreator()
	c.engine = opts.engineCreator(c.picker)
	return c
}

func (m *Cron) Start() error {
	var err error
	if e := m.picker.Start(); e != nil {
		err = e
	}
	if e := m.engine.Start(); e != nil {
		err = e
	}
	return err
}

func (m *Cron) Stop() (context.Context, error) {
	var err error
	if e := m.picker.Stop(); e != nil {
		err = e
	}
	ctx, e := m.engine.Stop()
	if e != nil {
		err = e
	}
	return ctx, err
}

// AddFunc adds a func to the Cron to be run on the given schedule.
// The spec is parsed using the time zone of this Cron instance as the default.
// An ID is returned that can be used to later remove it.
func (c *Cron) AddFunc(spec string, cmd func(), opt ...ScheduleOption) (EntryID, error) {
	return c.AddJob(spec, FuncJob(cmd), opt...)
}

// AddJob adds a Job to the Cron to be run on the given schedule.
// The spec is parsed using the time zone of this Cron instance as the default.
// An ID is returned that can be used to later remove it.
func (c *Cron) AddJob(spec string, cmd Job, opt ...ScheduleOption) (EntryID, error) {
	schedule, err := c.parser.Parse(spec)
	if err != nil {
		return "", err
	}
	return c.Schedule(schedule, cmd, opt...), nil
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
	j, cancel := opts.jobWrapper(schedule, c.picker, cmd)
	c.entries[opts.id] = &Entry{
		Job:    j,
		cancel: cancel,
	}
	c.lock.Unlock()
	return opts.id
}

// Location gets the time zone location
func (c *Cron) Location() *time.Location {
	return c.location
}

// Entry returns a snapshot of the given entry, or nil if it couldn't be found.
func (c *Cron) Entry(id EntryID) *Entry {
	c.lock.Lock()
	defer c.lock.Unlock()
	e := c.entries[id]
	return e
}

func (c *Cron) Remove(id EntryID) {
	c.lock.Lock()
	e := c.entries[id]
	delete(c.entries, id)
	c.lock.Unlock()
	if e != nil && e.cancel != nil {
		e.cancel()
	}
}
