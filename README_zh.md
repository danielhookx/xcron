> **Languages:** [English](README.md) | **中文**

# xcron

xcron 是一个高性能的 Go 语言定时任务调度库，基于 ants 协程池实现，支持灵活的定时任务配置和高效的任务执行。

架构：`Cron` → `Engine`（协程池 + 限流）→ `Scheduler`（计时 + Pick + JobWrapper）

## 特性

- 支持标准 cron 表达式
- 支持固定间隔调度（`Every` / `Schedule`）
- 基于 ants 协程池的高性能任务执行
- 支持自定义时区
- 支持任务取消和清理（`Remove`）
- 支持自定义任务包装器（`JobWrapper`）
- 支持自定义调度解析器（`WithParser`）
- 支持自定义 Engine / Scheduler 扩展执行策略
- 支持执行速率限制（`WithRate`）与协程池大小配置（`WithMaxWorkers`）
- 支持优雅关闭
- 可通过 JobWrapper 自行实现超时控制等扩展能力

## 安装

```bash
go get github.com/danielhookx/xcron
```

## 基本用法

### 简单定时任务

```go
package main

import (
    "github.com/danielhookx/xcron"
)

func main() {
    c := xcron.NewCron()

    // 添加一个每5分钟执行一次的任务
    c.AddFunc("*/5 * * * *", func() {
        println("执行定时任务")
    })

    c.Start()
    // 生产环境建议使用「优雅关闭」一节的方式，通过 <-ctx.Done() 等待正在执行的任务完成
    defer c.Stop()

    select {}
}
```

### 带时区的定时任务

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
        println("UTC 时间每天零点执行")
    })

    c.Start()
    defer c.Stop()

    select {}
}
```

### 固定间隔任务

除 cron 表达式外，也可使用 `Schedule` + `Every` 添加固定间隔任务：

```go
package main

import (
    "github.com/danielhookx/xcron"
    "time"
)

func main() {
    c := xcron.NewCron()

    c.Schedule(xcron.Every(30*time.Second), xcron.FuncJob(func() {
        println("每 30 秒执行")
    }))

    c.Start()
    defer c.Stop()

    select {}
}
```

### 自定义任务 ID 与移除

```go
package main

import (
    "github.com/danielhookx/xcron"
)

func main() {
    c := xcron.NewCron()

    id, _ := c.AddFunc("*/10 * * * *", func() {
        println("每10分钟执行一次")
    }, xcron.WithID("custom-task-1"))

    entry := c.Entry(id)
    if entry != nil {
        println("任务存在")
    }

    // 移除任务，会触发 CancelHandler 清理调度
    c.Remove(id)

    c.Start()
    defer c.Stop()

    select {}
}
```

## 高级用法

### 自定义 Engine（协程池与限流）

默认情况下 `NewCron()` 内部等价于 `NewEngine(NewJobScheduler(loc))`。如需自定义协程池大小或执行速率，可手动组装：

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
        xcron.WithMaxWorkers(100), // 协程池大小，0 表示使用 ants 默认值
        xcron.WithRate(50),        // 每秒最多触发 50 次，<=0 为全速模式
    )
    c := xcron.NewCron(xcron.WithEngine(engine))

    c.AddFunc("*/5 * * * *", func() {
        println("执行定时任务")
    })

    c.Start()
    defer c.Stop()

    select {}
}
```

### 自定义 JobWrapper

JobWrapper 可用于添加超时控制、重试机制等扩展能力：

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
            println("任务执行超时")
        case <-done:
            println("任务执行完成")
        }
    }), nil
}

func main() {
    c := xcron.NewCron()

    c.AddFunc("*/5 * * * *", func() {
        time.Sleep(40 * time.Second)
        println("任务执行完成")
    }, xcron.WithJobWrapper(timeoutJobWrapper))

    c.Start()
    defer c.Stop()

    select {}
}
```

### 优雅关闭

xcron 支持优雅关闭，确保正在执行的任务能够完成：

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
        println("执行定时任务")
    })

    c.Start()

    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

    <-sigChan

    ctx, _ := c.Stop()
    <-ctx.Done()

    println("所有任务已停止")
}
```

## 配置选项

### Cron 选项

- `WithLocation(loc *time.Location)` - 设置时区
- `WithParser(parser ScheduleParser)` - 设置 cron 表达式解析器
- `WithEngine(engine Engine)` - 注入自定义 Engine

### Engine 选项

- `WithMaxWorkers(n int)` - 设置 ants 协程池大小，0 表示使用默认值
- `WithRate(n int)` - 设置每秒最大触发次数，<=0 为全速模式

### JobScheduler 选项

- `WithPickTimeout(d time.Duration)` - 设置 Pick 等待超时，默认 5ms

### Schedule 选项

- `WithID(id EntryID)` - 设置任务 ID
- `WithJobWrapper(wrapper JobWrapper)` - 设置任务包装器

## 注意事项

1. 确保在程序退出前调用 `Stop()` 方法以清理资源；生产环境应等待 `Stop()` 返回的 context 完成，以确保正在执行的任务结束
2. 可通过 JobWrapper 自行实现超时控制等扩展逻辑
3. 合理设置协程池大小（`WithMaxWorkers`），避免资源浪费
4. 注意处理任务执行过程中的错误

## 许可证

MIT License
