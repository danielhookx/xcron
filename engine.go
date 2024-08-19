package xcron

import (
	"context"
	"log"
	"runtime"
	"time"
)

type Job interface {
	Run()
	Distory()
}

type Picker interface {
	PickSize(ctx context.Context, size int) ([]Job, int)
}

type Engine interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}

type engine struct {
	numWorkers int // work goroutine nums

	picker   Picker
	taskChan chan Job
	ticker   *time.Ticker
}

func NewEngine(numWorkers int, picker Picker) *engine {
	tickerDuraction := time.Millisecond * 50 * time.Duration(numWorkers)
	task := &engine{
		numWorkers: numWorkers,
		picker:     picker,
		taskChan:   make(chan Job, numWorkers),
		ticker:     time.NewTicker(tickerDuraction),
	}
	return task
}

func (e *engine) Start(ctx context.Context) error {
	go func(ctx context.Context) {
		for {
			select {
			case <-ctx.Done():
				return
			case <-e.ticker.C:
				// Main loop, timed batch processing tasks
				jobs, _ := e.picker.PickSize(ctx, e.numWorkers)
				for _, job := range jobs {
					if job != nil {
						e.taskChan <- job
					}
				}
			}
		}
	}(ctx)

	// work goroutines
	for i := 0; i < e.numWorkers; i++ {
		go func(ctx context.Context) {
			for {
				select {
				case <-ctx.Done():
					return
				case job := <-e.taskChan:
					if job != nil {
						defer Recover()

						job.Run()
					}
				}
			}
		}(ctx)
	}
	return nil
}

func (e *engine) Stop(ctx context.Context) error {
	e.ticker.Stop()
	return nil
}

func Recover() {
	if rerr := recover(); rerr != nil {
		buf := make([]byte, 64<<10) //nolint:gomnd
		n := runtime.Stack(buf, false)
		log.Printf("panic %v: \n%s\n", rerr, buf[:n])
	}
}
