package model

import "time"

// Timeframe represents a broker-agnostic interval for historical data.
type Timeframe string

const (
	Timeframe1Minute   Timeframe = "1m"
	Timeframe3Minutes  Timeframe = "3m"
	Timeframe5Minutes  Timeframe = "5m"
	Timeframe10Minutes Timeframe = "10m"
	Timeframe15Minutes Timeframe = "15m"
	Timeframe30Minutes Timeframe = "30m"
	Timeframe1Hour     Timeframe = "1h"
	Timeframe1Day      Timeframe = "1d"
)

// Exchange
type Exchange string

const (
	ExchangeNSE Exchange = "NSE"
	ExchangeBSE Exchange = "BSE"
	ExchangeNFO Exchange = "NFO"
	ExchangeBFO Exchange = "BFO"
	ExchangeMCX Exchange = "MCX"
)

// QuoteMode represents the market quote mode.
type QuoteMode string

const (
	QuoteModeLTP  QuoteMode = "LTP"
	QuoteModeOHLC QuoteMode = "OHLC"
	QuoteModeFull QuoteMode = "FULL"
)

// --------------- WebSocket common types -----------------

// SubscriptionMode represents the WebSocket subscription type.
type SubscriptionMode int

const (
	ModeLTP       SubscriptionMode = 1
	ModeQuote     SubscriptionMode = 2
	ModeSnapQuote SubscriptionMode = 3
)

// WSExchangeType represents the exchange identifier used in WebSocket subscriptions.
type WSExchangeType int

const (
	WSExchangeNseCM WSExchangeType = 1
	WSExchangeNseFO WSExchangeType = 2
	WSExchangeBseCM WSExchangeType = 3
	WSExchangeBseFO WSExchangeType = 4
	WSExchangeMcxFO WSExchangeType = 5
)

// WSTokenGroup groups tokens by exchange type for WebSocket subscription.
type WSTokenGroup struct {
	ExchangeType int      `json:"exchangeType"`
	Tokens       []string `json:"tokens"`
}

// WSRequestParams holds the WebSocket subscription parameters.
type WSRequestParams struct {
	Mode      int            `json:"mode"`
	TokenList []WSTokenGroup `json:"tokenList"`
}

// WSSubscribeRequest is the JSON payload sent to subscribe/unsubscribe via WebSocket.
type WSSubscribeRequest struct {
	CorrelationID string          `json:"correlationID,omitempty"`
	Action        int             `json:"action"` // 1=subscribe, 0=unsubscribe
	Params        WSRequestParams `json:"params"`
}

// WebSocketTicker defines the common interface for broker WebSocket tickers.
type WebSocketTicker interface {
	Serve()
	Stop()
	Subscribe(mode int, tokenList []WSTokenGroup) error
	OnConnect(f func())
	OnTick(f func(MarketQuoteResponse))
	OnError(f func(err error))
	OnReconnect(f func(attempt int, delay time.Duration))
}
