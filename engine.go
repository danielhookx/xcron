package xcron

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/panjf2000/ants/v2"
)

type Picker interface {
	PickSize(ctx context.Context, size int) ([]Job, int)
}

type Engine interface {
	Start() error
	Stop() (context.Context, error)
}

type engine struct {
	maxWorkers, rate int // work goroutine nums
	pool             *ants.Pool
	picker           Picker
	ticker           *time.Ticker

	isRunning *atomic.Bool
	cancel    func()
	wg        *sync.WaitGroup
}

func NewEngine(maxWorkers, rate int, picker Picker) *engine {
	pool, err := ants.NewPool(maxWorkers)
	if err != nil {
		panic(err)
	}
	var ticker *time.Ticker
	if rate > 0 {
		ticker = time.NewTicker(time.Second / time.Duration(rate))
	}
	task := &engine{
		maxWorkers: maxWorkers,
		rate:       rate,
		pool:       pool,
		picker:     picker,
		ticker:     ticker,
		isRunning:  &atomic.Bool{},
		wg:         &sync.WaitGroup{},
	}
	return task
}

func (e *engine) Start() error {
	if e.isRunning.CompareAndSwap(false, true) {
		ctx, cancel := context.WithCancel(context.Background())
		e.cancel = cancel
		go func() {
			if e.ticker != nil {
				e.doTicker(ctx)
			} else {
				e.doRange(ctx)
			}
		}()
	}
	return nil
}

func (e *engine) doTicker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-e.ticker.C:
			jobs, _ := e.picker.PickSize(ctx, 1)
			for _, job := range jobs {
				e.wg.Add(1)
				e.pool.Submit(func() {
					job.Run()
					e.wg.Done()
				})
			}
		}
	}
}

func (e *engine) doRange(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			jobs, _ := e.picker.PickSize(ctx, 1)
			for _, job := range jobs {
				e.wg.Add(1)
				e.pool.Submit(func() {
					job.Run()
					e.wg.Done()
				})
			}
		}
	}
}

func (e *engine) Stop() (context.Context, error) {
	if e.isRunning.CompareAndSwap(true, false) {
		if e.ticker != nil {
			e.ticker.Stop()
		}
		e.cancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		e.wg.Wait()
		cancel()
	}()
	return ctx, nil
}
