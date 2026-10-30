package mrc

import (
	"fmt"
	"time"
)

// Option represents a function that modifies server configuration.
type Option func(*ServerConfig) error

// WithBindIP sets the server bind address.
func WithBindIP(ip string) Option {
	return func(cfg *ServerConfig) error {
		if ip == "" {
			return fmt.Errorf("mrc: bind IP cannot be empty")
		}

		cfg.BindIP = ip
		return nil
	}
}

// WithPort sets the server port.
func WithPort(port uint16) Option {
	return func(cfg *ServerConfig) error {
		cfg.Port = port
		return nil
	}
}

// WithMaxInFlight sets the maximum number of concurrent requests.
func WithMaxInFlight(limit uint32) Option {
	return func(cfg *ServerConfig) error {
		if limit == 0 {
			return fmt.Errorf("mrc: max in-flight must be greater than zero")
		}

		cfg.MaxInFlight = limit
		return nil
	}
}

// WithMaxBatch sets the maximum batch size.
func WithMaxBatch(limit uint32) Option {
	return func(cfg *ServerConfig) error {
		if limit == 0 {
			return fmt.Errorf("mrc: max batch must be greater than zero")
		}

		cfg.MaxBatch = limit
		return nil
	}
}

func WithFramer(framer FramerConfig) Option {
	return func(cfg *ServerConfig) error {
		cfg.Framer = FramerConfig{
			MaxMessageLength: framer.MaxMessageLength,
			OpcodePos:        framer.OpcodePos,
			VersionPos:       framer.VersionPos,
			RouteLenPos:      framer.RouteLenPos,
			RouteOrder:       framer.RouteOrder,
			PayloadOrder:     framer.PayloadOrder,
			OptionsOrder:     framer.OptionsOrder,
			OptionsCountPos:  framer.OptionsCountPos,
			OptionsKeyFirst:  framer.OptionsKeyFirst,
			HeaderLength:     framer.HeaderLength,
			BigBytesConfig:   framer.BigBytesConfig,
		}

		return nil
	}
}

func WithMaxConnections(limit uint32) Option {
	return func(cfg *ServerConfig) error {
		if limit <= 0 {
			return fmt.Errorf("mrc: limit must be greater than zero")
		}

		cfg.MaxConnections = limit
		return nil
	}
}

func WithDrainTimeout(newTimeout time.Duration) Option {
	return func(cfg *ServerConfig) error {
		cfg.DrainTimeout = newTimeout
		return nil
	}
}

func WithShutdownGrace(newGrace time.Duration) Option {
	return func(cfg *ServerConfig) error {
		cfg.ShutdownGrace = newGrace
		return nil
	}
}

func WithRequestTimeout(newTimeout time.Duration) Option {
	return func(cfg *ServerConfig) error {
		cfg.RequestTimeout = newTimeout
		return nil
	}
}
