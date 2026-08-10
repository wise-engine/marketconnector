package ws

import (
	"encoding/binary"
	"fmt"
	"math"
	"time"

	"github.com/sunnyme20/marketconnector/model"
)

// parseTick decodes an Angel One WebSocket v2 binary packet into a common
// MarketQuoteResponse.
func parseTick(b []byte) (model.MarketQuoteResponse, error) {
	if len(b) < 2 {
		return model.MarketQuoteResponse{}, fmt.Errorf("packet too short: %d bytes", len(b))
	}

	subMode := int(b[0])

	// Token: 25-byte null-terminated string starting at byte 2.
	tokenBytes := b[2:27]
	token := nullTerminatedString(tokenBytes)

	offset := 27

	var seqNum int64
	if len(b) >= offset+8 {
		seqNum = int64(binary.LittleEndian.Uint64(b[offset : offset+8]))
	}
	offset += 8

	var exchTime int64
	if len(b) >= offset+8 {
		exchTime = int64(binary.LittleEndian.Uint64(b[offset : offset+8]))
	}
	offset += 8

	var ltp float64
	if len(b) >= offset+8 {
		ltpVal := int64(binary.LittleEndian.Uint64(b[offset : offset+8]))
		ltp = fromPaise(ltpVal)
	}
	offset += 8

	// Convert the exchange timestamp from UTC epoch ms to a readable IST string.
	istLoc := time.FixedZone("IST", 5*60*60+30*60)
	exchangeTimeStr := time.UnixMilli(exchTime).In(istLoc).Format("02-Jan-2006 15:04:05")

	tick := model.MarketQuoteResponse{
		SymbolToken:    token,
		LTP:            ltp,
		SequenceNumber: seqNum,
		ExchangeTime:   exchangeTimeStr,
	}

	// For LTP mode (mode=1), the packet ends here at 51 bytes.
	if subMode == 1 {
		return tick, nil
	}

	var lastTradedQty int64
	if len(b) >= offset+8 {
		lastTradedQty = int64(binary.LittleEndian.Uint64(b[offset : offset+8]))
	}
	offset += 8

	var avgPrice float64
	if len(b) >= offset+8 {
		avgVal := int64(binary.LittleEndian.Uint64(b[offset : offset+8]))
		avgPrice = fromPaise(avgVal)
	}
	offset += 8

	var volume int64
	if len(b) >= offset+8 {
		volume = int64(binary.LittleEndian.Uint64(b[offset : offset+8]))
	}
	offset += 8

	var totBuyQty, totSellQty float64
	if len(b) >= offset+8 {
		totBuyQty = math.Float64frombits(binary.LittleEndian.Uint64(b[offset : offset+8]))
	}
	offset += 8

	if len(b) >= offset+8 {
		totSellQty = math.Float64frombits(binary.LittleEndian.Uint64(b[offset : offset+8]))
	}
	offset += 8

	var open, high, low, closeVal float64
	if len(b) >= offset+8 {
		openVal := int64(binary.LittleEndian.Uint64(b[offset : offset+8]))
		open = fromPaise(openVal)
	}
	offset += 8

	if len(b) >= offset+8 {
		highVal := int64(binary.LittleEndian.Uint64(b[offset : offset+8]))
		high = fromPaise(highVal)
	}
	offset += 8

	if len(b) >= offset+8 {
		lowVal := int64(binary.LittleEndian.Uint64(b[offset : offset+8]))
		low = fromPaise(lowVal)
	}
	offset += 8

	if len(b) >= offset+8 {
		closeRaw := int64(binary.LittleEndian.Uint64(b[offset : offset+8]))
		closeVal = fromPaise(closeRaw)
	}
	offset += 8

	tick.LastTradedQty = lastTradedQty
	tick.AvgPrice = avgPrice
	tick.TradeVolume = volume
	tick.TotalBuyQty = totBuyQty
	tick.TotalSellQty = totSellQty
	tick.Open = open
	tick.High = high
	tick.Low = low
	tick.Close = closeVal
	tick.NetChange = ltp - closeVal

	if closeVal != 0 {
		tick.PercentChange = (ltp - closeVal) / closeVal * 100
	}

	// For Quote mode (mode=2), the packet ends here at 123 bytes.
	if subMode == 2 {
		return tick, nil
	}

	// SnapQuote fields (mode=3).
	var lastTradeTime int64
	if len(b) >= offset+8 {
		lastTradeTime = int64(binary.LittleEndian.Uint64(b[offset : offset+8]))
	}
	offset += 8

	var oi int64
	if len(b) >= offset+8 {
		oi = int64(binary.LittleEndian.Uint64(b[offset : offset+8]))
	}
	offset += 8

	tick.LastTradeTime = lastTradeTime
	tick.OpenInterest = oi

	// OI change % (double, dummy/garbage) — skip 8 bytes.
	offset += 8

	// Best five data: 200 bytes (10 packets of 20 bytes each).
	if len(b) >= offset+200 {
		depth := &model.MarketDepth{}
		for i := 0; i < 5; i++ {
			item := parseDepthToCommon(b[offset+i*20 : offset+i*20+20])
			depth.Buy = append(depth.Buy, item)
		}
		offset += 100

		for i := 0; i < 5; i++ {
			item := parseDepthToCommon(b[offset+i*20 : offset+i*20+20])
			depth.Sell = append(depth.Sell, item)
		}
		offset += 100
		tick.Depth = depth
	}

	if len(b) >= offset+8 {
		uc := int64(binary.LittleEndian.Uint64(b[offset : offset+8]))
		tick.UpperCircuit = fromPaise(uc)
	}
	offset += 8

	if len(b) >= offset+8 {
		lc := int64(binary.LittleEndian.Uint64(b[offset : offset+8]))
		tick.LowerCircuit = fromPaise(lc)
	}
	offset += 8

	if len(b) >= offset+8 {
		w52h := int64(binary.LittleEndian.Uint64(b[offset : offset+8]))
		tick.Week52High = fromPaise(w52h)
	}
	offset += 8

	if len(b) >= offset+8 {
		w52l := int64(binary.LittleEndian.Uint64(b[offset : offset+8]))
		tick.Week52Low = fromPaise(w52l)
	}

	return tick, nil
}

// parseDepthToCommon parses a 20-byte depth packet into a common DepthItem.
func parseDepthToCommon(b []byte) model.DepthItem {
	if len(b) < 20 {
		return model.DepthItem{}
	}
	qty := int64(binary.LittleEndian.Uint64(b[2:10]))
	priceRaw := int64(binary.LittleEndian.Uint64(b[10:18]))
	orders := int32(binary.LittleEndian.Uint16(b[18:20]))
	return model.DepthItem{
		Quantity: qty,
		Price:    fromPaise(priceRaw),
		Orders:   orders,
	}
}

// nullTerminatedString extracts a string from a byte buffer up to the first
// null byte.
func nullTerminatedString(b []byte) string {
	for i, v := range b {
		if v == 0 {
			return string(b[:i])
		}
	}
	return string(b)
}

// fromPaise converts paise (int64) to rupees (float64).
func fromPaise(val int64) float64 {
	return float64(val) / 100.0
}
