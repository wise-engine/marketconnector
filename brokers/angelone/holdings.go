package angelone

import (
	"github.com/wise-engine/marketconnector/brokers/angelone/internal/endpoints"
	"github.com/wise-engine/marketconnector/brokers/angelone/internal/wire"
	"github.com/wise-engine/marketconnector/model"
)

// GetHoldings returns the current holdings of the account.
func (a *Angelone) GetHoldings() (*model.Response[[]model.HoldingResponse], error) {
	httpClient := a.httpClient()
	httpClient.SetAccessToken(a.accessToken)

	var resp wire.Holdings
	if err := httpClient.Get(endpoints.API.Holding, nil, &resp); err != nil {
		return nil, err
	}

	items, err := wire.Decode[[]wire.HoldingItem](resp.Envelope, resp.Data)
	if err != nil {
		return nil, err
	}

	out := make([]model.HoldingResponse, 0, len(items))
	for _, item := range items {
		out = append(out, model.HoldingResponse{
			TradingSymbol: item.TradingSymbol,
			Exchange:      item.Exchange,
			T1Quantity:    item.T1Quantity,
			Quantity:      item.Quantity,
			Product:       item.Product,
			AveragePrice:  item.AveragePrice,
			LTP:           item.LTP,
			Close:         item.Close,
			Pnl:           item.Pnl,
			PnlPct:        item.PnlPct,
			Investment:    item.AveragePrice * float64(item.Quantity),
			Current:       item.Close * float64(item.Quantity),
			Return:        (item.Close - item.AveragePrice) * float64(item.Quantity),
		})
	}

	return &model.Response[[]model.HoldingResponse]{
		Success: true,
		Message: "SUCCESS",
		Broker:  "angelone",
		Data:    out,
	}, nil
}
