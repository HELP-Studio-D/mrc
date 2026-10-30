package mrc

import "errors"

var (
	ErrServerNotInitialized error = errors.New("Server not initialized")
)

// Run function for start a server
func (e *Engine) Run() error {
    return e.Start()
}

func (e *Engine) Stop() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e == nil || e.native == nil {
		return ErrServerNotInitialized
	}

	e.started = false
	return e.native.Stop()
}