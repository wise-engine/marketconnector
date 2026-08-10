// Package marketconnector provides a broker-agnostic client for Indian stock
// brokers (Angel One, Zerodha, Upstox, etc.).
//
// A broker is selected by name through [NewBroker]. Each broker implements the
// [Broker] interface, mapping its broker-specific API onto the common types in
// the [model] package, so application code can switch brokers without changing.
//
// Additional brokers can be plugged in at runtime with [Register]:
//
//	marketconnector.Register("zerodha", func() marketconnector.Broker {
//		return &zerodha.Client{}
//	})
package marketconnector

import "github.com/sunnyme20/marketconnector/model"

// Broker is the common interface implemented by every supported broker.
// It covers authentication, portfolio data, market data, historical data and
// real-time WebSocket streaming.
type Broker interface {
	NewSession(clientcode, apikey, password, totp string) (*model.Response[model.LoginResponse], error)
	SetAccessToken(accessToken string)
	SetFeedToken(feedToken string)
	SetClientCode(clientcode string)
	SetAPIKey(apikey string)
	GetAccessToken() string
	GetUserProfile() (*model.Response[model.UserProfileResponse], error)
	GetRMSData() (*model.Response[model.FundsResponse], error)
	GetHoldings() (*model.Response[[]model.HoldingResponse], error)
	GetPositions() (*model.Response[[]model.PositionResponse], error)
	GetHistoricalData(exchange model.Exchange, symbolToken string, interval model.Timeframe, fromDate, toDate string) (*model.Response[model.HistoricalResponse], error)
	FetchHistoricalDataBatch(requests []model.HistoricalBatchRequest) (*model.Response[[]model.HistoricalBatchItem], error)
	GetMarketQuote(mode model.QuoteMode, exchangeTokens map[model.Exchange][]string) (*model.Response[[]model.MarketQuoteResponse], error)
	GetWebSocket() (model.WebSocketTicker, error)
	Logout() error
}
