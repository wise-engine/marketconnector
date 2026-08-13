// Package zerodha implements the marketconnector.Broker interface for the
// Zerodha Kite Connect API using the official gokiteconnect SDK instead of raw
// HTTP endpoints. The file layout mirrors the angelone broker; only the data
// retrieval layer differs (SDK calls vs. hand-rolled REST client).
//
// Create an instance via marketconnector.NewBroker(model.BrokerZerodha) or by
// using &Zerodha{} directly and then calling NewSession.
package zerodha

import (
	"fmt"

	"github.com/wise-engine/marketconnector/brokers/zerodha/ws"
	"github.com/wise-engine/marketconnector/model"
	kiteconnect "github.com/zerodha/gokiteconnect/v4"
)

// Zerodha implements the marketconnector.Broker interface for the Zerodha Kite
// Connect API. All data retrieval goes through the gokiteconnect SDK held in
// client.
type Zerodha struct {
	clientCode   string
	apiKey       string
	apiSecret    string
	accessToken  string
	refreshToken string
	client       *kiteconnect.Client
}

// kite returns the shared gokiteconnect client, creating it lazily on first
// use with the current API key and applying the current access token.
func (z *Zerodha) kite() *kiteconnect.Client {
	if z.client == nil {
		z.client = kiteconnect.New(z.apiKey)
	}
	z.client.SetAccessToken(z.accessToken)
	return z.client
}

// SetClientCode sets the trading account client code.
func (z *Zerodha) SetClientCode(clientCode string) {
	z.clientCode = clientCode
}

// SetAPIKey sets the API key used to authenticate requests. The SDK client is
// recreated lazily so the new key takes effect.
func (z *Zerodha) SetAPIKey(apiKey string) {
	z.apiKey = apiKey
	z.client = nil
}

// SetAccessToken sets the access token obtained from NewSession.
func (z *Zerodha) SetAccessToken(accessToken string) {
	z.accessToken = accessToken
	if z.client != nil {
		z.client.SetAccessToken(accessToken)
	}
}

// SetFeedToken is a no-op for Zerodha: Kite Connect authenticates the
// WebSocket with the access token alone and has no feed-token concept. It
// exists only to satisfy the marketconnector.Broker interface.
func (z *Zerodha) SetFeedToken(feedToken string) {}

// SetRefreshToken sets the refresh token used for token renewal.
func (z *Zerodha) SetRefreshToken(refreshToken string) {
	z.refreshToken = refreshToken
}

// GetAccessToken returns the current access token.
func (z *Zerodha) GetAccessToken() string {
	return z.accessToken
}

// GetWebSocket returns a streaming ticker for real-time market data backed by
// the gokiteconnect ticker.
func (z *Zerodha) GetWebSocket() (model.WebSocketTicker, error) {
	if z.accessToken == "" {
		return nil, fmt.Errorf("zerodha: access token required for WebSocket; call NewSession first")
	}
	return ws.NewWebSocket(z.apiKey, z.accessToken), nil
}

// Logout invalidates the current access token on Zerodha and clears the local
// session state.
func (z *Zerodha) Logout() error {
	if z.client != nil {
		if _, err := z.client.InvalidateAccessToken(); err != nil {
			return fmt.Errorf("zerodha: invalidate access token: %w", err)
		}
	}
	z.accessToken = ""
	z.refreshToken = ""
	return nil
}
