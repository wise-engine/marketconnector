package zerodha

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/sunnyme20/marketconnector/model"
	kiteconnect "github.com/zerodha/gokiteconnect/v4"
	"github.com/zerodha/gokiteconnect/v4/models"
)

// maxQuoteInstruments is the maximum number of instruments the Kite Connect
// quote endpoint accepts in a single request.
const maxQuoteInstruments = 500

// fetchQuoteBatch calls the Kite Connect quote API for a single batch of
// instruments in "EXCHANGE:SYMBOL" format, honouring the requested mode.
func (z *Zerodha) fetchQuoteBatch(mode model.QuoteMode, instruments []string) ([]model.MarketQuoteResponse, error) {
	switch mode {
	case model.QuoteModeLTP:
		ltp, err := z.kite().GetLTP(instruments...)
		if err != nil {
			return nil, fmt.Errorf("fetch ltp: %w", err)
		}
		return ltpToCommon(ltp), nil
	case model.QuoteModeOHLC:
		ohlc, err := z.kite().GetOHLC(instruments...)
		if err != nil {
			return nil, fmt.Errorf("fetch ohlc: %w", err)
		}
		return ohlcToCommon(ohlc), nil
	default: // QuoteModeFull
		quotes, err := z.kite().GetQuote(instruments...)
		if err != nil {
			return nil, fmt.Errorf("fetch quote: %w", err)
		}
		return fullToCommon(quotes), nil
	}
}

// GetMarketQuote returns market quotes for the requested exchange tokens.
//
// Note: unlike Angel One (which keys quotes by instrument token), Kite Connect
// quotes are keyed by trading symbol, so the string values in exchangeTokens
// are treated as trading symbols (e.g. model.ExchangeNSE: {"INFY", "RELIANCE"}).
func (z *Zerodha) GetMarketQuote(mode model.QuoteMode, exchangeTokens map[model.Exchange][]string) (*model.Response[[]model.MarketQuoteResponse], error) {
	// Build a deterministic list of "EXCHANGE:SYMBOL" instruments.
	exchanges := make([]string, 0, len(exchangeTokens))
	for ex := range exchangeTokens {
		exchanges = append(exchanges, string(ex))
	}
	sort.Strings(exchanges)

	var instruments []string
	for _, exStr := range exchanges {
		symbols := exchangeTokens[model.Exchange(exStr)]
		for _, sym := range symbols {
			instruments = append(instruments, exStr+":"+sym)
		}
	}

	var allQuotes []model.MarketQuoteResponse
	for i := 0; i < len(instruments); i += maxQuoteInstruments {
		end := i + maxQuoteInstruments
		if end > len(instruments) {
			end = len(instruments)
		}
		quotes, err := z.fetchQuoteBatch(mode, instruments[i:end])
		if err != nil {
			return nil, fmt.Errorf("zerodha: quote batch %d: %w", i/maxQuoteInstruments, err)
		}
		allQuotes = append(allQuotes, quotes...)
	}

	return &model.Response[[]model.MarketQuoteResponse]{
		Success: true,
		Message: "SUCCESS",
		Broker:  "zerodha",
		Data:    allQuotes,
	}, nil
}

// fullToCommon maps the full quote response to the common quote shape.
func fullToCommon(q kiteconnect.Quote) []model.MarketQuoteResponse {
	out := make([]model.MarketQuoteResponse, 0, len(q))
	for key, item := range q {
		out = append(out, model.MarketQuoteResponse{
			Exchange:      exchangeFromKey(key),
			TradingSymbol: symbolFromKey(key),
			SymbolToken:   strconv.Itoa(item.InstrumentToken),
			LTP:           item.LastPrice,
			Open:          item.OHLC.Open,
			High:          item.OHLC.High,
			Low:           item.OHLC.Low,
			Close:         item.OHLC.Close,
			NetChange:     item.NetChange,
			PercentChange: percentChange(item.LastPrice, item.OHLC.Close),
			AvgPrice:      item.AveragePrice,
			TradeVolume:   int64(item.Volume),
			LastTradedQty: int64(item.LastQuantity),
			TotalBuyQty:   float64(item.BuyQuantity),
			TotalSellQty:  float64(item.SellQuantity),
			OpenInterest:  int64(item.OI),
			UpperCircuit:  item.UpperCircuitLimit,
			LowerCircuit:  item.LowerCircuitLimit,
			LastTradeTime: item.LastTradeTime.Unix(),
			ExchangeTime:  formatExchangeTime(item.Timestamp.Time),
			Depth:         depthToCommon(item.Depth),
		})
	}
	return out
}

// ohlcToCommon maps the OHLC quote response to the common quote shape.
func ohlcToCommon(q kiteconnect.QuoteOHLC) []model.MarketQuoteResponse {
	out := make([]model.MarketQuoteResponse, 0, len(q))
	for key, item := range q {
		out = append(out, model.MarketQuoteResponse{
			Exchange:      exchangeFromKey(key),
			TradingSymbol: symbolFromKey(key),
			SymbolToken:   strconv.Itoa(item.InstrumentToken),
			LTP:           item.LastPrice,
			Open:          item.OHLC.Open,
			High:          item.OHLC.High,
			Low:           item.OHLC.Low,
			Close:         item.OHLC.Close,
			PercentChange: percentChange(item.LastPrice, item.OHLC.Close),
		})
	}
	return out
}

// ltpToCommon maps the LTP quote response to the common quote shape.
func ltpToCommon(q kiteconnect.QuoteLTP) []model.MarketQuoteResponse {
	out := make([]model.MarketQuoteResponse, 0, len(q))
	for key, item := range q {
		out = append(out, model.MarketQuoteResponse{
			Exchange:      exchangeFromKey(key),
			TradingSymbol: symbolFromKey(key),
			SymbolToken:   strconv.Itoa(item.InstrumentToken),
			LTP:           item.LastPrice,
		})
	}
	return out
}

// depthToCommon converts Kite Connect depth (fixed 5x5 arrays) to the common
// depth shape, skipping empty levels.
func depthToCommon(d models.Depth) *model.MarketDepth {
	depth := &model.MarketDepth{}
	for _, b := range d.Buy {
		if b.Quantity > 0 {
			depth.Buy = append(depth.Buy, model.DepthItem{
				Quantity: int64(b.Quantity),
				Price:    b.Price,
				Orders:   int32(b.Orders),
			})
		}
	}
	for _, s := range d.Sell {
		if s.Quantity > 0 {
			depth.Sell = append(depth.Sell, model.DepthItem{
				Quantity: int64(s.Quantity),
				Price:    s.Price,
				Orders:   int32(s.Orders),
			})
		}
	}
	if len(depth.Buy) == 0 && len(depth.Sell) == 0 {
		return nil
	}
	return depth
}

// percentChange returns the percentage change of price relative to close.
func percentChange(price, close float64) float64 {
	if close == 0 {
		return 0
	}
	return (price - close) / close * 100
}

// formatExchangeTime renders an exchange timestamp as an IST string, or ""
// when zero.
func formatExchangeTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	ist := time.FixedZone("IST", 5*60*60+30*60)
	return t.In(ist).Format("02-Jan-2006 15:04:05")
}

// symbolFromKey extracts the trading symbol from a "EXCHANGE:SYMBOL" key.
func symbolFromKey(key string) string {
	if i := strings.Index(key, ":"); i >= 0 {
		return key[i+1:]
	}
	return key
}

// exchangeFromKey extracts the exchange from a "EXCHANGE:SYMBOL" key.
func exchangeFromKey(key string) string {
	if i := strings.Index(key, ":"); i >= 0 {
		return key[:i]
	}
	return ""
}
