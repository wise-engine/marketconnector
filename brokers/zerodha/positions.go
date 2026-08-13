package zerodha

import (
	"fmt"
	"strconv"

	"github.com/wise-engine/marketconnector/model"
)

// GetPositions returns the current open positions of the account. Kite Connect
// returns net and day positions; the net positions are mapped to the common
// shape. Carry-forward (CF) quantities are derived by subtracting today's day
// positions from the net positions.
func (z *Zerodha) GetPositions() (*model.Response[[]model.PositionResponse], error) {
	positions, err := z.kite().GetPositions()
	if err != nil {
		return nil, fmt.Errorf("zerodha: get positions: %w", err)
	}

	out := make([]model.PositionResponse, 0, len(positions.Net))
	for _, p := range positions.Net {
		out = append(out, model.PositionResponse{
			Exchange:      p.Exchange,
			SymbolToken:   strconv.Itoa(int(p.InstrumentToken)),
			ProductType:   p.Product,
			TradingSymbol: p.Tradingsymbol,
			BuyQty:        int32(p.BuyQuantity),
			SellQty:       int32(p.SellQuantity),
			BuyAmount:     p.BuyValue,
			SellAmount:    p.SellValue,
			BuyAvgPrice:   p.BuyPrice,
			SellAvgPrice:  p.SellPrice,
			AvgNetPrice:   p.AveragePrice,
			NetValue:      p.Value,
			// Carry-forward = net minus today's day positions.
			CFBuyQty:     int32(p.BuyQuantity - p.DayBuyQuantity),
			CFSellQty:    int32(p.SellQuantity - p.DaySellQuantity),
			CFBuyAmount:  p.BuyValue - p.DayBuyValue,
			CFSellAmount: p.SellValue - p.DaySellValue,
		})
	}

	return &model.Response[[]model.PositionResponse]{
		Success: true,
		Message: "SUCCESS",
		Broker:  "zerodha",
		Data:    out,
	}, nil
}
