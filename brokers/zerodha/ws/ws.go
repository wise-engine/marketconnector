// Package ws implements the Zerodha Kite Connect WebSocket ticker used for
// real-time market data streaming. It wraps the gokiteconnect ticker
// (package kiteticker) and adapts it to the broker-agnostic
// model.WebSocketTicker interface.
package ws

import (
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/sunnyme20/marketconnector/model"
	"github.com/zerodha/gokiteconnect/v4/models"
	kiteticker "github.com/zerodha/gokiteconnect/v4/ticker"
)

// Ticker is a Zerodha Kite Connect WebSocket ticker instance. Obtain one via
// NewWebSocket or marketconnector's GetWebSocket, and run it with Serve.
type Ticker struct {
	apiKey      string
	accessToken string
	ticker      *kiteticker.Ticker

	mu               sync.Mutex
	subscribedMode   int
	subscribedTokens []model.WSTokenGroup

	callbacks callbacks
}

// callbacks holds the callbacks a user can register on the ticker.
type callbacks struct {
	onConnect   func()
	onTick      func(model.MarketQuoteResponse)
	onError     func(error)
	onReconnect func(int, time.Duration)
}

// NewWebSocket creates a new Zerodha WebSocket ticker instance backed by the
// gokiteconnect ticker.
func NewWebSocket(apiKey, accessToken string) *Ticker {
	t := &Ticker{
		apiKey:      apiKey,
		accessToken: accessToken,
		ticker:      kiteticker.New(apiKey, accessToken),
	}

	// Bridge the SDK callbacks onto the common interface.
	t.ticker.OnConnect(func() {
		// Re-apply any subscription made before the connection was live.
		_ = t.applySubscription()
		if t.callbacks.onConnect != nil {
			t.callbacks.onConnect()
		}
	})
	t.ticker.OnTick(func(tick models.Tick) {
		if t.callbacks.onTick != nil {
			t.callbacks.onTick(convertTick(tick))
		}
	})
	t.ticker.OnError(func(err error) {
		if t.callbacks.onError != nil {
			t.callbacks.onError(err)
		}
	})
	t.ticker.OnReconnect(func(attempt int, delay time.Duration) {
		if t.callbacks.onReconnect != nil {
			t.callbacks.onReconnect(attempt, delay)
		}
	})

	return t
}

// Serve starts the connection to the ticker server. It blocks, so run it in a
// goroutine.
func (t *Ticker) Serve() {
	t.ticker.Serve()
}

// Stop stops the ticker and all the goroutines it has spawned.
func (t *Ticker) Stop() {
	t.ticker.Stop()
}

// Subscribe subscribes to tokens with the given mode. If called before the
// connection is established, the subscription is stored and applied
// automatically on connect.
//
// Kite Connect identifies instruments by numeric instrument token (the segment
// is encoded in the token itself), so tokens from every WSTokenGroup are
// flattened and subscribed together.
func (t *Ticker) Subscribe(mode int, tokenList []model.WSTokenGroup) error {
	t.mu.Lock()
	t.subscribedMode = mode
	t.subscribedTokens = tokenList
	t.mu.Unlock()

	return t.applySubscription()
}

// applySubscription sends the stored subscription to the underlying ticker if
// the connection is live, otherwise stores it for the next connect.
func (t *Ticker) applySubscription() error {
	t.mu.Lock()
	mode := t.subscribedMode
	tokens := t.subscribedTokens
	t.mu.Unlock()

	if t.ticker.Conn == nil {
		return nil
	}

	flat, err := flattenTokens(tokens)
	if err != nil {
		return err
	}
	if len(flat) == 0 {
		return nil
	}

	// Subscribe first, then set the mode (mirrors the SDK's Resubscribe).
	if err := t.ticker.Subscribe(flat); err != nil {
		return err
	}
	return t.ticker.SetMode(mapMode(mode), flat)
}

// OnConnect registers a callback invoked when the connection is established.
func (t *Ticker) OnConnect(f func()) {
	t.callbacks.onConnect = f
}

// OnTick registers a callback invoked for every parsed market tick.
func (t *Ticker) OnTick(f func(tick model.MarketQuoteResponse)) {
	t.callbacks.onTick = f
}

// OnError registers a callback invoked when an error occurs.
func (t *Ticker) OnError(f func(err error)) {
	t.callbacks.onError = f
}

// OnReconnect registers a callback invoked before each reconnect attempt.
func (t *Ticker) OnReconnect(f func(attempt int, delay time.Duration)) {
	t.callbacks.onReconnect = f
}

// mapMode converts the broker-agnostic subscription mode to the Kite Connect
// ticker mode.
func mapMode(mode int) kiteticker.Mode {
	switch mode {
	case int(model.ModeLTP):
		return kiteticker.ModeLTP
	case int(model.ModeQuote):
		return kiteticker.ModeQuote
	default: // ModeSnapQuote maps to the richest "full" feed.
		return kiteticker.ModeFull
	}
}

// flattenTokens converts WSTokenGroups into a de-duplicated list of uint32
// instrument tokens.
func flattenTokens(groups []model.WSTokenGroup) ([]uint32, error) {
	seen := make(map[uint32]struct{})
	var out []uint32
	for _, g := range groups {
		for _, token := range g.Tokens {
			t, err := strconv.ParseUint(token, 10, 32)
			if err != nil {
				return nil, fmt.Errorf("invalid instrument token %q: %w", token, err)
			}
			u := uint32(t)
			if _, ok := seen[u]; ok {
				continue
			}
			seen[u] = struct{}{}
			out = append(out, u)
		}
	}
	return out, nil
}
