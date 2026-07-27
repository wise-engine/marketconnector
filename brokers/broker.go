package brokers

import models "github.com/sunnyme20/marketconnector/brokers/model"

type Broker interface {
	NewSession(clientcode, apikey, password, totp string) (*models.Response[models.LoginResponse], error)
	SetAccessToken(accessToken string)
	SetFeedToken(feedToken string)
	SetClientCode(clientcode string)
	SetApiKey(apikey string)
	GetAccessToken() (string, error)
	GetUserProfile() (*models.Response[models.UserProfileResponse], error)
	GetRMSData() (*models.Response[models.FundsResponse], error)
	GetHoldings() (*models.Response[[]models.HoldingResponse], error)
	GetPositions() (*models.Response[[]models.PositionResponse], error)
	GetHistoricalData(exchange models.Exchange, symbolToken string, interval models.Timeframe, fromDate, toDate string) (*models.Response[models.HistoricalResponse], error)
	FetchHistoricalDataBatch(requests []models.HistoricalBatchRequest) (*models.Response[[]models.HistoricalBatchItem], error)
	Logout()
	GetBrokerageCharges()
	GetMargin()
	GetMarketQuote(mode models.QuoteMode, exchangeTokens map[models.Exchange][]string) (*models.Response[[]models.MarketQuoteResponse], error)
	GetWebSocket() (models.WebSocketTicker, error)
}
