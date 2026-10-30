package main

import (
	"encoding/json"
	"io"
	"testing"
)

var benchmarkSink []byte

// This isolates JSON encoding cost from network and protocol overhead.
func BenchmarkJSONMarshal(b *testing.B) {
	b.ReportAllocs()
	b.SetBytes(int64(len(payloadBytes)))
	for i := 0; i < b.N; i++ {
		encoded, err := json.Marshal(payload)
		if err != nil {
			b.Fatal(err)
		}
		benchmarkSink = encoded
	}
}

// This measures writing a previously serialized JSON response.
func BenchmarkWritePreEncodedJSON(b *testing.B) {
	b.ReportAllocs()
	b.SetBytes(int64(len(payloadBytes)))
	for i := 0; i < b.N; i++ {
		if _, err := io.Discard.Write(payloadBytes); err != nil {
			b.Fatal(err)
		}
		benchmarkSink = payloadBytes
	}
}
