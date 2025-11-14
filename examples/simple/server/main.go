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

func LoggingMiddleware(ctx *jsonrpc2.Context) {
	start := time.Now()
	log.Printf("--> Request: %s, Params: %s", ctx.Request.Method, string(ctx.Request.Params))

	ctx.Next()

	log.Printf("<-- Response in %v | Error: %v", time.Since(start), ctx.GetResponseError())
}

func AuthMiddleware(ctx *jsonrpc2.Context) {
	var params struct {
		Token string `json:"token"`
	}
	_ = ctx.Bind(&params)

	if params.Token != "secret-token" {
		log.Println("Auth failed!")
		ctx.Error(protocol.NewError(-32001, "Unauthorized", nil))
		return
	}

	log.Println("Auth success!")
	ctx.Set("user", "admin") // 在上下文中存入用户信息

	ctx.Next()
}

// Arith
type Arith struct{}

type ArithParamsWithToken struct {
	A     int    `json:"a"`
	B     int    `json:"b"`
	Token string `json:"token"` // Token 也在这里
}

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

func main() {
	server := jsonrpc2.NewServer()
	arith := &Arith{}

	server.Handle("Arith.Add", LoggingMiddleware, AuthMiddleware, arith.Add)

	log.Println("Starting jrpc server on :8080")
	if err := server.Listen(":8080"); err != nil {
		log.Fatal("Server error:", err)
	}
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	<-quit
	log.Println("Shutdown signal received, starting graceful shutdown...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Close(ctx); err != nil {
		log.Fatalf("Server shutdown failed: %v", err)
	}
	log.Println("Server gracefully stopped")

}
