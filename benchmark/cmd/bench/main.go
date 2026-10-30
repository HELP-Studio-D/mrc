package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	mrc "github.com/HELP-Studio-D/mrc"
	"github.com/gin-gonic/gin"
)

const (
	httpAddr = "127.0.0.1:8080"
	mrcAddr  = "127.0.0.1:9090"

	mrcVersion    = 1
	mrcOpcodeOnce = 2
	mrcStatusOK   = 1
	maxFrameBody  = 10 * 1024 * 1024
)

type responsePayload struct {
	ID     int               `json:"id"`
	Name   string            `json:"name"`
	Active bool              `json:"active"`
	Score  float64           `json:"score"`
	Tags   []string          `json:"tags"`
	Meta   map[string]string `json:"meta"`
}

var payload = responsePayload{
	ID:     42,
	Name:   "MRC benchmark payload",
	Active: true,
	Score:  98.75,
	Tags:   []string{"network", "benchmark", "go", "json"},
	Meta: map[string]string{
		"protocol": "MMA",
		"version":  "1",
		"source":   "benchmark",
	},
}

var payloadBytes []byte

func main() {
	if err := loadDotEnv(filepath.Join("cmd", "bench", ".env")); err != nil {
		fatalf("load benchmark .env: %v", err)
	}

	var err error
	payloadBytes, err = json.Marshal(payload)
	if err != nil {
		fatalf("marshal benchmark payload: %v", err)
	}

	requests := envInt("BENCH_REQUESTS", 500_000)
	concurrency := envInt("BENCH_CONCURRENCY", 64)
	warmup := envInt("BENCH_WARMUP", 20_000)
	rounds := envInt("BENCH_ROUNDS", 3)
	if requests < 1 || concurrency < 1 || warmup < 0 || rounds < 1 {
		fatalf("BENCH_REQUESTS, BENCH_CONCURRENCY and BENCH_ROUNDS must be > 0; BENCH_WARMUP must be >= 0")
	}

	httpServer := startHTTPServer()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(ctx)
	}()

	mrcServer := startMRCServer()
	defer mrcServer.Close()

	if err := waitTCP(httpAddr, 5*time.Second); err != nil {
		fatalf("HTTP server did not start: %v", err)
	}
	if err := waitTCP(mrcAddr, 5*time.Second); err != nil {
		fatalf("MRC server did not start: %v", err)
	}

	transport := &http.Transport{
		MaxIdleConns:        concurrency * 2,
		MaxIdleConnsPerHost: concurrency,
		MaxConnsPerHost:     concurrency,
		IdleConnTimeout:     90 * time.Second,
		DisableCompression:  true,
	}
	defer transport.CloseIdleConnections()
	httpClient := &http.Client{Transport: transport, Timeout: 10 * time.Second}

	fmt.Printf(
		"Payload size: %d bytes | requests/test/round: %d | concurrency: %d | warmup: %d | rounds: %d\n",
		len(payloadBytes), requests, concurrency, warmup, rounds,
	)
	fmt.Printf("Runtime: %s/%s | NumCPU: %d | GOMAXPROCS: %d\n\n", runtime.GOOS, runtime.GOARCH, runtime.NumCPU(), runtime.GOMAXPROCS(0))

	tests := []benchmarkScenario{
		{name: "HTTP / Gin / c.JSON", route: "/json", http: true},
		{name: "MRC / c.JSON", route: "/json"},
		{name: "HTTP / Gin / c.Data (pre-encoded)", route: "/bytes", http: true},
		{name: "MRC / c.Data (pre-encoded)", route: "/bytes"},
	}

	summaries := make(map[string][]roundSummary, len(tests))
	for round := 0; round < rounds; round++ {
		fmt.Printf("\n========== ROUND %d/%d ==========\n", round+1, rounds)
		for _, test := range orderedScenarios(tests, round) {
			if warmup > 0 {
				_, err := runScenario(
					test.name+" warmup", test.route, test.http, httpClient,
					warmup, concurrency, false, mrcServer.Framer, payloadBytes,
				)
				if err != nil {
					fatalf("%s warmup failed: %v", test.name, err)
				}
			}

			result, err := runScenario(
				test.name, test.route, test.http, httpClient,
				requests, concurrency, true, mrcServer.Framer, payloadBytes,
			)
			if err != nil {
				fatalf("%s failed: %v", test.name, err)
			}
			printStats(result)
			summaries[test.name] = append(summaries[test.name], summarizeRound(result))
		}
	}

	fmt.Println("\n=== Median across rounds ===")
	medianRPS := make(map[string]float64, len(tests))
	for _, test := range tests {
		values := summaries[test.name]
		median := medianRoundSummary(values)
		medianRPS[test.name] = median.rps
		var failed int64
		for _, value := range values {
			failed += value.failed
		}
		fmt.Printf("%s\n", test.name)
		fmt.Printf("  Throughput median: %.0f req/sec (min %.0f, max %.0f)\n", median.rps, minRPS(values), maxRPS(values))
		fmt.Printf("  Latency median: p50 %s | p95 %s | p99 %s\n", median.p50.Round(time.Microsecond), median.p95.Round(time.Microsecond), median.p99.Round(time.Microsecond))
		fmt.Printf("  Errors total: %d across %d rounds\n\n", failed, len(values))
	}

	fmt.Println("=== Relative throughput (median) ===")
	printRatio("JSON handler", medianRPS["HTTP / Gin / c.JSON"], medianRPS["MRC / c.JSON"])
	printRatio("Pre-encoded bytes handler", medianRPS["HTTP / Gin / c.Data (pre-encoded)"], medianRPS["MRC / c.Data (pre-encoded)"])
}

type benchmarkScenario struct {
	name  string
	route string
	http  bool
}

type roundSummary struct {
	rps    float64
	p50    time.Duration
	p95    time.Duration
	p99    time.Duration
	failed int64
}

// Alternate the order across rounds so one implementation does not always
// benefit from being measured first or last.
func orderedScenarios(tests []benchmarkScenario, round int) []benchmarkScenario {
	orders := [][]int{
		{0, 1, 2, 3},
		{3, 2, 1, 0},
		{2, 3, 0, 1},
		{1, 0, 3, 2},
	}
	order := orders[round%len(orders)]
	result := make([]benchmarkScenario, 0, len(tests))
	for _, index := range order {
		if index < len(tests) {
			result = append(result, tests[index])
		}
	}
	return result
}

func summarizeRound(result stats) roundSummary {
	return roundSummary{
		rps:    result.rps,
		p50:    percentile(result.latencies, 50),
		p95:    percentile(result.latencies, 95),
		p99:    percentile(result.latencies, 99),
		failed: result.failed,
	}
}

func medianRoundSummary(values []roundSummary) roundSummary {
	if len(values) == 0 {
		return roundSummary{}
	}
	rpsValues := make([]float64, len(values))
	p50Values := make([]time.Duration, len(values))
	p95Values := make([]time.Duration, len(values))
	p99Values := make([]time.Duration, len(values))
	var failed int64
	for i, value := range values {
		rpsValues[i] = value.rps
		p50Values[i] = value.p50
		p95Values[i] = value.p95
		p99Values[i] = value.p99
		failed += value.failed
	}
	sort.Float64s(rpsValues)
	sort.Slice(p50Values, func(i, j int) bool { return p50Values[i] < p50Values[j] })
	sort.Slice(p95Values, func(i, j int) bool { return p95Values[i] < p95Values[j] })
	sort.Slice(p99Values, func(i, j int) bool { return p99Values[i] < p99Values[j] })
	middle := len(values) / 2
	median := func(durations []time.Duration) time.Duration {
		if len(durations)%2 == 1 {
			return durations[middle]
		}
		return (durations[middle-1] + durations[middle]) / 2
	}
	medianRPS := rpsValues[middle]
	if len(rpsValues)%2 == 0 {
		medianRPS = (rpsValues[middle-1] + rpsValues[middle]) / 2
	}
	return roundSummary{
		rps: medianRPS, p50: median(p50Values), p95: median(p95Values), p99: median(p99Values), failed: failed,
	}
}

func minRPS(values []roundSummary) float64 {
	minimum := values[0].rps
	for _, value := range values[1:] {
		if value.rps < minimum {
			minimum = value.rps
		}
	}
	return minimum
}

func maxRPS(values []roundSummary) float64 {
	maximum := values[0].rps
	for _, value := range values[1:] {
		if value.rps > maximum {
			maximum = value.rps
		}
	}
	return maximum
}

func startHTTPServer() *http.Server {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()

	router.GET("/json", func(c *gin.Context) {
		c.JSON(http.StatusOK, payload)
	})
	router.GET("/bytes", func(c *gin.Context) {
		c.Data(http.StatusOK, "application/json; charset=utf-8", payloadBytes)
	})

	server := &http.Server{
		Addr:              httpAddr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			fatalf("HTTP ListenAndServe: %v", err)
		}
	}()
	return server
}

func startMRCServer() *mrc.Engine {
	engine, err := mrc.NewWithOptions(
		mrc.WithBindIP("127.0.0.1"),
		mrc.WithPort(9090),
	)
	if err != nil {
		fatalf("create MRC engine: %v", err)
	}

	if err := engine.Handle("/json", func(c *mrc.Context) {
		if err := c.JSON(mrc.StatusOK, payload); err != nil {
			log.Printf("MRC /json response write failed: %v", err)
		}
	}); err != nil {
		fatalf("register MRC /json: %v", err)
	}

	if err := engine.Handle("/bytes", func(c *mrc.Context) {
		if err := c.Data(
			mrc.StatusOK,
			"application/json; charset=utf-8",
			payloadBytes,
		); err != nil {
			log.Printf("MRC /bytes response write failed: %v", err)
		}
	}); err != nil {
		fatalf("register MRC /bytes: %v", err)
	}

	if err := engine.Start(); err != nil {
		fatalf("start MRC server: %v", err)
	}
	return engine
}

type stats struct {
	name        string
	total       int
	failed      int64
	elapsed     time.Duration
	rps         float64
	latencies   []time.Duration
	firstErrors []string
}

type requestFunc func() error
type workerFactory func(workerID int) (requestFunc, func(), error)

func runScenario(
	name string,
	route string,
	useHTTP bool,
	httpClient *http.Client,
	total int,
	concurrency int,
	keepLatencies bool,
	framer mrc.FramerConfig,
	expectedPayload []byte,
) (stats, error) {
	factory := func(workerID int) (requestFunc, func(), error) {
		if useHTTP {
			return makeHTTPRequest(
				httpClient,
				"http://"+httpAddr+route,
				expectedPayload,
			)
		}
		return makeMRCRequest(mrcAddr, route, framer, expectedPayload)
	}

	requests := make([]requestFunc, concurrency)
	closers := make([]func(), concurrency)
	for workerID := 0; workerID < concurrency; workerID++ {
		fn, closeFn, err := factory(workerID)
		if err != nil {
			for _, closeExisting := range closers {
				if closeExisting != nil {
					closeExisting()
				}
			}
			return stats{}, fmt.Errorf("initialize worker %d: %w", workerID, err)
		}
		requests[workerID] = fn
		closers[workerID] = closeFn
	}
	defer func() {
		for _, closeFn := range closers {
			if closeFn != nil {
				closeFn()
			}
		}
	}()

	var latencies []time.Duration
	if keepLatencies {
		latencies = make([]time.Duration, total)
	}
	var failed atomic.Int64
	var sampleMu sync.Mutex
	firstErrors := make([]string, 0, 3)
	var wg sync.WaitGroup
	var ready sync.WaitGroup
	startGate := make(chan struct{})
	ready.Add(concurrency)

	for workerID := 0; workerID < concurrency; workerID++ {
		startIndex := total * workerID / concurrency
		endIndex := total * (workerID + 1) / concurrency
		wg.Add(1)
		go func(workerID, startIndex, endIndex int) {
			defer wg.Done()
			ready.Done()
			<-startGate
			var workerFailed int64
			for index := startIndex; index < endIndex; index++ {
				requestStarted := time.Now()
				err := requests[workerID]()
				latency := time.Since(requestStarted)
				if err != nil {
					workerFailed++
					sampleMu.Lock()
					if len(firstErrors) < cap(firstErrors) {
						firstErrors = append(firstErrors, fmt.Sprintf("worker %d, request %d: %v", workerID, index+1, err))
					}
					sampleMu.Unlock()
					continue
				}
				if keepLatencies {
					latencies[index] = latency
				}
			}
			if workerFailed > 0 {
				failed.Add(workerFailed)
			}
		}(workerID, startIndex, endIndex)
	}

	// Start timing only after every worker is ready. Each worker owns a fixed
	// request-index range, avoiding a contended atomic counter on every request.
	ready.Wait()
	started := time.Now()
	close(startGate)
	wg.Wait()
	elapsed := time.Since(started)
	failedCount := failed.Load()

	if !keepLatencies {
		if failedCount > 0 {
			errorSample := "no error detail captured"
			if len(firstErrors) > 0 {
				errorSample = firstErrors[0]
			}
			return stats{}, fmt.Errorf(
				"%s: warmup had %d/%d failed requests; first error: %s",
				name, failedCount, total, errorSample,
			)
		}
		return stats{name: name}, nil
	}

	successfulLatencies := make([]time.Duration, 0, total-int(failedCount))
	for _, latency := range latencies {
		if latency > 0 {
			successfulLatencies = append(successfulLatencies, latency)
		}
	}

	successful := total - int(failedCount)
	rps := float64(successful) / elapsed.Seconds()
	return stats{
		name:        name,
		total:       total,
		failed:      failedCount,
		elapsed:     elapsed,
		rps:         rps,
		latencies:   successfulLatencies,
		firstErrors: firstErrors,
	}, nil
}

func makeHTTPRequest(client *http.Client, url string, expectedPayload []byte) (requestFunc, func(), error) {
	return func() error {
		request, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			return err
		}
		response, err := client.Do(request)
		if err != nil {
			return err
		}
		body, readErr := io.ReadAll(response.Body)
		closeErr := response.Body.Close()
		if readErr != nil {
			return readErr
		}
		if closeErr != nil {
			return closeErr
		}
		if response.StatusCode != http.StatusOK {
			return fmt.Errorf("unexpected HTTP status: %d", response.StatusCode)
		}
		if response.Header.Get("Content-Type") == "" {
			return fmt.Errorf("HTTP response is missing Content-Type")
		}
		if !bytes.Equal(body, expectedPayload) {
			return fmt.Errorf("HTTP response payload mismatch: got %d bytes, want %d bytes", len(body), len(expectedPayload))
		}
		return nil
	}, func() {}, nil
}

func makeMRCRequest(
	address string,
	route string,
	framer mrc.FramerConfig,
	expectedPayload []byte,
) (requestFunc, func(), error) {
	connection, err := net.DialTimeout("tcp", address, 5*time.Second)
	if err != nil {
		return nil, nil, err
	}
	if tcp, ok := connection.(*net.TCPConn); ok {
		_ = tcp.SetNoDelay(true)
	}

	var requestID uint32
	request := func() error {
		requestID++
		frame, err := encodeMRCRequest(route, requestID, framer)
		if err != nil {
			return err
		}
		if err := writeAll(connection, frame); err != nil {
			return fmt.Errorf("write MRC request: %w", err)
		}
		return readMRCResponse(connection, requestID, framer, expectedPayload)
	}
	return request, func() { _ = connection.Close() }, nil
}

// encodeMRCRequest uses the FramerConfig exposed by the running MRC engine.
// Positions in the frame configuration are one-based; section orders are 1..3.
func encodeMRCRequest(route string, requestID uint32, framer mrc.FramerConfig) ([]byte, error) {
	routeBytes := []byte(route)
	if len(routeBytes) == 0 || len(routeBytes) > 255 {
		return nil, fmt.Errorf("MRC route length must be in range 1..255")
	}

	headerLength := int(framer.HeaderLength)
	if headerLength < 12 {
		return nil, fmt.Errorf("invalid MRC header length: %d", headerLength)
	}
	frame := make([]byte, headerLength)

	setHeaderByte := func(position uint8, value byte) error {
		if position == 0 || int(position) > headerLength {
			return fmt.Errorf("invalid MRC header position %d for header length %d", position, headerLength)
		}
		frame[int(position)-1] = value
		return nil
	}
	for _, field := range []struct {
		position uint8
		value    byte
	}{
		{framer.OpcodePos, mrcOpcodeOnce},
		{framer.VersionPos, mrcVersion},
		{framer.RouteLenPos, byte(len(routeBytes))},
		{framer.OptionsCountPos, 0},
	} {
		if err := setHeaderByte(field.position, field.value); err != nil {
			return nil, err
		}
	}

	bigBytesStart := headerLength - 12
	putBigUint32 := func(order uint8, value uint32) error {
		if order < 1 || order > 3 {
			return fmt.Errorf("invalid MRC big-bytes order %d", order)
		}
		position := bigBytesStart + (int(order)-1)*4
		binary.BigEndian.PutUint32(frame[position:position+4], value)
		return nil
	}
	if err := putBigUint32(framer.BigBytesConfig.OptionsLenOrder, 0); err != nil {
		return nil, err
	}
	if err := putBigUint32(framer.BigBytesConfig.PayloadLenOrder, 0); err != nil {
		return nil, err
	}
	if err := putBigUint32(framer.BigBytesConfig.ReqIDOrder, requestID); err != nil {
		return nil, err
	}

	sections := []struct {
		order uint8
		data  []byte
	}{
		{framer.RouteOrder, routeBytes},
		{framer.OptionsOrder, nil},
		{framer.PayloadOrder, nil},
	}
	sort.Slice(sections, func(i, j int) bool { return sections[i].order < sections[j].order })
	for index, section := range sections {
		if section.order < 1 || section.order > 3 || (index > 0 && sections[index-1].order == section.order) {
			return nil, fmt.Errorf("invalid MRC body section order")
		}
		frame = append(frame, section.data...)
	}
	return frame, nil
}

func readMRCResponse(
	connection net.Conn,
	expectedRequestID uint32,
	framer mrc.FramerConfig,
	expectedPayload []byte,
) error {
	headerLength := int(framer.HeaderLength)
	if headerLength < 12 {
		return fmt.Errorf("invalid MRC response header length: %d", headerLength)
	}
	header := make([]byte, headerLength)
	if _, err := io.ReadFull(connection, header); err != nil {
		return fmt.Errorf("read MRC response header: %w", err)
	}

	readHeaderByte := func(position uint8, field string) (byte, error) {
		if position == 0 || int(position) > len(header) {
			return 0, fmt.Errorf("invalid MRC %s position %d", field, position)
		}
		return header[int(position)-1], nil
	}
	status, err := readHeaderByte(framer.OpcodePos, "status")
	if err != nil {
		return err
	}
	if status != mrcStatusOK {
		return fmt.Errorf("MRC returned status %d (want %d)", status, mrcStatusOK)
	}
	optionsCount, err := readHeaderByte(framer.OptionsCountPos, "options count")
	if err != nil {
		return err
	}

	bigBytesStart := headerLength - 12
	readBigUint32 := func(order uint8, field string) (uint32, error) {
		if order < 1 || order > 3 {
			return 0, fmt.Errorf("invalid MRC %s order %d", field, order)
		}
		position := bigBytesStart + (int(order)-1)*4
		return binary.BigEndian.Uint32(header[position : position+4]), nil
	}
	optionsLength, err := readBigUint32(framer.BigBytesConfig.OptionsLenOrder, "options length")
	if err != nil {
		return err
	}
	payloadLength, err := readBigUint32(framer.BigBytesConfig.PayloadLenOrder, "payload length")
	if err != nil {
		return err
	}
	requestID, err := readBigUint32(framer.BigBytesConfig.ReqIDOrder, "request ID")
	if err != nil {
		return err
	}
	if requestID != expectedRequestID {
		return fmt.Errorf("MRC request ID mismatch: got %d, want %d", requestID, expectedRequestID)
	}
	if uint64(optionsLength)+uint64(payloadLength) > maxFrameBody {
		return fmt.Errorf("MRC response is too large: options=%d payload=%d", optionsLength, payloadLength)
	}

	body := make([]byte, int(optionsLength)+int(payloadLength))
	if _, err := io.ReadFull(connection, body); err != nil {
		return fmt.Errorf("read MRC response body: %w", err)
	}

	var optionsData, payloadData []byte
	if framer.PayloadOrder > framer.OptionsOrder {
		optionsData = body[:int(optionsLength)]
		payloadData = body[int(optionsLength):]
	} else {
		payloadData = body[:int(payloadLength)]
		optionsData = body[int(payloadLength):]
	}

	options, err := decodeMRCOptions(optionsData, optionsCount, framer.OptionsKeyFirst)
	if err != nil {
		return fmt.Errorf("decode MRC response options: %w", err)
	}
	if _, ok := options["content-type"]; !ok {
		return fmt.Errorf("MRC response does not contain content-type option (count=%d, bytes=%d)", optionsCount, optionsLength)
	}
	if !bytes.Equal(payloadData, expectedPayload) {
		return fmt.Errorf(
			"MRC response payload mismatch: got %d bytes, want %d bytes",
			len(payloadData), len(expectedPayload),
		)
	}
	return nil
}

func decodeMRCOptions(data []byte, count uint8, keyFirst bool) (map[string]string, error) {
	if count == 0 {
		if len(data) != 0 {
			return nil, fmt.Errorf("received %d option bytes with zero options", len(data))
		}
		return map[string]string{}, nil
	}

	options := make(map[string]string, int(count))
	position := 0
	for index := 0; index < int(count); index++ {
		if len(data)-position < 4 {
			return nil, fmt.Errorf("option %d has an incomplete length header", index)
		}
		keyLength := int(binary.BigEndian.Uint16(data[position : position+2]))
		valueLength := int(binary.BigEndian.Uint16(data[position+2 : position+4]))
		position += 4
		if keyLength == 0 || valueLength == 0 || len(data)-position < keyLength+valueLength {
			return nil, fmt.Errorf("option %d has invalid key/value lengths", index)
		}

		var key, value []byte
		if keyFirst {
			key = data[position : position+keyLength]
			position += keyLength
			value = data[position : position+valueLength]
			position += valueLength
		} else {
			value = data[position : position+valueLength]
			position += valueLength
			key = data[position : position+keyLength]
			position += keyLength
		}
		keyString := string(key)
		if _, exists := options[keyString]; exists {
			return nil, fmt.Errorf("duplicate option key %q", keyString)
		}
		options[keyString] = string(value)
	}
	if position != len(data) {
		return nil, fmt.Errorf("%d trailing option bytes", len(data)-position)
	}
	return options, nil
}

func writeAll(writer io.Writer, data []byte) error {
	for len(data) > 0 {
		n, err := writer.Write(data)
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
		data = data[n:]
	}
	return nil
}

func waitTCP(address string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		connection, err := net.DialTimeout("tcp", address, 100*time.Millisecond)
		if err == nil {
			_ = connection.Close()
			return nil
		}
		lastErr = err
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("%s: %w", address, lastErr)
}

func printStats(result stats) {
	latencies := append([]time.Duration(nil), result.latencies...)
	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
	fmt.Printf("=== %s ===\n", result.name)
	fmt.Printf("Requests:  %d\n", result.total)
	fmt.Printf("Errors:    %d\n", result.failed)
	fmt.Printf("Time:      %s\n", result.elapsed.Round(time.Millisecond))
	fmt.Printf("Throughput: %.0f req/sec\n", result.rps)
	fmt.Printf("Latency p50: %s\n", percentile(latencies, 50).Round(time.Microsecond))
	fmt.Printf("Latency p95: %s\n", percentile(latencies, 95).Round(time.Microsecond))
	fmt.Printf("Latency p99: %s\n", percentile(latencies, 99).Round(time.Microsecond))
	for _, sample := range result.firstErrors {
		fmt.Printf("Error sample: %s\n", sample)
	}
	fmt.Println()
}

func percentile(values []time.Duration, percentile int) time.Duration {
	if len(values) == 0 {
		return 0
	}
	index := (len(values)*percentile + 99) / 100
	if index > len(values) {
		index = len(values)
	}
	return values[index-1]
}

func printRatio(label string, httpRPS, mrcRPS float64) {
	if httpRPS <= 0 || mrcRPS <= 0 {
		fmt.Printf("%s: not available (one of the tests had no successful requests)\n", label)
		return
	}
	ratio := mrcRPS / httpRPS
	fmt.Printf("%s: MRC / HTTP = %.2fx\n", label, ratio)
}

func loadDotEnv(path string) error {
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, found := strings.Cut(line, "=")
		if !found {
			return fmt.Errorf("line %d: expected KEY=VALUE", lineNumber)
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" {
			return fmt.Errorf("line %d: empty environment variable name", lineNumber)
		}
		if len(value) >= 2 && ((value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'')) {
			value = value[1 : len(value)-1]
		}
		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		if err := os.Setenv(key, value); err != nil {
			return fmt.Errorf("line %d: set %s: %w", lineNumber, key, err)
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return nil
}

func envInt(name string, fallback int) int {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		fatalf("invalid %s=%q: %v", name, value, err)
	}
	return parsed
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
