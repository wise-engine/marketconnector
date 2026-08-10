# MarketConnector

A **broker-agnostic** Go library for connecting to Indian stock brokers (Angel One, Zerodha, Upstox, etc.). Provides a unified interface for authentication, market data, portfolio management, and WebSocket streaming — write your application logic once, switch brokers with a single config change.

```go
broker, _ := marketconnector.NewBroker("angelone")
resp, _ := broker.NewSession("clientcode", "apikey", "password", "totp")
holdings, _ := broker.GetHoldings()
```

---

## Features

- **Unified Broker Interface** — Common API for login, profile, holdings, positions, quotes, historical data, and WebSocket streaming.
- **Pluggable Architecture** — Add new brokers by implementing the `Broker` interface. No changes needed in your application code.
- **WebSocket Support** — Real-time market data with automatic reconnection and exponential backoff.
- **Generic Response Types** — Type-safe `Response[T]` wrapper for consistent error handling across all brokers.
- **Go 1.26+ Generics** — Modern Go patterns throughout.

---

## Supported Brokers

| Broker | Status | Features |
|--------|--------|----------|
| Angel One | ✅ Complete | Login, Profile, Holdings, Positions, Quotes, Historical (candles + OI), WebSocket v2 (SmartAPI), RMS |
| Zerodha | ❌ Planned | — |
| Upstox | ❌ Planned | — |
| 5Paisa | ❌ Planned | — |
| ICICI Direct | ❌ Planned | — |

---

## Installation

```bash
go get github.com/sunnyme20/marketconnector
```

---

## Quick Start

### 1. Initialize a Broker

```go
package main

import (
    "fmt"
    "log"

    "github.com/sunnyme20/marketconnector"
    "github.com/sunnyme20/marketconnector/model"
)

func main() {
    broker, err := marketconnector.NewBroker("angelone")
    if err != nil {
        log.Fatal(err)
    }

    // Login
    resp, err := broker.NewSession("clientcode", "apikey", "password", "totp")
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println("Logged in:", resp.Data.AccessToken)
}
```

### 2. Fetch Data

```go
// Holdings
holdings, _ := broker.GetHoldings()
for _, h := range holdings.Data {
    fmt.Printf("%s: %.2f\n", h.TradingSymbol, h.Investment)
}

// Positions
positions, _ := broker.GetPositions()

// Market Quotes
quotes, _ := broker.GetMarketQuote(model.QuoteModeFull, map[model.Exchange][]string{
    model.ExchangeNSE: {"2885", "1394"},
})

// Historical Data
hist, _ := broker.GetHistoricalData(
    model.ExchangeNSE, "2885",
    model.Timeframe1Day,
    "2026-06-01 09:00", "2026-07-01 15:30",
)
```

### 3. WebSocket Streaming

```go
ws, _ := broker.GetWebSocket()

ws.OnConnect(func() {
    fmt.Println("Connected!")
    ws.Subscribe(int(model.ModeLTP), []model.WSTokenGroup{
        {ExchangeType: int(model.WSExchangeNseCM), Tokens: []string{"2885", "1394"}},
    })
})

ws.OnTick(func(tick model.MarketQuoteResponse) {
    fmt.Printf("%s: %.2f\n", tick.TradingSymbol, tick.LTP)
})

ws.OnError(func(err error) {
    log.Println("WS Error:", err)
})

go ws.Serve()
time.Sleep(30 * time.Second)
ws.Stop()
```

---

## Project Structure

```
marketconnector/
├── marketconnector.go            # Broker interface + package docs
├── factory.go                    # NewBroker(), Register(), broker registry
├── model/                        # Broker-agnostic shared types
│   ├── common.go                 #   Timeframe, Exchange, QuoteMode, WebSocketTicker
│   └── response.go               #   Response[T], Holdings, Positions, Quotes, etc.
├── brokers/
│   └── angelone/                 # Angel One implementation
│       ├── angelone.go           #   Angelone struct + token accessors
│       ├── session.go            #   Login, profile, RMS
│       ├── holdings.go           #   GetHoldings()
│       ├── positions.go          #   GetPositions()
│       ├── quote.go              #   GetMarketQuote()
│       ├── historical.go         #   GetHistoricalData() + retry/rate-limit
│       ├── historical_batch.go   #   Batch splitting + worker pool
│       ├── ws/                   #   WebSocket ticker (package ws)
│       │   ├── ws.go             #     Ticker lifecycle + streaming
│       │   └── parse.go          #     Binary packet parsing
│       └── internal/             #   Internal implementation (not public API)
│           ├── client/           #     HTTP client wrapper
│           ├── endpoints/        #     API endpoint URLs
│           ├── wire/             #     Wire types + mapping functions
│           └── util/             #     Shared helpers
└── cmd/
    └── marketconnector/
        └── main.go               # Demo CLI (login + data + optional WebSocket)
```

---

## Architecture & Patterns

### Broker Interface

Every broker must implement `marketconnector.Broker` (`marketconnector.go:12`):

```go
type Broker interface {
    // Session
    NewSession(clientcode, apikey, password, totp string) (*model.Response[model.LoginResponse], error)
    SetAccessToken(token string)
    SetFeedToken(token string)
    SetClientCode(code string)
    SetAPIKey(key string)
    GetAccessToken() string

    // Account
    GetUserProfile() (*model.Response[model.UserProfileResponse], error)
    GetRMSData() (*model.Response[model.FundsResponse], error)
    Logout() error

    // Portfolio
    GetHoldings() (*model.Response[[]model.HoldingResponse], error)
    GetPositions() (*model.Response[[]model.PositionResponse], error)

    // Market Data
    GetMarketQuote(mode model.QuoteMode, exchangeTokens map[model.Exchange][]string) (*model.Response[[]model.MarketQuoteResponse], error)
    GetHistoricalData(exchange model.Exchange, symbolToken string, interval model.Timeframe, fromDate, toDate string) (*model.Response[model.HistoricalResponse], error)
    FetchHistoricalDataBatch(requests []model.HistoricalBatchRequest) (*model.Response[[]model.HistoricalBatchItem], error)

    // WebSocket
    GetWebSocket() (model.WebSocketTicker, error)

    // Orders (planned)
    PlaceOrder(...)
    ModifyOrder(...)
    CancelOrder(...)
}
```

### Response Wrapper

All API responses use the generic wrapper (`model/response.go:3`):

```go
type Response[T any] struct {
    Success bool   `json:"success"`
    Message string `json:"message"`
    Broker  string `json:"broker"`
    Data    T      `json:"data"`
}
```

### WebSocket Ticker Interface

Real-time data consumers implement `model.WebSocketTicker` (`model/common.go:81`):

```go
type WebSocketTicker interface {
    Serve()
    Stop()
    Subscribe(mode int, tokenList []WSTokenGroup) error
    OnConnect(fn func())
    OnTick(fn func(MarketQuoteResponse))
    OnError(fn func(error))
    OnReconnect(fn func(attempt int, delay time.Duration))
    // ...
}
```

### Per-Broker Package Structure

Each broker lives in its own package under `brokers/<brokername>/`:

| File | Responsibility |
|------|---------------|
| `angelone.go` | Main broker struct + token accessors |
| `session.go` | Session/profile/RMS implementations |
| `holdings.go` | `GetHoldings()` implementation |
| `positions.go` | `GetPositions()` implementation |
| `quote.go` | `GetMarketQuote()` implementation |
| `historical.go` | `GetHistoricalData()` implementation (retry + rate-limit) |
| `historical_batch.go` | Date-range splitting + concurrent worker pool |
| `ws/` | WebSocket `Ticker` (lifecycle + binary packet parsing) |
| `internal/client/` | HTTP client wrapper (headers, auth, base URL) |
| `internal/endpoints/` | All API endpoint URLs as constants |
| `internal/wire/` | Broker-specific wire types + mapping functions |
| `internal/util/` | Shared helper functions |

Internal packages can only be imported by the `angelone` package itself — they are
not part of the public API and can change freely.

---

## Coding Conventions

### Naming

| Convention | Example |
|------------|---------|
| Exported = PascalCase | `NewSession`, `GetMarketQuote` |
| Unexported = camelCase | `getLocalIP`, `parseTick` |
| Acronyms uppercase | `APIKey`, `AccessToken`, `FeedToken`, `LTP`, `RMS` |
| Json tags snake_case | `trading_symbol`, `symbol_token` |

### Error Handling

- Always return `(result, error)` — never swallow errors.
- Use `fmt.Errorf("...: %w", err)` to wrap errors with context.
- Parse failures should be returned, not silently zeroed.

```go
// Good
func (a *Angelone) GetHoldings() (*model.Response[[]model.HoldingResponse], error) {
    var raw holdings
    if err := a.httpClient().Get(api.Holding, nil, &raw); err != nil {
        return nil, fmt.Errorf("holdings: %w", err)
    }
    // ...
}

// Avoid — silent parse failures
parseFloat64(s string) float64 { n, _ := strconv.ParseFloat(s, 64); return n }
```

### Imports

- Import the `model` package without an alias: `"github.com/sunnyme20/marketconnector/model"`.
- Group standard library, external, and internal imports with blank lines.

### Struct Embedding

- Broker-specific response types embed a common response struct for status/message/error.
- Use Go generics (`Response[T]`) for the public-facing API.
- Keep broker-internal wire types in the broker package; map to `model.*` types at the boundary.

### Testing

- Every broker implementation file should have a corresponding `*_test.go` file.
- Use table-driven tests for mapping/conversion functions.
- Use `httptest.Server` for testing HTTP clients.
- Use interface mocks for testing business logic without live API calls.

---

## How to Add a New Broker

1. **Create the package**
   ```
   brokers/<brokername>/
   ```

2. **Implement the files** following the per-broker structure above.

3. **Define broker-specific wire types** in `model.go` — request/response structs with `json` tags.

4. **Implement mapping functions** in `model.go` to convert:
   - Your broker's timeframe strings → `model.Timeframe`
   - Your broker's exchange codes → `model.Exchange`
   - Your broker's response structs → `model.Response[T]`

5. **Implement the `Broker` interface** on your main struct (e.g. in your broker's root package).

6. **Register the broker** in `factory.go`:
   ```go
   func init() {
       Register("angelone", func() Broker { return &angelone.Angelone{} })
       Register("zerodha", func() Broker { return &zerodha.Zerodha{} }) // add this line
   }
   ```
   External brokers can also be registered at runtime by consumers:
   ```go
   marketconnector.Register("mybroker", func() marketconnector.Broker { return &mybroker.Client{} })
   ```

7. **Write tests** — unit tests for mapping functions, integration test structure (credentials outside repo).

8. **Create a PR** 🚀

---

## Contributing

We welcome contributions! Here's how to get started:

### Development Setup

1. Fork the repository.
2. Clone your fork:
   ```bash
   git clone https://github.com/<your-username>/marketconnector.git
   ```
3. Install Go 1.26+.
4. Run the tests:
   ```bash
   go test ./...
   ```

### What to Work On

| Area | How to Help |
|------|-------------|
| **New Brokers** | Implement Zerodha, Upstox, 5Paisa, etc. |
| **Tests** | Add unit tests for existing AngelOne code (especially `parseTick` in `ws/parse.go`) |
| **Order APIs** | Add `PlaceOrder`, `ModifyOrder`, `CancelOrder` to the interface |
| **Bug Fixes** | Check [issues](https://github.com/sunnyme20/marketconnector/issues) |

### Pull Request Process

1. Create a feature branch from `main`:
   ```bash
   git checkout -b feat/zerodha-support
   ```
2. Write tests for your changes.
3. Ensure all tests pass:
   ```bash
   go test -v -race ./...
   ```
4. Run `gofmt` / `go vet`:
   ```bash
   gofmt -l . && go vet ./...
   ```
5. Open a PR with a clear title and description.

### Guidelines

- **No hardcoded credentials** — ever. Use environment variables or config files in examples.
- **Match the existing patterns** — file structure, error handling, naming conventions.
- **One feature per PR** — keep changes focused.
- **Document public API** — exported types and functions should have godoc comments.
- **No breaking changes** to the `Broker` interface without discussion.

---

## License

MIT — see [LICENSE](LICENSE).

---

## Disclaimer

This library is **not officially affiliated** with any broker. Use at your own risk. Market data and trading involve financial risk. Verify all data with your broker before making trading decisions.
