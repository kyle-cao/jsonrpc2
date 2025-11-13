package jsonrpc2

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net"
	"sync"

	"github.com/kyle-cao/jsonrpc2/protocol"
)

type Server struct {
	router   *router
	mu       sync.Mutex // 保护 listener 字段
	listener net.Listener
	wg       sync.WaitGroup // 用于追踪活动的连接处理 goroutine
}

func NewServer() *Server {
	return &Server{
		router: newRouter(),
	}
}

func (s *Server) Handle(method string, handlers ...HandlerFunc) {
	s.router.add(method, handlers...)
}

func (s *Server) Listen(addr string) error {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	s.mu.Lock()
	s.listener = listener
	s.mu.Unlock()

	go s.acceptLoop()
	return nil
}

func (s *Server) acceptLoop() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				log.Println("jsonrpc2: listener closed, shutting down accept loop.")
				return
			}
			log.Printf("jsonrpc2: failed to accept connection: %v", err)
			continue
		}
		s.wg.Add(1)
		go s.handleConnection(conn)
	}
}

func (s *Server) Close(ctx context.Context) error {
	s.mu.Lock()
	listener := s.listener
	s.mu.Unlock()

	if listener == nil {
		return errors.New("jsonrpc2: server not started")
	}

	err := listener.Close()

	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *Server) handleConnection(conn net.Conn) {
	defer s.wg.Done()
	defer conn.Close()

	decoder := json.NewDecoder(conn)
	encoder := json.NewEncoder(conn)
	var sendMutex sync.Mutex

	for {
		var req protocol.Request
		if err := decoder.Decode(&req); err != nil {
			if err != io.EOF {
				s.writeResponse(encoder, &sendMutex, nil, protocol.ParseError(err.Error()))
			}
			return
		}
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.handleRequest(encoder, &sendMutex, conn, &req)
		}()
	}
}

func (s *Server) handleRequest(encoder *json.Encoder, sendMutex *sync.Mutex, conn net.Conn, req *protocol.Request) {
	if req.ID == nil {
		s.writeResponse(encoder, sendMutex, nil, protocol.ParseError(req.ID))
		return
	}
	entry, found := s.router.find(req.Method)
	if !found {
		s.writeResponse(encoder, sendMutex, req.ID, protocol.MethodNotFoundError(req.Method))
		return
	}
	ctx := &Context{
		Context:      context.Background(),
		Conn:         conn,
		Request:      req,
		handlerChain: entry.chain,
		handlerIdx:   -1,
	}
	ctx.Next()
	if ctx.responseError != nil {
		s.writeResponse(encoder, sendMutex, req.ID, ctx.responseError)
	} else {
		s.writeResponse(encoder, sendMutex, req.ID, ctx.responseResult)
	}
}

func (s *Server) writeResponse(encoder *json.Encoder, m *sync.Mutex, id interface{}, data interface{}) {
	m.Lock()
	defer m.Unlock()
	if err := encoder.Encode(createResponse(id, data)); err != nil {
		log.Printf("jsonrpc2: failed to write response: %v", err)
	}
}

// createResponse 是一个辅助函数，用于构建响应对象
func createResponse(id interface{}, data interface{}) protocol.Response {
	resp := protocol.Response{
		Jsonrpc: "2.0",
		ID:      id,
	}
	switch val := data.(type) {
	case *protocol.ErrorObject:
		resp.Error = val
	default:
		resp.Result = val
	}
	return resp
}
