# MRC — документация

MRC — высокоуровневая Go-библиотека для работы с MMA-протоколом. Она предоставляет API для создания сервера, регистрации маршрутов, обработки запросов, формирования ответов и подключения middleware.

Библиотека построена поверх низкоуровневого пакета `cabi`, который взаимодействует с нативным сервером. Основная работа с запросами и ответами выполняется через `Engine`, `Context` и `Request`.

## 1. Первый сервер

Минимальный сервер состоит из создания Engine, регистрации маршрута и запуска.

```go
package main

import (
	"log"

	mrc "github.com/Edestuss0/MMA-Network-protocol"
)

func main() {
	engine, err := mrc.NewWithOptions(
		mrc.WithBindIP("127.0.0.1"),
		mrc.WithPort(9000),
	)
	if err != nil {
		log.Fatal(err)
	}

	defer engine.Close()

	err = engine.Handle("/hello", func(c *mrc.Context) {
		_ = c.JSON(mrc.StatusOK, mrc.H{
			"message": "Hello, MMA!",
		})
	})
	if err != nil {
		log.Fatal(err)
	}

	if err := engine.Run(); err != nil {
		log.Fatal(err)
	}
}
```

В примере:

- `NewWithOptions()` создаёт сервер с настройками.
- `Handle()` регистрирует маршрут и обработчик.
- `Context` предоставляет методы для чтения запроса и формирования ответа.
- `Run()` запускает сервер.
- `Close()` освобождает ресурсы нативного сервера.

`Run()` является алиасом `Start()`. Оба метода запускают нативный сервер.

## 2. Создание сервера и конфигурация

Для создания сервера предусмотрено два способа настройки.

### 2.1. Через структуру ServerConfig

Функция `DefaultConfig()` получает конфигурацию по умолчанию из нативного C ABI. После этого отдельные параметры можно изменить вручную.

```go
package main

import (
	"log"

	mrc "github.com/Edestuss0/MMA-Network-protocol"
)

func main() {
	config, err := mrc.DefaultConfig()
	if err != nil {
		log.Fatal(err)
	}

	config.BindIP = "127.0.0.1"
	config.Port = 9000
	config.MaxInFlight = 1000

	engine, err := mrc.New(config)
	if err != nil {
		log.Fatal(err)
	}
	defer engine.Close()

	if err := engine.Run(); err != nil {
		log.Fatal(err)
	}
}
```

Этот вариант подходит для случаев, когда требуется получить стандартную конфигурацию, изменить несколько параметров или полностью настроить структуру сервера.

### 2.2. Через функциональные опции

Второй способ — `NewWithOptions()`. Он автоматически загружает конфигурацию по умолчанию и последовательно применяет переданные опции.

```go
engine, err := mrc.NewWithOptions(
	mrc.WithBindIP("0.0.0.0"),
	mrc.WithPort(9000),
	mrc.WithMaxBatch(256),
	mrc.WithMaxInFlight(1000),
	mrc.WithMaxConnections(5000),
)
```

Доступные опции:

| Опция | Назначение |
|---|---|
| `WithBindIP(ip)` | Адрес привязки сервера |
| `WithPort(port)` | Порт сервера |
| `WithMaxBatch(limit)` | Максимальный размер batch |
| `WithMaxInFlight(limit)` | Максимальное количество одновременно обрабатываемых запросов |
| `WithMaxConnections(limit)` | Максимальное количество соединений |
| `WithFramer(config)` | Настройка протокольного фреймера |
| `WithDrainTimeout(duration)` | Время ожидания завершения обработки запросов при остановке |
| `WithShutdownGrace(duration)` | Параметр периода завершения работы сервера |
| `WithRequestTimeout(duration)` | Таймаут обработки запроса на стороне нативного сервера |

Опции применяются в том порядке, в котором они переданы. При повторном указании одного параметра используется последнее значение.

`WithBindIP` не принимает пустой адрес. `WithMaxBatch`, `WithMaxInFlight` и `WithMaxConnections` требуют ненулевого значения. Остальные ограничения конфигурации проверяются нативной реализацией либо не проверяются текущими функциями опций.

### 2.3. FramerConfig

`FramerConfig` описывает структуру протокольного сообщения: расположение полей заголовка, порядок байтов и максимальный размер сообщения.

```go
type FramerConfig struct {
	MaxMessageLength uint32

	OpcodePos       uint8
	VersionPos      uint8
	RouteLenPos     uint8
	RouteOrder      uint8
	PayloadOrder    uint8
	OptionsOrder    uint8
	OptionsCountPos uint8

	OptionsKeyFirst bool
	HeaderLength    uint8

	BigBytesConfig BigBytesConfig
}

type BigBytesConfig struct {
	OptionsLenOrder uint8
	PayloadLenOrder uint8
	ReqIDOrder      uint8
}
```

Для передачи собственной конфигурации фреймера используется:

```go
config, err := mrc.DefaultConfig()
if err != nil {
	log.Fatal(err)
}

config.Framer = customFramerConfig

engine, err := mrc.New(config)
```

Поля `FramerConfig` и `BigBytesConfig` используют беззнаковые числовые типы, поэтому отрицательные значения для них невозможны.

## 3. Engine

`Engine` — основной объект библиотеки. Он отвечает за управление сервером, хранение маршрутов, регистрацию middleware и взаимодействие с нативной реализацией.

Основные методы:

| Метод | Описание |
|---|---|
| `New(config)` | Создаёт сервер с готовой конфигурацией |
| `NewWithOptions(options...)` | Создаёт сервер через функциональные опции |
| `Start()` | Запускает сервер |
| `Run()` | Алиас `Start()` |
| `Stop()` | Останавливает нативный сервер |
| `Close()` | Освобождает ресурсы нативного сервера |
| `Handle(route, handlers...)` | Регистрирует маршрут |
| `Use(handlers...)` | Регистрирует глобальные middleware |
| `Group(prefix, handlers...)` | Создаёт группу маршрутов |

Маршруты необходимо регистрировать до запуска сервера. После вызова `Start()` регистрация новых маршрутов запрещается.

`Stop()` и `Close()` выполняют разные задачи: первый останавливает сервер, второй освобождает его ресурсы. В текущей реализации `Stop()` не сбрасывает внутренний флаг `started`, поэтому повторный запуск того же Engine не предусмотрен.

## 4. Маршрутизация

Маршрут регистрируется через `Handle()`:

```go
engine.Handle("/users", func(c *mrc.Context) {
	_ = c.JSON(mrc.StatusOK, mrc.H{
		"users": []string{"Alex", "Max"},
	})
})
```

В качестве обработчика используется `HandlerFunc`:

```go
type HandlerFunc func(*Context)
```

Один маршрут может содержать несколько обработчиков:

```go
engine.Handle(
	"/profile",
	authMiddleware,
	logMiddleware,
	profileHandler,
)
```

Обработчики выполняются последовательно. Для управления цепочкой используются `Next()` и `Abort()`.

Регистрация маршрута возвращает ошибку, если:

- маршрут пустой;
- список обработчиков пустой;
- маршрут уже зарегистрирован;
- сервер закрыт;
- сервер уже запущен.

Имена маршрутов хранятся в виде строк. В текущем API отсутствуют отдельные методы `GET`, `POST`, `PUT` и `DELETE`, а также параметризованные маршруты.

## 5. Группы маршрутов

`RouterGroup` позволяет объединять маршруты с общим префиксом и middleware.

```go
api := engine.Group("/api")

api.Handle("/status", func(c *mrc.Context) {
	_ = c.JSON(mrc.StatusOK, mrc.H{
		"status": "online",
	})
})
```

Вложенные группы наследуют middleware родительской группы:

```go
api := engine.Group("/api", authMiddleware)

v1 := api.Group("/v1")

v1.Handle("/users", usersHandler)
v1.Handle("/posts", postsHandler)
```

В данном примере `authMiddleware` будет применяться ко всем маршрутам группы `/api/v1`.

Добавить middleware к уже созданной группе можно через `Use()`:

```go
api.Use(logMiddleware)
```

Глобальные middleware регистрируются через `engine.Use()`, а локальные — через `Group.Use()`.

Порядок выполнения обработчиков:

1. Глобальные middleware Engine.
2. Middleware родительских групп.
3. Middleware вложенных групп.
4. Обработчики конкретного маршрута.

Пути групп и маршрутов объединяются через `path.Join`, поэтому лишние разделители и сегменты пути нормализуются.

Middleware следует регистрировать до маршрутов: при регистрации маршрута цепочка обработчиков копируется и передаётся нативному серверу.

## 6. Context

`Context` — основной объект, передаваемый каждому обработчику. Он объединяет информацию о запросе, управление цепочкой middleware, хранение временных данных и формирование ответа.

Пример:

```go
func profileHandler(c *mrc.Context) {
	userID := "123"

	c.Set("user_id", userID)

	value, exists := c.Get("user_id")
	if !exists {
		_ = c.JSON(mrc.StatusInternalError, mrc.H{
			"error": "User ID not found",
		})
		return
	}

	_ = c.JSON(mrc.StatusOK, mrc.H{
		"user_id": value,
	})
}
```

Основные методы:

| Метод | Назначение |
|---|---|
| `Next()` | Выполняет оставшуюся часть цепочки обработчиков |
| `Abort()` | Прерывает выполнение оставшейся цепочки |
| `IsAborted()` | Проверяет, была ли цепочка прервана |
| `Context()` | Возвращает стандартный `context.Context` запроса |
| `Set(key, value)` | Сохраняет данные в текущем Context |
| `Get(key)` | Получает сохранённые данные |
| `MustGet(key)` | Получает данные или вызывает panic, если ключ отсутствует |
| `Error(err)` | Сохраняет ошибку в контексте запроса |
| `Errors()` | Возвращает список сохранённых ошибок |
| `Written()` | Проверяет, был ли записан payload ответа |
| `ResponseStatus()` | Возвращает текущий статус ответа |
| `ResponseSize()` | Возвращает размер записанного payload |

### Next и Abort

Middleware может выполнить часть логики до и после остальных обработчиков:

```go
func timingMiddleware(c *mrc.Context) {
	start := time.Now()

	c.Next()

	log.Printf("Request processed in %s", time.Since(start))
}
```

`Next()` позволяет вложенно выполнить оставшуюся часть цепочки. Для полного прекращения её выполнения используется `Abort()`.

```go
func authMiddleware(c *mrc.Context) {
	if !isAuthorized(c) {
		_ = c.AbortWithStatusJSON(
			mrc.StatusUnauthorized,
			mrc.H{"error": "Unauthorized"},
		)
		return
	}

	c.Next()
}
```

Контекст предназначен для использования в пределах обработки текущего запроса. После завершения callback он становится недействительным. Его не следует сохранять для последующего использования в других goroutine.

## 7. Request

`Request` содержит данные полученного MMA-запроса.

```go
type Request struct {
	Opcode  cabi.RequestOpcode
	Version uint8
	ReqID   uint32

	Route   string
	Options []option
	Payload []byte
}
```

Основные поля:

| Поле | Назначение |
|---|---|
| `Opcode` | Тип операции протокола |
| `Version` | Версия протокола |
| `ReqID` | Идентификатор запроса |
| `Route` | Название маршрута |
| `Options` | Параметры запроса |
| `Payload` | Тело запроса в виде байтов |

Получить данные запроса можно через `c.Request`:

```go
func requestHandler(c *mrc.Context) {
	request := c.Request

	log.Printf(
		"Route: %s, ID: %d, Payload size: %d",
		request.Route,
		request.ReqID,
		len(request.Payload),
	)
}
```

Получение стандартного контекста:

```go
ctx := c.Request.Context()
```

Получение параметров протокола:

```go
value, exists := c.Request.GetOption("authorization")
if exists {
	log.Println(value)
}
```

Доступен также метод:

```go
valueBytes, exists := c.Request.GetOptionBytes("authorization")
```

`GetOption()` возвращает значение параметра в виде строки, а `GetOptionBytes()` — в виде `[]byte`.

## 8. Обработка JSON

Для декодирования JSON предусмотрены два метода.

### ShouldBindJSON

Декодирует payload в переданную структуру и возвращает ошибку при некорректном JSON или пустом payload.

```go
type CreateUserRequest struct {
	Name string `json:"name"`
	Age  int    `json:"age"`
}

func createUser(c *mrc.Context) {
	var request CreateUserRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		_ = c.JSON(mrc.StatusBadRequest, mrc.H{
			"error": "Invalid JSON",
		})
		return
	}

	_ = c.JSON(mrc.StatusOK, mrc.H{
		"name": request.Name,
		"age":  request.Age,
	})
}
```

### BindJSON

`BindJSON()` выполняет декодирование и автоматически формирует ответ при ошибке:

```go
func createUser(c *mrc.Context) {
	var request CreateUserRequest

	if err := c.BindJSON(&request); err != nil {
		return
	}

	_ = c.JSON(mrc.StatusOK, mrc.H{
		"created": true,
	})
}
```

При ошибке `BindJSON()`:

- сохраняет ошибку через `Context.Error()`;
- отправляет `StatusBadRequest`;
- записывает JSON `{"error":"Invalid JSON payload"}`;
- прерывает цепочку обработчиков;
- возвращает исходную ошибку.

Декодирование выполняется стандартным `encoding/json`. Дополнительная валидация полей структуры не выполняется.

## 9. Формирование ответов

Для формирования ответов используется `Context`.

### JSON

```go
c.JSON(mrc.StatusOK, mrc.H{
	"message": "Success",
})
```

`H` — сокращённое название типа `map[string]any`.

### String

Отправляет текстовый payload с типом `text/plain; charset=utf-8`.

```go
c.String(mrc.StatusOK, "Hello, %s!", "MMA")
```

### Data

Отправляет произвольные байты с указанным типом содержимого.

```go
c.Data(
	mrc.StatusOK,
	"application/octet-stream",
	[]byte{0x01, 0x02, 0x03},
)
```

### Status

Устанавливает статус ответа без записи payload:

```go
c.Status(mrc.StatusNotFound)
```

После этого можно записать payload отдельным вызовом `JSON()`, `String()` или `Data()`.

### Response options

Параметры ответа передаются через `AddResponseOption()`:

```go
c.AddResponseOption("content-type", "application/json")
```

При отправке JSON тип содержимого `application/json` добавляется автоматически. Для `String()` автоматически устанавливается `text/plain; charset=utf-8`.

Ключ и значение параметра не могут быть пустыми. Повторное добавление того же ключа с тем же значением допускается, а с другим значением возвращает ошибку.

### Прерывание цепочки с ответом

```go
c.AbortWithStatus(mrc.StatusForbidden)
```

Либо с JSON:

```go
c.AbortWithStatusJSON(
	mrc.StatusBadRequest,
	mrc.H{"error": "Invalid request"},
)
```

Записать payload ответа можно только один раз. Повторная запись возвращает `ErrResponseAlreadySent`.

Основные статусы:

| Константа | Назначение |
|---|---|
| `StatusOK` | Успешный ответ |
| `StatusBadRequest` | Некорректный запрос |
| `StatusUnauthorized` | Требуется авторизация |
| `StatusForbidden` | Доступ запрещён |
| `StatusNotFound` | Ресурс не найден |
| `StatusConflict` | Конфликт |
| `StatusInternalError` | Внутренняя ошибка |
| `StatusMessage` | Сообщение |
| `StatusChannelConnected` | Подключение канала |
| `StatusTimeout` | Таймаут |
| `StatusShutdown` | Завершение работы |

Статусы являются алиасами `cabi.ResponseOpcode`.

## 10. Middleware

Middleware — обработчики, выполняемые во время обработки запроса. Они позволяют реализовать авторизацию, логирование, проверку данных и другие общие операции.

Регистрация глобального middleware:

```go
engine.Use(
	middleware.Logger(),
	middleware.Recovery(),
)
```

Middleware имеет тот же тип, что и обычный обработчик:

```go
type HandlerFunc func(*Context)
```

### Logger

Пакет `middleware` содержит готовый Logger.

```go
engine.Use(middleware.Logger())
```

Логирование по умолчанию выводится в `stdout`.

Доступные варианты настройки:

```go
middleware.Logger()

middleware.LoggerWithFormatter(formatter)

middleware.LoggerWithWriter(writer, "/health")

middleware.LoggerWithConfig(middleware.LogConf{})
```

`LogConf` поддерживает следующие параметры:

| Поле | Назначение |
|---|---|
| `Formatter` | Пользовательский формат записи |
| `Output` | Поток вывода |
| `SkipRoutes` | Список маршрутов, исключённых из логирования |
| `Skip` | Функция проверки необходимости пропуска запроса |

В стандартную запись входят время обработки, статус, задержка, Opcode, ID запроса, маршрут, размер входящего и исходящего payload, количество options и сохранённые ошибки.

Управление ANSI-цветами:

```go
middleware.DisableConsoleColor()
middleware.ForceConsoleColor()
```

### Recovery

Recovery перехватывает panic во время выполнения middleware и обработчиков.

```go
engine.Use(middleware.Recovery())
```

При восстановлении после panic middleware:

- сохраняет информацию об ошибке в Context;
- записывает stack trace;
- прерывает оставшуюся цепочку;
- устанавливает `StatusInternalError`, если payload ответа ещё не был записан.

Для изменения поведения предусмотрены:

```go
middleware.Recovery()

middleware.CustomRecovery(handle)

middleware.RecoveryWithWriter(writer)

middleware.CustomRecoveryWithWriter(writer, handle)
```

Пример пользовательского обработчика:

```go
func recoveryHandler(c *mrc.Context, recovered any) {
	if c.Written() {
		return
	}

	_ = c.JSON(mrc.StatusInternalError, mrc.H{
		"error": "Internal server error",
	})
}
```

Подключение:

```go
engine.Use(
	middleware.Logger(),
	middleware.CustomRecovery(recoveryHandler),
)
```

По умолчанию Recovery записывает информацию о panic в `stderr`.

## 11. Ошибки

Библиотека предоставляет набор ошибок для обработки основных ситуаций.

| Ошибка | Причина |
|---|---|
| `ErrServerNotInitialized` | Нативный сервер не инициализирован |
| `ErrInvalidRoute` | Некорректная регистрация маршрута |
| `ErrRouteAlreadyExists` | Маршрут уже существует |
| `ErrServerStarted` | Попытка зарегистрировать маршрут после запуска |
| `ErrContextExpired` | Попытка использовать истёкший Context при записи ответа |
| `ErrResponseAlreadySent` | Повторная запись ответа |
| `ErrResponseUnavailable` | Недоступен механизм формирования ответа |
| `ErrEmptyPayload` | Пустой payload при декодировании JSON |

Ошибки C ABI также могут возвращаться вызывающим методам.

Для сохранения ошибок текущего запроса используется:

```go
c.Error(err)
```

Получение сохранённых ошибок:

```go
for _, err := range c.Errors() {
	log.Println(err)
}
```

Сам по себе вызов `Error()` не формирует ответ и не прерывает цепочку обработчиков.

## 12. Важные особенности текущей реализации

- Конфигурация передаётся нативному серверу во время создания Engine. Изменение экспортированных полей `engine.ServerConfig` после создания не перенастраивает уже созданный нативный сервер.
- Регистрация маршрутов и middleware рассчитана на этап настройки до запуска сервера. Методы `Use()` не обеспечивают синхронизацию конкурентных изменений.
- Цепочка middleware и обработчиков копируется при регистрации маршрута. Последующая регистрация middleware не изменяет уже зарегистрированные маршруты.
- `RequestTimeout` передаётся в конфигурацию C ABI, однако Go-контекст запроса в текущем адаптере создаётся через `context.WithCancel(context.Background())`. Он отменяется при завершении callback; дедлайн из `RequestTimeout` в этот контекст непосредственно не устанавливается.
- `Context` не предназначен для параллельного использования или хранения после завершения обработки запроса.
- `Status()` устанавливает статус, но не отмечает payload как записанный. Для проверки факта записи payload используется `Written()`.
- Маршрутизация выполняется по строковым именам маршрутов. Параметры URL, wildcard-маршруты и отдельная маршрутизация по Opcode в текущем API не предусмотрены.
- `Close()` освобождает нативные ресурсы и является завершающим этапом жизненного цикла Engine.

## 13. Полный пример с группами и middleware

```go
package main

import (
	"log"

	mrc "github.com/Edestuss0/MMA-Network-protocol"
	"github.com/Edestuss0/MMA-Network-protocol/middleware"
)

func main() {
	engine, err := mrc.NewWithOptions(
		mrc.WithBindIP("0.0.0.0"),
		mrc.WithPort(9000),
		mrc.WithMaxInFlight(1000),
	)
	if err != nil {
		log.Fatal(err)
	}

	defer engine.Close()

	// Global middleware.
	engine.Use(
		middleware.Logger(),
		middleware.Recovery(),
	)

	// Public routes.
	if err := engine.Handle("/health", func(c *mrc.Context) {
		_ = c.JSON(mrc.StatusOK, mrc.H{
			"status": "online",
		})
	}); err != nil {
		log.Fatal(err)
	}

	// API group.
	api := engine.Group("/api")

	if err := api.Handle("/users", func(c *mrc.Context) {
		_ = c.JSON(mrc.StatusOK, mrc.H{
			"users": []string{"Alex", "Max"},
		})
	}); err != nil {
		log.Fatal(err)
	}

	// Start server.
	if err := engine.Run(); err != nil {
		log.Fatal(err)
	}
}
```

Этот пример объединяет основные возможности MRC: функциональную конфигурацию, глобальные middleware, регистрацию отдельных маршрутов, группы маршрутов, формирование JSON-ответов и запуск нативного сервера.