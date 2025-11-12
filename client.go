package jsonrpc2

import (
	"encoding/json"
	"errors"
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
	conn      net.Conn
	encoder   *json.Encoder
	sendMutex sync.Mutex // 保护发送
	mutex     sync.Mutex // 保护 Client 内部状态
	seq       uint64
	pending   map[uint64]*Call
	closing   bool
	shutdown  bool
}

func Dial(addr string) (*Client, error) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}
	client := &Client{
		conn:    conn,
		encoder: json.NewEncoder(conn),
		pending: make(map[uint64]*Call),
	}
	go client.receiveLoop()
	return client, nil
}

func (c *Client) receiveLoop() {
	var err error
	var res protocol.Response
	decoder := json.NewDecoder(c.conn)

	for err == nil {
		err = decoder.Decode(&res)
		if err != nil {
			break
		}

		seq, ok := res.ID.(float64) // JSON 数字默认为 float64
		if !ok {
			log.Printf("jsonrpc2: unexpected response ID type: %T", res.ID)
			continue
		}

		c.mutex.Lock()
		call := c.pending[uint64(seq)]
		delete(c.pending, uint64(seq))
		c.mutex.Unlock()

		if call != nil {
			if res.Error != nil {
				call.Error = res.Error
			} else {
				if call.Reply != nil {
					// 将结果 unmarshal 到 call.Reply 指针中
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
	for _, call := range c.pending {
		call.Error = err
		call.Done <- call
	}
	c.mutex.Unlock()
}

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

// Go 发起一个异步调用。
func (c *Client) Go(method string, args, reply interface{}, done chan *Call) *Call {
	call := &Call{
		Method: method,
		Args:   args,
		Reply:  reply,
	}
	if done == nil {
		done = make(chan *Call, 10) // 缓冲以避免阻塞
	}
	call.Done = done

	c.send(call)
	return call
}

// Call 发起一个同步调用。
func (c *Client) Call(method string, args, reply interface{}, timeout time.Duration) error {
	// 设置超时
	if timeout == 0 {
		timeout = 5
	}
	select {
	case call := <-c.Go(method, args, reply, make(chan *Call, 1)).Done:
		return call.Error
	case <-time.After(timeout * time.Second): // 5秒超时
		return errors.New("jsonrpc2: call timeout")
	}
}

func (c *Client) send(call *Call) {
	c.mutex.Lock()
	if c.shutdown || c.closing {
		c.mutex.Unlock()
		call.Error = errors.New("client is shut down or closing")
		call.Done <- call
		return
	}
	c.seq++
	seqID := c.seq
	c.pending[seqID] = call
	c.mutex.Unlock()

	params, _ := json.Marshal(call.Args)
	req := &protocol.Request{
		Jsonrpc: "2.0",
		Method:  call.Method,
		Params:  params,
		ID:      seqID,
	}

	c.sendMutex.Lock()
	err := c.encoder.Encode(req)
	c.sendMutex.Unlock()

	if err != nil {
		c.mutex.Lock()
		call = c.pending[seqID]
		delete(c.pending, seqID)
		c.mutex.Unlock()
		if call != nil {
			call.Error = err
			call.Done <- call
		}
	}
}
