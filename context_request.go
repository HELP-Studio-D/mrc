package mrc

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/Edestuss0/MMA-Network-protocol/cabi"
)

type RequestOption = cabi.Option

type Request struct {
	Opcode  cabi.RequestOpcode
	Version uint8
	ReqID   uint32

	Route   string
	Options []RequestOption
	Payload []byte

	ctx context.Context
}

var ErrEmptyPayload = errors.New("mrc: request payload is empty")

func NewRequest(source *cabi.Request, ctx context.Context) *Request {
	options := append([]RequestOption(nil), source.Options...)
	payload := append([]byte(nil), source.Payload...)

	return &Request{
		Opcode:  source.Opcode,
		Version: source.Version,
		ReqID:   source.ReqID,
		Route:   source.Route,
		Options: options,
		Payload: payload,
		ctx:     ctx,
	}
}

// Context returns the request execution context
func (r *Request) Context() context.Context {
	if r == nil || r.ctx == nil {
		return context.Background()
	}

	return r.ctx
}

// GetOption retrieves an option by its key
func (r *Request) GetOption(key string) (string, bool) {
	for _, option := range r.Options {
		if option.Key == key {
			return option.Value, true
		}
	}

	return "", false
}

// GetOptionBytes returns the option value as bytes
func (r *Request) GetOptionBytes(key string) ([]byte, bool) {
	value, exists := r.GetOption(key)
	if !exists {
		return nil, false
	}

	return []byte(value), true
}

// ShouldBindJSON decodes JSON from Request.Payload
func (c *Context) ShouldBindJSON(destination any) error {
	if c.Request == nil {
		return errors.New("mrc: request is unavailable")
	}

	if len(c.Request.Payload) == 0 {
		return ErrEmptyPayload
	}

	return json.Unmarshal(c.Request.Payload, destination)
}

// BindJSON decodes JSON and automatically responds with BadRequest
// if the payload is invalid
func (c *Context) BindJSON(destination any) error {
	if err := c.ShouldBindJSON(destination); err != nil {
		c.Error(err)

		writeErr := c.AbortWithStatusJSON(
			StatusBadRequest,
			H{"error": "Invalid JSON payload"},
		)

		if writeErr != nil {
			return errors.Join(err, writeErr)
		}

		return err
	}

	return nil
}
