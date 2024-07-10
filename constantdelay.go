package xcron

import "time"

type ConstantDelaySchedule struct {
	Delay time.Duration
}

func Every(duration time.Duration) ConstantDelaySchedule {
	return ConstantDelaySchedule{
		Delay: duration,
	}
}

func (schedule ConstantDelaySchedule) Next(t time.Time) time.Time {
	return t.Add(schedule.Delay)
}
