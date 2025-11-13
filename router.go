package jsonrpc2

import "sync"

// handlerEntry 直接存储处理器链
type handlerEntry struct {
	chain []HandlerFunc
}

type router struct {
	mu       sync.RWMutex
	handlers map[string]*handlerEntry
}

func newRouter() *router {
	return &router{
		handlers: make(map[string]*handlerEntry),
	}
}

// add 接收一个或多个 HandlerFunc，它们共同构成一个处理链
func (r *router) add(method string, handlers ...HandlerFunc) {
	if len(handlers) == 0 {
		panic("jsonrpc2: handler chain cannot be empty")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handlers[method] = &handlerEntry{
		chain: handlers,
	}
}

func (r *router) find(method string) (*handlerEntry, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	entry, ok := r.handlers[method]
	return entry, ok
}
