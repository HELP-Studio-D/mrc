package mrc

import (
	"context"
	"errors"

	cabi "github.com/Edestuss0/MMA-Network-protocol/cabi"
)

// ChannelHandlerFunc handles messages received through an established channel.
type ChannelHandlerFunc func(c *ChannelContext)

// ChannelMessage represents an outgoing channel message.
type ChannelMessage struct {
	ReqID   uint32
	Payload []byte
	Options []RequestOption
}

var (
	ErrChannelContextExpired = errors.New("mrc: channel context expired")
	ErrChannelUnavailable    = errors.New("mrc: channel sender unavailable")
)

// Send sends a message to the current channel.
func (c *ChannelContext) Send(payload []byte) error {
	return c.SendMessage(ChannelMessage{
		Payload: payload,
	})
}

// SendMessage sends a message with optional metadata.
func (c *ChannelContext) SendMessage(message ChannelMessage) error {
	if c == nil || !c.active {
		return ErrChannelContextExpired
	}

	if c.engine == nil {
		return ErrChannelUnavailable
	}

	err := c.engine.SendToChannel(c.ConnectionID, message)

	if err != nil && c.writeErr == nil {
		c.writeErr = err
	}

	return err
}

// Broadcast sends a message to all connected channels.
func (c *ChannelContext) Broadcast(payload []byte) error {
	if c == nil || !c.active {
		return ErrChannelContextExpired
	}

	if c.engine == nil {
		return ErrChannelUnavailable
	}

	err := c.engine.Broadcast(ChannelMessage{
		Payload: payload,
	})

	if err != nil && c.writeErr == nil {
		c.writeErr = err
	}

	return err
}

// expire invalidates the channel context after callback execution.
func (c *ChannelContext) expire() {
	c.active = false
	c.engine = nil
}

func (e *Engine) HandleChannel(
	route string,
	handler ChannelHandlerFunc,
) error {
	if e == nil || e.native == nil {
		return ErrServerNotInitialized
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if e.closed {
		return cabi.ErrClosed
	}

	if e.started {
		return ErrServerStarted
	}

	if route == "" || handler == nil {
		return ErrInvalidRoute
	}

	// A route cannot be registered as both Once and Channel.
	if _, exists := e.routes[route]; exists {
		return ErrRouteAlreadyExists
	}

	if _, exists := e.channelRoutes[route]; exists {
		return ErrRouteAlreadyExists
	}

	// Convert the public handler into a native callback.
	callback := makeChannelRouteHandler(e, handler)

	// Register the Channel handler in the native server.
	if err := e.native.RegisterChannelHandler(route, callback); err != nil {
		return err
	}

	e.channelRoutes[route] = struct{}{}

	return nil
}

// SendToChannel sends a message to a specific channel connection.
// Connection ID 0 is reserved for broadcasting.
func (e *Engine) SendToChannel(
	connectionID uint32,
	message ChannelMessage,
) error {
	if e == nil {
		return ErrServerNotInitialized
	}

	e.channelHandleMu.RLock()
	handle := e.channelHandle
	e.channelHandleMu.RUnlock()

	if handle == nil {
		return ErrChannelUnavailable
	}

	return handle.SendToChannel(connectionID, cabi.Response{
		Opcode:  cabi.StatusMessage,
		ReqID:   message.ReqID,
		Payload: append([]byte(nil), message.Payload...),
		Options: append([]cabi.Option(nil), message.Options...),
	})
}

// Broadcast sends a message to all connected channels.
func (e *Engine) Broadcast(message ChannelMessage) error {
	return e.SendToChannel(0, message)
}

// makeChannelRouteHandler converts a public handler into a native C ABI callback.
func makeChannelRouteHandler(
	engine *Engine,
	handler ChannelHandlerFunc,
) cabi.ChannelHandler {
	return func(
		nativeRequest *cabi.Request,
		connectionID uint32,
	) error {
		if nativeRequest == nil || connectionID == 0 {
			return cabi.ErrInvalidArgument
		}

		requestCtx, cancel := context.WithCancel(context.Background())
		defer cancel()

		request := NewRequest(nativeRequest, requestCtx)

		ctx := &ChannelContext{
			Request:      request,
			ConnectionID: connectionID,
			engine:       engine,
			active:       true,
		}

		defer ctx.expire()

		handler(ctx)

		return ctx.writeErr
	}
}
