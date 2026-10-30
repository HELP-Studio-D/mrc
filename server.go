package mrc

import (
	"errors"

	cabi "github.com/Edestuss0/MMA-Network-protocol/cabi"
)

var (
	ErrServerNotInitialized error = errors.New("Server not initialized")
)

// Run function for start a server
func (e *Engine) Run() error {
    return e.Start()
}

func (e *Engine) Stop() error {
	if e == nil || e.native == nil {
		return ErrServerNotInitialized
	}

	e.lifecycleMu.Lock()
	defer e.lifecycleMu.Unlock()

	e.mu.Lock()

	if e.closed {
		e.mu.Unlock()
		return cabi.ErrClosed
	}

	if !e.started {
		e.mu.Unlock()
		return nil
	}

	e.mu.Unlock()

	handle := e.takeChannelHandle()

	if handle != nil {
		_ = handle.Close()
	}

	return e.native.Stop()
}

func (e *Engine) takeChannelHandle() *cabi.ServerHandle {
	e.channelHandleMu.Lock()
	defer e.channelHandleMu.Unlock()

	handle := e.channelHandle
	e.channelHandle = nil

	return handle
}