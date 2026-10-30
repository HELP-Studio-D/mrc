package middleware

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync/atomic"
	"time"
	"mrc"
)

// consoleColorModeValue defines how console colors are handled.
type consoleColorModeValue uint32

const (
	color consoleColorModeValue = iota
	disableColor
	forceColor
)

// consoleColorMode stores the current console color mode.
var consoleColorMode atomic.Uint32

// ANSI color codes.
const (
	green   = "\033[97;42m"
	white   = "\033[90;47m"
	yellow  = "\033[90;43m"
	red     = "\033[97;41m"
	blue    = "\033[97;44m"
	magenta = "\033[97;45m"
	cyan    = "\033[97;46m"
	reset   = "\033[0m"
)

// DefaultWriter is the default destination for log output.
var DefaultWriter io.Writer = os.Stdout

// LogConf defines the configuration for Logger middleware.
type LogConf struct {
	// Formatter defines a custom log formatter.
	// Optional. Defaults to defaultLogFormatter.
	Formatter LogFormatter

	// Output defines where log messages are written.
	// Optional. Defaults to DefaultWriter.
	Output io.Writer

	// SkipRoutes contains route names that should not be logged.
	// Optional.
	SkipRoutes []string

	// Skip defines a function that determines whether a request should be skipped.
	// Optional.
	Skip Skipper
}

// Skipper defines a function that determines whether a request should be skipped.
type Skipper func(ctx *mrc.Context) bool

// LogFormatter defines the signature of a custom log formatter.
type LogFormatter func(params LogFormatterParams) string

// LogFormatterParams contains all information passed to a log formatter.
type LogFormatterParams struct {
	// Request contains the incoming request.
	Request *mrc.Request

	// TimeStamp is the time when request processing finished.
	TimeStamp time.Time

	// StatusCode is the mrc response status code.
	StatusCode int

	// Latency is the time spent processing the request.
	Latency time.Duration

	// RequestSize is the size of the incoming payload in bytes.
	RequestSize int

	// ResponseSize is the size of the outgoing payload in bytes.
	ResponseSize int

	// OptionsCount is the number of incoming options.
	OptionsCount int

	// ErrorMessage contains errors reported during request processing.
	ErrorMessage string

	// isTerm indicates whether the output supports terminal colors.
	isTerm bool
}

// validStatusCodes contains all known mrc response status codes.
var validStatusCodes = map[int]struct{}{
	int(mrc.StatusShutdown):         {},
	int(mrc.StatusOK):               {},
	int(mrc.StatusBadRequest):       {},
	int(mrc.StatusForbidden):        {},
	int(mrc.StatusNotFound):         {},
	int(mrc.StatusConflict):         {},
	int(mrc.StatusInternalError):    {},
	int(mrc.StatusMessage):           {},
	int(mrc.StatusChannelConnected): {},
	int(mrc.StatusTimeout):          {},
	int(mrc.StatusUnauthorized):     {},
}

// StatusCodeColor returns the ANSI color associated with an mrc status.
func (p *LogFormatterParams) StatusCodeColor() string {
	code := p.StatusCode

	if _, exists := validStatusCodes[code]; !exists {
		return white
	}

	switch code {
	case int(mrc.StatusOK), int(mrc.StatusChannelConnected):
		return green
	case int(mrc.StatusMessage):
		return cyan
	case int(mrc.StatusTimeout):
		return yellow
	case int(mrc.StatusShutdown):
		return magenta
	default:
		return red
	}
}

// LatencyColor returns the ANSI color associated with request latency.
func (p *LogFormatterParams) LatencyColor() string {
	latency := p.Latency

	switch {
	case latency < 100*time.Millisecond:
		return white
	case latency < 200*time.Millisecond:
		return green
	case latency < 300*time.Millisecond:
		return cyan
	case latency < 500*time.Millisecond:
		return blue
	case latency < time.Second:
		return yellow
	case latency < 2*time.Second:
		return magenta
	default:
		return red
	}
}

// ResetColor resets all ANSI color attributes.
func (p *LogFormatterParams) ResetColor() string {
	return reset
}

// IsOutputColor indicates whether ANSI colors should be used.
func (p *LogFormatterParams) IsOutputColor() bool {
	mode := consoleColorModeValue(consoleColorMode.Load())

	return mode == forceColor ||
		(mode == color && p.isTerm)
}

// statusName returns a readable name for an mrc status code.
func statusName(code int) string {
	switch code {
	case int(mrc.StatusOK):
		return "OK"
	case int(mrc.StatusBadRequest):
		return "BAD_REQUEST"
	case int(mrc.StatusUnauthorized):
		return "UNAUTHORIZED"
	case int(mrc.StatusForbidden):
		return "FORBIDDEN"
	case int(mrc.StatusNotFound):
		return "NOT_FOUND"
	case int(mrc.StatusConflict):
		return "CONFLICT"
	case int(mrc.StatusInternalError):
		return "INTERNAL_ERROR"
	case int(mrc.StatusMessage):
		return "MESSAGE"
	case int(mrc.StatusChannelConnected):
		return "CHANNEL_CONNECTED"
	case int(mrc.StatusTimeout):
		return "TIMEOUT"
	case int(mrc.StatusShutdown):
		return "SHUTDOWN"
	default:
		return "UNKNOWN"
	}
}

// opcodeName returns a readable name for an mrc request opcode.
func opcodeName(opcode uint8) string {
	switch opcode {
	case 1:
		return "CHANNEL"
	case 2:
		return "ONCE"
	default:
		return "UNKNOWN"
	}
}

// defaultLogFormatter is the default log format used by Logger middleware.
var defaultLogFormatter LogFormatter = func(param LogFormatterParams) string {
	var (
		statusColor  string
		latencyColor string
		resetColor   string
	)

	if param.IsOutputColor() {
		statusColor = param.StatusCodeColor()
		latencyColor = param.LatencyColor()
		resetColor = param.ResetColor()
	}

	latency := param.Latency

	switch {
	case latency > time.Minute:
		latency = latency.Truncate(10 * time.Second)
	case latency > time.Second:
		latency = latency.Truncate(10 * time.Millisecond)
	case latency > time.Millisecond:
		latency = latency.Truncate(10 * time.Microsecond)
	}

	var (
		route     string
		reqID     uint32
		opcode    string
	)

	if param.Request != nil {
		route = param.Request.Route
		reqID = param.Request.ReqID
		opcode = opcodeName(uint8(param.Request.Opcode))
	}

	if route == "" {
		route = "-"
	}

	status := statusName(param.StatusCode)

	result := fmt.Sprintf(
		"[MRC] %s |%s %-18s %s|%s %8v %s| %-7s | req_id=%d | route=%q | request=%dB response=%dB options=%d",
		param.TimeStamp.Format("2006/01/02 - 15:04:05"),
		statusColor,
		status,
		resetColor,
		latencyColor,
		latency,
		resetColor,
		opcode,
		reqID,
		route,
		param.RequestSize,
		param.ResponseSize,
		param.OptionsCount,
	)

	if param.ErrorMessage != "" {
		result += fmt.Sprintf(" | error=%q", param.ErrorMessage)
	}

	return result
}

// DisableConsoleColor disables ANSI color output.
func DisableConsoleColor() {
	consoleColorMode.Store(uint32(disableColor))
}

// ForceConsoleColor forces ANSI color output.
func ForceConsoleColor() {
	consoleColorMode.Store(uint32(forceColor))
}

// Logger creates a Logger middleware with default configuration.
func Logger() mrc.HandlerFunc {
	return LoggerWithConfig(LogConf{})
}

// LoggerWithFormatter creates a Logger middleware with a custom formatter.
func LoggerWithFormatter(formatter LogFormatter) mrc.HandlerFunc {
	return LoggerWithConfig(LogConf{
		Formatter: formatter,
	})
}

// LoggerWithWriter creates a Logger middleware with a custom output writer.
// The optional route names specify which routes should not be logged.
func LoggerWithWriter(out io.Writer, notLogged ...string) mrc.HandlerFunc {
	return LoggerWithConfig(LogConf{
		Output:     out,
		SkipRoutes: notLogged,
	})
}

// LoggerWithConfig creates a Logger middleware with the provided configuration.
func LoggerWithConfig(conf LogConf) mrc.HandlerFunc {
	formatter := conf.Formatter
	if formatter == nil {
		formatter = defaultLogFormatter
	}

	out := conf.Output
	if out == nil {
		out = DefaultWriter
	}

	// Use log.Logger to make concurrent writes safe.
	logger := log.New(out, "", 0)

	// Detect whether the output supports terminal colors.
	isTerm := isTerminalWriter(out)

	// Prepare the route skip map for efficient lookup.
	var skip map[string]struct{}

	if len(conf.SkipRoutes) > 0 {
		skip = make(map[string]struct{}, len(conf.SkipRoutes))

		for _, route := range conf.SkipRoutes {
			skip[route] = struct{}{}
		}
	}

	return func(c *mrc.Context) {
		// Start measuring request processing time.
		start := time.Now()

		// Process the remaining middleware and route handlers.
		c.Next()

		// Skip logging for configured routes.
		if c.Request != nil {
			if _, exists := skip[c.Request.Route]; exists {
				return
			}
		}

		// Apply the custom skip function.
		if conf.Skip != nil && conf.Skip(c) {
			return
		}

		// Stop measuring request processing time.
		finished := time.Now()

		// Collect request information.
		params := LogFormatterParams{
			Request:   c.Request,
			TimeStamp: finished,
			Latency:   finished.Sub(start),
			isTerm:    isTerm,
		}

		if c.Request != nil {
			params.RequestSize = len(c.Request.Payload)
			params.OptionsCount = len(c.Request.Options)
		}

		// Collect response information.
		params.StatusCode = int(c.ResponseStatus())
		params.ResponseSize = c.ResponseSize()

		// Collect errors reported during request processing.
		params.ErrorMessage = formatLogErrors(c.Errors())

		// Format and write the final log entry.
		logger.Print(formatter(params))
	}
}

// formatLogErrors converts request errors into a readable string.
func formatLogErrors(errs []error) string {
	if len(errs) == 0 {
		return ""
	}

	messages := make([]string, 0, len(errs))

	for _, err := range errs {
		if err != nil {
			messages = append(messages, err.Error())
		}
	}

	return strings.Join(messages, "; ")
}

// isTerminalWriter checks whether the provided writer is a terminal.
func isTerminalWriter(writer io.Writer) bool {
	file, ok := writer.(*os.File)
	if !ok {
		return false
	}

	if os.Getenv("TERM") == "dumb" {
		return false
	}

	info, err := file.Stat()
	if err != nil {
		return false
	}

	return info.Mode()&os.ModeCharDevice != 0
}
