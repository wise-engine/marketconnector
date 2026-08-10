// Package ws implements the Angel One SmartAPI WebSocket v2 ticker used for
// real-time market data streaming.
package ws

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sunnyme20/marketconnector/model"
)

// errorResponse represents a WebSocket error from Angel One.
type errorResponse struct {
	CorrelationID string `json:"correlationID"`
	ErrorCode     string `json:"errorCode"`
	ErrorMessage  string `json:"errorMessage"`
}

// WebSocket constants.
const (
	// heartbeatInterval is how often a "ping" is sent to keep the connection
	// alive even when the market is closed (no ticks).
	heartbeatInterval = 10 * time.Second

	// Auto-reconnect defaults.
	defaultReconnectMaxAttempts = 9999 // effectively unlimited
	reconnectMinDelay           = 5 * time.Second
	defaultReconnectMaxDelay    = 60 * time.Second
	defaultConnectTimeout       = 7 * time.Second
	connectionCheckInterval     = 15 * time.Second
	// dataTimeoutInterval only kills an idle connection after 5 minutes. This
	// prevents premature disconnection during market closure or low-activity
	// periods, as heartbeats keep the connection alive anyway.
	dataTimeoutInterval = 5 * time.Minute

	// WebSocket defaults.
	defaultWSScheme = "wss"
	defaultWSHost   = "smartapisocket.angelone.in"
	defaultWSPath   = "/smart-stream"
)

// Ticker is an Angel One SmartAPI WebSocket v2 ticker instance. Obtain one via
// NewWebSocket or marketconnector's GetWebSocket, and run it with Serve.
type Ticker struct {
	conn *websocket.Conn

	apiKey      string
	clientCode  string
	accessToken string
	feedToken   string

	url                 url.URL
	callbacks           callbacks
	lastPingTime        atomicTime
	autoReconnect       bool
	reconnectMaxRetries int
	reconnectMaxDelay   time.Duration
	connectTimeout      time.Duration

	reconnectAttempt int

	subscribedTokens []model.WSTokenGroup
	subscribedMode   int

	cancel context.CancelFunc
}

// atomicTime safely wraps a time.Time for concurrent access.
type atomicTime struct {
	v atomic.Value
}

// Get returns the current timestamp.
func (a *atomicTime) Get() time.Time {
	return a.v.Load().(time.Time)
}

// Set stores the current timestamp.
func (a *atomicTime) Set(value time.Time) {
	a.v.Store(value)
}

// callbacks holds the callbacks a user can register on the ticker.
type callbacks struct {
	onTick        func(model.MarketQuoteResponse)
	onMessage     func(int, []byte)
	onNoReconnect func(int)
	onReconnect   func(int, time.Duration)
	onConnect     func()
	onClose       func(int, string)
	onError       func(error)
}

// NewWebSocket creates a new Angel One WebSocket ticker instance.
func NewWebSocket(apiKey, clientCode, accessToken, feedToken string) *Ticker {
	return &Ticker{
		apiKey:              apiKey,
		clientCode:          clientCode,
		accessToken:         accessToken,
		feedToken:           feedToken,
		url:                 url.URL{Scheme: defaultWSScheme, Host: defaultWSHost, Path: defaultWSPath},
		autoReconnect:       true,
		reconnectMaxDelay:   defaultReconnectMaxDelay,
		reconnectMaxRetries: defaultReconnectMaxAttempts,
		connectTimeout:      defaultConnectTimeout,
	}
}

// SetRootURL sets a custom WebSocket URL.
func (t *Ticker) SetRootURL(u url.URL) {
	t.url = u
}

// SetAccessToken sets the JWT access token.
func (t *Ticker) SetAccessToken(token string) {
	t.accessToken = token
}

// SetFeedToken sets the feed token.
func (t *Ticker) SetFeedToken(token string) {
	t.feedToken = token
}

// SetClientCode sets the client code (trading account id).
func (t *Ticker) SetClientCode(code string) {
	t.clientCode = code
}

// SetConnectTimeout sets the timeout for the initial connect handshake.
func (t *Ticker) SetConnectTimeout(val time.Duration) {
	t.connectTimeout = val
}

// SetAutoReconnect enables or disables automatic reconnection.
func (t *Ticker) SetAutoReconnect(val bool) {
	t.autoReconnect = val
}

// SetReconnectMaxDelay sets the maximum auto-reconnect delay.
func (t *Ticker) SetReconnectMaxDelay(val time.Duration) error {
	if val < reconnectMinDelay {
		return fmt.Errorf("reconnect max delay can't be less than %v", reconnectMinDelay)
	}
	t.reconnectMaxDelay = val
	return nil
}

// SetReconnectMaxRetries sets the maximum reconnect attempts.
func (t *Ticker) SetReconnectMaxRetries(val int) {
	t.reconnectMaxRetries = val
}

// OnConnect registers a callback invoked when the connection is established.
func (t *Ticker) OnConnect(f func()) {
	t.callbacks.onConnect = f
}

// OnError registers a callback invoked when an error occurs.
func (t *Ticker) OnError(f func(err error)) {
	t.callbacks.onError = f
}

// OnClose registers a callback invoked when the connection closes.
func (t *Ticker) OnClose(f func(code int, reason string)) {
	t.callbacks.onClose = f
}

// OnMessage registers a callback invoked for every raw WebSocket message.
func (t *Ticker) OnMessage(f func(messageType int, message []byte)) {
	t.callbacks.onMessage = f
}

// OnReconnect registers a callback invoked before each reconnect attempt.
func (t *Ticker) OnReconnect(f func(attempt int, delay time.Duration)) {
	t.callbacks.onReconnect = f
}

// OnNoReconnect registers a callback invoked when reconnection is abandoned.
func (t *Ticker) OnNoReconnect(f func(attempt int)) {
	t.callbacks.onNoReconnect = f
}

// OnTick registers a callback invoked for every parsed market tick.
func (t *Ticker) OnTick(f func(tick model.MarketQuoteResponse)) {
	t.callbacks.onTick = f
}

// Serve starts the connection to the ticker server. It blocks, so run it in a
// goroutine.
func (t *Ticker) Serve() {
	t.ServeWithContext(context.Background())
}

// ServeWithContext starts the connection with a cancellable context.
func (t *Ticker) ServeWithContext(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	t.cancel = cancel

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if t.reconnectAttempt > t.reconnectMaxRetries {
			t.triggerNoReconnect(t.reconnectAttempt)
			return
		}

		if t.reconnectAttempt > 0 {
			nextDelay := time.Duration(math.Pow(2, float64(t.reconnectAttempt))) * time.Second
			if nextDelay > t.reconnectMaxDelay || nextDelay <= 0 {
				nextDelay = t.reconnectMaxDelay
			}
			t.triggerReconnect(t.reconnectAttempt, nextDelay)
			time.Sleep(nextDelay)
			if t.conn != nil {
				_ = t.conn.Close()
			}
		}

		// Browser-based clients can authenticate via query params instead of
		// headers.
		q := t.url.Query()
		q.Set("clientCode", t.clientCode)
		q.Set("feedToken", t.feedToken)
		q.Set("apiKey", t.apiKey)
		t.url.RawQuery = q.Encode()

		d := websocket.DefaultDialer
		d.HandshakeTimeout = t.connectTimeout

		// Prefer header-based authentication for non-browser clients.
		header := http.Header{}
		header.Set("Authorization", "Bearer "+t.accessToken)
		header.Set("x-api-key", t.apiKey)
		header.Set("x-client-code", t.clientCode)
		header.Set("x-feed-token", t.feedToken)

		conn, _, err := d.Dial(t.url.String(), header)
		if err != nil {
			t.triggerError(fmt.Errorf("dial error: %w", err))
			if t.autoReconnect {
				t.reconnectAttempt++
				continue
			}
			return
		}
		t.conn = conn

		t.triggerConnect()

		// Resubscribe to stored tokens after a reconnect.
		if t.reconnectAttempt > 0 {
			if err := t.resubscribe(); err != nil {
				t.triggerError(fmt.Errorf("resubscribe error: %w", err))
			}
		}

		t.reconnectAttempt = 0
		t.lastPingTime.Set(time.Now())

		var wg sync.WaitGroup
		wg.Add(1)
		go t.readMessage(ctx, &wg)

		wg.Add(1)
		go t.heartbeatLoop(ctx, &wg)

		if t.autoReconnect {
			wg.Add(1)
			go t.checkConnection(ctx, &wg)
		}

		wg.Wait()
		_ = conn.Close()
	}
}

// heartbeatLoop sends periodic pings to keep the connection alive.
func (t *Ticker) heartbeatLoop(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if t.conn != nil {
				if err := t.conn.WriteMessage(websocket.TextMessage, []byte("ping")); err != nil {
					t.triggerError(fmt.Errorf("heartbeat write error: %w", err))
					return
				}
			}
		}
	}
}

// checkConnection closes the connection if no data has arrived for
// dataTimeoutInterval, forcing a reconnect.
func (t *Ticker) checkConnection(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		time.Sleep(connectionCheckInterval)
		if time.Since(t.lastPingTime.Get()) > dataTimeoutInterval {
			if t.conn != nil {
				_ = t.conn.Close()
			}
			t.reconnectAttempt++
			return
		}
	}
}

// readMessage reads messages from the connection until it closes or ctx is
// cancelled, dispatching ticks and text messages to the registered callbacks.
func (t *Ticker) readMessage(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		msgType, msg, err := t.conn.ReadMessage()
		if err != nil {
			t.triggerError(fmt.Errorf("read error: %w", err))
			return
		}

		t.lastPingTime.Set(time.Now())
		t.triggerMessage(msgType, msg)

		switch msgType {
		case websocket.TextMessage:
			t.handleTextMessage(msg)
		case websocket.BinaryMessage:
			tick, err := parseTick(msg)
			if err != nil {
				t.triggerError(fmt.Errorf("parse error: %w", err))
				continue
			}
			t.triggerTick(tick)
		}
	}
}

// handleTextMessage processes a text message, surfacing server errors.
func (t *Ticker) handleTextMessage(msg []byte) {
	text := string(msg)
	if text == "pong" {
		return
	}

	var errResp errorResponse
	if err := json.Unmarshal(msg, &errResp); err == nil && errResp.ErrorCode != "" {
		t.triggerError(fmt.Errorf("server error [%s]: %s", errResp.ErrorCode, errResp.ErrorMessage))
	}
}

// Subscribe subscribes to tokens with the given mode. If called before the
// connection is established, the subscription is stored and applied
// automatically on connect.
func (t *Ticker) Subscribe(mode int, tokenList []model.WSTokenGroup) error {
	t.subscribedTokens = tokenList
	t.subscribedMode = mode

	if t.conn == nil {
		return nil
	}

	req := model.WSSubscribeRequest{
		Action: 1,
		Params: model.WSRequestParams{
			Mode:      mode,
			TokenList: tokenList,
		},
	}
	data, err := json.Marshal(req)
	if err != nil {
		return err
	}
	return t.conn.WriteMessage(websocket.TextMessage, data)
}

// Unsubscribe unsubscribes from the given tokens.
func (t *Ticker) Unsubscribe(tokenList []model.WSTokenGroup) error {
	if t.conn == nil {
		return nil
	}
	req := model.WSSubscribeRequest{
		Action: 0,
		Params: model.WSRequestParams{
			Mode:      t.subscribedMode,
			TokenList: tokenList,
		},
	}
	data, err := json.Marshal(req)
	if err != nil {
		return err
	}
	return t.conn.WriteMessage(websocket.TextMessage, data)
}

// resubscribe re-applies the previously stored subscription after a reconnect.
func (t *Ticker) resubscribe() error {
	if len(t.subscribedTokens) == 0 {
		return nil
	}
	return t.Subscribe(t.subscribedMode, t.subscribedTokens)
}

// Close attempts a graceful close of the connection.
func (t *Ticker) Close() error {
	if t.conn == nil {
		return nil
	}
	return t.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
}

// Stop cancels the connection context and stops all goroutines.
func (t *Ticker) Stop() {
	if t.cancel != nil {
		t.cancel()
	}
}

func (t *Ticker) triggerError(err error) {
	if t.callbacks.onError != nil {
		t.callbacks.onError(err)
	}
}

func (t *Ticker) triggerConnect() {
	if t.callbacks.onConnect != nil {
		t.callbacks.onConnect()
	}
}

func (t *Ticker) triggerReconnect(attempt int, delay time.Duration) {
	if t.callbacks.onReconnect != nil {
		t.callbacks.onReconnect(attempt, delay)
	}
}

func (t *Ticker) triggerNoReconnect(attempt int) {
	if t.callbacks.onNoReconnect != nil {
		t.callbacks.onNoReconnect(attempt)
	}
}

func (t *Ticker) triggerMessage(messageType int, message []byte) {
	if t.callbacks.onMessage != nil {
		t.callbacks.onMessage(messageType, message)
	}
}

func (t *Ticker) triggerTick(tick model.MarketQuoteResponse) {
	if t.callbacks.onTick != nil {
		t.callbacks.onTick(tick)
	}
}
