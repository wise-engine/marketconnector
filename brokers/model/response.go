package models

type Response[T any] struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Broker  string `json:"broker"`
	Data    T      `json:"data"`
}

type HistoricalCandle struct {
	Timestamp string  `json:"timestamp"`
	Open      float64 `json:"open"`
	High      float64 `json:"high"`
	Low       float64 `json:"low"`
	Close     float64 `json:"close"`
	Volume    int64   `json:"volume"`
}

type HistoricalOIItem struct {
	Timestamp string `json:"timestamp"`
	OI        int64  `json:"oi"`
}

type HistoricalResponse struct {
	Candles []HistoricalCandle `json:"candles"`
	OI      []HistoricalOIItem `json:"oi"`
}

// HistoricalBatchRequest describes one symbol's historical data request for batch fetching.
type HistoricalBatchRequest struct {
	Exchange    Exchange
	SymbolToken string
	Interval    Timeframe
	FromDate    string
	ToDate      string
}

// HistoricalBatchItem holds the aggregated result for one symbol in a batch fetch.
type HistoricalBatchItem struct {
	SymbolToken string              `json:"symbol_token"`
	Success     bool                `json:"success"`
	Message     string              `json:"message"`
	Broker      string              `json:"broker"`
	Data        *HistoricalResponse `json:"data,omitempty"`
	Error       string              `json:"error,omitempty"`
}

type MarketQuoteResponse struct {
	Exchange       string       `json:"exchange"`
	TradingSymbol  string       `json:"trading_symbol"`
	SymbolToken    string       `json:"symbol_token"`
	LTP            float64      `json:"ltp"`
	Open           float64      `json:"open"`
	High           float64      `json:"high"`
	Low            float64      `json:"low"`
	Close          float64      `json:"close"`
	NetChange      float64      `json:"net_change"`
	PercentChange  float64      `json:"percent_change"`
	AvgPrice       float64      `json:"avg_price"`
	TradeVolume    int64        `json:"trade_volume"`
	LastTradedQty  int64        `json:"last_traded_qty,omitempty"`
	TotalBuyQty    float64      `json:"total_buy_qty,omitempty"`
	TotalSellQty   float64      `json:"total_sell_qty,omitempty"`
	OpenInterest   int64        `json:"open_interest"`
	UpperCircuit   float64      `json:"upper_circuit"`
	LowerCircuit   float64      `json:"lower_circuit"`
	Week52High     float64      `json:"52_week_high,omitempty"`
	Week52Low      float64      `json:"52_week_low,omitempty"`
	LastTradeTime  int64        `json:"last_trade_time,omitempty"`
	ExchangeTime   string       `json:"exchange_time,omitempty"`
	SequenceNumber int64        `json:"sequence_number,omitempty"`
	Depth          *MarketDepth `json:"depth,omitempty"`
}

// DepthItem represents a single entry in the market depth.
type DepthItem struct {
	Quantity int64   `json:"quantity"`
	Price    float64 `json:"price"`
	Orders   int32   `json:"orders"`
}

// MarketDepth holds the best buy and sell data.
type MarketDepth struct {
	Buy  []DepthItem `json:"buy"`
	Sell []DepthItem `json:"sell"`
}

type LoginResponse struct {
	AccessToken string `json:"access_token"`
	FeedToken   string `json:"feed_token"`
}

type UserProfileResponse struct {
	ClientCode string   `json:"client_code"`
	Username   string   `json:"name"`
	Email      string   `json:"email"`
	Exchanges  []string `json:"exchanges"`
	Products   []string `json:"products"`
}

type FundsResponse struct {
	NetMargin     float64 `json:"net_margin"`
	AvailableCash float64 `json:"available_cash"`
}

type HoldingResponse struct {
	TradingSymbol string  `json:"trading_symbol"`
	Exchange      string  `json:"exchange"`
	T1Quantity    int32   `json:"t1_quantity"`
	Quantity      int32   `json:"quantity"`
	Product       string  `json:"product"`
	AveragePrice  float64 `json:"average_price"`
	LTP           float64 `json:"ltp"`
	Close         float64 `json:"close"`
	Pnl           float64 `json:"pnl"`
	PnlPct        float32 `json:"pnl_pct"`
	Investment    float64 `json:"investment"`
	Current       float64 `json:"current"`
	Return        float64 `json:"return"`
}

type PositionResponse struct {
	Exchange      string  `json:"exchange"`
	SymbolToken   string  `json:"symbol_token"`
	ProductType   string  `json:"product_type"`
	TradingSymbol string  `json:"trading_symbol"`
	BuyQty        int32   `json:"buy_qty"`
	SellQty       int32   `json:"sell_qty"`
	BuyAmount     float64 `json:"buy_amount"`
	SellAmount    float64 `json:"sell_amount"`
	BuyAvgPrice   float64 `json:"buy_avg_price"`
	SellAvgPrice  float64 `json:"sell_avg_price"`
	AvgNetPrice   float64 `json:"avg_net_price"`
	NetValue      float64 `json:"net_value"`
	CFBuyQty      int32   `json:"cf_buy_qty"`
	CFSellQty     int32   `json:"cf_sell_qty"`
	CFBuyAmount   float64 `json:"cf_buy_amount"`
	CFSellAmount  float64 `json:"cf_sell_amount"`
	LotSize       int32   `json:"lot_size"`
}

type BrokerageCalResponse struct {
}

type MarginCalResponse struct {
}

type OptionChainResponse struct {
}

type OptionInterestResponse struct {
}
