package xcron

import "time"

// EngineConfig configures Engine concurrency and execution rate
type EngineConfig struct {
	MaxWorkers int // goroutine pool size, 0 means use ants default
	Rate       int // executions per second, <=0 means full speed mode
}

type CronOptions struct {
	loc            *time.Location
	pickerCreator  PickerCreator
	engineCreator  EngineCreator // nil means use engineConfig to create default Engine
	engineConfig   EngineConfig
	scheduleParser ScheduleParser
	pickTimeout    time.Duration // timeout for Pick when no jobs ready, 0 means use default
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

func WithEngine(ec EngineCreator) *cronOption {
	return newCronOption(func(opt *CronOptions) {
		opt.engineCreator = ec
	})
}

// WithEngineConfig configures Engine concurrency and execution rate.
// Takes effect when WithEngine is not used to specify a custom Engine.
func WithEngineConfig(maxWorkers, rate int) *cronOption {
	return newCronOption(func(opt *CronOptions) {
		opt.engineConfig.MaxWorkers = maxWorkers
		opt.engineConfig.Rate = rate
	})
}

func WithPicker(pc PickerCreator) *cronOption {
	return newCronOption(func(opt *CronOptions) {
		opt.pickerCreator = pc
	})
}

// WithPickTimeout sets the timeout for Pick when waiting for ready jobs.
// Only applies when using the default picker. Zero means use default (5ms).
func WithPickTimeout(d time.Duration) *cronOption {
	return newCronOption(func(opt *CronOptions) {
		opt.pickTimeout = d
	})
}

func WithParser(sp ScheduleParser) *cronOption {
	return newCronOption(func(opt *CronOptions) {
		opt.scheduleParser = sp
	})
}

type ScheduleOptions struct {
	id         EntryID
	jobWrapper func(Schedule, Job) (Job, CancelHandler)
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

func WithJobWrapper(jobWrapper func(Schedule, Job) (Job, CancelHandler)) *scheduleOption {
	return newScheduleOption(func(opt *ScheduleOptions) {
		opt.jobWrapper = jobWrapper
	})
}
