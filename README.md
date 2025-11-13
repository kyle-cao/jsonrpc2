# JSONRPC2: 一个简单的 Go JSON-RPC 2.0 库
`JSONRPC2` 是一个为 Go 语言设计的、轻量级库，用于构建基于 TCP 的 JSON-RPC 2.0 客户端和服务器并且支持中间件。

## ✨ 核心特性
- **完整的 JSON-RPC 2.0 支持**: 实现了完整的服务端和客户端规范。
- **富有表现力的中间件**: 采用类似 Gin/Express.js 的洋葱模型中间件，可以在业务逻辑执行**之前**和**之后**执行代码。
- **请求上下文 (`Context`)**: 在中间件和处理器之间轻松传递数据、管理请求生命周期。
- **优雅关闭 (`Graceful Shutdown`)**: 服务器支持优雅关闭，确保在服务停止前完成所有正在处理的请求。
- **高性能**: 基于纯 TCP 连接，开销低，性能卓越。
- **简单并发的客户端**: 提供简单易用的同步 (`Call`) 和异步 (`Go`) 调用接口。
- **类型安全的处理器**: 处理器是强类型的函数，易于编写和测试。

## 📦 安装

```bash
go get -u github.com/kyle-cao/jsonrpc2
```

## 🚀 快速开始

下面是一个简单的示例，演示了如何创建一个带日志中间件的服务端和一个调用它的客户端。

### 服务端 (`server.go`)

```go
package main

import (
	"log"
	"time"
	"github.com/kyle-cao/jsonrpc2"
)

// LoggingMiddleware 是一个简单的日志中间件
func LoggingMiddleware(ctx *jsonrpc2.Context) {
	start := time.Now()
	log.Printf("--> Request: %s", ctx.Request.Method)

	// 调用处理链中的下一个环节
	ctx.Next()

	log.Printf("<-- Response in %v", time.Since(start))
}

// Ping 是我们的最终业务逻辑处理器
func Ping(ctx *jsonrpc2.Context) {
	ctx.Result("pong")
}

func main() {
	server := jsonrpc2.NewServer()

	// 注册 "System.Ping" 方法，并为其应用日志中间件
	// 请求将首先通过 LoggingMiddleware，然后到达 Ping 处理器
	server.Handle("System.Ping", LoggingMiddleware, Ping)

	log.Println("Starting jrpc server on :8080")
	if err := server.Listen(":8080"); err != nil {
		log.Fatal("Server error:", err)
	}
	// 2. 设置信号监听，用于捕获 Ctrl+C 等中断信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// 阻塞 main goroutine，直到收到一个信号
	<-quit
	log.Println("Shutdown signal received, starting graceful shutdown...")

	// 3. 创建一个带有超时的上下文，用于 Shutdown 方法
	// 我们给服务器 5 秒钟的时间来处理完剩余的请求
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 4. 调用 Shutdown 方法
	if err := server.Close(ctx); err != nil {
		log.Fatalf("Server shutdown failed: %v", err)
	}
	log.Println("Server gracefully stopped")
}
```

### 客户端 (`client.go`)

```go
package main

import (
	"log"
	"github.com/kyle-cao/jsonrpc2"
)

func main() {
	client, err := jsonrpc2.Dial("localhost:8080")
	if err != nil {
		log.Fatalf("Dial failed: %v", err)
	}
	defer client.Close()

	var reply string
	err = client.Call("System.Ping", nil, &reply)
	if err != nil {
		log.Fatalf("Call failed: %v", err)
	}

	log.Printf("Reply from System.Ping: %s", reply) // 输出: Reply from System.Ping: pong
}
```

## 深入指南

### 1. 中间件

中间件是 `JSONRPC2` 的核心。它允许你将横切关注点（如日志、鉴权、监控）从业务逻辑中分离出来。

所有处理器（包括中间件）都共享同一个函数签名： `func(ctx *jsonrpc2.Context)`。

#### 洋葱模型

中间件遵循“洋葱模型”。请求从外到内穿过每一层中间件，到达核心的业务处理器，然后响应再从内到外依次返回。

```go
func LoggingMiddleware(ctx *jsonrpc2.Context) {
    // 1. 在调用下一个处理器之前执行 (请求阶段)
    log.Println("Entering middleware...")

    ctx.Next() // 将控制权交给下一个处理器

    // 3. 在下一个处理器完成后执行 (响应阶段)
    log.Println("Exiting middleware...")
}
```

#### 中断请求链

中间件可以决定是否继续执行。例如，一个鉴权中间件在验证失败时可以直接设置错误并返回，而**不调用 `ctx.Next()`**，从而中断请求链。

```go
func AuthMiddleware(ctx *jsonrpc2.Context) {
    var params struct { Token string `json:"token"` }
    _ = ctx.Bind(&params)

    if params.Token != "secret-token" {
        ctx.Error(protocol.NewError(-32001, "Unauthorized", nil))
        // 验证失败，直接返回，不再执行后续处理器
        return
    }

    // 验证成功，继续
    ctx.Next()
}
```

### 2. 上下文 (`jsonrpc2.Context`)

`Context` 对象是请求生命周期内的信息载体。

- `ctx.Next()`: 调用处理链中的下一个环节。
- `ctx.Bind(v interface{}) error`: 将请求的 `params` 解析到指定的结构体指针中。
- `ctx.Result(data interface{})`: 设置成功的响应数据。
- `ctx.Error(err *protocol.ErrorObject)`: 设置一个 JSON-RPC 格式的错误响应。
- `ctx.Set(key string, value interface{})`: 在中间件之间传递数据。
- `ctx.Get(key string) (interface{}, bool)`: 从上下文中获取数据。

### 3. 优雅关闭

`JSONRPC2` 服务器支持优雅关闭，这对于构建可靠的生产服务至关重要。

```go
func main() {
	server := jsonrpc2.NewServer()
	// ... 注册你的处理器 ...

	// 1. 非阻塞地启动服务器
	go func() {
		log.Println("Starting server on :8080")
		if err := server.ListenAndServe(":8080"); err != nil {
			// 在实际应用中，这里应该处理错误，而不是 panic
			log.Printf("ListenAndServe error: %v", err)
		}
	}()

	// 2. 监听中断信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutdown signal received...")

	// 3. 调用 Shutdown，并设置一个超时上下文
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server shutdown failed: %v", err)
	}

	log.Println("Server gracefully stopped")
}
```

### 4. 客户端用法

#### 同步调用 (`Call`)

`Call` 方法会阻塞，直到收到响应或发生超时。

```go
var reply int
err := client.Call("Arith.Add", map[string]int{"a": 1, "b": 2}, &reply)
```

#### 异步调用 (`Go`)

`Go` 方法不会阻塞，它立即返回一个 `*Call` 对象，你可以通过其 `Done` 通道等待结果。

```go
params := map[string]int{"a": 10, "b": 20}
var reply int

// 发起异步调用
addCall := client.Go("Arith.Add", params, &reply, make(chan *jsonrpc2.Call, 1))

// ... 在这里可以执行其他任务 ...

// 等待异步调用完成
callResult := <-addCall.Done
if callResult.Error != nil {
    log.Fatalf("Async call failed: %v", callResult.Error)
}
log.Printf("Async call result: %d", reply)
```

## 🤝 贡献
欢迎任何形式的贡献！如果您有任何想法、建议或发现 Bug，请随时提交 Issue 或 Pull Request。

## ☕️ 打赏
![c661dc1c34a9f57768218049b845e251](https://github.com/user-attachments/assets/8bcfb62e-5c18-4cbd-b886-a2dcaf7433a9)


