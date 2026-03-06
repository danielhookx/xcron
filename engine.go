package xcron

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/panjf2000/ants/v2"
)

type Engine interface {
	Start() error
	Stop() (context.Context, error)
}

type engine struct {
	pool     *ants.Pool
	picker   Picker
	strategy PacingStrategy

	isRunning *atomic.Bool
	cancel    func()
	wg        *sync.WaitGroup
}

func NewEngine(maxWorkers, rate int, picker Picker) *engine {
	pool, err := ants.NewPool(maxWorkers)
	if err != nil {
		panic(err)
	}
	return &engine{
		pool:     pool,
		picker:   picker,
		strategy: NewPacingStrategy(rate),
		isRunning: &atomic.Bool{},
		wg:       &sync.WaitGroup{},
	}
}

func (e *engine) Start() error {
	if e.isRunning.CompareAndSwap(false, true) {
		ctx, cancel := context.WithCancel(context.Background())
		e.cancel = cancel
		go e.run(ctx)
	}
	return nil
}

func (e *engine) run(ctx context.Context) {
	for e.strategy.WaitForNext(ctx) {
		jobs, err := e.picker.Pick(ctx, 1)
		if err != nil {
			return
		}
		for _, job := range jobs {
			e.wg.Add(1)
			j := job
			e.pool.Submit(func() {
				j.Run()
				e.wg.Done()
			})
		}
	}
}

func (e *engine) Stop() (context.Context, error) {
	if e.isRunning.CompareAndSwap(true, false) {
		e.strategy.Stop()
		e.cancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		e.wg.Wait()
		cancel()
	}()
	return ctx, nil
}
