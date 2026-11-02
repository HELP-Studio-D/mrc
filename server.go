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
    if err := e.start(); err != nil {
        return err
    }

    <-e.done
    return nil
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

// Start starts the underlying native server.
func (e *Engine) start() error {
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

	if e.started {
		e.mu.Unlock()
		return cabi.ErrServerRunning
	}

	if err := e.native.Start(); err != nil {
		e.mu.Unlock()
		return err
	}

	e.started = true

	handle, err := e.native.GetHandle()
	if err != nil {
		e.mu.Unlock()
		_ = e.native.Stop()
		return err
	}

	e.channelHandleMu.Lock()
	e.channelHandle = handle
	e.channelHandleMu.Unlock()

	e.mu.Unlock()

	return nil
}

// Close releases the native server resources.
func (e *Engine) Close() error {
	if e == nil {
		return nil
	}

	e.lifecycleMu.Lock()
	defer e.lifecycleMu.Unlock()

	e.mu.Lock()

	if e.closed {
		e.mu.Unlock()
		return nil
	}

	e.closed = true
	native := e.native

	e.closeOnce.Do(func() {
        close(e.done)
    })

	e.mu.Unlock()

	handle := e.takeChannelHandle()

	if handle != nil {
		_ = handle.Close()
	}

	if native == nil {
		return ErrServerNotInitialized
	}

	return native.Close()
}

func (e *Engine) takeChannelHandle() *cabi.ServerHandle {
	e.channelHandleMu.Lock()
	defer e.channelHandleMu.Unlock()

	handle := e.channelHandle
	e.channelHandle = nil

	return handle
}