package zerodha

import (
	"fmt"

	"github.com/wise-engine/marketconnector/model"
)

// GetHoldings returns the current holdings of the account.
func (z *Zerodha) GetHoldings() (*model.Response[[]model.HoldingResponse], error) {
	holdings, err := z.kite().GetHoldings()
	if err != nil {
		return nil, fmt.Errorf("zerodha: get holdings: %w", err)
	}

	out := make([]model.HoldingResponse, 0, len(holdings))
	for _, h := range holdings {
		out = append(out, model.HoldingResponse{
			TradingSymbol: h.Tradingsymbol,
			Exchange:      h.Exchange,
			T1Quantity:    int32(h.T1Quantity),
			Quantity:      int32(h.Quantity),
			Product:       h.Product,
			AveragePrice:  h.AveragePrice,
			LTP:           h.LastPrice,
			Close:         h.ClosePrice,
			Pnl:           h.PnL,
			PnlPct:        float32(h.DayChangePercentage),
			Investment:    h.AveragePrice * float64(h.Quantity),
			Current:       h.ClosePrice * float64(h.Quantity),
			Return:        (h.ClosePrice - h.AveragePrice) * float64(h.Quantity),
		})
	}

	return &model.Response[[]model.HoldingResponse]{
		Success: true,
		Message: "SUCCESS",
		Broker:  "zerodha",
		Data:    out,
	}, nil
}
