package angelone

import (
	"github.com/sunnyme20/marketconnector/brokers/angelone/internal/endpoints"
	"github.com/sunnyme20/marketconnector/brokers/angelone/internal/util"
	"github.com/sunnyme20/marketconnector/brokers/angelone/internal/wire"
	"github.com/sunnyme20/marketconnector/model"
)

// GetPositions returns the current open positions of the account.
func (a *Angelone) GetPositions() (*model.Response[[]model.PositionResponse], error) {
	httpClient := a.httpClient()
	httpClient.SetAccessToken(a.accessToken)

	var resp wire.Positions
	if err := httpClient.Get(endpoints.API.Position, nil, &resp); err != nil {
		return nil, err
	}

	items, err := wire.Decode[[]wire.PositionItem](resp.Envelope, resp.Data)
	if err != nil {
		return nil, err
	}

	out := make([]model.PositionResponse, 0, len(items))
	for _, item := range items {
		out = append(out, model.PositionResponse{
			Exchange:      item.Exchange,
			SymbolToken:   item.SymbolToken,
			ProductType:   item.ProductType,
			TradingSymbol: item.TradingSymbol,
			BuyQty:        util.ParseInt32(item.BuyQty),
			SellQty:       util.ParseInt32(item.SellQty),
			BuyAmount:     util.ParseFloat64(item.BuyAmount),
			SellAmount:    util.ParseFloat64(item.SellAmount),
			BuyAvgPrice:   util.ParseFloat64(item.BuyAvgPrice),
			SellAvgPrice:  util.ParseFloat64(item.SellAvgPrice),
			AvgNetPrice:   util.ParseFloat64(item.AvgNetPrice),
			NetValue:      util.ParseFloat64(item.NetValue),
			CFBuyQty:      util.ParseInt32(item.CFBuyQty),
			CFSellQty:     util.ParseInt32(item.CFSellQty),
			CFBuyAmount:   util.ParseFloat64(item.CFBuyAmount),
			CFSellAmount:  util.ParseFloat64(item.CFSellAmount),
			LotSize:       util.ParseInt32(item.LotSize),
		})
	}

	return &model.Response[[]model.PositionResponse]{
		Success: true,
		Message: "SUCCESS",
		Broker:  "angelone",
		Data:    out,
	}, nil
}
