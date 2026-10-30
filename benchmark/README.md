# MRC vs HTTP benchmark

This is an end-to-end TCP benchmark comparing the same JSON response through Gin/HTTP and MRC.

## Add dependency

Run the commands from the `benchmark` directory. Dependencies are already listed in `go.mod`.

```bash
go mod tidy
go test ./cmd/bench
```

## Run the network benchmark

```bash
go run ./cmd/bench
```

The benchmark reads optional defaults from `cmd/bench/.env`. Variables already set in the process environment take precedence.

Optional parameters (the `.env` file in `cmd/bench` sets practical defaults):

```powershell
$env:BENCH_REQUESTS=500000
$env:BENCH_CONCURRENCY=64
$env:BENCH_WARMUP=20000
$env:BENCH_ROUNDS=3
go run ./cmd/bench
```

Each round re-runs all four scenarios, alternates their order, and the final comparison uses median throughput across rounds. Clear any old `BENCH_*` process environment variables to let `.env` values take effect. Environment variables override `.env`.

Endpoints are started locally on `127.0.0.1:8080` (HTTP) and `127.0.0.1:9090` (MRC).

## Run the serialization microbenchmarks

```bash
go test ./cmd/bench -run '^$' -bench 'Benchmark(JSONMarshal|WritePreEncodedJSON)$' -benchmem
```

The network benchmark measures the full request/response path, including protocol framing, TCP, handler execution, response encoding, and client-side read. The microbenchmarks isolate JSON serialization versus writing already encoded bytes.

The benchmark starts all workers behind a barrier and assigns each worker a fixed request range, avoiding a shared atomic increment on every request. It reports per-round values and median throughput across rounds. The MRC handlers also log response-write errors, which helps diagnose rare responses that fall back to an internal error.

The MRC client reads the active `FramerConfig` from the running engine and uses it for request encoding and response decoding. Response options (including the `content-type` option emitted by `c.JSON` and `c.Data`) are parsed rather than treated as request failures. The benchmark verifies the returned payload bytes and reports sample errors when a request fails.
