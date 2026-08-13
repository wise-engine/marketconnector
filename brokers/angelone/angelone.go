package angelone

import (
	"github.com/wise-engine/marketconnector/brokers/angelone/internal/client"
	"github.com/wise-engine/marketconnector/brokers/angelone/ws"
	"github.com/wise-engine/marketconnector/model"
)

// Angelone implements the marketconnector.Broker interface for the Angel One
// SmartAPI. Create an instance via marketconnector.NewBroker(model.BrokerAngelOne)
// or by using &Angelone{} directly and then calling NewSession.
type Angelone struct {
	clientCode   string
	apiKey       string
	accessToken  string
	feedToken    string
	refreshToken string
	client       *client.Client
}

// httpClient returns the shared HTTP client, creating it lazily on first use.
func (a *Angelone) httpClient() *client.Client {
	if a.client == nil {
		a.client = client.New(a.apiKey)
	}
	return a.client
}

// SetClientCode sets the trading account client code.
func (a *Angelone) SetClientCode(clientCode string) {
	a.clientCode = clientCode
}

// SetAPIKey sets the API key used to authenticate requests.
func (a *Angelone) SetAPIKey(apiKey string) {
	a.apiKey = apiKey
	if a.client != nil {
		a.client.SetAPIKey(apiKey)
	}
}

// SetAccessToken sets the JWT access token obtained from NewSession.
func (a *Angelone) SetAccessToken(accessToken string) {
	a.accessToken = accessToken
}

// SetFeedToken sets the feed token used for WebSocket streaming.
func (a *Angelone) SetFeedToken(feedToken string) {
	a.feedToken = feedToken
}

// SetRefreshToken sets the refresh token used by getProfile and token renewal.
func (a *Angelone) SetRefreshToken(refreshToken string) {
	a.refreshToken = refreshToken
}

// GetAccessToken returns the current JWT access token.
func (a *Angelone) GetAccessToken() string {
	return a.accessToken
}

// GetWebSocket returns a streaming ticker for real-time market data.
func (a *Angelone) GetWebSocket() (model.WebSocketTicker, error) {
	return ws.NewWebSocket(a.apiKey, a.clientCode, a.accessToken, a.feedToken), nil
}

// Logout clears the current session tokens locally.
func (a *Angelone) Logout() error {
	a.accessToken = ""
	a.feedToken = ""
	return nil
}
