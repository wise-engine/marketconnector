// Package wire contains the raw request/response types and mapping functions
// for the Angel One SmartAPI REST API, plus the constants that describe its
// historical data limits. It is internal to the angelone broker implementation.
package wire

import (
	"encoding/json"
	"fmt"

	"github.com/wise-engine/marketconnector/model"
)

// Envelope holds the status fields shared by every Angel One REST response. The
// Data field is kept as raw JSON because Angel One returns an object on success
// but a string (error text) on failure.
type Envelope struct {
	Status    bool   `json:"status"`
	Message   string `json:"message"`
	ErrorCode string `json:"errorcode"`
}

// Err returns a descriptive error when the API reported status=false, or nil on
// success.
func (e Envelope) Err() error {
	if e.Status {
		return nil
	}
	if e.Message != "" {
		return fmt.Errorf("%s (errorcode %q)", e.Message, e.ErrorCode)
	}
	return fmt.Errorf("request failed (errorcode %q)", e.ErrorCode)
}

// Decode validates the response envelope and, on success, unmarshals raw into a
// value of type T. Empty or quoted-empty data yields the zero value.
func Decode[T any](env Envelope, raw json.RawMessage) (T, error) {
	var out T
	if err := env.Err(); err != nil {
		return out, err
	}
	if len(raw) == 0 || string(raw) == `""` {
		return out, nil
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return out, fmt.Errorf("decode response data: %w", err)
	}
	return out, nil
}

// LoginRequest is the payload for the login endpoint.
type LoginRequest struct {
	ClientCode string `json:"clientcode"`
	Password   string `json:"password"`
	TOTP       string `json:"totp"`
	State      string `json:"state"`
}

// LoginData is the data payload of a successful login response.
type LoginData struct {
	JWTToken     string `json:"jwtToken"`
	RefreshToken string `json:"refreshToken"`
	FeedToken    string `json:"feedToken"`
	State        string `json:"state"`
}

// LoginResponse is the raw response of the login endpoint.
type LoginResponse struct {
	Envelope
	Data json.RawMessage `json:"data"`
}

// ProfileData is the data payload of a successful getProfile response.
type ProfileData struct {
	ClientCode string   `json:"clientcode"`
	Username   string   `json:"name"`
	Email      string   `json:"email"`
	Exchanges  []string `json:"exchanges"`
	Products   []string `json:"products"`
}

// Profile is the raw response of the getProfile endpoint.
type Profile struct {
	Envelope
	Data json.RawMessage `json:"data"`
}

// FundsData is the data payload of a successful getRMS response.
type FundsData struct {
	NetMargin     string `json:"net"`
	AvailableCash string `json:"availablecash"`
}

// Funds is the raw response of the getRMS endpoint.
type Funds struct {
	Envelope
	Data json.RawMessage `json:"data"`
}

// HoldingItem is a single holding in the getHolding response data.
type HoldingItem struct {
	TradingSymbol string  `json:"tradingsymbol"`
	Exchange      string  `json:"exchange"`
	T1Quantity    int32   `json:"t1quantity"`
	Quantity      int32   `json:"quantity"`
	Product       string  `json:"product"`
	AveragePrice  float64 `json:"averageprice"`
	LTP           float64 `json:"ltp"`
	Close         float64 `json:"close"`
	Pnl           float64 `json:"profitandloss"`
	PnlPct        float32 `json:"pnlpercentage"`
}

// Holdings is the raw response of the getHolding endpoint.
type Holdings struct {
	Envelope
	Data json.RawMessage `json:"data"`
}

// PositionItem is a single position in the getPosition response data. Numeric
// fields are strings because Angel One returns them as strings.
type PositionItem struct {
	Exchange      string `json:"exchange"`
	SymbolToken   string `json:"symboltoken"`
	ProductType   string `json:"producttype"`
	TradingSymbol string `json:"tradingsymbol"`
	SymbolName    string `json:"symbolname"`
	BuyQty        string `json:"buyqty"`
	SellQty       string `json:"sellqty"`
	BuyAmount     string `json:"buyamount"`
	SellAmount    string `json:"sellamount"`
	BuyAvgPrice   string `json:"buyavgprice"`
	SellAvgPrice  string `json:"sellavgprice"`
	AvgNetPrice   string `json:"avgnetprice"`
	NetValue      string `json:"netvalue"`
	CFBuyQty      string `json:"cfbuyqty"`
	CFSellQty     string `json:"cfsellqty"`
	CFBuyAmount   string `json:"cfbuyamount"`
	CFSellAmount  string `json:"cfsellamount"`
	LotSize       string `json:"lotsize"`
}

// Positions is the raw response of the getPosition endpoint.
type Positions struct {
	Envelope
	Data json.RawMessage `json:"data"`
}

// QuoteRequest is the payload for the market quote endpoint.
type QuoteRequest struct {
	Mode           string              `json:"mode"`
	ExchangeTokens map[string][]string `json:"exchangeTokens"`
}

// QuoteDepthLevel is a single level of market depth.
type QuoteDepthLevel struct {
	Price    float64 `json:"price"`
	Quantity int32   `json:"quantity"`
	Orders   int32   `json:"orders"`
}

// QuoteDepth holds the best buy and sell depth for a symbol.
type QuoteDepth struct {
	Buy  []QuoteDepthLevel `json:"buy"`
	Sell []QuoteDepthLevel `json:"sell"`
}

// QuoteFetched is a single fetched quote in the quote response data.
type QuoteFetched struct {
	Exchange      string     `json:"exchange"`
	TradingSymbol string     `json:"tradingSymbol"`
	SymbolToken   string     `json:"symbolToken"`
	LTP           float64    `json:"ltp"`
	Open          float64    `json:"open"`
	High          float64    `json:"high"`
	Low           float64    `json:"low"`
	Close         float64    `json:"close"`
	LastTradeQty  int32      `json:"lastTradeQty"`
	ExchFeedTime  string     `json:"exchFeedTime"`
	ExchTradeTime string     `json:"exchTradeTime"`
	NetChange     float64    `json:"netChange"`
	PercentChange float64    `json:"percentChange"`
	AvgPrice      float64    `json:"avgPrice"`
	TradeVolume   int64      `json:"tradeVolume"`
	OpnInterest   int64      `json:"opnInterest"`
	UpperCircuit  float64    `json:"upperCircuit"`
	LowerCircuit  float64    `json:"lowerCircuit"`
	TotBuyQuan    int32      `json:"totBuyQuan"`
	TotSellQuan   int32      `json:"totSellQuan"`
	Week52High    float64    `json:"52WeekHigh"`
	Week52Low     float64    `json:"52WeekLow"`
	Depth         QuoteDepth `json:"depth"`
}

// QuoteData is the data payload of a successful quote response.
type QuoteData struct {
	Fetched   []QuoteFetched `json:"fetched"`
	Unfetched []any          `json:"unfetched"`
}

// QuoteResponse is the raw response of the market quote endpoint.
type QuoteResponse struct {
	Envelope
	Data json.RawMessage `json:"data"`
}

// HistoricalRequest is the payload for the historical data endpoints.
type HistoricalRequest struct {
	Exchange    string `json:"exchange"`
	SymbolToken string `json:"symboltoken"`
	Interval    string `json:"interval"`
	FromDate    string `json:"fromdate"`
	ToDate      string `json:"todate"`
}

// HistoricalCandleData is the raw response of the getCandleData endpoint. Data
// is kept as raw JSON because candles come back as arrays of arrays.
type HistoricalCandleData struct {
	Envelope
	Data json.RawMessage `json:"data"`
}

// Candles parses the raw Data field into candle records. Empty or invalid data
// yields nil.
func (h *HistoricalCandleData) Candles() ([][]any, error) {
	if err := h.Err(); err != nil {
		return nil, err
	}
	if h.Data == nil || string(h.Data) == `""` || string(h.Data) == "" {
		return nil, nil
	}
	var records [][]any
	if err := json.Unmarshal(h.Data, &records); err != nil {
		return nil, fmt.Errorf("decode candles: %w", err)
	}
	return records, nil
}

// HistoricalOIItemRaw is a single open-interest record. OI is kept as a raw
// JSON number because Angel One returns integer-valued floats (e.g.
// "1.567111E7" or "214800.0") that encoding/json refuses to decode into an
// int64 directly.
type HistoricalOIItemRaw struct {
	Time string      `json:"time"`
	OI   json.Number `json:"oi"`
}

// HistoricalOIData is the raw response of the getOIData endpoint.
type HistoricalOIData struct {
	Envelope
	Data json.RawMessage `json:"data"`
}

// OIRecords parses the raw Data field into OI items. Empty or invalid data
// yields nil.
func (h *HistoricalOIData) OIRecords() ([]HistoricalOIItemRaw, error) {
	if err := h.Err(); err != nil {
		return nil, err
	}
	if h.Data == nil || string(h.Data) == `""` || string(h.Data) == "" {
		return nil, nil
	}
	var items []HistoricalOIItemRaw
	if err := json.Unmarshal(h.Data, &items); err != nil {
		return nil, fmt.Errorf("decode OI data: %w", err)
	}
	return items, nil
}

// Angel One historical-data interval codes.
const (
	oneMinute   = "ONE_MINUTE"
	threeMinute = "THREE_MINUTE"
	fiveMinute  = "FIVE_MINUTE"
	tenMinute   = "TEN_MINUTE"
	fifteenMin  = "FIFTEEN_MINUTE"
	thirtyMin   = "THIRTY_MINUTE"
	oneHour     = "ONE_HOUR"
	oneDay      = "ONE_DAY"
)

// Historical rate limits (requests/second) and retry settings. Kept here so
// every caller uses the same values.
const (
	HistoricalRateLimit = 2.5 // max requests per second for historical data
	HistoricalRateBurst = 1   // burst size for the rate limiter
	MaxRetries          = 3   // max retries on 403 rate-limit errors
)

// MapExchange converts a broker-agnostic Exchange to the Angel One API string.
func MapExchange(e model.Exchange) string {
	return string(e)
}

// MapTimeframe converts a broker-agnostic Timeframe to the Angel One interval.
func MapTimeframe(tf model.Timeframe) string {
	switch tf {
	case model.Timeframe1Minute:
		return oneMinute
	case model.Timeframe3Minutes:
		return threeMinute
	case model.Timeframe5Minutes:
		return fiveMinute
	case model.Timeframe10Minutes:
		return tenMinute
	case model.Timeframe15Minutes:
		return fifteenMin
	case model.Timeframe30Minutes:
		return thirtyMin
	case model.Timeframe1Hour:
		return oneHour
	case model.Timeframe1Day:
		return oneDay
	default:
		return oneDay
	}
}

// IntervalMaxDays defines the maximum number of days Angel One allows in a
// single historical data request for each interval.
var IntervalMaxDays = map[model.Timeframe]int{
	model.Timeframe1Minute:   30,
	model.Timeframe3Minutes:  60,
	model.Timeframe5Minutes:  100,
	model.Timeframe10Minutes: 100,
	model.Timeframe15Minutes: 200,
	model.Timeframe30Minutes: 200,
	model.Timeframe1Hour:     400,
	model.Timeframe1Day:      2000,
}
