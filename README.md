<div align="center">

# MRC

**A Go framework for server applications built on the binary TCP protocol [MMA](https://github.com/Edestuss0/MMA-Network-protocol).**

**English** · [Русский](README.ru.md)

</div>

> **Status:** pre-release. MRC uses the native MMA server through its C ABI. The public MRC API and C ABI are under active development.

MRC provides a Go API for creating a server, registering routes and handlers, composing middleware chains, working with requests and responses, handling bidirectional Channel connections, and implementing basic authentication with JWT and bcrypt.

The main MRC objects are `Engine`, `Context`, `Request`, and `AuthManager`. Low-level TCP connections, protocol framing, and network I/O are handled by the MMA engine. MRC adapts its callbacks into Go handlers.

MRC runs on the custom MMA binary protocol, not HTTP. A route is a string command identifier; `Options` are frame metadata; `Payload` is the message body in bytes. HTTP methods, URL parameters, and HTTP header parsing are not part of the MRC API.

## Contents

- [1. Overview](#1-overview)
- [2. Installation and quick start](#2-installation-and-quick-start)
- [3. Server configuration](#3-server-configuration)
- [4. Engine and lifecycle](#4-engine-and-lifecycle)
- [5. Routing and groups](#5-routing-and-groups)
- [6. Request and Context](#6-request-and-context)
- [7. Request binding](#7-request-binding)
- [8. Responses](#8-responses)
- [9. Middleware](#9-middleware)
- [10. Channel](#10-channel)
- [11. Authentication: JWT, Claims, and bcrypt](#11-authentication-jwt-claims-and-bcrypt)
- [12. Errors and statuses](#12-errors-and-statuses)
- [13. Implementation details and limitations](#13-implementation-details-and-limitations)
- [14. Tests and benchmarks](#14-tests-and-benchmarks)
- [15. Additional types and low-level APIs](#15-additional-types-and-low-level-apis)
- [16. Repository layout](#16-repository-layout)

---

## 1. Overview

### 1.1. Application model

MRC separates server functionality into several layers:

| Layer | Responsibility |
|---|---|
| `Engine` | Server configuration and lifecycle, handler registration, global middleware, and access to Auth |
| `RouterGroup` | Route prefixes and group middleware |
| `Context` | Handler-chain control, request-scoped storage, JSON binding, and response generation |
| `Request` | Opcode, version, ID, route, options, payload, and request `context.Context` |
| `ChannelContext` | Access to a Channel connection and outgoing messages |
| `AuthManager` | JWT issuance and verification, password hashing and verification |
| `middleware` | Built-in Logger and Recovery |
| MMA C ABI | TCP connections, binary framing, callback execution, and response transmission |

### 1.2. Properties

| Property | Implementation |
|---|---|
| API language | Go |
| Network transport | TCP through MMA |
| Message format | MMA binary frames with configurable `FramerConfig` |
| Connection modes | `Once` — request/response; `Channel` — persistent connection and push messages |
| Routes | String names, case-sensitive |
| Handler | `func(*mrc.Context)` |
| Middleware | Global, group-level, and route-level handlers |
| JSON request decoding | Standard `encoding/json` |
| JSON response encoding | `github.com/goccy/go-json` |
| Authentication | JWT HS256 and bcrypt |
| SQL / ORM | Not present in the current source tree |

### 1.3. Packages

| Package | Contents |
|---|---|
| `github.com/HELP-Studio-D/mrc` | Engine, configuration, routing, Context, Request, Channel, JWT, Claims, and bcrypt |
| `github.com/HELP-Studio-D/mrc/middleware` | Logger and Recovery middleware |
| `github.com/HELP-Studio-D/mrc/auth` | Configuration-only types `Config`, `JWTConfig`, and `PasswordConfig`; the runtime authentication manager is in the root `mrc` package |

The `mrc/auth` subpackage exports configuration structs only. Its `Config` contains `JWT` and `Password`; its `JWTConfig` contains `SigningKey`, `Issuer`, and `AccessTTL`; its `PasswordConfig` contains `Algorithm` and `BcryptCost`. These types are distinct from the root-package `mrc.AuthConfig`, `mrc.JWTConfig`, and `mrc.PasswordConfig` accepted by `Engine.ConfigureAuth()`; they are not aliases and are not accepted by that method.

---

## 2. Installation and quick start

### 2.1. Requirements

- Go `1.26.0` or newer, as declared in `go.mod`.
- Go modules.
- The native MMA C ABI library for the target platform, provided through the `github.com/Edestuss0/MMA-Network-protocol/cabi` dependency.

Install MRC:

```bash
go get github.com/HELP-Studio-D/mrc
```

### 2.2. Minimal server

`NewWithOptions()` creates a server from the MMA default configuration and applies functional options. `Handle()` registers a handler under a string route name. `Run()` starts the server and blocks until the Engine is closed.

```go
package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/HELP-Studio-D/mrc"
	"github.com/HELP-Studio-D/mrc/middleware"
)

func main() {
	engine, err := mrc.NewWithOptions(
		mrc.WithBindIP("127.0.0.1"),
		mrc.WithPort(8080),
	)
	if err != nil {
		log.Fatal(err)
	}

	engine.Use(
		middleware.Logger(),
		middleware.Recovery(),
	)

	engine.Handle("health", func(c *mrc.Context) {
		if err := c.JSON(mrc.StatusOK, mrc.H{
			"status": "online",
		}); err != nil {
			c.Error(err)
		}
	})

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)

	go func() {
		<-signals
		if err := engine.Close(); err != nil {
			log.Printf("server close: %v", err)
		}
	}()

	if err := engine.Run(); err != nil {
		log.Printf("server run: %v", err)
	}

	if err := engine.Close(); err != nil {
		log.Printf("server close: %v", err)
	}
}
```

`health` is an MMA route name, not an HTTP URL. A client sends an MMA frame with that route and the `Once` opcode.

### 2.3. What the quick start does

1. `NewWithOptions()` gets the default configuration from the C ABI and applies `WithBindIP` and `WithPort`.
2. `Use()` adds global middleware.
3. `Handle()` registers a route and copies the current middleware chain into it.
4. `Run()` starts the native server and waits for the Engine to close.
5. `Close()` stops and releases the native server and unblocks `Run()`.

---

## 3. Server configuration

### 3.1. `ServerConfig`

The configuration can be obtained with `DefaultConfig()` and modified directly, or created with `NewWithOptions()`.

```go
config, err := mrc.DefaultConfig()
if err != nil {
	log.Fatal(err)
}

config.BindIP = "127.0.0.1"
config.Port = 9000
config.MaxInFlight = 2048
config.RequestTimeout = 5 * time.Second

engine, err := mrc.New(config)
if err != nil {
	log.Fatal(err)
}
```

`ServerConfig` fields:

| Field | Type | Purpose |
|---|---|---|
| `BindIP` | `string` | Listener bind IP address |
| `Port` | `uint16` | TCP port |
| `MaxBatch` | `uint32` | Maximum number of responses batched before a flush |
| `MaxInFlight` | `uint32` | Global limit on concurrently handled requests |
| `Framer` | `FramerConfig` | Binary frame layout and message-body limit |
| `MaxConnections` | `uint32` | Connection limit |
| `DrainTimeout` | `time.Duration` | Time to flush queued responses when a connection closes |
| `ShutdownGrace` | `time.Duration` | Grace period for in-flight tasks during shutdown |
| `RequestTimeout` | `time.Duration` | Request timeout in native configuration and Go request context deadline |

`DefaultConfig()` obtains the defaults from the MMA C ABI. The current default configuration is:

| Parameter | Default |
|---|---:|
| `BindIP` | `0.0.0.0` |
| `Port` | `8080` |
| `MaxBatch` | `16` |
| `MaxInFlight` | `1024` |
| `MaxConnections` | `128000` |
| `DrainTimeout` | `2s` |
| `ShutdownGrace` | `10s` |
| `RequestTimeout` | `10s` |
| `Framer.HeaderLength` | `29` |
| `Framer.MaxMessageLength` | `10000000` bytes |

### 3.2. Functional options

`Option` is a function with the signature `func(*ServerConfig) error`. Options are passed to `NewWithOptions()` and applied in order. When the same setting is specified more than once, the last option wins.

```go
engine, err := mrc.NewWithOptions(
	mrc.WithBindIP("0.0.0.0"),
	mrc.WithPort(9000),
	mrc.WithMaxBatch(64),
	mrc.WithMaxInFlight(2048),
	mrc.WithMaxConnections(10000),
	mrc.WithDrainTimeout(2*time.Second),
	mrc.WithShutdownGrace(10*time.Second),
	mrc.WithRequestTimeout(3*time.Second),
)
if err != nil {
	log.Fatal(err)
}
```

| Option | Field | Validation in the Go option |
|---|---|---|
| `WithBindIP(ip)` | `BindIP` | Empty string is rejected |
| `WithPort(port)` | `Port` | No additional option-level validation; the C ABI validates the configuration |
| `WithMaxBatch(limit)` | `MaxBatch` | Zero is rejected |
| `WithMaxInFlight(limit)` | `MaxInFlight` | Zero is rejected |
| `WithMaxConnections(limit)` | `MaxConnections` | Zero is rejected |
| `WithFramer(config)` | `Framer` | Copies all fields; native configuration performs validation |
| `WithDrainTimeout(duration)` | `DrainTimeout` | Passed through without additional option-level validation |
| `WithShutdownGrace(duration)` | `ShutdownGrace` | Passed through without additional option-level validation |
| `WithRequestTimeout(duration)` | `RequestTimeout` | Passed through without additional option-level validation |

`NewWithOptions()` starts with `DefaultConfig()`, so options modify the native defaults rather than a zero-valued configuration. Port `0` is rejected by native configuration validation; an ephemeral port is not supported by the current MMA server configuration.

### 3.3. `FramerConfig`

`FramerConfig` defines the MMA wire layout. Client and server configurations must match.

```go
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

type BigBytesConfig struct {
	OptionsLenOrder uint8
	PayloadLenOrder uint8
	ReqIDOrder      uint8
}
```

| Field | Purpose |
|---|---|
| `HeaderLength` | Header size; 29 bytes in the default layout |
| `MaxMessageLength` | Maximum size of frame body sections |
| `OpcodePos` | Header position of opcode, numbered from 1 |
| `VersionPos` | Header position of protocol version, numbered from 1 |
| `RouteLenPos` | Header position of route length, numbered from 1 |
| `OptionsCountPos` | Header position of option count, numbered from 1 |
| `RouteOrder` | Position of the route section in the body |
| `OptionsOrder` | Position of the options section in the body |
| `PayloadOrder` | Position of the payload section in the body |
| `OptionsKeyFirst` | Key/value order inside an option |
| `BigBytesConfig.OptionsLenOrder` | Slot of the 4-byte options length field |
| `BigBytesConfig.PayloadLenOrder` | Slot of the 4-byte payload length field |
| `BigBytesConfig.ReqIDOrder` | Slot of the 4-byte request ID field |

To modify one field, start from the default configuration:

```go
config, err := mrc.DefaultConfig()
if err != nil {
	log.Fatal(err)
}

config.Framer.HeaderLength = 32
config.Framer.MaxMessageLength = 2_000_000

engine, err := mrc.New(config)
if err != nil {
	log.Fatal(err)
}
```

All wire parameters, except the server-side `MaxMessageLength` limit, must match the client configuration. Changing the layout on only one side breaks frame decoding.

### 3.4. Runtime configuration changes

`Engine.ChangeConfig(newConfig)` sends a new configuration to a running C ABI server. It can be called after the Engine has started.

```go
next, err := mrc.DefaultConfig()
if err != nil {
	log.Fatal(err)
}

next.RequestTimeout = 5 * time.Second
if err := engine.ChangeConfig(next); err != nil {
	log.Fatal(err)
}
```

Runtime changes are delegated to the MMA C ABI. Not every setting is applied to an already running server: the listener address, `MaxBatch`, `MaxConnections`, `MaxInFlight`, and `ShutdownGrace` are not fully dynamic in the current C ABI. Wire parameters must also be updated on the client side at the same time.

The MRC Go request context gets its timeout from the configuration when the Engine is created. `ChangeConfig()` updates the native server configuration and the exported `Engine.ServerConfig`, but does not update the cached Go-side timeout value.

---

## 4. Engine and lifecycle

`Engine` is the central application object. It contains the server configuration, global middleware chain, registered route tables, Channel handle, and `AuthManager`.

### 4.1. Engine methods

| Method | Signature | Purpose |
|---|---|---|
| `New` | `New(config ServerConfig) (*Engine, error)` | Creates an Engine from a configuration struct |
| `NewWithOptions` | `NewWithOptions(options ...Option) (*Engine, error)` | Creates an Engine from defaults and functional options |
| `DefaultConfig` | `DefaultConfig() (ServerConfig, error)` | Gets the MMA default configuration |
| `Use` | `Use(handlers ...HandlerFunc)` | Adds global middleware |
| `Handle` | `Handle(route string, handlers ...HandlerFunc)` | Registers a regular route |
| `Group` | `Group(prefix string, handlers ...HandlerFunc) *RouterGroup` | Creates a route group |
| `HandleChannel` | `HandleChannel(route string, handler ChannelHandlerFunc) error` | Registers a Channel handler |
| `Run` | `Run() error` | Starts the server and blocks until `Close()` |
| `Stop` | `Stop() error` | Stops the native server |
| `Close` | `Close() error` | Stops the server, releases native resources, and unblocks `Run()` |
| `BoundPort` | `BoundPort() (int, error)` | Returns the port on which the server is listening |
| `ChangeConfig` | `ChangeConfig(ServerConfig) error` | Passes dynamically applicable settings to the native server |
| `SendToChannel` | `SendToChannel(connectionID uint32, message ChannelMessage) error` | Sends a message to a specific Channel connection |
| `Broadcast` | `Broadcast(message ChannelMessage) error` | Sends a message to all Channel connections |
| `ConfigureAuth` | `ConfigureAuth(cfg AuthConfig) error` | Creates and attaches an `AuthManager` |

`ServerConfig` and `RouterGroup` are embedded in `Engine`. Configuration fields are therefore promoted to Engine, and `Group`, `Handle`, and `Use` are available directly on the Engine. `Engine.Auth` is `nil` until `ConfigureAuth()` is called.

### 4.2. Lifecycle

```text
New / NewWithOptions
        |
        v
Use / Group / Handle / HandleChannel
        |
        v
      Run()
        |
        +---- starts the native server
        |
        +---- blocks the calling goroutine
                    |
                    v
                 Close()
                    |
                    +---- closes done
                    +---- stops the native server
                    +---- releases resources
                    |
                    v
                Run() returns
```

The current public API exports `Run()`, but does not expose a separate public `Start()`. The internal `start()` method starts the native server, after which `Run()` waits on the `done` channel.

`Stop()` stops the native server but does not close `done`. Therefore, `Stop()` alone does not unblock a running `Run()`; `Close()` is used to finish the Engine lifecycle and return from `Run()`.

`Close()` can be called more than once; subsequent calls return `nil`. It closes `done` before calling the native close operation, so `Run()` may return while `Close()` is still releasing native resources. `Stop()` does not put the Engine into a state that supports restarting. Restarting the same Engine is not a supported workflow.

### 4.3. Bound port

```go
port, err := engine.BoundPort()
if err != nil {
	log.Fatal(err)
}
log.Printf("MMA server listens on port %d", port)
```

The port is available after the native server has successfully started and bound its listener. The method returns an error if the server is not initialized or has not completed binding.

---

## 5. Routing and groups

### 5.1. Regular routes

A handler has the following type:

```go
type HandlerFunc func(*Context)
```

Registration:

```go
engine.Handle("users.get", func(c *mrc.Context) {
	_ = c.JSON(mrc.StatusOK, mrc.H{
		"id":   "user-42",
		"name": "Alex",
	})
})
```

A route is an MMA command name. The current API does not provide separate `GET`, `POST`, `PUT`, `PATCH`, or `DELETE` methods. URL parameters, wildcard routes, and automatic routing by HTTP method are not implemented.

A route is registered before the server starts. It must be a non-empty UTF-8 string within the protocol limit of 255 bytes. Route names are case-sensitive.

**`Engine.Handle()` does not return an error.** In the current implementation, route validation errors and native `RegisterRoute` errors are not returned to the caller. Routes therefore need to be valid and unique before registration; `Handle()` does not expose a separate registration result.

### 5.2. Groups

`RouterGroup` combines routes under a common prefix and adds local middleware.

```go
api := engine.Group("/api")

api.Handle("/health", func(c *mrc.Context) {
	_ = c.JSON(mrc.StatusOK, mrc.H{"status": "ok"})
})

v1 := api.Group("/v1")
v1.Handle("/users", listUsers)
v1.Handle("/profile", getProfile)
```

Nested groups inherit middleware from their parent group as it exists when the child group is created:

```go
api := engine.Group("/api", apiAccessMiddleware)

v1 := api.Group("/v1", versionMiddleware)
v1.Handle("/users", listUsers)
```

Group prefixes and route names are joined with `path.Join`. Repeated separators and `.` / `..` segments are normalized according to `path.Join` rules.

### 5.3. Middleware order

Global middleware is registered through `Engine.Use()`, group middleware through `RouterGroup.Use()` or `Group()` arguments, and route-specific handlers through `Handle()`.

The resulting chain order is:

1. Engine global middleware.
2. Parent group middleware.
3. Nested group middleware, from outer to inner.
4. Route handlers in the order passed to `Handle()`.

```go
engine.Use(requestLogMiddleware)

api := engine.Group("api", apiMiddleware)
v1 := api.Group("v1", versionMiddleware)

v1.Handle("users", authMiddleware, listUsers)
```

The handler chain is copied when a route is registered. Middleware added with `engine.Use()` after a route has been registered is not added to that existing route. Likewise, `Group()` copies the parent middleware when the child group is created, and `RouterGroup.Use()` affects routes registered after that call rather than routes already registered. Registration is performed before the Engine starts.

`Engine.Handle()` and `RouterGroup.Handle()` do not return errors.

### 5.4. `Next()` and `Abort()` semantics

`Context.Next()` runs the rest of the chain. The main `Next()` loop also advances to the next handler after the current handler returns. Simply returning from middleware does not stop the chain.

```go
func requestTiming(c *mrc.Context) {
	started := time.Now()

	c.Next()

	log.Printf("route=%q elapsed=%s", c.Request.Route, time.Since(started))
}
```

`Abort()` sets the aborted flag; remaining handlers are not executed:

```go
func requireOption(c *mrc.Context) {
	if _, exists := c.Request.GetOption("client-id"); !exists {
		c.Abort()
		_ = c.JSON(mrc.StatusBadRequest, mrc.H{"error": "client-id is required"})
	}
}
```

`IsAborted()` returns the current aborted state.

---

## 6. Request and Context

### 6.1. Request

`Context.Request` contains a `*Request`.

```go
type Request struct {
	Opcode  cabi.RequestOpcode
	Version uint8
	ReqID   uint32
	Route   string
	Options []RequestOption
	Payload []byte
}
```

| Field | Purpose |
|---|---|
| `Opcode` | MMA request type: `ChannelConnect`, `Once`, or `ChannelMessage` |
| `Version` | Protocol version |
| `ReqID` | Request identifier echoed in the response |
| `Route` | Registered route name |
| `Options` | Frame metadata; `RequestOption` is an alias of `cabi.Option` |
| `Payload` | Request body in bytes |

Read options:

```go
authorization, exists := c.Request.GetOption("authorization")
if !exists {
	// The option is absent.
}

raw, exists := c.Request.GetOptionBytes("authorization")
```

`GetOption()` returns a string; `GetOptionBytes()` returns bytes. Keys are compared exactly and are case-sensitive. MMA `Options` are binary frame metadata, not HTTP headers.

`Request.Context()` returns the standard request `context.Context`. If the Request or its internal context is absent, it returns `context.Background()`. `NewRequest()` copies the incoming options slice and payload bytes into Go-owned slices before exposing them through `Request`.

`NewRequest(source *cabi.Request, ctx context.Context) *Request` is exported for low-level integration. In regular applications, the MRC adapter creates each Request automatically.

### 6.2. Context

`Context` is passed to each `HandlerFunc`. It provides the Request, middleware-chain control, request-scoped storage, request errors, and response methods. The `NewContext(request, response, handlers)` constructor is also exported for low-level integration; regular routes create a Context automatically.

| Method | Purpose |
|---|---|
| `Next()` | Executes the remaining handler chain |
| `Abort()` | Stops the remaining handler chain |
| `IsAborted()` | Returns whether the chain was aborted |
| `Context()` | Returns `Request.Context()` |
| `Set(key, value)` | Stores data in the current request context |
| `Get(key)` | Returns a value and a presence flag |
| `MustGet(key)` | Returns a value or panics if the key is missing |
| `Delete(key)` | Removes data from request-scoped storage |
| `Error(err)` | Adds an error to the current request error list |
| `Errors()` | Returns a copy of the request error list |
| `Written()` | Indicates whether the response payload has been written |
| `ResponseStatus()` | Returns the current response status |
| `ResponseSize()` | Returns the written payload size in bytes |

Request-scoped storage example:

```go
func loadUser(c *mrc.Context) {
	c.Set("user-id", "user-42")
}

func profile(c *mrc.Context) {
	userID, exists := c.Get("user-id")
	if !exists {
		_ = c.JSON(mrc.StatusInternalError, mrc.H{"error": "user context missing"})
		return
	}

	_ = c.JSON(mrc.StatusOK, mrc.H{"user_id": userID})
}
```

The storage exists only for the lifetime of the current `Context`. It is not a session store shared across requests.

### 6.3. Request errors

`Error()` appends an error to the list returned by `Context.Errors()`. Calling `Error()` does not change the response status, write a payload, or abort the handler chain by itself.

```go
if err := doWork(c.Context()); err != nil {
	c.Error(err)
	_ = c.JSON(mrc.StatusInternalError, mrc.H{"error": "operation failed"})
	return
}
```

`Errors()` returns a copy of the slice, so callers cannot mutate the internal list through the returned slice. These errors are used by middleware such as Logger.

### 6.4. Context lifetime

`Context` and `ChannelContext` belong to a single callback. When the callback returns, MRC calls an internal `expire()`: the Context becomes inactive, the response writer and handlers are released, and the request-scoped map is cleared.

A Context must not be retained after the handler returns. Do not pass it to background goroutines or use it after the callback completes. Copy the required values and manage background tasks with a separate `context.Context`.

---

## 7. Request binding

### 7.1. `ShouldBindJSON`

`ShouldBindJSON(destination)` decodes `Request.Payload` into the provided structure using the standard `encoding/json` package.

```go
type CreateUserRequest struct {
	Name string `json:"name"`
	Age  int    `json:"age"`
}

func createUser(c *mrc.Context) {
	var input CreateUserRequest

	if err := c.ShouldBindJSON(&input); err != nil {
		_ = c.JSON(mrc.StatusBadRequest, mrc.H{"error": "invalid JSON payload"})
		return
	}

	_ = c.JSON(mrc.StatusOK, mrc.H{
		"name": input.Name,
		"age":  input.Age,
	})
}
```

The method returns `ErrEmptyPayload` when the payload is empty. Required-field validation and business-rule validation are not performed.

### 7.2. `BindJSON`

`BindJSON(destination)` uses `ShouldBindJSON()`. On error it:

1. Adds the original error to `Context.Errors()`.
2. Writes a JSON response with `StatusBadRequest` and body `{"error":"Invalid JSON payload"}`.
3. Aborts the remaining handler chain through `Abort()`.
4. Returns the original error; if writing the error response also fails, the returned error is combined through `errors.Join()`.

```go
func createUser(c *mrc.Context) {
	var input CreateUserRequest
	if err := c.BindJSON(&input); err != nil {
		return
	}

	_ = c.JSON(mrc.StatusOK, mrc.H{"created": true})
}
```

Request JSON decoding uses `encoding/json`; response JSON encoding uses a separate library, `go-json`.

---

## 8. Responses

### 8.1. JSON

`JSON(status, value)` serializes a value with `github.com/goccy/go-json`, disables HTML escaping, removes the encoder's trailing newline, and writes the result to the payload.

```go
if err := c.JSON(mrc.StatusOK, mrc.H{
	"id":   "user-42",
	"name": "Alex",
}); err != nil {
	c.Error(err)
}
```

`mrc.H` is an alias for `map[string]any`.

JSON buffers are reused through `sync.Pool`. Buffers with capacity greater than 1 MiB are not returned to the pool.

### 8.2. String

`String(status, format, values...)` formats a string with `fmt.Sprintf` and sets the content type to `text/plain; charset=utf-8`.

```go
_ = c.String(mrc.StatusOK, "hello, %s", "MMA")
```

### 8.3. Data

`Data(status, contentType, data)` writes an arbitrary byte payload. If `contentType` is non-empty, it is added as the `content-type` response option.

```go
_ = c.Data(
	mrc.StatusOK,
	"application/octet-stream",
	[]byte{0x01, 0x02, 0x03},
)
```

### 8.4. Status

`Status(status)` sets the response status without writing a payload:

```go
if err := c.Status(mrc.StatusNotFound); err != nil {
	c.Error(err)
}
```

`Status()` does not set `Written()` to `true`. A payload can be written afterward with `JSON`, `String`, or `Data`; those methods pass their own status argument to the write operation.

### 8.5. Response options

`AddResponseOption(key, value)` adds response metadata. The standard implementation follows these rules:

- key and value must not be empty;
- keys are compared case-sensitively;
- adding the same key with the same value again returns `nil`;
- adding the same key with a different value returns an error;
- after a payload has been written, subsequent write operations are rejected.

```go
_ = c.AddResponseOption("cache-control", "no-store")
_ = c.JSON(mrc.StatusOK, mrc.H{"status": "ok"})
```

Constants:

| Constant | Value |
|---|---|
| `OptionContentType` | `content-type` |
| `MIMEApplicationJSON` | `application/json` |
| `MIMETextPlain` | `text/plain; charset=utf-8` |

`JSON()` adds `application/json` automatically; `String()` adds `text/plain; charset=utf-8`; `Data()` uses the supplied content type.

### 8.6. Abort with a response

```go
_ = c.AbortWithStatus(mrc.StatusForbidden)
```

Or with a JSON payload:

```go
_ = c.AbortWithStatusJSON(
	mrc.StatusBadRequest,
	mrc.H{"error": "invalid request"},
)
```

Both methods call `Abort()`. The first sets a status without a payload; the second serializes JSON and writes a payload.

### 8.7. Single write

After a successful payload write, `Context.Written()` returns `true`. Further payload writes through `JSON`, `String`, or `Data` are rejected with `ErrResponseAlreadySent`. `Status()` and `AddResponseOption()` also check whether writing is still available.

Write-operation errors are returned by the invoked method. Some errors are also stored in the internal `writeErr` and returned from the native route callback after the handler chain completes. In the current implementation, the failed writable-state check in `JSON()` is recorded in `writeErr`; `String()` and `Data()` return the same `ErrResponseAlreadySent` directly through the shared `write()` path. Errors from `Status()` and `AddResponseOption()` are recorded as well. JSON serialization errors are returned with the `mrc: JSON encode` prefix and are not stored in `writeErr`. `Context.Error()` is a separate application/logging error list; it does not replace `writeErr`.

---

## 9. Middleware

### 9.1. Custom middleware

Middleware has the same type as a regular handler:

```go
type HandlerFunc func(*Context)
```

Register global middleware:

```go
engine.Use(requestLog, authentication)
```

Middleware can also be passed to `Group()` and `Handle()`.

### 9.2. Logger

The `github.com/HELP-Studio-D/mrc/middleware` package contains a built-in request Logger.

```go
engine.Use(middleware.Logger())
```

Functions:

| Function | Purpose |
|---|---|
| `Logger()` | Logger with default configuration |
| `LoggerWithFormatter(formatter)` | Logger with a custom formatter |
| `LoggerWithWriter(writer, notLogged...)` | Logger with a custom `io.Writer` and route-name exclusions |
| `LoggerWithConfig(config)` | Logger with full configuration |
| `DisableConsoleColor()` | Disables ANSI colors |
| `ForceConsoleColor()` | Forces ANSI colors |

`LogConf` fields:

| Field | Type | Purpose |
|---|---|---|
| `Formatter` | `LogFormatter` | Function that formats a log line |
| `Output` | `io.Writer` | Output destination; defaults to `DefaultWriter` |
| `SkipRoutes` | `[]string` | Route names excluded from logging |
| `Skip` | `Skipper` | Function returning `true` when a request should be skipped |

`DefaultWriter` defaults to `os.Stdout`. The built-in formatter includes timestamp, status, latency, opcode, request ID, route, request/response payload sizes, option count, and errors from `Context.Errors()`.

`LogFormatterParams` provides:

- `Request *mrc.Request`;
- `TimeStamp time.Time`;
- `StatusCode int`;
- `Latency time.Duration`;
- `RequestSize`, `ResponseSize`, `OptionsCount`;
- `ErrorMessage string`.

`LogFormatterParams` methods are `StatusCodeColor()`, `LatencyColor()`, `ResetColor()`, and `IsOutputColor()`. Automatic terminal detection is performed for `*os.File`; colors are not enabled by default for other writer types.

Example custom formatter:

```go
formatter := func(p middleware.LogFormatterParams) string {
	return fmt.Sprintf(
		"route=%s status=%d latency=%s request=%dB response=%dB",
		p.Request.Route,
		p.StatusCode,
		p.Latency,
		p.RequestSize,
		p.ResponseSize,
	)
}

engine.Use(middleware.LoggerWithFormatter(formatter))
```

### 9.3. Recovery

Recovery middleware catches a Go panic raised while the remaining request middleware/handler chain is executing.

```go
engine.Use(middleware.Recovery())
```

Available functions:

| Function | Purpose |
|---|---|
| `Recovery()` | Recovery with the default handler and output to `stderr` |
| `CustomRecovery(handler)` | Recovery with a custom handler |
| `RecoveryWithWriter(writer)` | Recovery with a custom writer |
| `CustomRecoveryWithWriter(writer, handler)` | Recovery with a writer and custom handler |

Custom handler type:

```go
type RecoveryFunc func(c *mrc.Context, recovered any)
```

Recovery stores the panic in `Context.Errors()`, captures a stack trace, calls `Abort()`, and invokes the recovery handler. If the handler has not written a payload, `StatusInternalError` is set. A panic inside the recovery handler is also recovered by Recovery and added to the error list.

```go
func handlePanic(c *mrc.Context, recovered any) {
	if c.Written() {
		return
	}

	_ = c.JSON(mrc.StatusInternalError, mrc.H{
		"error": "internal server error",
	})
}

func configureRecovery(engine *mrc.Engine) {
	engine.Use(middleware.CustomRecovery(handlePanic))
}
```

`DefaultErrorWriter` defaults to `os.Stderr`. Passing `nil` to `CustomRecoveryWithWriter` disables stack-trace logging by that middleware.

---

## 10. Channel

MMA supports two connection modes: `Once` and `Channel`. `Once` handles individual requests with responses. `Channel` switches a TCP connection into a persistent message mode.

### 10.1. Register a Channel handler

```go
engine.HandleChannel("chat.message", func(c *mrc.ChannelContext) {
	log.Printf("connection=%d route=%q payload=%q",
		c.ConnectionID,
		c.Request.Route,
		c.Request.Payload,
	)

	if err := c.Send([]byte("message received")); err != nil {
		log.Printf("channel send: %v", err)
	}
})
```

`HandleChannel()` returns an error and checks that:

- the Engine exists and the native server is initialized;
- the Engine is not closed or started;
- the route is non-empty and the handler is non-nil;
- the route is not already registered as a regular or Channel route.

A regular route and a Channel route cannot share the same name. `HandleChannel()` checks for an existing regular route, but `Handle()` does not symmetrically inspect `channelRoutes` in the Go layer. If a Channel route is registered first and a regular route with the same name is added afterward, the duplicate can be detected when the native router is assembled at server start.

### 10.2. ChannelContext

| Field / method | Purpose |
|---|---|
| `Request` | Request received by the Channel handler |
| `ConnectionID` | Identifier of the current Channel connection |
| `Context()` | Standard context for the current callback |
| `Send(payload)` | Sends a payload to the current connection |
| `SendMessage(message)` | Sends a message with `ReqID` and options |
| `Broadcast(payload)` | Sends a payload to all Channel connections |

`ChannelContext` is valid only while the callback is running. Sending after the callback returns produces `ErrChannelContextExpired`.

### 10.3. Sending from Engine

`Engine.SendToChannel(connectionID, message)` sends a message to a specific connection. Connection ID `0` is reserved for broadcast.

```go
message := mrc.ChannelMessage{
	ReqID:   100,
	Payload: []byte("server notification"),
	Options: []mrc.RequestOption{
		{Key: "event", Value: "notification"},
	},
}

if err := engine.SendToChannel(connectionID, message); err != nil {
	log.Printf("send to channel: %v", err)
}

if err := engine.Broadcast(message); err != nil {
	log.Printf("broadcast: %v", err)
}
```

`ChannelMessage` contains `ReqID`, `Payload`, and `Options`. MRC sets the outgoing opcode to `StatusMessage` for these messages.

### 10.4. Protocol semantics

- A client switches a connection to Channel mode with the `ChannelConnect` opcode.
- After connecting, the client sends messages with the `ChannelMessage` opcode and the route of a registered Channel handler.
- A successful `ChannelMessage` does not create a regular request/response reply.
- `SendToChannel(0, ...)` and `Broadcast()` send to all channel subscribers.
- A successful send call means that the native queue accepted the message; it is not a delivery acknowledgement.
- The MMA implementation uses a bounded broadcast queue. A slow receiver may miss messages when the queue lags.
- Channel pushes and regular responses share the connection's outgoing stream; a strict ordering between them is not guaranteed.

`ChannelContext.Context()` is created with `context.WithCancel()` and canceled after the callback. `RequestTimeout` does not set a deadline on this Go Channel context.

---

## 11. Authentication: JWT, Claims, and bcrypt

Authentication is integrated into the root `mrc` package and configured through `Engine.ConfigureAuth()`. A separate runtime package is not required.

### 11.1. Auth configuration

```go
err := engine.ConfigureAuth(mrc.AuthConfig{
	JWT: mrc.JWTConfig{
		SigningKey: []byte(os.Getenv("MRC_JWT_SECRET")),
		Issuer:     "example-api",
		Audience:   []string{"example-clients"},
		AccessTTL:  15 * time.Minute,
		Leeway:     5 * time.Second,
	},
	Password: mrc.PasswordConfig{
		BcryptCost: 12,
	},
})
if err != nil {
	log.Fatal(err)
}
```

| Type / field | Purpose and validation |
|---|---|
| `AuthConfig.JWT` | JWT signing and verification settings |
| `AuthConfig.Password` | Password hashing settings |
| `JWTConfig.SigningKey` | HS256 HMAC key; at least 32 bytes |
| `JWTConfig.Issuer` | Expected issuer; checked when non-empty |
| `JWTConfig.Audience` | Expected audience; checked when the list is non-empty |
| `JWTConfig.AccessTTL` | Access token lifetime; must be positive |
| `JWTConfig.Leeway` | Allowed clock difference; must not be negative |
| `PasswordConfig.BcryptCost` | bcrypt cost; `0` means `bcrypt.DefaultCost` |

`NewAuthManager(cfg)` creates a standalone `*AuthManager`. `ConfigureAuth()` performs the same initialization and stores the manager in `engine.Auth`. Auth must be configured before the Engine starts. Calling it again before startup replaces the current AuthManager.

### 11.2. Claims

`Claims` contains user information serialized into a JWT.

```go
claims := mrc.NewClaims("user-42")
claims.AddRole("admin")
claims.AddPermission("users:read")
claims.AddPermission("users:write")
claims.Set("tenant_id", "tenant-7")

jwtString, err := engine.Auth.GenerateToken(claims)
if err != nil {
	log.Fatal(err)
}
```

Structure:

| Field | JSON claim | Purpose |
|---|---|---|
| `Subject` | `sub` | User identifier |
| `Roles` | `roles` | Roles |
| `Permissions` | `permissions` | Permissions |
| `Data` | `data` | Additional application-defined values |

`Claims` methods:

| Method | Purpose |
|---|---|
| `NewClaims(subject)` | Creates claims with a subject and initialized Data map |
| `AddRole(role)` / `RemoveRole(role)` | Adds or removes a role; duplicate roles are not added |
| `HasRole(role)` | Checks for a role |
| `HasAnyRole(roles...)` | Checks whether at least one role exists |
| `HasAllRoles(roles...)` | Checks whether all roles exist |
| `AddPermission(permission)` / `RemovePermission(permission)` | Adds or removes a permission; duplicates are not added |
| `HasPermission(permission)` | Checks a permission |
| `HasAnyPermission(permissions...)` | Checks whether at least one permission exists |
| `HasAllPermissions(permissions...)` | Checks whether all permissions exist |
| `Set(key, value)` | Stores an additional claim in `Data` |
| `Value(key)` | Retrieves an additional claim |

`GenerateToken()` automatically adds `iss`, `sub`, `aud` (when configured), `iat`, and `exp`. Expiration is calculated from `AccessTTL`.

### 11.3. Generate and verify JWTs

```go
claims := mrc.NewClaims(user.ID)
claims.AddRole("user")

accessToken, err := engine.Auth.GenerateToken(claims)
if err != nil {
	return err
}

verifiedClaims, err := engine.Auth.ParseToken(accessToken)
if err != nil {
	return err
}

log.Println("authenticated subject:", verifiedClaims.Subject)
```

`ParseToken()`:

- accepts HS256 only;
- verifies the HMAC signature;
- requires `exp` and checks expiration;
- validates `iat` when it is present;
- validates `iss` when `Issuer` is configured;
- validates `aud` when `Audience` is configured;
- requires a non-empty `sub`.

An expired token returns an error compatible with `ErrTokenExpired`; other parsing and validation errors return `ErrInvalidToken`. JWTs are signed, not encrypted: their contents are readable by anyone who holds the token.

The current Auth implementation provides access tokens. Refresh tokens, server-side session storage, and revocation of issued tokens are not implemented.

### 11.4. Password hashing

```go
passwordHash, err := engine.Auth.HashPass("correct horse battery staple")
if err != nil {
	log.Fatal(err)
}

valid, err := engine.Auth.CheckPass(
	"correct horse battery staple",
	passwordHash,
)
if err != nil {
	log.Fatal(err)
}
if !valid {
	log.Println("password mismatch")
}
```

`HashPass()` creates a bcrypt hash with a random salt and the configured cost. `CheckPass()` returns:

- `(true, nil)` — the password matches;
- `(false, nil)` — the password does not match;
- `(false, error)` — the hash is malformed or another verification error occurred.

Empty passwords are rejected. bcrypt limits password input to 72 bytes; the implementation checks byte length.

### 11.5. JWT middleware

MMA passes metadata through `Request.Options`, so token extraction is configured with a `TokenExtractor` function.

```go
extractToken := func(c *mrc.Context) (string, error) {
	authorization, exists := c.Request.GetOption("authorization")
	if !exists {
		return "", mrc.ErrTokenMissing
	}

	return mrc.ExtractBearerToken(authorization)
}

authMiddleware, err := engine.Auth.Middleware(extractToken)
if err != nil {
	log.Fatal(err)
}

engine.Use(authMiddleware)
```

`ExtractBearerToken()` parses a string in the form `Bearer <token>`; the `Bearer` scheme is matched case-insensitively. Extracting the option and choosing its key remain the responsibility of `TokenExtractor`.

When verification succeeds, the middleware stores `*Claims` in the current Context through `SetClaims()`. When token extraction or verification fails, the middleware:

- aborts the handler chain;
- writes `StatusUnauthorized`;
- writes JSON `{"error":"unauthorized"}`.

Claims are available to handlers:

```go
func profile(c *mrc.Context) {
	claims, exists := c.Claims()
	if !exists {
		_ = c.JSON(mrc.StatusUnauthorized, mrc.H{"error": "unauthorized"})
		return
	}

	if !claims.HasPermission("profile:read") {
		_ = c.JSON(mrc.StatusForbidden, mrc.H{"error": "forbidden"})
		return
	}

	_ = c.JSON(mrc.StatusOK, mrc.H{
		"user_id": claims.Subject,
		"roles":   claims.Roles,
	})
}
```

### 11.6. Claims and Token in Context

`Context` provides methods for storing verified claims:

| Method | Purpose |
|---|---|
| `SetClaims(*Claims)` | Stores claims in the current request context |
| `Claims() (*Claims, bool)` | Retrieves claims |
| `MustClaims() *Claims` | Retrieves claims or panics with `ErrClaimsNotFound` |
| `ClearClaims()` | Removes claims from the current Context |

A separate `mrc.Token` structure is also available:

```go
type Token struct {
	Token     string
	ExpiresAt time.Time
	IssuedAt  time.Time
	Claims    map[string]string
}
```

`SetToken(*Token)`, `Token() (*Token, bool)`, `MustToken() any`, and `ClearToken()` use request-scoped storage. This is a manual Token value store; the JWT middleware automatically stores `Claims`, not `Token`.

---

## 12. Errors and statuses

### 12.1. Response statuses

`mrc.Status` is an alias of `cabi.ResponseOpcode`. These are MMA response codes, not HTTP status codes.

| Constant | Code | Meaning |
|---|---:|---|
| `StatusOK` | 1 | Success |
| `StatusBadRequest` | 2 | Invalid request |
| `StatusUnauthorized` | 3 | Unauthorized |
| `StatusForbidden` | 4 | Forbidden |
| `StatusNotFound` | 5 | Route not found |
| `StatusConflict` | 6 | Conflict |
| `StatusInternalError` | 7 | Internal error |
| `StatusMessage` | 8 | Channel message |
| `StatusShutdown` | 9 | Shutdown opcode |
| `StatusChannelConnected` | 10 | Channel connection established |
| `StatusTimeout` | 11 | Timeout |

### 12.2. MRC errors

| Error | Condition |
|---|---|
| `ErrServerNotInitialized` | Engine or native server is not initialized |
| `ErrInvalidRoute` | Empty route or invalid handler when registering a Channel route |
| `ErrRouteAlreadyExists` | Route already exists; `HandleChannel()` checks regular/Channel route conflicts |
| `ErrServerStarted` | Attempt to register a route or configure Auth after startup |
| `ErrContextExpired` | Use of an expired Context in operations that check its lifecycle |
| `ErrResponseAlreadySent` | Attempt to write after a successful payload write |
| `ErrResponseUnavailable` | Response writer is unavailable |
| `ErrEmptyPayload` | Empty payload during JSON binding |
| `ErrChannelContextExpired` | Use of ChannelContext after its callback |
| `ErrChannelUnavailable` | Channel sender/handle is unavailable |
| `ErrClaimsNotFound` | `MustClaims()` called without claims |
| `ErrAuthNotConfigured` | AuthManager has not been configured |
| `ErrInvalidAuthConfig` | Invalid AuthConfig |
| `ErrInvalidClaims` | Claims do not contain a valid subject |
| `ErrInvalidToken` | JWT validation failed |
| `ErrTokenExpired` | JWT has expired |
| `ErrTokenMissing` | Token is missing |
| `ErrEmptyPassword` | Password is empty |
| `ErrPasswordTooLong` | Password exceeds 72 bytes |
| `ErrEngineClosed` | Auth configuration called on a closed Engine |
| `ErrInvalidAuthorizationHeader` | String does not match the Bearer format |

`Handle()` and `RouterGroup.Handle()` do not return errors. `HandleChannel()` returns a registration error.

### 12.3. Errors stored in Context

`Context.Error(err)` stores an application error, and `Context.Errors()` returns the list. This is separate from response-write errors: writer errors use an internal `writeErr` field and may be returned from the callback to the C ABI.

---

## 13. Implementation details and limitations

- MRC is not an HTTP server. It does not implement HTTP methods, URL path parameters, wildcard routing, cookies, HTTP header parsing, or `net/http` handler compatibility.
- A route is an MMA string identifier. Its length is limited to 255 bytes and it must be valid UTF-8.
- `Engine.Handle()` and `RouterGroup.Handle()` do not return errors. The current `Engine.Handle()` implementation does not expose route validation failures or native registration errors to the caller.
- `HandleChannel()` returns registration errors. A regular and Channel route cannot share a name; if a Channel route is registered first, regular `Handle()` does not inspect `channelRoutes` in the Go layer, and a duplicate can surface while the native router is being assembled at startup.
- Routes and middleware are registered before server startup. Handler chains are copied when `Handle()` is called.
- The public API has a blocking `Run()` and no public `Start()`. `Run()` waits for `done`; `Stop()` stops the native server but does not close `done`. `Close()` unblocks `Run()`.
- `Stop()` does not leave the Engine in a state that supports restarting.
- `ChangeConfig()` delegates to the C ABI. Not all configuration fields are dynamically applied to a running server; the Go-side request timeout is also not refreshed by this method.
- The request callback receives a `context.WithTimeout()` using `ServerConfig.RequestTimeout`. The context signals deadline expiration, but it cannot forcibly terminate arbitrary Go code. Long-running operations should use `c.Context()` or `c.Request.Context()` and support cancellation.
- Synchronous Go callbacks do not become cancellable just because `RequestTimeout` is set. A blocking call that does not observe the context continues until it returns.
- `ChannelContext.Context()` is created with `context.WithCancel()`; it does not receive a deadline from `RequestTimeout`.
- `Context` and `ChannelContext` have limited lifetimes. They must not be retained after a callback or accessed from background goroutines. The request-scoped map uses a mutex for its map operations, but the entire Context object is not a general-purpose concurrent-safe object; the handler chain, response state, and error list are not designed for concurrent mutation.
- `Context.Errors()` contains only errors explicitly added through `Context.Error()`. Response-write errors are handled separately.
- `Claims.Data` is serialized as JSON. After JWT parsing, dynamic numeric values in `map[string]any` are decoded according to the JSON decoder's standard rules (typically as `float64`).
- JWTs are signed with HS256 but are not encrypted. Do not place secrets or passwords in Claims.
- Auth implements access JWTs and bcrypt. Refresh tokens, token revocation, OAuth/OIDC, and server-side session storage are not implemented.
- bcrypt limits passwords to 72 bytes. Store the string returned by `HashPass()` as the password hash in the database.
- SQL connections, a Query Builder, ORM, migrations, Redis/cache, automatic struct validation, and a DB repository API are not implemented in this source tree.

---

## 14. Tests and benchmarks

### 14.1. Tests

From the module root:

```bash
go test ./...
go test -v ./...
go test -race ./...
go vet ./...
go test -cover ./...
```

### 14.2. Benchmark

The `benchmark/` directory contains a separate network benchmark comparing MRC with HTTP/Gin, plus JSON serialization microbenchmarks.

```bash
cd benchmark
go test ./cmd/bench
go run ./cmd/bench
```

The network benchmark reads optional `BENCH_*` environment variables; practical defaults are in `benchmark/cmd/bench/.env`. Process environment variables take precedence over `.env`.

Example run with explicit settings:

```powershell
$env:BENCH_REQUESTS=500000
$env:BENCH_CONCURRENCY=64
$env:BENCH_WARMUP=20000
$env:BENCH_ROUNDS=3
go run ./cmd/bench
```

Serialization microbenchmarks:

```bash
go test ./cmd/bench -run '^$' -bench 'Benchmark(JSONMarshal|WritePreEncodedJSON)$' -benchmem
```

The network benchmark measures the full request/response path: framing, TCP, handler execution, response encoding, and client-side reading. The microbenchmarks isolate JSON serialization and writing pre-encoded JSON. Benchmark results depend on the execution environment and runtime parameters; fixed performance figures are not specified here.

---

## 15. Additional types and low-level APIs

The root package also exports the following types:

| Type | Purpose |
|---|---|
| `Status` | Alias of `cabi.ResponseOpcode` for MMA response statuses |
| `HandlerFunc` | Regular request handler type: `func(*Context)` |
| `HandlersChain` | Slice of `[]HandlerFunc` |
| `H` | Alias for `map[string]any`, used in JSON responses |
| `Option` | Functional option that modifies `ServerConfig` |
| `RequestOption` | Alias of `cabi.Option` for request/response options |
| `ChannelHandlerFunc` | Channel callback type: `func(*ChannelContext)` |
| `ChannelMessage` | Outgoing Channel message (`ReqID`, `Payload`, `Options`) |
| `TokenExtractor` | Function that extracts a JWT from the current Context |
| `RouteHandler` | Low-level callback `func(*cabi.Request, *cabi.ResponseBuilder) error`; regular routing uses `HandlerFunc` |

`NewContext()` and `NewRequest()` are available for adapters and low-level integrations. Regular application code uses Context and Request instances created by the MRC route adapter.

---

## 16. Repository layout

| Path | Contents |
|---|---|
| `config.go`, `options_conf.go` | Server configuration and functional options |
| `engine.go`, `server.go` | Engine creation, start, stop, and close |
| `router.go`, `router_group.go` | Regular routes, callbacks, and groups |
| `context.go` | Context, handler chain, request-scoped storage, and errors |
| `context_request.go` | Request and JSON binding |
| `context_response.go` | JSON, String, Data, status, and response options |
| `context_token.go`, `claims.go` | Token and Claims storage in Context |
| `channel.go`, `channel_context.go` | Channel handlers, sending, and broadcast |
| `auth.go`, `jwt.go`, `password.go`, `middleware_auth.go` | JWT, bcrypt, and authentication middleware |
| `middleware/middleware_logger.go` | Logger middleware and formatting |
| `middleware/middleware_recovery.go` | Recovery middleware |
| `examples/FastStart/main.go` | Minimal server example |
| `benchmark/` | Network and JSON serialization benchmarks |
| `go.mod`, `go.sum` | Go module dependencies |

The frame format and C ABI specifications are maintained in [MMA Network Protocol](https://github.com/Edestuss0/MMA-Network-protocol) and its protocol documentation examples.
