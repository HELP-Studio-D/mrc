<div align="center">

# MRC

**Go-фреймворк для серверных приложений поверх бинарного TCP-протокола [MMA](https://github.com/Edestuss0/MMA-Network-protocol).**

[English](README.md) · **Русский**

</div>

> **Статус:** pre-release. MRC использует нативный сервер MMA через C ABI. Публичный API MRC и интерфейс C ABI находятся в активной разработке.

MRC предоставляет Go API для создания сервера, регистрации маршрутов и обработчиков, организации middleware-цепочек, работы с запросами и ответами, двусторонних Channel-соединений и базовой авторизации на JWT и bcrypt.

В основе MRC лежат четыре основных объекта: `Engine`, `Context`, `Request` и `AuthManager`. Низкоуровневая обработка TCP-соединений, кадров протокола и сетевого ввода-вывода выполняется движком MMA. MRC преобразует его callbacks в более удобные Go-обработчики.

MRC работает поверх собственного бинарного протокола MMA, а не поверх HTTP. Маршрут — строковый идентификатор команды протокола; `Options` — метаданные кадра; `Payload` — байтовое тело сообщения. HTTP-методы, URL-параметры и HTTP-заголовки в API MRC не реализованы.

## Содержание

- [1. Обзор](#1-обзор)
- [2. Установка и быстрый старт](#2-установка-и-быстрый-старт)
- [3. Конфигурация сервера](#3-конфигурация-сервера)
- [4. Engine и жизненный цикл](#4-engine-и-жизненный-цикл)
- [5. Маршрутизация и группы](#5-маршрутизация-и-группы)
- [6. Request и Context](#6-request-и-context)
- [7. Разбор входных данных](#7-разбор-входных-данных)
- [8. Формирование ответов](#8-формирование-ответов)
- [9. Middleware](#9-middleware)
- [10. Channel](#10-channel)
- [11. Авторизация: JWT, Claims и bcrypt](#11-авторизация-jwt-claims-и-bcrypt)
- [12. Ошибки и статусы](#12-ошибки-и-статусы)
- [13. Ограничения и особенности реализации](#13-ограничения-и-особенности-реализации)
- [14. Проверка и бенчмарки](#14-проверка-и-бенчмарки)
- [15. Дополнительные типы и низкоуровневые API](#15-дополнительные-типы-и-низкоуровневые-api)
- [16. Структура репозитория](#16-структура-репозитория)

---

## 1. Обзор

### 1.1. Модель приложения

MRC разделяет серверную логику на несколько уровней:

| Уровень | Ответственность |
|---|---|
| `Engine` | Конфигурация, жизненный цикл сервера, регистрация обработчиков, глобальные middleware и доступ к Auth |
| `RouterGroup` | Префиксы маршрутов и групповые middleware |
| `Context` | Управление цепочкой, хранение request-scoped данных, разбор JSON и формирование ответа |
| `Request` | Opcode, версия, ID, маршрут, options, payload и `context.Context` запроса |
| `ChannelContext` | Доступ к Channel-соединению и отправка сообщений |
| `AuthManager` | Выпуск и проверка JWT, хеширование и проверка паролей |
| `middleware` | Готовые Logger и Recovery |
| MMA C ABI | TCP-соединения, бинарный framing, исполнение callback и отправка ответов |

### 1.2. Основные свойства

| Свойство | Реализация |
|---|---|
| Язык API | Go |
| Сетевой транспорт | TCP через MMA |
| Формат сообщений | Бинарные кадры MMA с настраиваемым `FramerConfig` |
| Режимы соединения | `Once` — запрос/ответ; `Channel` — постоянное соединение и push-сообщения |
| Маршруты | Строковые имена, чувствительные к регистру |
| Обработчик | `func(*mrc.Context)` |
| Middleware | Глобальные, групповые и маршрутные обработчики |
| JSON request decode | Стандартный `encoding/json` |
| JSON response encode | `github.com/goccy/go-json` |
| Авторизация | JWT HS256 и bcrypt |
| SQL / ORM | В текущем исходном коде отсутствуют |

### 1.3. Пакеты

| Пакет | Содержимое |
|---|---|
| `github.com/HELP-Studio-D/mrc` | Engine, конфигурация, маршруты, Context, Request, Channel, JWT, Claims и bcrypt |
| `github.com/HELP-Studio-D/mrc/middleware` | Logger и Recovery middleware |
| `github.com/HELP-Studio-D/mrc/auth` | Только структуры конфигурации `Config`, `JWTConfig` и `PasswordConfig`; runtime-менеджер авторизации находится в корневом пакете `mrc` |

Подпакет `mrc/auth` экспортирует только структуры конфигурации. `Config` содержит `JWT` и `Password`; `JWTConfig` содержит `SigningKey`, `Issuer` и `AccessTTL`; `PasswordConfig` содержит `Algorithm` и `BcryptCost`. Эти типы отличаются от `mrc.AuthConfig`, `mrc.JWTConfig` и `mrc.PasswordConfig` корневого пакета, которые принимает `Engine.ConfigureAuth()`: это не алиасы, и передать структуры из `mrc/auth` в этот метод нельзя.

---

## 2. Установка и быстрый старт

### 2.1. Требования

- Go `1.26.0` или новее — версия, указанная в `go.mod`.
- Go modules.
- Нативная библиотека MMA C ABI для целевой платформы. Она подключается зависимостью `github.com/Edestuss0/MMA-Network-protocol/cabi`.

Установка:

```bash
go get github.com/HELP-Studio-D/mrc
```

### 2.2. Быстрый старт

`NewWithOptions()` создаёт сервер с конфигурацией MMA по умолчанию и применяет функциональные опции. `Handle()` регистрирует обработчик для строкового имени маршрута. `Run()` запускает сервер и блокирует вызывающую горутину до закрытия Engine.

```go
package main

import (
	"fmt"
	"log"

	"github.com/HELP-Studio-D/mrc"
	"github.com/HELP-Studio-D/mrc/middleware"
)

func main()  {
	// Главный элемент приложения для управлением всем фреймворком
	server, err := mrc.NewWithOptions(
		mrc.WithBindIP("127.0.0.1"),
		mrc.WithPort(8080),
	)
	if err != nil {
		fmt.Errorf(err.Error())
		return 
	}

	// Объявление логгера запросов на сервер
	server.Use(middleware.Logger())

	// Регистрация публичного роута
	server.Handle("hello", func(c *mrc.Context) {
		// Вы можете использовать также c.JSON(status, mrc.H{...:...})
		_ = c.String(mrc.StatusOK, "World!")
	})

	// Запуск сервера 
	if err := server.Run(); err != nil {
		log.Fatal("Server isn't running, error:" + err.Error())
	}
}
```

В примере `hello` — имя MMA route, а не HTTP URL. Для отправки запроса клиент должен сформировать кадр MMA с соответствующим route и opcode `Once`.

### 2.3. Что делает быстрый старт

1. `NewWithOptions()` получает настройки по умолчанию из C ABI и применяет `WithBindIP` и `WithPort`.
2. `Use()` добавляет глобальные middleware.
3. `Handle()` регистрирует маршрут и копирует в него текущую middleware-цепочку.
4. `Run()` запускает нативный сервер и ожидает закрытия Engine.

---

## 3. Конфигурация сервера

### 3.1. `ServerConfig`

Конфигурация может быть получена через `DefaultConfig()` и изменена вручную, либо сформирована через `NewWithOptions()`.

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

Поля `ServerConfig`:

| Поле | Тип | Назначение |
|---|---|---|
| `BindIP` | `string` | IP-адрес привязки listener |
| `Port` | `uint16` | TCP-порт |
| `MaxBatch` | `uint32` | Максимальное число ответов в batch перед flush |
| `MaxInFlight` | `uint32` | Глобальный лимит одновременно обрабатываемых запросов |
| `Framer` | `FramerConfig` | Раскладка бинарного кадра и лимит тела сообщения |
| `MaxConnections` | `uint32` | Лимит соединений |
| `DrainTimeout` | `time.Duration` | Время на отправку накопленных ответов при закрытии соединения |
| `ShutdownGrace` | `time.Duration` | Период ожидания выполняющихся задач при остановке сервера |
| `RequestTimeout` | `time.Duration` | Таймаут обработки запроса в нативной конфигурации и deadline Go request context |

`DefaultConfig()` получает значения по умолчанию от C ABI MMA. Для текущей конфигурации по умолчанию используются:

| Параметр | Значение по умолчанию |
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
| `Framer.MaxMessageLength` | `10000000` байт |

### 3.2. Функциональные опции

`Option` — функция `func(*ServerConfig) error`. Опции передаются в `NewWithOptions()` и применяются последовательно. При повторном указании одного параметра побеждает последняя опция.

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

| Опция | Изменяемое поле | Проверка в Go-опции |
|---|---|---|
| `WithBindIP(ip)` | `BindIP` | Пустая строка отклоняется |
| `WithPort(port)` | `Port` | Дополнительной проверки нет; конфигурацию проверяет C ABI |
| `WithMaxBatch(limit)` | `MaxBatch` | Ноль отклоняется |
| `WithMaxInFlight(limit)` | `MaxInFlight` | Ноль отклоняется |
| `WithMaxConnections(limit)` | `MaxConnections` | Ноль отклоняется |
| `WithFramer(config)` | `Framer` | Копирует все поля; проверка выполняется нативной конфигурацией |
| `WithDrainTimeout(duration)` | `DrainTimeout` | Значение передаётся без дополнительной проверки опцией |
| `WithShutdownGrace(duration)` | `ShutdownGrace` | Значение передаётся без дополнительной проверки опцией |
| `WithRequestTimeout(duration)` | `RequestTimeout` | Значение передаётся без дополнительной проверки опцией |

`NewWithOptions()` сначала вызывает `DefaultConfig()`, поэтому опции изменяют значения по умолчанию, а не создают конфигурацию с нулевыми полями.

### 3.3. `FramerConfig`

`FramerConfig` определяет wire-раскладку протокола MMA. Конфигурации клиента и сервера должны совпадать.

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

| Поле | Назначение |
|---|---|
| `HeaderLength` | Размер заголовка; в стандартной раскладке — 29 байт |
| `MaxMessageLength` | Максимальный размер секций тела кадра |
| `OpcodePos` | Позиция opcode в заголовке, нумерация с 1 |
| `VersionPos` | Позиция версии протокола, нумерация с 1 |
| `RouteLenPos` | Позиция длины route, нумерация с 1 |
| `OptionsCountPos` | Позиция количества options, нумерация с 1 |
| `RouteOrder` | Порядок секции route в теле кадра |
| `OptionsOrder` | Порядок секции options в теле кадра |
| `PayloadOrder` | Порядок секции payload в теле кадра |
| `OptionsKeyFirst` | Порядок key/value внутри option |
| `BigBytesConfig.OptionsLenOrder` | Позиция 4-байтового поля длины options |
| `BigBytesConfig.PayloadLenOrder` | Позиция 4-байтового поля длины payload |
| `BigBytesConfig.ReqIDOrder` | Позиция 4-байтового поля request ID |

Для изменения одного поля удобно сначала получить конфигурацию по умолчанию:

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

Все wire-параметры, кроме серверного лимита `MaxMessageLength`, должны соответствовать конфигурации клиента. Изменение раскладки только на одной стороне нарушает разбор кадров.

### 3.4. Изменение конфигурации во время работы

`Engine.ChangeConfig(newConfig)` передаёт новую конфигурацию запущенному C ABI серверу. Вызов разрешён после запуска Engine.

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

Изменение конфигурации делегируется MMA C ABI. Не все параметры применяются к уже работающему серверу: адрес listener, `MaxBatch`, `MaxConnections`, `MaxInFlight` и `ShutdownGrace` не являются полностью динамическими настройками в текущем C ABI. Wire-параметры должны синхронно обновляться и на клиентской стороне.

Go request context MRC получает timeout из конфигурации при создании `Engine`; `ChangeConfig()` обновляет конфигурацию нативного сервера и экспортированную `Engine.ServerConfig`, но не обновляет сохранённое внутреннее значение Go timeout.

---

## 4. Engine и жизненный цикл

`Engine` — центральный объект приложения. Он содержит конфигурацию сервера, глобальную middleware-цепочку, таблицы зарегистрированных маршрутов, Channel handle и `AuthManager`.

### 4.1. Методы Engine

| Метод | Сигнатура | Назначение |
|---|---|---|
| `New` | `New(config ServerConfig) (*Engine, error)` | Создаёт Engine из структуры конфигурации |
| `NewWithOptions` | `NewWithOptions(options ...Option) (*Engine, error)` | Создаёт Engine из конфигурации по умолчанию и опций |
| `DefaultConfig` | `DefaultConfig() (ServerConfig, error)` | Получает конфигурацию MMA по умолчанию |
| `Use` | `Use(handlers ...HandlerFunc)` | Добавляет глобальные middleware |
| `Handle` | `Handle(route string, handlers ...HandlerFunc)` | Регистрирует обычный маршрут |
| `Group` | `Group(prefix string, handlers ...HandlerFunc) *RouterGroup` | Создаёт группу маршрутов |
| `HandleChannel` | `HandleChannel(route string, handler ChannelHandlerFunc) error` | Регистрирует Channel-обработчик |
| `Run` | `Run() error` | Запускает сервер и блокируется до `Close()` |
| `Stop` | `Stop() error` | Останавливает нативный сервер |
| `Close` | `Close() error` | Останавливает сервер, освобождает нативные ресурсы и разблокирует `Run()` |
| `BoundPort` | `BoundPort() (int, error)` | Возвращает порт, на котором слушает сервер |
| `ChangeConfig` | `ChangeConfig(ServerConfig) error` | Передаёт динамически применяемые настройки нативному серверу |
| `SendToChannel` | `SendToChannel(connectionID uint32, message ChannelMessage) error` | Отправляет сообщение конкретному Channel-соединению |
| `Broadcast` | `Broadcast(message ChannelMessage) error` | Отправляет сообщение всем Channel-соединениям |
| `ConfigureAuth` | `ConfigureAuth(cfg AuthConfig) error` | Создаёт и подключает `AuthManager` |

`ServerConfig` и `RouterGroup` встроены в `Engine`. Поэтому параметры конфигурации доступны через поля Engine, а методы `Group`, `Handle` и `Use` доступны на самом Engine. Поле `Engine.Auth` равно `nil`, пока не вызван `ConfigureAuth()`.

### 4.2. Жизненный цикл

```text
New / NewWithOptions
        |
        v
Use / Group / Handle / HandleChannel
        |
        v
      Run()
        |
        +---- запускает нативный сервер
        |
        +---- блокирует вызывающую горутину
                    |
                    v
                 Close()
                    |
                    +---- закрывает done
                    +---- останавливает нативный сервер
                    +---- освобождает ресурсы
                    |
                    v
                Run() возвращает
```

В текущем публичном API экспортирован `Run()`, но отдельного публичного `Start()` нет. Внутренний метод `start()` запускает нативный сервер, после чего `Run()` ожидает закрытия канала `done`.

`Stop()` останавливает нативный сервер, но не закрывает `done`. Поэтому `Stop()` сам по себе не разблокирует выполняющийся `Run()`; для завершения жизненного цикла и возврата из `Run()` используется `Close()`.

`Close()` можно вызывать повторно: последующие вызовы возвращают `nil`. Метод закрывает `done` до вызова нативного закрытия, поэтому `Run()` может вернуться, пока `Close()` ещё освобождает нативные ресурсы. `Stop()` не переводит Engine в состояние, пригодное для повторного запуска; после остановки повторный `Run()` не является поддерживаемым сценарием.

### 4.3. Получение bound port

```go
port, err := engine.BoundPort()
if err != nil {
	log.Fatal(err)
}
log.Printf("MMA server listens on port %d", port)
```

Порт доступен после успешного запуска нативного сервера. Если сервер не инициализирован или ещё не выполнил bind, метод возвращает ошибку.

---

## 5. Маршрутизация и группы

### 5.1. Обычные маршруты

Обработчик имеет тип:

```go
type HandlerFunc func(*Context)
```

Регистрация:

```go
engine.Handle("users.get", func(c *mrc.Context) {
	_ = c.JSON(mrc.StatusOK, mrc.H{
		"id":   "user-42",
		"name": "Alex",
	})
})
```

Маршрут — имя команды MMA. В текущем API нет отдельных методов `GET`, `POST`, `PUT`, `PATCH` или `DELETE`; нет URL-параметров, wildcard-маршрутов и автоматического разделения по HTTP method.

Маршруты регистрируются до запуска сервера. Route должен быть непустой UTF-8 строкой и укладываться в протокольный лимит 255 байт. Имена чувствительны к регистру.

**`Engine.Handle()` не возвращает `error`.** В текущей реализации ошибки проверки маршрута и ошибка `RegisterRoute` не возвращаются вызывающему коду. Поэтому маршруты должны быть уникальными и корректными до запуска; ошибка регистрации обычного маршрута не имеет отдельного результата, доступного через `Handle()`.

### 5.2. Группы

`RouterGroup` объединяет маршруты под общим префиксом и добавляет локальные middleware.

```go
api := engine.Group("/api")

api.Handle("/health", func(c *mrc.Context) {
	_ = c.JSON(mrc.StatusOK, mrc.H{"status": "ok"})
})

v1 := api.Group("/v1")
v1.Handle("/users", listUsers)
v1.Handle("/profile", getProfile)
```

Вложенные группы наследуют middleware родительской группы, зарегистрированные на момент создания дочерней группы:

```go
api := engine.Group("/api", apiAccessMiddleware)

v1 := api.Group("/v1", versionMiddleware)
v1.Handle("/users", listUsers)
```

Префиксы и route объединяются через `path.Join`. Лишние `/`, сегменты `.` и `..` нормализуются стандартными правилами `path.Join`.

### 5.3. Middleware и порядок выполнения

Глобальные middleware регистрируются через `Engine.Use()`, групповые — через `RouterGroup.Use()` или аргументы `Group()`, маршрутные обработчики передаются в `Handle()`.

Порядок цепочки:

1. Глобальные middleware Engine.
2. Middleware родительской группы.
3. Middleware дочерних групп по уровню вложенности.
4. Обработчики маршрута в порядке передачи в `Handle()`.

```go
engine.Use(requestLogMiddleware)

api := engine.Group("api", apiMiddleware)
v1 := api.Group("v1", versionMiddleware)

v1.Handle("users", authMiddleware, listUsers)
```

Цепочка копируется при регистрации маршрута. Middleware, добавленное через `engine.Use()` после регистрации route, не попадёт в уже зарегистрированную цепочку. Аналогично `Group()` копирует middleware родителя при создании дочерней группы. Регистрация выполняется до запуска Engine.

`Engine.Handle()` и `RouterGroup.Handle()` не возвращают ошибку.

### 5.4. Семантика `Next()` и `Abort()`

`Context.Next()` запускает оставшуюся часть цепочки. Основной цикл `Next()` также продолжает переходить к следующим обработчикам после возврата текущего обработчика. Поэтому простое завершение middleware не останавливает цепочку.

```go
func requestTiming(c *mrc.Context) {
	started := time.Now()

	c.Next()

	log.Printf("route=%q elapsed=%s", c.Request.Route, time.Since(started))
}
```

`Abort()` устанавливает флаг остановки; оставшиеся обработчики не запускаются:

```go
func requireOption(c *mrc.Context) {
	if _, exists := c.Request.GetOption("client-id"); !exists {
		c.Abort()
		_ = c.JSON(mrc.StatusBadRequest, mrc.H{"error": "client-id is required"})
	}
}
```

`IsAborted()` возвращает состояние флага.

---

## 6. Request и Context

### 6.1. Request

`Context.Request` содержит объект `*Request`.

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

| Поле | Назначение |
|---|---|
| `Opcode` | Тип запроса MMA: `ChannelConnect`, `Once` или `ChannelMessage` |
| `Version` | Версия протокола |
| `ReqID` | Идентификатор запроса, возвращаемый в ответе |
| `Route` | Имя зарегистрированного маршрута |
| `Options` | Метаданные кадра; `RequestOption` — alias типа `cabi.Option` |
| `Payload` | Тело запроса в байтах |

Доступ к options:

```go
authorization, exists := c.Request.GetOption("authorization")
if !exists {
	// Option отсутствует.
}

raw, exists := c.Request.GetOptionBytes("authorization")
```

`GetOption()` возвращает строку, `GetOptionBytes()` — байты. Сравнение ключей выполняется посимвольно и чувствительно к регистру. `Options` протокола MMA — это метаданные бинарного кадра, а не HTTP-заголовки.

`Request.Context()` возвращает стандартный `context.Context`. Если Request или его внутренний контекст отсутствует, возвращается `context.Background()`.

### 6.2. Context

`Context` передаётся каждому `HandlerFunc`. Он предоставляет Request, работу с middleware-цепочкой, request-scoped storage, ошибки и методы ответа. Конструктор `NewContext(request, response, handlers)` также экспортирован для низкоуровневой интеграции; стандартный маршрут создаёт Context автоматически.

| Метод | Назначение |
|---|---|
| `Next()` | Выполняет оставшуюся часть цепочки |
| `Abort()` | Прерывает оставшуюся часть цепочки |
| `IsAborted()` | Возвращает признак прерывания |
| `Context()` | Возвращает `Request.Context()` |
| `Set(key, value)` | Сохраняет значение в хранилище текущего запроса |
| `Get(key)` | Возвращает значение и признак наличия |
| `MustGet(key)` | Возвращает значение или вызывает panic при отсутствии ключа |
| `Delete(key)` | Удаляет значение из request-scoped storage |
| `Error(err)` | Добавляет ошибку в список ошибок запроса |
| `Errors()` | Возвращает копию списка ошибок запроса |
| `Written()` | Показывает, записан ли payload ответа |
| `ResponseStatus()` | Возвращает текущий статус ответа |
| `ResponseSize()` | Возвращает размер записанного payload в байтах |

Пример request-scoped storage:

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

Хранилище существует только в пределах текущего `Context`. Оно не является хранилищем сессий между запросами.

### 6.3. Ошибки запроса

`Error()` добавляет ошибку в список `Context.Errors()`. Сам по себе вызов `Error()` не меняет response status, не записывает payload и не прерывает цепочку.

```go
if err := doWork(c.Context()); err != nil {
	c.Error(err)
	_ = c.JSON(mrc.StatusInternalError, mrc.H{"error": "operation failed"})
	return
}
```

`Errors()` возвращает копию среза, чтобы вызывающий код не мог изменить внутренний список через полученный срез. Ошибки используются, в частности, Logger middleware.

### 6.4. Срок жизни Context

`Context` и `ChannelContext` относятся к одному callback. После его завершения MRC вызывает внутренний `expire()`: Context становится неактивным, response writer и handlers освобождаются, request-scoped map очищается.

Context не предназначен для хранения после возврата обработчика. Не передавайте его в фоновые горутины и не используйте после завершения callback. Для фоновой работы копируются необходимые значения, а жизненным циклом задачи управляет отдельный `context.Context`.

---

## 7. Разбор входных данных

### 7.1. `ShouldBindJSON`

`ShouldBindJSON(destination)` декодирует `Request.Payload` в переданную структуру с помощью стандартного `encoding/json`.

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

Метод возвращает `ErrEmptyPayload`, если payload пустой. Валидация обязательных полей и проверка бизнес-правил не выполняются.

### 7.2. `BindJSON`

`BindJSON(destination)` использует `ShouldBindJSON()`. При ошибке он:

1. Добавляет исходную ошибку в `Context.Errors()`.
2. Формирует JSON-ответ со статусом `StatusBadRequest` и телом `{"error":"Invalid JSON payload"}`.
3. Прерывает дальнейшее выполнение цепочки через `Abort()`.
4. Возвращает исходную ошибку; если запись ответа тоже завершилась ошибкой, возвращается объединённая ошибка через `errors.Join()`.

```go
func createUser(c *mrc.Context) {
	var input CreateUserRequest
	if err := c.BindJSON(&input); err != nil {
		return
	}

	_ = c.JSON(mrc.StatusOK, mrc.H{"created": true})
}
```

Разбор JSON запроса выполняется `encoding/json`; сериализация JSON-ответа выполняется отдельной библиотекой `go-json`.

---

## 8. Формирование ответов

### 8.1. JSON

`JSON(status, value)` сериализует значение через `github.com/goccy/go-json`, отключает HTML escaping, удаляет завершающий newline кодировщика и записывает результат в payload.

```go
if err := c.JSON(mrc.StatusOK, mrc.H{
	"id":   "user-42",
	"name": "Alex",
}); err != nil {
	c.Error(err)
}
```

`mrc.H` — alias `map[string]any`.

JSON buffer переиспользуется через `sync.Pool`. Буферы ёмкостью больше 1 MiB не возвращаются в пул.

### 8.2. String

`String(status, format, values...)` формирует строку через `fmt.Sprintf` и задаёт content type `text/plain; charset=utf-8`.

```go
_ = c.String(mrc.StatusOK, "hello, %s", "MMA")
```

### 8.3. Data

`Data(status, contentType, data)` записывает произвольный byte payload. Если `contentType` непустой, он добавляется как response option `content-type`.

```go
_ = c.Data(
	mrc.StatusOK,
	"application/octet-stream",
	[]byte{0x01, 0x02, 0x03},
)
```

### 8.4. Status

`Status(status)` устанавливает статус без записи payload:

```go
if err := c.Status(mrc.StatusNotFound); err != nil {
	c.Error(err)
}
```

`Status()` не устанавливает `Written() == true`. После него payload может быть записан отдельно через `JSON`, `String` или `Data`; эти методы передают собственный аргумент status в операцию записи.

### 8.5. Response options

`AddResponseOption(key, value)` добавляет метаданные в ответ. В стандартной реализации используются следующие правила:

- ключ и значение не должны быть пустыми;
- ключи сравниваются с учётом регистра;
- повторное добавление того же ключа с тем же значением возвращает `nil`;
- повторное добавление того же ключа с другим значением возвращает ошибку;
- после записи payload ответ считается завершённым, и последующие операции записи не разрешаются.

```go
_ = c.AddResponseOption("cache-control", "no-store")
_ = c.JSON(mrc.StatusOK, mrc.H{"status": "ok"})
```

Константы:

| Константа | Значение |
|---|---|
| `OptionContentType` | `content-type` |
| `MIMEApplicationJSON` | `application/json` |
| `MIMETextPlain` | `text/plain; charset=utf-8` |

`JSON()` автоматически добавляет `application/json`; `String()` — `text/plain; charset=utf-8`; `Data()` использует переданный тип содержимого.

### 8.6. Прерывание цепочки с ответом

```go
_ = c.AbortWithStatus(mrc.StatusForbidden)
```

Или с JSON payload:

```go
_ = c.AbortWithStatusJSON(
	mrc.StatusBadRequest,
	mrc.H{"error": "invalid request"},
)
```

Оба метода вызывают `Abort()`. Первый устанавливает status без payload, второй сериализует JSON и записывает payload.

### 8.7. Однократная запись

После успешной записи payload `Context.Written()` возвращает `true`. Последующие записи payload через `JSON`, `String` или `Data` отклоняются с `ErrResponseAlreadySent`. `Status()` и `AddResponseOption()` также проверяют, доступна ли запись.

Ошибки операций записи возвращаются вызывающему методу. Некоторые ошибки дополнительно сохраняются во внутреннем `writeErr` и возвращаются из нативного route callback после выполнения цепочки. В текущей реализации неуспешная проверка доступности записи в `JSON()` сохраняется в `writeErr`; `String()` и `Data()` возвращают `ErrResponseAlreadySent` напрямую через общую функцию `write()`. Ошибки `Status()` и `AddResponseOption()` также сохраняются. Ошибка сериализации JSON возвращается с префиксом `mrc: JSON encode` и не записывается в `writeErr`. `Context.Error()` — отдельный список ошибок приложения и логирования; он не заменяет `writeErr`.

---

## 9. Middleware

### 9.1. Собственные middleware

Middleware имеет тот же тип, что и обработчик:

```go
type HandlerFunc func(*Context)
```

Регистрация глобального middleware:

```go
engine.Use(requestLog, authentication)
```

Middleware также передаются в `Group()` и `Handle()`.

### 9.2. Logger

Пакет `github.com/HELP-Studio-D/mrc/middleware` содержит готовое middleware логирования.

```go
engine.Use(middleware.Logger())
```

Функции:

| Функция | Назначение |
|---|---|
| `Logger()` | Logger с настройками по умолчанию |
| `LoggerWithFormatter(formatter)` | Logger с пользовательским форматтером |
| `LoggerWithWriter(writer, notLogged...)` | Logger с заданным `io.Writer` и исключениями по именам route |
| `LoggerWithConfig(config)` | Logger с полной конфигурацией |
| `DisableConsoleColor()` | Отключает ANSI-цвета |
| `ForceConsoleColor()` | Принудительно включает ANSI-цвета |

Конфигурация `LogConf`:

| Поле | Тип | Назначение |
|---|---|---|
| `Formatter` | `LogFormatter` | Функция форматирования строки лога |
| `Output` | `io.Writer` | Приёмник вывода; по умолчанию `DefaultWriter` |
| `SkipRoutes` | `[]string` | Имена route, которые не логируются |
| `Skip` | `Skipper` | Функция, возвращающая `true`, если запрос нужно пропустить |

`DefaultWriter` по умолчанию равен `os.Stdout`. Встроенный форматтер включает timestamp, статус, latency, opcode, request ID, route, размер request/response payload, количество options и ошибки из `Context.Errors()`.

`LogFormatterParams` передаёт форматтеру:

- `Request *mrc.Request`;
- `TimeStamp time.Time`;
- `StatusCode int`;
- `Latency time.Duration`;
- `RequestSize`, `ResponseSize`, `OptionsCount`;
- `ErrorMessage string`.

Методы `LogFormatterParams`: `StatusCodeColor()`, `LatencyColor()`, `ResetColor()` и `IsOutputColor()`. Автоматическое определение терминала выполняется для `*os.File`; для других writer цвет по умолчанию не включается.

Пример пользовательского форматтера:

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

Recovery middleware перехватывает Go panic, возникший во время выполнения оставшейся части request middleware/handler chain.

```go
engine.Use(middleware.Recovery())
```

Доступные функции:

| Функция | Назначение |
|---|---|
| `Recovery()` | Recovery с обработчиком по умолчанию и выводом в `stderr` |
| `CustomRecovery(handler)` | Recovery с пользовательским обработчиком |
| `RecoveryWithWriter(writer)` | Recovery с заданным writer |
| `CustomRecoveryWithWriter(writer, handler)` | Recovery с writer и пользовательским обработчиком |

Тип пользовательского обработчика:

```go
type RecoveryFunc func(c *mrc.Context, recovered any)
```

Recovery сохраняет panic в `Context.Errors()`, собирает stack trace, вызывает `Abort()` и запускает recovery handler. Если обработчик не записал payload, устанавливается `StatusInternalError`. Если сам recovery handler вызывает panic, он также перехватывается внутри Recovery и добавляется в список ошибок.

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

`DefaultErrorWriter` по умолчанию равен `os.Stderr`. При передаче `nil` в `CustomRecoveryWithWriter` stack trace не выводится этим middleware.

---

## 10. Channel

MMA поддерживает два режима соединения: `Once` и `Channel`. `Once` обслуживает отдельные запросы с ответами. `Channel` переводит TCP-соединение в постоянный режим обмена сообщениями.

### 10.1. Регистрация Channel handler

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

`HandleChannel()` возвращает ошибку и проверяет:

- Engine существует и нативный сервер инициализирован;
- Engine не закрыт и ещё не запущен;
- route непустой, handler не `nil`;
- route не зарегистрирован как обычный или Channel route.

Обычный `Handle()` не возвращает ошибку и не выполняет симметричную Go-проверку `channelRoutes`. Поэтому одно и то же имя нельзя регистрировать одновременно через `Handle()` и `HandleChannel()`; дубликат может обнаружиться при сборке нативного router во время запуска.

### 10.2. ChannelContext

| Поле / метод | Назначение |
|---|---|
| `Request` | Запрос, пришедший в Channel handler |
| `ConnectionID` | Идентификатор текущего Channel-соединения |
| `Context()` | Стандартный контекст текущего callback |
| `Send(payload)` | Отправляет payload в текущее соединение |
| `SendMessage(message)` | Отправляет сообщение с `ReqID` и options |
| `Broadcast(payload)` | Отправляет payload всем Channel-соединениям |

`ChannelContext` действителен только во время callback. После его завершения методы отправки возвращают `ErrChannelContextExpired`.

### 10.3. Отправка из Engine

`Engine.SendToChannel(connectionID, message)` отправляет сообщение конкретному соединению. Идентификатор `0` зарезервирован для широковещательной отправки.

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

`ChannelMessage` содержит `ReqID`, `Payload` и `Options`. Исходящий opcode для этих сообщений устанавливается MRC как `StatusMessage`.

### 10.4. Протокольная семантика

- Клиент переводит соединение в Channel mode через opcode `ChannelConnect`.
- После подключения клиент отправляет сообщения с opcode `ChannelMessage` и route зарегистрированного Channel handler.
- Успешная обработка `ChannelMessage` не создаёт обычный request/response-ответ.
- `SendToChannel(0, ...)` и `Broadcast()` отправляют сообщение всем channel-подписчикам.
- Успешный вызов отправки означает принятие сообщения нативной очередью, а не подтверждение доставки клиенту.
- В реализации MMA используется ограниченная broadcast-очередь. Медленный получатель может пропустить сообщения при переполнении очереди.
- Channel push и обычные ответы используют общий исходящий поток соединения; строгий взаимный порядок между ними не гарантируется.

`ChannelContext.Context()` создаётся через `context.WithCancel()` и отменяется после callback. `RequestTimeout` не задаёт deadline этому Go-контексту Channel handler.

---

## 11. Авторизация: JWT, Claims и bcrypt

Auth встроен в корневой пакет `mrc` и подключается через `Engine.ConfigureAuth()`. Для использования не требуется отдельный runtime-пакет.

### 11.1. Конфигурация Auth

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

| Тип / поле | Назначение и проверка |
|---|---|
| `AuthConfig.JWT` | Параметры подписи и проверки JWT |
| `AuthConfig.Password` | Настройки хеширования паролей |
| `JWTConfig.SigningKey` | HMAC-ключ HS256; минимум 32 байта |
| `JWTConfig.Issuer` | Ожидаемый issuer; проверяется, если задан |
| `JWTConfig.Audience` | Ожидаемая audience; проверяется, если список не пуст |
| `JWTConfig.AccessTTL` | Срок действия access token; должен быть больше нуля |
| `JWTConfig.Leeway` | Допустимое расхождение часов; не может быть отрицательным |
| `PasswordConfig.BcryptCost` | Стоимость bcrypt; `0` означает `bcrypt.DefaultCost` |

`NewAuthManager(cfg)` создаёт самостоятельный `*AuthManager`. `ConfigureAuth()` выполняет ту же инициализацию и сохраняет менеджер в `engine.Auth`. Конфигурация может быть установлена до запуска Engine. Повторный вызов до запуска заменяет текущий AuthManager.

### 11.2. Claims

`Claims` — данные пользователя, сериализуемые в JWT.

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

Структура:

| Поле | JSON claim | Назначение |
|---|---|---|
| `Subject` | `sub` | Идентификатор пользователя |
| `Roles` | `roles` | Роли |
| `Permissions` | `permissions` | Разрешения |
| `Data` | `data` | Дополнительные пользовательские значения |

Методы `Claims`:

| Метод | Назначение |
|---|---|
| `NewClaims(subject)` | Создаёт claims с subject и инициализированной Data map |
| `AddRole(role)` / `RemoveRole(role)` | Добавляет или удаляет роль; повторная роль не добавляется |
| `HasRole(role)` | Проверяет наличие роли |
| `HasAnyRole(roles...)` | Проверяет наличие хотя бы одной из ролей |
| `HasAllRoles(roles...)` | Проверяет наличие всех ролей |
| `AddPermission(permission)` / `RemovePermission(permission)` | Добавляет или удаляет permission; дубликаты не добавляются |
| `HasPermission(permission)` | Проверяет permission |
| `HasAnyPermission(permissions...)` | Проверяет хотя бы один permission |
| `HasAllPermissions(permissions...)` | Проверяет все permissions |
| `Set(key, value)` | Сохраняет дополнительный claim в `Data` |
| `Value(key)` | Получает дополнительный claim |

`GenerateToken()` автоматически добавляет `iss`, `sub`, `aud` (если настроена), `iat` и `exp`. Срок действия вычисляется из `AccessTTL`.

### 11.3. Генерация и проверка JWT

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

- принимает только HS256;
- проверяет подпись HMAC;
- требует наличие `exp` и проверяет срок действия;
- проверяет `iat`, если он присутствует;
- проверяет `iss`, если `Issuer` задан;
- проверяет `aud`, если `Audience` задан;
- требует непустой `sub`.

Истёкший токен возвращает ошибку, совместимую с `ErrTokenExpired`; остальные ошибки разбора и проверки возвращают `ErrInvalidToken`. Токен подписывается, но не шифруется: содержимое JWT доступно для чтения его обладателю.

В текущем Auth реализованы access-токены. Refresh token, серверное хранилище сессий и отзыв уже выданных токенов не реализованы.

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

`HashPass()` генерирует bcrypt hash со случайной солью и настроенной стоимостью. `CheckPass()` возвращает:

- `(true, nil)` — пароль совпадает;
- `(false, nil)` — пароль не совпадает;
- `(false, error)` — некорректный hash или другая ошибка проверки.

Пустой пароль отклоняется. Длина пароля ограничена 72 байтами — это ограничение bcrypt. Проверяется именно длина в байтах.

### 11.5. JWT middleware

MMA передаёт метаданные через `Request.Options`, поэтому извлечение токена настраивается функцией `TokenExtractor`.

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

`ExtractBearerToken()` разбирает строку формата `Bearer <token>`; схема `Bearer` сравнивается без учёта регистра. Извлечение option и его ключа остаётся ответственностью `TokenExtractor`.

При успешной проверке middleware сохраняет `*Claims` в текущем Context через `SetClaims()`. При ошибке извлечения или проверки токена middleware:

- прерывает цепочку;
- отправляет `StatusUnauthorized`;
- записывает JSON `{"error":"unauthorized"}`.

Данные claims доступны обработчикам:

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

### 11.6. Claims и Token в Context

`Context` предоставляет методы для хранения проверенных claims:

| Метод | Назначение |
|---|---|
| `SetClaims(*Claims)` | Сохраняет claims текущего запроса |
| `Claims() (*Claims, bool)` | Получает claims |
| `MustClaims() *Claims` | Получает claims или вызывает panic `ErrClaimsNotFound` |
| `ClearClaims()` | Удаляет claims из текущего Context |

Отдельно существует структура `mrc.Token`:

```go
type Token struct {
	Token     string
	ExpiresAt time.Time
	IssuedAt  time.Time
	Claims    map[string]string
}
```

Методы `SetToken(*Token)`, `Token() (*Token, bool)`, `MustToken() any` и `ClearToken()` работают с request-scoped storage. Это ручное хранилище значения Token; JWT middleware автоматически помещает в Context именно `Claims`, а не `Token`.

---

## 12. Ошибки и статусы

### 12.1. Статусы ответа

`mrc.Status` — alias `cabi.ResponseOpcode`. Это коды ответа MMA, а не HTTP status codes.

| Константа | Код | Значение |
|---|---:|---|
| `StatusOK` | 1 | Успех |
| `StatusBadRequest` | 2 | Некорректный запрос |
| `StatusUnauthorized` | 3 | Не авторизован |
| `StatusForbidden` | 4 | Доступ запрещён |
| `StatusNotFound` | 5 | Маршрут не найден |
| `StatusConflict` | 6 | Конфликт |
| `StatusInternalError` | 7 | Внутренняя ошибка |
| `StatusMessage` | 8 | Channel-сообщение |
| `StatusShutdown` | 9 | Shutdown opcode |
| `StatusChannelConnected` | 10 | Channel-соединение установлено |
| `StatusTimeout` | 11 | Таймаут |

### 12.2. Ошибки MRC

| Ошибка | Условие |
|---|---|
| `ErrServerNotInitialized` | Engine или нативный сервер не инициализирован |
| `ErrInvalidRoute` | Пустой route или некорректный handler при регистрации Channel route |
| `ErrRouteAlreadyExists` | Route уже зарегистрирован; для `HandleChannel()` проверяется конфликт обычного и Channel route |
| `ErrServerStarted` | Попытка зарегистрировать route или изменить Auth после запуска |
| `ErrContextExpired` | Использование истёкшего Context при операциях, проверяющих его жизненный цикл |
| `ErrResponseAlreadySent` | Повторная запись после успешного payload |
| `ErrResponseUnavailable` | Response writer отсутствует |
| `ErrEmptyPayload` | Пустой payload при JSON binding |
| `ErrChannelContextExpired` | Использование ChannelContext после callback |
| `ErrChannelUnavailable` | Channel sender/handle недоступен |
| `ErrClaimsNotFound` | `MustClaims()` вызван без claims |
| `ErrAuthNotConfigured` | AuthManager не настроен |
| `ErrInvalidAuthConfig` | Неверная AuthConfig |
| `ErrInvalidClaims` | Claims не содержат корректный subject |
| `ErrInvalidToken` | JWT не прошёл проверку |
| `ErrTokenExpired` | JWT истёк |
| `ErrTokenMissing` | Токен отсутствует |
| `ErrEmptyPassword` | Пустой пароль |
| `ErrPasswordTooLong` | Пароль превышает 72 байта |
| `ErrEngineClosed` | Auth configuration вызывается для закрытого Engine |
| `ErrInvalidAuthorizationHeader` | Строка не соответствует схеме Bearer |

`Handle()` и `RouterGroup.Handle()` не возвращают ошибки. `HandleChannel()` возвращает ошибку регистрации.

### 12.3. Ошибки в Context

`Context.Error(err)` сохраняет ошибку приложения, а `Context.Errors()` возвращает список этих ошибок. Это не то же самое, что ошибка записи ответа: ошибки writer сохраняются отдельно и могут быть возвращены из callback в C ABI.

---

## 13. Ограничения и особенности реализации

- MRC не является HTTP-сервером и не реализует HTTP request methods, URL path parameters, wildcard routing, cookies, HTTP header parsing или `net/http` handler compatibility.
- Route — строковый идентификатор MMA. Его длина ограничена 255 байтами и route должен быть UTF-8.
- `Engine.Handle()` и `RouterGroup.Handle()` не возвращают ошибку. В текущем `Engine.Handle()` ошибки проверки и нативной регистрации не передаются вызывающему коду.
- `HandleChannel()` возвращает ошибки регистрации. Для одного route нельзя одновременно регистрировать обычный и Channel handler; при регистрации Channel первым обычный `Handle()` не проверяет `channelRoutes` в Go-слое, и конфликт может проявиться при старте нативного router.
- Маршруты и middleware регистрируются до запуска сервера. Цепочка обработчиков копируется при `Handle()`.
- В публичном API есть блокирующий `Run()`, отдельного публичного `Start()` нет. `Run()` ожидает закрытия `done`; `Stop()` останавливает нативный сервер, но не закрывает `done`. Для разблокировки `Run()` вызывается `Close()`.
- `Stop()` не переводит Engine в состояние, допускающее повторный запуск.
- `ChangeConfig()` делегирует обновление C ABI. Не все поля конфигурации динамически применяются к работающему серверу; Go-side request timeout также не обновляется этим методом.
- Request callback получает `context.WithTimeout()` с `ServerConfig.RequestTimeout`. Контекст сообщает об истечении deadline, но не может принудительно завершить произвольный Go-код. Долгие операции должны использовать `c.Context()` или `c.Request.Context()` и поддерживать отмену.
- Синхронные Go callbacks не становятся отменяемыми только из-за `RequestTimeout`. Блокирующий вызов, который не проверяет контекст, продолжает выполняться до своего возврата.
- `ChannelContext.Context()` создаётся с `context.WithCancel()`; отдельный deadline из `RequestTimeout` в него не устанавливается.
- `Context` и `ChannelContext` имеют ограниченный срок жизни. Нельзя хранить их после callback или обращаться к ним из фоновых горутин. Операции с request-scoped map используют mutex, но весь объект Context не является общей потокобезопасной структурой: цепочка обработчиков, состояние ответа и список ошибок не рассчитаны на конкурентное изменение.
- `Context.Errors()` содержит только ошибки, добавленные через `Context.Error()`. Ошибки записи ответа имеют отдельную внутреннюю обработку.
- `Claims.Data` сериализуется в JSON. После разбора JWT динамические числовые значения в `map[string]any` декодируются JSON-декодером в числовые типы по его стандартным правилам (обычно `float64`).
- JWT подписывается HS256, но не шифруется. В Claims не следует помещать секреты и пароли.
- В Auth реализованы access JWT и bcrypt. Refresh tokens, отзыв токенов, OAuth/OIDC и серверное session storage отсутствуют.
- bcrypt ограничивает пароль 72 байтами. В качестве password hash в БД сохраняется строка, возвращённая `HashPass()`.
- SQL-подключения, Query Builder, ORM, миграции, Redis/cache, автоматическая валидация структур и DB repository API в этом исходном коде не реализованы.

---

## 14. Проверка и бенчмарки

### 14.1. Тесты

Из корня модуля:

```bash
go test ./...
go test -v ./...
go test -race ./...
go vet ./...
go test -cover ./...
```

### 14.2. Benchmark

В каталоге `benchmark/` находится отдельный сетевой benchmark MRC против HTTP/Gin, а также микробенчмарки JSON serialization.

```bash
cd benchmark
go test ./cmd/bench
go run ./cmd/bench
```

Параметры сетевого benchmark читаются из переменных окружения `BENCH_*`; локальные значения по умолчанию находятся в `benchmark/cmd/bench/.env`. Переменные окружения имеют приоритет над `.env`.

Пример запуска с настройками:

```powershell
$env:BENCH_REQUESTS=500000
$env:BENCH_CONCURRENCY=64
$env:BENCH_WARMUP=20000
$env:BENCH_ROUNDS=3
go run ./cmd/bench
```

Микробенчмарки сериализации:

```bash
go test ./cmd/bench -run '^$' -bench 'Benchmark(JSONMarshal|WritePreEncodedJSON)$' -benchmem
```

Сетевой benchmark измеряет полный request/response путь: framing, TCP, обработчик, кодирование ответа и чтение на стороне клиента. Микробенчмарки отдельно сравнивают сериализацию JSON и запись заранее закодированного JSON. Результаты benchmark зависят от среды выполнения и параметров запуска; фиксированные значения производительности в документации не приводятся.

---

## 15. Дополнительные типы и низкоуровневые API

Корневой пакет также экспортирует следующие типы:

| Тип | Назначение |
|---|---|
| `Status` | Alias `cabi.ResponseOpcode` для статусов ответа MMA |
| `HandlerFunc` | Тип обычного request handler: `func(*Context)` |
| `HandlersChain` | Срез `[]HandlerFunc` |
| `H` | Alias `map[string]any`, используемый для JSON-ответов |
| `Option` | Функциональная опция изменения `ServerConfig` |
| `RequestOption` | Alias `cabi.Option` для request/response options |
| `ChannelHandlerFunc` | Тип Channel callback: `func(*ChannelContext)` |
| `ChannelMessage` | Структура исходящего Channel-сообщения (`ReqID`, `Payload`, `Options`) |
| `TokenExtractor` | Функция извлечения JWT из текущего Context |
| `RouteHandler` | Низкоуровневый callback `func(*cabi.Request, *cabi.ResponseBuilder) error`; обычная маршрутизация использует `HandlerFunc` |

`NewContext()` и `NewRequest()` доступны для адаптеров и низкоуровневых интеграций. Обычный прикладной код работает с Context и Request, которые создаёт route adapter MRC.

---

## 16. Структура репозитория

| Путь | Содержимое |
|---|---|
| `config.go`, `options_conf.go` | Конфигурация сервера и функциональные опции |
| `engine.go`, `server.go` | Создание Engine, запуск, остановка и закрытие |
| `router.go`, `router_group.go` | Обычные маршруты, callbacks и группы |
| `context.go` | Context, middleware chain, request-scoped storage и ошибки |
| `context_request.go` | Request и JSON binding |
| `context_response.go` | JSON, String, Data, status и response options |
| `context_token.go`, `claims.go` | Хранение Token и Claims в Context |
| `channel.go`, `channel_context.go` | Channel handlers, отправка и broadcast |
| `auth.go`, `jwt.go`, `password.go`, `middleware_auth.go` | JWT, bcrypt и auth middleware |
| `middleware/middleware_logger.go` | Logger middleware и форматирование |
| `middleware/middleware_recovery.go` | Recovery middleware |
| `examples/FastStart/main.go` | Минимальный пример запуска |
| `benchmark/` | Сетевые и JSON serialization benchmarks |
| `go.mod`, `go.sum` | Зависимости Go-модуля |

Дополнительная спецификация формата кадров и C ABI находится в [MMA Network Protocol](https://github.com/Edestuss0/MMA-Network-protocol) и в примерах протокольной документации.
