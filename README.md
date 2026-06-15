> **Languages:** **English** | [中文](README_zh.md)

# xcron

xcron is a high-performance Go cron scheduling library built on the [ants](https://github.com/panjf2000/ants) goroutine pool. It supports flexible job configuration and efficient task execution.

Architecture: `Cron` → `Engine` (goroutine pool + rate limiting) → `Scheduler` (timing + Pick + JobWrapper)

## Features

- Standard cron expressions
- Fixed-interval scheduling (`Every` / `Schedule`)
- High-performance execution via ants goroutine pool
- Custom time zones
- Job cancellation and cleanup (`Remove`)
- Custom job wrappers (`JobWrapper`)
- Custom schedule parsers (`WithParser`)
- Custom `Engine` / `Scheduler` for extended execution strategies
- Execution rate limiting (`WithRate`) and goroutine pool sizing (`WithMaxWorkers`)
- Graceful shutdown
- Extensible capabilities such as timeout control via `JobWrapper`

## Installation

```bash
go get github.com/danielhookx/xcron
```

## Basic Usage

### Simple Cron Job

```go
package main

import (
    "github.com/danielhookx/xcron"
)

func main() {
    c := xcron.NewCron()

    // Run every 5 minutes
    c.AddFunc("*/5 * * * *", func() {
        println("running scheduled job")
    })

    c.Start()
    // In production, prefer the graceful shutdown pattern in the section below
    // and wait for <-ctx.Done() so in-flight jobs can finish
    defer c.Stop()

    select {}
}
```

### Cron Job with Time Zone

```go
package main

import (
    "github.com/danielhookx/xcron"
    "time"
)

func main() {
    c := xcron.NewCron(
        xcron.WithLocation(time.UTC),
    )

    c.AddFunc("0 0 * * *", func() {
        println("runs daily at midnight UTC")
    })

    c.Start()
    defer c.Stop()

    select {}
}
```

### Fixed-Interval Job

In addition to cron expressions, use `Schedule` + `Every` for fixed intervals:

```go
package main

import (
    "github.com/danielhookx/xcron"
    "time"
)

func main() {
    c := xcron.NewCron()

    c.Schedule(xcron.Every(30*time.Second), xcron.FuncJob(func() {
        println("runs every 30 seconds")
    }))

    c.Start()
    defer c.Stop()

    select {}
}
```

### Custom Job ID and Removal

```go
package main

import (
    "github.com/danielhookx/xcron"
)

func main() {
    c := xcron.NewCron()

    id, _ := c.AddFunc("*/10 * * * *", func() {
        println("runs every 10 minutes")
    }, xcron.WithID("custom-task-1"))

    entry := c.Entry(id)
    if entry != nil {
        println("job exists")
    }

    // Remove the job; triggers CancelHandler to clean up scheduling
    c.Remove(id)

    c.Start()
    defer c.Stop()

    select {}
}
```

## Advanced Usage

### Custom Engine (Goroutine Pool and Rate Limiting)

By default, `NewCron()` is equivalent to `NewEngine(NewJobScheduler(loc))`. To customize pool size or execution rate, assemble components manually:

```go
package main

import (
    "github.com/danielhookx/xcron"
    "time"
)

func main() {
    scheduler := xcron.NewJobScheduler(
        time.Local,
        xcron.WithPickTimeout(10*time.Millisecond),
    )
    engine := xcron.NewEngine(
        scheduler,
        xcron.WithMaxWorkers(100), // pool size; 0 uses ants default
        xcron.WithRate(50),        // max 50 triggers per second; <=0 is full speed
    )
    c := xcron.NewCron(xcron.WithEngine(engine))

    c.AddFunc("*/5 * * * *", func() {
        println("running scheduled job")
    })

    c.Start()
    defer c.Stop()

    select {}
}
```

### Custom JobWrapper

`JobWrapper` can add timeout control, retries, and other extensions:

```go
package main

import (
    "context"
    "github.com/danielhookx/xcron"
    "time"
)

func timeoutJobWrapper(schedule xcron.Schedule, job xcron.Job) (xcron.Job, xcron.CancelHandler) {
    return xcron.FuncJob(func() {
        ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
        defer cancel()

        done := make(chan struct{})
        go func() {
            job.Run()
            close(done)
        }()

        select {
        case <-ctx.Done():
            println("job timed out")
        case <-done:
            println("job completed")
        }
    }), nil
}

func main() {
    c := xcron.NewCron()

    c.AddFunc("*/5 * * * *", func() {
        time.Sleep(40 * time.Second)
        println("job completed")
    }, xcron.WithJobWrapper(timeoutJobWrapper))

    c.Start()
    defer c.Stop()

    select {}
}
```

### Graceful Shutdown

xcron supports graceful shutdown so in-flight jobs can finish:

```go
package main

import (
    "github.com/danielhookx/xcron"
    "os"
    "os/signal"
    "syscall"
)

func main() {
    c := xcron.NewCron()

    c.AddFunc("*/5 * * * *", func() {
        println("running scheduled job")
    })

    c.Start()

    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

    <-sigChan

    ctx, _ := c.Stop()
    <-ctx.Done()

    println("all jobs stopped")
}
```

## Configuration Options

### Cron Options

- `WithLocation(loc *time.Location)` — set time zone
- `WithParser(parser ScheduleParser)` — set cron expression parser
- `WithEngine(engine Engine)` — inject a custom Engine

### Engine Options

- `WithMaxWorkers(n int)` — ants pool size; 0 uses default
- `WithRate(n int)` — max triggers per second; <=0 is full speed

### JobScheduler Options

- `WithPickTimeout(d time.Duration)` — Pick wait timeout; default 5ms

### Schedule Options

- `WithID(id EntryID)` — set job ID
- `WithJobWrapper(wrapper JobWrapper)` — set job wrapper

## Notes

1. Call `Stop()` before exit to release resources; in production, wait for the context returned by `Stop()` so running jobs can finish
2. Use `JobWrapper` to implement timeout control and other extensions
3. Set goroutine pool size (`WithMaxWorkers`) appropriately to avoid wasting resources
4. Handle errors during job execution

## License

MIT License
