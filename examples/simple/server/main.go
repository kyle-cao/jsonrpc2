package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kyle-cao/jsonrpc2"
	"github.com/kyle-cao/jsonrpc2/protocol"
)

// LoggingMiddleware 现在是一个简单的 HandlerFunc。
func LoggingMiddleware(ctx *jsonrpc2.Context) {
	start := time.Now()
	log.Printf("--> Request: %s, Params: %s", ctx.Request.Method, string(ctx.Request.Params))

	// 调用链中的下一个处理器（可能是另一个中间件，也可能是最终的业务逻辑）
	ctx.Next()

	log.Printf("<-- Response in %v | Error: %v", time.Since(start), ctx.GetResponseError())
}

// AuthMiddleware 也是一个简单的 HandlerFunc。
func AuthMiddleware(ctx *jsonrpc2.Context) {
	// 假设参数是一个 JSON 对象
	var params struct {
		Token string `json:"token"`
	}

	// 尝试绑定 token
	// 即使绑定失败，我们也不立即返回错误，因为可能其他字段绑定会出错
	_ = ctx.Bind(&params)

	if params.Token != "secret-token" {
		log.Println("Auth failed!")
		ctx.Error(protocol.NewError(-32001, "Unauthorized", nil))
		// 关键：鉴权失败，我们直接返回，不再调用 ctx.Next()，从而中断处理链
		return
	}

	log.Println("Auth success!")
	ctx.Set("user", "admin") // 在上下文中存入用户信息

	// 鉴权成功，继续执行下一个处理器
	ctx.Next()
}

// Arith 是我们的业务服务
type Arith struct{}

// ArithParamsWithToken 定义了 Add 方法需要的参数
type ArithParamsWithToken struct {
	A     int    `json:"a"`
	B     int    `json:"b"`
	Token string `json:"token"` // Token 也在这里
}

// Add 方法，这是处理链的最后一个环节
func (t *Arith) Add(ctx *jsonrpc2.Context) {
	user, _ := ctx.Get("user")
	log.Printf("Executing Add method for user: %v", user)

	var params ArithParamsWithToken
	if err := ctx.Bind(&params); err != nil {
		ctx.Error(protocol.InvalidParamsError(err.Error()))
		return
	}
	ctx.Result(params.A + params.B)
}

// Ping 方法，也是处理链的最后一个环节
func (t *Arith) Ping(ctx *jsonrpc2.Context) {
	ctx.Result("pong")
}

func main() {
	server := jsonrpc2.NewServer()
	arith := &Arith{}

	// 注册全局中间件
	// server.Use(dbLogger)

	// 正确的注册方式：
	// 将中间件和最终处理器按执行顺序列出。
	// 请求会先经过 LoggingMiddleware，然后是 AuthMiddleware，最后到达 arith.Add。
	server.Handle("Arith.Add", LoggingMiddleware, AuthMiddleware, arith.Add)

	// Ping 方法的处理链只有两个环节
	server.Handle("Arith.Ping", LoggingMiddleware, arith.Ping)

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
