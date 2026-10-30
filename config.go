package mrc

import (
	"time"

	cabi "github.com/Edestuss0/MMA-Network-protocol/cabi"
)

// General server config
type ServerConfig struct {
	BindIP         string
	Port           uint16
	MaxBatch       uint32
	MaxInFlight    uint32
	Framer         FramerConfig
	MaxConnections uint32
	DrainTimeout   time.Duration
	ShutdownGrace  time.Duration
	RequestTimeout time.Duration
}

// Codec-config
type FramerConfig struct {
	MaxMessageLength uint32
	OpcodePos        uint8
	VersionPos       uint8
	RouteLenPos      uint8
	RouteOrder       uint8
	PayloadOrder     uint8
	OptionsOrder     uint8
	OptionsCountPos  uint8
	OptionsKeyFirst  bool
	HeaderLength     uint8
	BigBytesConfig   BigBytesConfig
}

// Need to place trailing bytes in header  
type BigBytesConfig struct {
	OptionsLenOrder uint8
	PayloadLenOrder uint8
	ReqIDOrder      uint8
}

func DefaultConfig() (ServerConfig, error) {
	output, err := cabi.DefaultServerConfig()
	if err != nil {
		return ServerConfig{}, err
	}

	return ServerConfig{
		BindIP:         output.BindIP,
		Port:           output.Port,
		MaxBatch:       output.MaxBatch,
		MaxInFlight:    output.MaxInFlight,
		MaxConnections: output.MaxConnections,
		DrainTimeout:   output.DrainTimeout,
		ShutdownGrace:  output.ShutdownGrace,
		RequestTimeout: output.RequestTimeout,
		Framer: FramerConfig{
			MaxMessageLength: output.Framer.MaxMessageLength,
			OpcodePos:        output.Framer.OpcodePos,
			VersionPos:       output.Framer.VersionPos,
			RouteLenPos:      output.Framer.RouteLenPos,
			RouteOrder:       output.Framer.RouteOrder,
			PayloadOrder:     output.Framer.PayloadOrder,
			OptionsOrder:     output.Framer.OptionsOrder,
			OptionsCountPos:  output.Framer.OptionsCountPos,
			OptionsKeyFirst:  output.Framer.OptionsKeyFirst,
			HeaderLength:     output.Framer.HeaderLength,
			BigBytesConfig: BigBytesConfig{
				OptionsLenOrder: output.Framer.BigBytesConfig.OptionsLenOrder,
				PayloadLenOrder: output.Framer.BigBytesConfig.PayloadLenOrder,
				ReqIDOrder:      output.Framer.BigBytesConfig.ReqIDOrder,
			},
		},
	}, nil	
}

func toCABIConfig(config ServerConfig) cabi.ServerConfig {
	return cabi.ServerConfig{
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
}
