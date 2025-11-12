package protocol

// 标准 JSON-RPC 2.0 错误码
const (
	CodeParseError     = -32700
	CodeInvalidRequest = -32600
	CodeMethodNotFound = -32601
	CodeInvalidParams  = -32602
	CodeInternalError  = -32603
)

func NewError(code int, message string, data interface{}) *ErrorObject {
	return &ErrorObject{Code: code, Message: message, Data: data}
}

func ParseError(data interface{}) *ErrorObject {
	return NewError(CodeParseError, "Parse error", data)
}

func InvalidRequestError(data interface{}) *ErrorObject {
	return NewError(CodeInvalidRequest, "Invalid Request", data)
}

func MethodNotFoundError(data interface{}) *ErrorObject {
	return NewError(CodeMethodNotFound, "Method not found", data)
}

func InvalidParamsError(data interface{}) *ErrorObject {
	return NewError(CodeInvalidParams, "Invalid params", data)
}

func InternalError(data interface{}) *ErrorObject {
	return NewError(CodeInternalError, "Internal error", data)
}
