package mrc

// HandlersChain represents a chain of request handlers and middleware.
type HandlersChain []HandlerFunc

// Use registers middleware for the engine.
func (e *Engine) Use(handlers ...HandlerFunc) {
    e.mu.Lock()
    defer e.mu.Unlock()

    if handlers == nil {
        return
    }
    e.middleware = append(e.middleware, handlers...)
}