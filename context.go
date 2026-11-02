package mrc

import (
	"context"
	"errors"
	"sync"

	"github.com/Edestuss0/MMA-Network-protocol/cabi"
)

type Status = cabi.ResponseOpcode

type HandlerFunc func(*Context)
type H map[string]any

var (
	ErrContextExpired      = errors.New("mrc: context expired")
	ErrResponseAlreadySent = errors.New("mrc: response already written")
	ErrResponseUnavailable = errors.New("mrc: response is unavailable")
)

type responseWriter interface {
	SetStatus(Status) error
	SetPayload([]byte) error
	AddOption(key, value string) error
}

type Context struct {
	// Public info about request
	Request *Request

	// Internal response builder
	response responseWriter

	// Middlewares & Handlers
	handlers []HandlerFunc
	index    int
	aborted  bool

	// Current request data
	keys map[string]any

	// Server error
	errors []error

	// Response status
	status       Status
	responseSize int
	written      bool
	writeErr     error

	// Response options, analog HTTP header
	responseOptions map[string]string

	// Live cycle
	active bool

	mu sync.RWMutex
}

func NewContext(request *Request, response responseWriter, handlers []HandlerFunc) *Context {
	return &Context{
		Request:         request,
		response:        response,
		handlers:        handlers,
		index:           -1,
		keys:            make(map[string]any),
		responseOptions: make(map[string]string),
		status:          StatusOK,
		active:          true,
	}
}

// Next triggers the remainder of the chain
func (c *Context) Next() {
	if !c.active {
		return
	}

	c.index++

	for c.index < len(c.handlers) && !c.aborted {
		handler := c.handlers[c.index]
		handler(c)
		c.index++
	}
}

// Abort terminates the execution of the chain
func (c *Context) Abort() {
	if !c.active {
		return
	}

	c.aborted = true
}

// IsAborted checks whether the chain has been aborted
func (c *Context) IsAborted() bool {
	return c.aborted
}

// Context returns the request's standard context.Context
func (c *Context) Context() context.Context {
	return c.Request.Context()
}

// Set stores data within the current request.
func (c *Context) Set(key string, value any) {
	if c == nil || !c.active {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.active {
		return
	}

	if c.keys == nil {
		c.keys = make(map[string]any)
	}

	c.keys[key] = value
}

// Get retrieves data from the current request.
func (c *Context) Get(key string) (any, bool) {
	if c == nil {
		return nil, false
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.active {
		return nil, false
	}

	value, exists := c.keys[key]
	return value, exists
}

// MustGet retrieves data or panics.
func (c *Context) MustGet(key string) any {
	value, exists := c.Get(key)
	if !exists {
		panic("mrc: key does not exist: " + key)
	}

	return value
}

// Delete removes data from the current request.
func (c *Context) Delete(key string) {
	if c == nil || !c.active {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.active {
		return
	}

	delete(c.keys, key)
}

// Error stores a server error
func (c *Context) Error(err error) {
	if !c.active {
		return
	}

	if err == nil {
		return
	}

	c.errors = append(c.errors, err)
}

// Errors returns the errors for the current request
func (c *Context) Errors() []error {
	return append([]error(nil), c.errors...)
}

// Written indicates whether the response payload has been generated.
func (c *Context) Written() bool {
	return c.written
}

// ResponseStatus returns the current status.
func (c *Context) ResponseStatus() Status {
	return c.status
}

// ResponseSize returns the size of the outgoing payload.
func (c *Context) ResponseSize() int {
	return c.responseSize
}

func (c *Context) expire() {
	if c == nil {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.active = false
	c.response = nil
	c.handlers = nil
	c.keys = nil
}