package xcron

import "time"

// EngineConfig configures Engine concurrency and execution rate
type EngineConfig struct {
	MaxWorkers int // goroutine pool size, 0 means use ants default
	Rate       int // executions per second, <=0 means full speed mode
}

type CronOptions struct {
	loc            *time.Location
	engine         Engine // nil means use engineConfig to create default Engine
	scheduleParser ScheduleParser
}

type CronOption interface {
	apply(*CronOptions)
}

type cronOption struct {
	f func(opts *CronOptions)
}

func (o *cronOption) apply(opts *CronOptions) {
	o.f(opts)
}

func newCronOption(f func(*CronOptions)) *cronOption {
	return &cronOption{
		f: f,
	}
}

func WithLocation(loc *time.Location) *cronOption {
	return newCronOption(func(opt *CronOptions) {
		opt.loc = loc
	})
}

func WithEngine(engine Engine) *cronOption {
	return newCronOption(func(opt *CronOptions) {
		opt.engine = engine
	})
}

func WithParser(sp ScheduleParser) *cronOption {
	return newCronOption(func(opt *CronOptions) {
		opt.scheduleParser = sp
	})
}

type ScheduleOptions struct {
	id         EntryID
	jobWrapper JobWrapper
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

func WithID(id EntryID) *scheduleOption {
	return newScheduleOption(func(opt *ScheduleOptions) {
		opt.id = id
	})
}

func WithJobWrapper(jobWrapper JobWrapper) *scheduleOption {
	return newScheduleOption(func(opt *ScheduleOptions) {
		opt.jobWrapper = jobWrapper
	})
}
