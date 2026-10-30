package middleware

import (
    "fmt"
    "io"
    "log"
    "os"
    "runtime/debug"
    "time"
    "mrc"
)

// RecoveryFunc defines a custom panic recovery handler.
type RecoveryFunc func(c *mrc.Context, recovered any)

// DefaultErrorWriter is the default destination for recovery logs.
var DefaultErrorWriter io.Writer = os.Stderr

// Recovery returns a middleware that recovers from panics
// and sets an internal server error status.
func Recovery() mrc.HandlerFunc {
    return RecoveryWithWriter(DefaultErrorWriter)
}

// CustomRecovery returns a middleware with a custom recovery handler.
func CustomRecovery(handle RecoveryFunc) mrc.HandlerFunc {
    return CustomRecoveryWithWriter(DefaultErrorWriter, handle)
}

// RecoveryWithWriter returns a recovery middleware with a custom log writer.
func RecoveryWithWriter(out io.Writer) mrc.HandlerFunc {
    return CustomRecoveryWithWriter(out, defaultHandleRecovery)
}

// CustomRecoveryWithWriter returns a recovery middleware
// with a custom writer and recovery handler.
func CustomRecoveryWithWriter(
    out io.Writer,
    handle RecoveryFunc,
) mrc.HandlerFunc {
    if handle == nil {
        handle = defaultHandleRecovery
    }

    // Create a logger only when an output writer is provided.
    var logger *log.Logger

    if out != nil {
        logger = log.New(out, "", log.LstdFlags)
    }

    return func(c *mrc.Context) {
        // Recover must be called inside a deferred function.
        defer func() {
            recovered := recover()

            if recovered == nil {
                return
            }

            // Convert the recovered value into a Go error.
            panicErr := panicToError(recovered)

            // Save the panic in the current request context.
            c.Error(panicErr)

            // Capture the current goroutine stack trace.
            stackTrace := debug.Stack()

            // Log panic information without exposing request payloads
            // or potentially sensitive options.
            if logger != nil {
                logger.Printf(
                    "[Recovery] %s panic recovered: route=%q req_id=%d opcode=%d panic=%v\n%s",
                    time.Now().Format("2006/01/02 - 15:04:05"),
                    requestRoute(c),
                    requestID(c),
                    requestOpcode(c),
                    recovered,
                    stackTrace,
                )
            }

            // Stop the remaining middleware chain.
            c.Abort()

            // Execute the configured recovery handler safely.
            callRecoveryHandler(c, handle, recovered, logger)

            // Ensure that the request receives an internal error
            // response when the custom handler did not write one.
            if !c.Written() {
                if err := c.AbortWithStatus(mrc.StatusInternalError); err != nil {
                    c.Error(err)

                    if logger != nil {
                        logger.Printf(
                            "[Recovery] failed to write internal error response: %v",
                            err,
                        )
                    }
                }
            }
        }()

        // Execute the remaining middleware and route handlers.
        c.Next()
    }
}

// defaultHandleRecovery sets the default MMA internal error status.
func defaultHandleRecovery(c *mrc.Context, recovered any) {
    if c.Written() {
        return
    }

    if err := c.AbortWithStatus(mrc.StatusInternalError); err != nil {
        c.Error(err)
    }
}

// callRecoveryHandler protects the recovery handler itself from panics.
func callRecoveryHandler(
    c *mrc.Context,
    handle RecoveryFunc,
    recovered any,
    logger *log.Logger,
) {
    defer func() {
        if recoveryPanic := recover(); recoveryPanic != nil {
            err := panicToError(recoveryPanic)

            c.Error(fmt.Errorf("recovery handler panicked: %w", err))

            if logger != nil {
                logger.Printf(
                    "[Recovery] custom recovery handler panicked: %v\n%s",
                    recoveryPanic,
                    debug.Stack(),
                )
            }
        }
    }()

    handle(c, recovered)
}

// panicToError converts a recovered panic value into an error.
func panicToError(value any) error {
    if err, ok := value.(error); ok {
        return fmt.Errorf("panic: %w", err)
    }

    return fmt.Errorf("panic: %v", value)
}

// requestRoute returns the route associated with the current request.
func requestRoute(c *mrc.Context) string {
    if c == nil || c.Request == nil {
        return "<unknown>"
    }

    return c.Request.Route
}

// requestID returns the ID of the current request.
func requestID(c *mrc.Context) uint32 {
    if c == nil || c.Request == nil {
        return 0
    }

    return c.Request.ReqID
}

// requestOpcode returns the opcode of the current request.
func requestOpcode(c *mrc.Context) uint8 {
    if c == nil || c.Request == nil {
        return 0
    }

    return uint8(c.Request.Opcode)
}
