package mrc

import "context"

// ChannelContext provides access to an established channel.
type ChannelContext struct {
	Request      *Request
	ConnectionID uint32

	engine *Engine
	active bool
	writeErr error
}

// Context returns the standard request context.
func (c *ChannelContext) Context() context.Context {
	if c == nil || c.Request == nil {
		return context.Background()
	}

	return c.Request.Context()
}