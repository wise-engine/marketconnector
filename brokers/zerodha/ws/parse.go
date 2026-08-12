package ws

import (
	"strconv"
	"time"

	"github.com/sunnyme20/marketconnector/model"
	"github.com/zerodha/gokiteconnect/v4/models"
	kiteticker "github.com/zerodha/gokiteconnect/v4/ticker"
)

// ist is the Indian Standard Time zone used to render exchange timestamps.
var ist = time.FixedZone("IST", 5*60*60+30*60)

// convertTick maps a gokiteconnect models.Tick into the broker-agnostic
// MarketQuoteResponse.
func convertTick(tick models.Tick) model.MarketQuoteResponse {
	out := model.MarketQuoteResponse{
		Exchange:      segmentToExchange(tick.InstrumentToken & 0xFF),
		SymbolToken:   strconv.Itoa(int(tick.InstrumentToken)),
		LTP:           tick.LastPrice,
		Open:          tick.OHLC.Open,
		High:          tick.OHLC.High,
		Low:           tick.OHLC.Low,
		Close:         tick.OHLC.Close,
		NetChange:     tick.NetChange,
		AvgPrice:      tick.AverageTradePrice,
		TradeVolume:   int64(tick.VolumeTraded),
		LastTradedQty: int64(tick.LastTradedQuantity),
		TotalBuyQty:   float64(tick.TotalBuyQuantity),
		TotalSellQty:  float64(tick.TotalSellQuantity),
		OpenInterest:  int64(tick.OI),
	}

	if tick.OHLC.Close != 0 {
		out.PercentChange = (tick.LastPrice - tick.OHLC.Close) / tick.OHLC.Close * 100
	}

	if !tick.Timestamp.IsZero() {
		out.ExchangeTime = tick.Timestamp.In(ist).Format("02-Jan-2006 15:04:05")
	}
	if !tick.LastTradeTime.IsZero() {
		out.LastTradeTime = tick.LastTradeTime.Unix()
	}

	if depth := depthToCommon(tick.Depth); depth != nil {
		out.Depth = depth
	}

	return out
}

// depthToCommon converts the fixed 5x5 Kite Connect depth to the common depth
// shape, skipping empty levels.
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

// segmentToExchange maps a Kite Connect instrument segment to an exchange name.
func segmentToExchange(segment uint32) string {
	switch segment {
	case kiteticker.NseCM:
		return "NSE"
	case kiteticker.NseFO:
		return "NFO"
	case kiteticker.BseCM:
		return "BSE"
	case kiteticker.BseFO:
		return "BFO"
	case kiteticker.McxFO:
		return "MCX"
	default:
		return ""
	}
}
