package mrc

import (
	"context"
	"errors"

	cabi "github.com/Edestuss0/MMA-Network-protocol/cabi"
)

// cabiResponseWriter adapts the native C ABI response builder
// to the response interface used by the high-level Context.
type cabiResponseWriter struct {
	builder *cabi.ResponseBuilder
}

// Native callback.
type RouteHandler func(
	request *cabi.Request,
	response *cabi.ResponseBuilder,
) error

var (
	ErrInvalidRoute       = errors.New("mrc: invalid route")
	ErrRouteAlreadyExists = errors.New("mrc: route already exists")
	ErrServerStarted      = errors.New("mrc: cannot register routes after Start")
)

// Handle registers a new route and its handler chain.
func (e *Engine) Handle(
	route string,
	handlers ...HandlerFunc,
) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.closed {
		return cabi.ErrClosed
	}

	if e.started {
		return ErrServerStarted
	}

	if route == "" || len(handlers) == 0 {
		return ErrInvalidRoute
	}

	if _, exists := e.routes[route]; exists {
		return ErrRouteAlreadyExists
	}

	// Copy global middleware.
	chain := make(HandlersChain, 0, len(e.middleware)+len(handlers))
	chain = append(chain, e.middleware...)

	// Append route-specific handlers.
	chain = append(chain, handlers...)

	// Convert the high-level handler chain into a C ABI callback.
	callback := makeRouteHandler(chain)

	// Register the route in the underlying Rust server.
	if err := e.native.RegisterRoute(route, callback); err != nil {
		return err
	}

	e.routes[route] = struct{}{}

	return nil
}

// AddOption adds a key-value option to the native response.
func (w cabiResponseWriter) AddOption(key, value string) error {
	if w.builder == nil {
		return cabi.ErrInvalidArgument
	}

	return w.builder.AddOption(key, value)
}

// SetStatus sets the response status in the native response.
func (w cabiResponseWriter) SetStatus(status Status) error {
    if w.builder == nil {
        return cabi.ErrInvalidArgument
    }

    return w.builder.SetStatus(cabi.ResponseOpcode(status))
}

// SetPayload writes the response payload.
func (w cabiResponseWriter) SetPayload(payload []byte) error {
    if w.builder == nil {
        return cabi.ErrInvalidArgument
    }

    return w.builder.SetPayload(payload)
}

func makeRouteHandler(handlers HandlersChain) cabi.RouteHandler {
	// Create a copy so the registered callback has its own
	// immutable handler chain.
	chain := append(HandlersChain(nil), handlers...)

	return func(
		nativeRequest *cabi.Request,
		nativeResponse *cabi.ResponseBuilder,
	) error {
		if nativeRequest == nil || nativeResponse == nil {
			return cabi.ErrInvalidArgument
		}

		// Create a context for the current request.
		requestCtx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Convert the native request into the public MMA request.
		request := NewRequest(nativeRequest, requestCtx)

		// Adapt the native response builder.
		writer := cabiResponseWriter{
			builder: nativeResponse,
		}

		// Create the high-level request context.
		ctx := NewContext(
			request,
			writer,
			chain,
		)

		// The context must not be used after the callback returns.
		defer ctx.expire()

		// Execute all middleware and the route handler.
		ctx.Next()

		// Propagate response writing errors to the C ABI layer.
		return ctx.writeErr
	}
}

// Ensure cabiResponseWriter implements responseWriter.
var _ responseWriter = cabiResponseWriter{}

// Keep this import useful for future adapter-level error wrapping.
var _ = errors.Is
