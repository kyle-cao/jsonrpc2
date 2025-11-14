package protocol

import "encoding/json"

// Request 代表一个 JSON-RPC 2.0 请求对象
type Request struct {
	Jsonrpc string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
	ID      interface{}     `json:"id"`
}

// Response 代表一个 JSON-RPC 2.0 响应对象
type Response struct {
	Jsonrpc string       `json:"jsonrpc"`
	Result  interface{}  `json:"result,omitempty"`
	Error   *ErrorObject `json:"error,omitempty"`
	ID      interface{}  `json:"id"`
}

// ErrorObject 代表响应中的错误详情
type ErrorObject struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Error 实现了 Go 的 error 接口
func (e *ErrorObject) Error() string {
	return e.Message
}
