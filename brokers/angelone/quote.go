package angelone

import (
	"fmt"
	"sort"

	"github.com/sunnyme20/marketconnector/brokers/angelone/internal/endpoints"
	"github.com/sunnyme20/marketconnector/brokers/angelone/internal/wire"
	"github.com/sunnyme20/marketconnector/model"
)

const maxTokensPerRequest = 50

// fetchQuoteBatch calls the market quote API for a single batch of tokens.
func (a *Angelone) fetchQuoteBatch(mode model.QuoteMode, exchangeTokens map[string][]string) ([]model.MarketQuoteResponse, error) {
	httpClient := a.httpClient()
	httpClient.SetAccessToken(a.accessToken)

	req := wire.QuoteRequest{
		Mode:           string(mode),
		ExchangeTokens: exchangeTokens,
	}

	var resp wire.QuoteResponse
	if err := httpClient.Post(endpoints.API.Quote, req, &resp); err != nil {
		return nil, fmt.Errorf("fetch quote: %w", err)
	}

	data, err := wire.Decode[wire.QuoteData](resp.Envelope, resp.Data)
	if err != nil {
		return nil, fmt.Errorf("fetch quote: %w", err)
	}

	quotes := make([]model.MarketQuoteResponse, 0, len(data.Fetched))
	for _, item := range data.Fetched {
		var depth *model.MarketDepth
		if len(item.Depth.Buy) > 0 || len(item.Depth.Sell) > 0 {
			depth = &model.MarketDepth{}
			for _, d := range item.Depth.Buy {
				depth.Buy = append(depth.Buy, model.DepthItem{
					Quantity: int64(d.Quantity),
					Price:    d.Price,
					Orders:   d.Orders,
				})
			}
			for _, d := range item.Depth.Sell {
				depth.Sell = append(depth.Sell, model.DepthItem{
					Quantity: int64(d.Quantity),
					Price:    d.Price,
					Orders:   d.Orders,
				})
			}
		}
		quotes = append(quotes, model.MarketQuoteResponse{
			Exchange:      item.Exchange,
			TradingSymbol: item.TradingSymbol,
			SymbolToken:   item.SymbolToken,
			LTP:           item.LTP,
			Open:          item.Open,
			High:          item.High,
			Low:           item.Low,
			Close:         item.Close,
			NetChange:     item.NetChange,
			PercentChange: item.PercentChange,
			AvgPrice:      item.AvgPrice,
			TradeVolume:   int64(item.TradeVolume),
			OpenInterest:  int64(item.OpnInterest),
			UpperCircuit:  item.UpperCircuit,
			LowerCircuit:  item.LowerCircuit,
			Depth:         depth,
		})
	}
	return quotes, nil
}

// GetMarketQuote returns market quotes for the requested exchange tokens,
// split into batches of up to maxTokensPerRequest tokens per API call.
func (a *Angelone) GetMarketQuote(mode model.QuoteMode, exchangeTokens map[model.Exchange][]string) (*model.Response[[]model.MarketQuoteResponse], error) {
	// Flatten all (exchange, token) pairs with a deterministic exchange order.
	type pair struct {
		exchange string
		token    string
	}
	exchanges := make([]string, 0, len(exchangeTokens))
	for ex := range exchangeTokens {
		exchanges = append(exchanges, string(ex))
	}
	sort.Strings(exchanges)

	var all []pair
	for _, exStr := range exchanges {
		tokens := exchangeTokens[model.Exchange(exStr)]
		for _, t := range tokens {
			all = append(all, pair{exchange: exStr, token: t})
		}
	}

	var allQuotes []model.MarketQuoteResponse
	for i := 0; i < len(all); i += maxTokensPerRequest {
		end := i + maxTokensPerRequest
		if end > len(all) {
			end = len(all)
		}
		batch := make(map[string][]string)
		for _, p := range all[i:end] {
			batch[p.exchange] = append(batch[p.exchange], p.token)
		}
		quotes, err := a.fetchQuoteBatch(mode, batch)
		if err != nil {
			return nil, fmt.Errorf("quote batch %d: %w", i/maxTokensPerRequest, err)
		}
		allQuotes = append(allQuotes, quotes...)
	}

	return &model.Response[[]model.MarketQuoteResponse]{
		Success: true,
		Message: "SUCCESS",
		Broker:  "angelone",
		Data:    allQuotes,
	}, nil
}
