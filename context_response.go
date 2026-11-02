package mrc

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/goccy/go-json"
	"sync"
)

const (
	OptionContentType = "content-type"

	MIMEApplicationJSON = "application/json"
	MIMETextPlain       = "text/plain; charset=utf-8"
)

// Status sets the response status without writing the payload
func (c *Context) Status(status Status) error {
	if err := c.ensureWritable(); err != nil {
		return c.recordWriteError(err)
	}

	if err := c.response.SetStatus(status); err != nil {
		return c.recordWriteError(err)
	}

	c.status = status
	return nil
}

const maxPooledJSONBufferCap = 1 << 20 // 1 MiB

// JSON serializes the object and sets it in the payload.
func (c *Context) JSON(status Status, value any) error {
	if err := c.ensureWritable(); err != nil {
		return c.recordWriteError(err)
	}

	buf := jsonBufferPool.Get().(*bytes.Buffer)
	buf.Reset()

	defer func() {
		if buf.Cap() <= maxPooledJSONBufferCap {
			buf.Reset()
			jsonBufferPool.Put(buf)
		}
	}()

	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(false)

	if err := enc.Encode(value); err != nil {
		return fmt.Errorf("mrc: JSON encode: %w", err)
	}

	payload := buf.Bytes()

	if n := len(payload); n > 0 && payload[n-1] == '\n' {
		payload = payload[:n-1]
	}

	return c.write(status, MIMEApplicationJSON, payload)
}

// Data writes arbitrary bytes to the payload
func (c *Context) Data(
	status Status,
	contentType string,
	data []byte,
) error {
	return c.write(status, contentType, data)
}

// String sends a text payload
func (c *Context) String(
	status Status,
	format string,
	values ...any,
) error {
	value := fmt.Sprintf(format, values...)

	return c.write(
		status,
		MIMETextPlain,
		[]byte(value),
	)
}

// Unified internal operation for writing a response
func (c *Context) write(
	status Status,
	contentType string,
	payload []byte,
) error {
	if err := c.ensureWritable(); err != nil {
		return err
	}

	if contentType != "" {
		if err := c.AddResponseOption(
			OptionContentType,
			contentType,
		); err != nil {
			return err
		}
	}

	if err := c.response.SetStatus(status); err != nil {
		return c.recordWriteError(err)
	}

	if err := c.response.SetPayload(payload); err != nil {
		return c.recordWriteError(err)
	}

	c.status = status
	c.responseSize = len(payload)
	c.written = true

	return nil
}

// AddResponseOption adds response metadata
func (c *Context) AddResponseOption(key, value string) error {
	if err := c.ensureWritable(); err != nil {
		return c.recordWriteError(err)
	}

	if key == "" || value == "" {
		return c.recordWriteError(
			errors.New("mrc: option key and value cannot be empty"),
		)
	}

	if previous, exists := c.responseOptions[key]; exists {
		if previous == value {
			return nil
		}

		return c.recordWriteError(
			fmt.Errorf("mrc: response option %q already exists", key),
		)
	}

	if err := c.response.AddOption(key, value); err != nil {
		return c.recordWriteError(err)
	}

	c.responseOptions[key] = value
	return nil
}

// AbortWithStatusJSON generates a JSON response and aborts the chain
func (c *Context) AbortWithStatusJSON(
	status Status,
	value any,
) error {
	err := c.JSON(status, value)
	c.Abort()
	return err
}

// AbortWithStatus sets the status and interrupts the chain
func (c *Context) AbortWithStatus(status Status) error {
	err := c.Status(status)
	c.Abort()
	return err
}

func (c *Context) ensureWritable() error {
	if c == nil || !c.active {
		return ErrContextExpired
	}

	if c.response == nil {
		return ErrResponseUnavailable
	}

	if c.written {
		return ErrResponseAlreadySent
	}

	return nil
}

func (c *Context) recordWriteError(err error) error {
	if err != nil && c.writeErr == nil {
		c.writeErr = err
	}

	return err
}

var jsonBufferPool = sync.Pool{
	New: func() any {
		return new(bytes.Buffer)
	},
}
