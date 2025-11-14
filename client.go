package jsonrpc2

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/kyle-cao/jsonrpc2/protocol"
)

// Call 代表一个挂起的 RPC 调用。
type Call struct {
	Method string
	Args   interface{}
	Reply  interface{}
	Error  error
	Done   chan *Call
}

type Client struct {
	conn    net.Conn
	encoder *json.Encoder

	sendMutex sync.Mutex // 保护对 conn 的写入
	mutex     sync.Mutex // 保护 Client 内部状态 (seq, pending, closing, shutdown)
	seq       uint64
	pending   map[string]*Call
	closing   bool
	shutdown  bool
}

// Dial 连接到指定的 RPC 服务器。
func Dial(addr string) (*Client, error) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}
	client := &Client{
		conn:    conn,
		encoder: json.NewEncoder(conn),
		pending: make(map[string]*Call),
	}
	go client.receiveLoop()
	return client, nil
}

// receiveLoop 循环接收服务端的响应。
func (c *Client) receiveLoop() {
	var err error
	var res protocol.Response
	decoder := json.NewDecoder(c.conn)

	for err == nil {
		err = decoder.Decode(&res)
		if err != nil {
			break
		}
		idKey, errKey := idToKey(res.ID)
		if errKey != nil {
			log.Printf("jsonrpc2: unexpected response ID type: %T, value: %v", res.ID, res.ID)
			continue
		}

		c.mutex.Lock()
		call := c.pending[idKey]
		delete(c.pending, idKey)
		c.mutex.Unlock()

		if call != nil {
			if res.Error != nil {
				call.Error = res.Error
			} else {
				if call.Reply != nil {
					jsonData, _ := json.Marshal(res.Result)
					call.Error = json.Unmarshal(jsonData, call.Reply)
				}
			}
			call.Done <- call
		}
	}

	// 发生错误，终止所有挂起的调用
	c.mutex.Lock()
	c.shutdown = true
	for key, call := range c.pending {
		call.Error = err
		call.Done <- call
		delete(c.pending, key)
	}
	c.mutex.Unlock()
}

// Close 关闭客户端连接。
func (c *Client) Close() error {
	c.mutex.Lock()
	if c.closing {
		c.mutex.Unlock()
		return errors.New("client is closing")
	}
	c.closing = true
	c.mutex.Unlock()
	return c.conn.Close()
}

// Call 发起一个同步调用，使用内部自增 ID。
func (c *Client) Call(method string, args, reply interface{}, timeout time.Duration) error {
	c.mutex.Lock()
	c.seq++
	seqID := c.seq
	c.mutex.Unlock()

	// 调用新的底层 CallWithID 方法
	return c.CallWithID(seqID, method, args, reply, timeout)
}

// Go 发起一个异步调用，使用内部自增 ID。
func (c *Client) Go(method string, args, reply interface{}, done chan *Call) *Call {
	c.mutex.Lock()
	c.seq++
	seqID := c.seq
	c.mutex.Unlock()

	// 调用新的底层 GoWithID 方法
	return c.GoWithID(seqID, method, args, reply, done)
}

// CallWithID 发起一个同步调用，允许用户指定请求 ID。
func (c *Client) CallWithID(id interface{}, method string, args, reply interface{}, timeout time.Duration) error {
	// 默认超时时间
	if timeout.Seconds() == 0 {
		timeout = 5 * time.Second
	}
	call := c.GoWithID(id, method, args, reply, make(chan *Call, 1))
	select {
	case <-call.Done:
		return call.Error
	case <-time.After(timeout * time.Second):
		return errors.New("jsonrpc2: call timeout")
	}
}

// GoWithID 发起一个异步调用，允许用户指定请求 ID。
func (c *Client) GoWithID(id interface{}, method string, args, reply interface{}, done chan *Call) *Call {
	if done == nil {
		done = make(chan *Call, 10) // 缓冲以避免阻塞
	}
	call := &Call{
		Method: method,
		Args:   args,
		Reply:  reply,
		Done:   done,
	}

	c.send(id, call)
	return call
}

// Ping 用于检测客户端连接是否仍然活跃。
func (c *Client) Ping() bool {
	var reply string // 期望收到 "pong"

	err := c.Call("ping", nil, &reply, 5)
	if err != nil {
		return false
	}
	if reply == "pong" {
		return true
	}

	return false
}

// send 是一个底层的发送函数，处理所有类型的 ID。
func (c *Client) send(id interface{}, call *Call) {
	if id == nil {
		call.Error = errors.New("jsonrpc2: request id cannot be null for a call that expects a reply")
		call.Done <- call
		return
	}

	c.mutex.Lock()
	if c.shutdown || c.closing {
		c.mutex.Unlock()
		call.Error = errors.New("client is shut down or closing")
		call.Done <- call
		return
	}

	idKey, err := idToKey(id)
	if err != nil {
		c.mutex.Unlock()
		call.Error = err
		call.Done <- call
		return
	}
	c.pending[idKey] = call
	c.mutex.Unlock()

	params, _ := json.Marshal(call.Args)
	req := &protocol.Request{
		Jsonrpc: "2.0",
		Method:  call.Method,
		Params:  params,
		ID:      id,
	}

	c.sendMutex.Lock()
	err = c.encoder.Encode(req)
	c.sendMutex.Unlock()

	if err != nil {
		c.mutex.Lock()
		// 确保我们删除的是同一个 call
		if c.pending[idKey] == call {
			delete(c.pending, idKey)
		}
		c.mutex.Unlock()

		call.Error = err
		call.Done <- call
	}
}

// idToKey 辅助函数，将各种 ID 类型转换为唯一的字符串 key，用于 map。
func idToKey(id interface{}) (string, error) {
	switch v := id.(type) {
	case string:
		return v, nil
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%d", v), nil
	case float32, float64:
		return fmt.Sprintf("%.0f", v), nil // 优化点
	default:
		return "", fmt.Errorf("jsonrpc2: unsupported id type '%T' for map key", v)
	}
}
