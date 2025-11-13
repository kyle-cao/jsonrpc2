package jsonrpc2

import (
	"context"
	"encoding/json"
	"net"
	"sync"

	"github.com/kyle-cao/jsonrpc2/protocol"
)

// Context 封装了单次 RPC 调用的所有信息。
type Context struct {
	context.Context
	Conn       net.Conn
	Request    *protocol.Request
	store      map[string]interface{}
	storeMutex sync.RWMutex
	// 内部字段
	responseResult interface{}
	responseError  *protocol.ErrorObject
	handlerChain   []HandlerFunc
	handlerIdx     int
}

// Next 调用处理链中的下一个处理器。
func (c *Context) Next() {
	c.handlerIdx++
	if c.handlerIdx < len(c.handlerChain) {
		c.handlerChain[c.handlerIdx](c)
	}
}

// GetResponseError 获取由处理器设置的错误响应。
func (c *Context) GetResponseError() *protocol.ErrorObject {
	return c.responseError
}

// GetResponseResult 返回由处理器设置的成功响应结果。
func (c *Context) GetResponseResult() interface{} {
	return c.responseResult
}

// Bind 将请求的 Params 解析到指定的结构体指针中。
func (c *Context) Bind(v interface{}) error {
	if c.Request.Params == nil {
		return protocol.InvalidParamsError("params are null")
	}
	return json.Unmarshal(c.Request.Params, v)
}

// Result 设置成功的响应结果。
func (c *Context) Result(data interface{}) {
	c.responseResult = data
}

// Error 设置失败的响应。
func (c *Context) Error(err *protocol.ErrorObject) {
	c.responseError = err
}

// Set 在中间件之间安全地传递数据。
func (c *Context) Set(key string, value interface{}) {
	c.storeMutex.Lock()
	defer c.storeMutex.Unlock()
	if c.store == nil {
		c.store = make(map[string]interface{})
	}
	c.store[key] = value
}

// Get 从上下文中安全地获取数据。
func (c *Context) Get(key string) (interface{}, bool) {
	c.storeMutex.RLock()
	defer c.storeMutex.RUnlock()
	value, ok := c.store[key]
	return value, ok
}
