package jsonrpc2

// HandlerFunc 是处理 RPC 请求的最终函数类型。
type HandlerFunc func(ctx *Context)
