package xcron

import "time"

type CronOptions struct {
	loc            *time.Location
	pickerCreator  PickerCreator
	engineCreator  EngineCreator
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

func WithEngine(ec EngineCreator) *cronOption {
	return newCronOption(func(opt *CronOptions) {
		opt.engineCreator = ec
	})
}

func WithPicker(pc PickerCreator) *cronOption {
	return newCronOption(func(opt *CronOptions) {
		opt.pickerCreator = pc
	})
}

func WithParser(sp ScheduleParser) *cronOption {
	return newCronOption(func(opt *CronOptions) {
		opt.scheduleParser = sp
	})
}

type ScheduleOptions struct {
	id         EntryID
	jobWrapper func(Schedule, Picker, Job) (Job, CancelHandler)
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

func WithJobWrapper(jobWrapper func(Schedule, Picker, Job) (Job, CancelHandler)) *scheduleOption {
	return newScheduleOption(func(opt *ScheduleOptions) {
		opt.jobWrapper = jobWrapper
	})
}
