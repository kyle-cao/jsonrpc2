package jsonrpc2

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net"
	"sync"

	"github.com/kyle-cao/jsonrpc2/protocol"
)

type Server struct {
	router *router
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
	defer listener.Close()

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("jsonrpc2: failed to accept connection: %v", err)
			continue
		}
		go s.handleConnection(conn)
	}
}

func (s *Server) handleConnection(conn net.Conn) {
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
		go s.handleRequest(encoder, &sendMutex, conn, &req)
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

	var resp protocol.Response
	resp.Jsonrpc = "2.0"
	resp.ID = id
	switch val := data.(type) {
	case *protocol.ErrorObject:
		resp.Error = val
	default:
		resp.Result = val
	}
	if err := encoder.Encode(&resp); err != nil {
		log.Printf("jsonrpc2: failed to write response: %v", err)
	}
}
