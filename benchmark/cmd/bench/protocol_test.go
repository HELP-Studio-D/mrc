package main

import (
	"encoding/binary"
	"net"
	"testing"

	mrc "github.com/HELP-Studio-D/mrc"
)

func testFramerConfig() mrc.FramerConfig {
	return mrc.FramerConfig{
		HeaderLength:    29,
		OpcodePos:       1,
		VersionPos:      2,
		RouteLenPos:     3,
		OptionsCountPos: 4,
		RouteOrder:      1,
		OptionsOrder:    2,
		PayloadOrder:    3,
		OptionsKeyFirst: true,
		BigBytesConfig: mrc.BigBytesConfig{
			OptionsLenOrder: 1,
			PayloadLenOrder: 2,
			ReqIDOrder:      3,
		},
	}
}

func TestEncodeMRCRequestUsesConfiguredHeader(t *testing.T) {
	framer := testFramerConfig()
	frame, err := encodeMRCRequest("/json", 0x10203040, framer)
	if err != nil {
		t.Fatalf("encodeMRCRequest() error = %v", err)
	}

	if got, want := len(frame), int(framer.HeaderLength)+len("/json"); got != want {
		t.Fatalf("frame length = %d, want %d", got, want)
	}
	if got := frame[int(framer.OpcodePos)-1]; got != mrcOpcodeOnce {
		t.Fatalf("opcode = %d, want %d", got, mrcOpcodeOnce)
	}
	if got := frame[int(framer.VersionPos)-1]; got != mrcVersion {
		t.Fatalf("version = %d, want %d", got, mrcVersion)
	}
	if got := frame[int(framer.RouteLenPos)-1]; got != byte(len("/json")) {
		t.Fatalf("route length = %d, want %d", got, len("/json"))
	}

	requestIDPosition := int(framer.HeaderLength) - 12 + (int(framer.BigBytesConfig.ReqIDOrder)-1)*4
	if got := binary.BigEndian.Uint32(frame[requestIDPosition : requestIDPosition+4]); got != 0x10203040 {
		t.Fatalf("request ID = %#x, want %#x", got, uint32(0x10203040))
	}
	if got := string(frame[int(framer.HeaderLength):]); got != "/json" {
		t.Fatalf("route body = %q, want %q", got, "/json")
	}
}

func TestReadMRCResponseAcceptsContentTypeOption(t *testing.T) {
	payload := []byte(`{"ok":true}`)
	tests := []struct {
		name   string
		framer mrc.FramerConfig
	}{
		{name: "default layout", framer: testFramerConfig()},
		{
			name: "reversed option data and body order",
			framer: func() mrc.FramerConfig {
				config := testFramerConfig()
				config.OptionsKeyFirst = false
				config.RouteOrder = 3
				config.OptionsOrder = 2
				config.PayloadOrder = 1
				return config
			}(),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client, server := net.Pipe()
			defer client.Close()
			defer server.Close()

			go func() {
				_ = writeTestMRCResponse(server, 7, test.framer, payload, "application/json")
			}()

			if err := readMRCResponse(client, 7, test.framer, payload); err != nil {
				t.Fatalf("readMRCResponse() error = %v", err)
			}
		})
	}
}

func writeTestMRCResponse(
	connection net.Conn,
	requestID uint32,
	framer mrc.FramerConfig,
	payload []byte,
	contentType string,
) error {
	key := []byte("content-type")
	value := []byte(contentType)
	options := make([]byte, 4+len(key)+len(value))
	binary.BigEndian.PutUint16(options[0:2], uint16(len(key)))
	binary.BigEndian.PutUint16(options[2:4], uint16(len(value)))
	position := 4
	if framer.OptionsKeyFirst {
		copy(options[position:], key)
		position += len(key)
		copy(options[position:], value)
	} else {
		copy(options[position:], value)
		position += len(value)
		copy(options[position:], key)
	}

	frame := make([]byte, int(framer.HeaderLength))
	frame[int(framer.OpcodePos)-1] = mrcStatusOK
	frame[int(framer.VersionPos)-1] = mrcVersion
	frame[int(framer.OptionsCountPos)-1] = 1
	bigBytesStart := len(frame) - 12
	binary.BigEndian.PutUint32(
		frame[bigBytesStart+(int(framer.BigBytesConfig.OptionsLenOrder)-1)*4:],
		uint32(len(options)),
	)
	binary.BigEndian.PutUint32(
		frame[bigBytesStart+(int(framer.BigBytesConfig.PayloadLenOrder)-1)*4:],
		uint32(len(payload)),
	)
	binary.BigEndian.PutUint32(
		frame[bigBytesStart+(int(framer.BigBytesConfig.ReqIDOrder)-1)*4:],
		requestID,
	)

	if framer.PayloadOrder > framer.OptionsOrder {
		frame = append(frame, options...)
		frame = append(frame, payload...)
	} else {
		frame = append(frame, payload...)
		frame = append(frame, options...)
	}
	return writeAll(connection, frame)
}
