package xcron

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/panjf2000/ants/v2"
)

type Scheduler interface {
	Picker
	JobWrapper() JobWrapper
	Start() error
	Stop() error
}

// Picker selects ready jobs from the scheduler.
// Similar to io.Reader: pull-based, may return fewer than requested.
type Picker interface {
	// Pick returns up to size ready jobs. Blocks until at least one is available
	// or ctx is done. May return fewer than size if deadline exceeded.
	// Returns (nil, ctx.Err()) when ctx is canceled.
	Pick(ctx context.Context, size int) ([]Job, error)
}

var _ Engine = (*engine)(nil)

type EngineOptions struct {
	maxWorkers int // goroutine pool size, 0 means use ants default
	rate       int // executions per second, <=0 means full speed mode
}

type EngineOption interface {
	apply(*EngineOptions)
}

type engineOption struct {
	f func(opts *EngineOptions)
}

func (o *engineOption) apply(opts *EngineOptions) {
	o.f(opts)
}

func newEngineOption(f func(*EngineOptions)) *engineOption {
	return &engineOption{
		f: f,
	}
}

func WithMaxWorkers(maxWorkers int) *engineOption {
	return newEngineOption(func(opt *EngineOptions) {
		opt.maxWorkers = maxWorkers
	})
}

func WithRate(rate int) *engineOption {
	return newEngineOption(func(opt *EngineOptions) {
		opt.rate = rate
	})
}

type engine struct {
	pool      *ants.Pool
	scheduler Scheduler
	strategy  PacingStrategy

	isRunning *atomic.Bool
	cancel    func()
	wg        *sync.WaitGroup
}

func NewEngine(scheduler Scheduler, opt ...EngineOption) *engine {
	opts := EngineOptions{
		maxWorkers: 0,
		rate:       0,
	}
	for _, o := range opt {
		o.apply(&opts)
	}

	pool, err := ants.NewPool(opts.maxWorkers)
	if err != nil {
		panic(err)
	}
	return &engine{
		pool:      pool,
		scheduler: scheduler,
		strategy:  NewPacingStrategy(opts.rate),
		isRunning: &atomic.Bool{},
		wg:        &sync.WaitGroup{},
	}
}

func (e *engine) JobWrapper() JobWrapper {
	return e.scheduler.JobWrapper()
}

func (e *engine) Start() error {
	if e.isRunning.CompareAndSwap(false, true) {
		ctx, cancel := context.WithCancel(context.Background())
		e.cancel = cancel
		go e.run(ctx)
		return e.scheduler.Start()
	}
	return nil
}

func (e *engine) run(ctx context.Context) {
	for e.strategy.WaitForNext(ctx) {
		jobs, err := e.scheduler.Pick(ctx, 1)
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
		e.scheduler.Stop()
		e.cancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		e.wg.Wait()
		cancel()
	}()
	return ctx, nil
}
