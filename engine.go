package mrc

import (
	"fmt"
	"sync"

	cabi "github.com/Edestuss0/MMA-Network-protocol/cabi"
)

type Engine struct {
	// Native is the underlying Rust server.
	native *cabi.Server

	// Middleware contains globally registered middleware.
	middleware HandlersChain

	// Routes stores registered route names.
	routes map[string]struct{}

	// Protects engine state.
	mu sync.Mutex

	// Indicates whether the server has started.
	started bool

	// Indicates whether the engine has been closed.
	closed bool

	// Add more methods to engine
	ServerConfig
	RouterGroup
}

// NewWithOptions creates a server using functional options.
func NewWithOptions(options ...Option) (*Engine, error) {
	cfg, err := DefaultConfig()
	if err != nil {
		return nil, err
	}

	for _, option := range options {
		if option == nil {
			return nil, fmt.Errorf("mrc: nil configuration option")
		}

		if err := option(&cfg); err != nil {
			return nil, err
		}
	}

	return New(cfg)
}

// New creates a server using a configuration structure.
func New(config ServerConfig) (*Engine, error) {
	nativeConfig := cabi.ServerConfig{
		BindIP:         config.BindIP,
		Port:           config.Port,
		MaxBatch:       config.MaxBatch,
		MaxInFlight:    config.MaxInFlight,
		MaxConnections: config.MaxConnections,
		DrainTimeout:   config.DrainTimeout,
		ShutdownGrace:  config.ShutdownGrace,
		RequestTimeout: config.RequestTimeout,
		Framer: cabi.FramerConfig{
			MaxMessageLength: config.Framer.MaxMessageLength,
			OpcodePos:        config.Framer.OpcodePos,
			VersionPos:       config.Framer.VersionPos,
			RouteLenPos:      config.Framer.RouteLenPos,
			RouteOrder:       config.Framer.RouteOrder,
			PayloadOrder:     config.Framer.PayloadOrder,
			OptionsOrder:     config.Framer.OptionsOrder,
			OptionsCountPos:  config.Framer.OptionsCountPos,
			OptionsKeyFirst:  config.Framer.OptionsKeyFirst,
			HeaderLength:     config.Framer.HeaderLength,
			BigBytesConfig: cabi.BigBytesConfig{
				OptionsLenOrder: config.Framer.BigBytesConfig.OptionsLenOrder,
				PayloadLenOrder: config.Framer.BigBytesConfig.PayloadLenOrder,
				ReqIDOrder:      config.Framer.BigBytesConfig.ReqIDOrder,
			},
		},
	}

	native, err := cabi.NewServer(nativeConfig)
	if err != nil {
		return nil, err
	}

	engine := &Engine{
		ServerConfig: config,
		native:       native,
		routes:       make(map[string]struct{}),
	}

	engine.RouterGroup = RouterGroup{
		engine: engine,
	}

	return engine, nil
}

// Start starts the underlying native server.
func (e *Engine) Start() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e == nil || e.native == nil {
		return ErrServerNotInitialized
	}

	if e.closed {
		return cabi.ErrClosed
	}

	if e.started {
		return cabi.ErrServerRunning
	}

	if err := e.native.Start(); err != nil {
		return err
	}

	e.started = true
	return nil
}

// Close releases the native server resources.
func (e *Engine) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e == nil || e.native == nil {
		return ErrServerNotInitialized
	}

	if e.closed {
		return nil
	}

	e.closed = true
	return e.native.Close()
}