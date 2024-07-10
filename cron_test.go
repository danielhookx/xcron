package xcron

import (
	"context"
	"sync"
	"testing"
	"time"
)

type testJob struct {
	wg   *sync.WaitGroup
	name string
}

func (t testJob) Run() {
	t.wg.Done()
}

func TestJob(t *testing.T) {
	wg := &sync.WaitGroup{}
	wg.Add(1)

	ctx := context.Background()
	cron := NewCron(1)
	cron.Schedule(Every(5*time.Second), testJob{wg, "job4"})

	cron.Start(ctx)
	defer cron.Stop(ctx)

	select {
	case <-time.After(5*time.Second + 10*time.Millisecond):
		t.FailNow()
	case <-wait(wg):
	}
}

func wait(wg *sync.WaitGroup) chan bool {
	ch := make(chan bool)
	go func() {
		wg.Wait()
		ch <- true
	}()
	return ch
}
